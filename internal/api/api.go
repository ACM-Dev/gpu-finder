package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/ACM-Dev/gpu-finder/internal/styles"
	"github.com/ACM-Dev/gpu-finder/internal/types"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/organizations"
	"github.com/aws/aws-sdk-go-v2/service/pricing"
	pricingtypes "github.com/aws/aws-sdk-go-v2/service/pricing/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

func CheckAuth() types.AuthMsg {
	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithRetryMode(aws.RetryModeAdaptive),
		config.WithRetryMaxAttempts(10),
	)
	if err != nil {
		return types.AuthMsg{Err: err}
	}

	stsClient := sts.NewFromConfig(cfg)
	identity, err := stsClient.GetCallerIdentity(context.TODO(), &sts.GetCallerIdentityInput{})
	if err != nil {
		return types.AuthMsg{Err: fmt.Errorf("AWS Auth failed: %w", err)}
	}

	msg := types.AuthMsg{
		Cfg:       cfg,
		AccountID: aws.ToString(identity.Account),
		Arn:       aws.ToString(identity.Arn),
	}

	orgClient := organizations.NewFromConfig(cfg)
	orgResp, err := orgClient.DescribeOrganization(context.TODO(), &organizations.DescribeOrganizationInput{})
	if err == nil && orgResp.Organization != nil {
		org := orgResp.Organization
		msg.OrgID = aws.ToString(org.Id)
		msg.OrgMasterID = aws.ToString(org.MasterAccountId)
		msg.OrgMasterEmail = aws.ToString(org.MasterAccountEmail)
	}

	return msg
}

func LoadRegionsCmd(cfg aws.Config) types.RegionsLoadedMsg {
	baseRegion := cfg.Region
	if baseRegion == "" {
		baseRegion = "us-east-1"
	}

	client := ec2.NewFromConfig(cfg, func(o *ec2.Options) { o.Region = baseRegion })
	regionsOut, err := client.DescribeRegions(context.TODO(), &ec2.DescribeRegionsInput{})
	if err != nil {
		return nil
	}

	var mu sync.Mutex
	var wg sync.WaitGroup
	items := make([]types.CheckableItem, len(regionsOut.Regions))

	for i, reg := range regionsOut.Regions {
		wg.Add(1)
		go func(idx int, rCode string) {
			defer wg.Done()
			rClient := ec2.NewFromConfig(cfg, func(o *ec2.Options) { o.Region = rCode })
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_, err := rClient.DescribeAvailabilityZones(ctx, &ec2.DescribeAvailabilityZonesInput{})

			detailStr := "Accessible"
			if err != nil {
				var apiErr interface{ ErrorCode() string }
				if errors.As(err, &apiErr) {
					code := apiErr.ErrorCode()
					if code == "AuthFailure" || code == "UnauthorizedOperation" || code == "OptInRequired" {
						detailStr = "Not Opted-In / SCP Blocked"
					} else {
						detailStr = code
					}
				} else {
					detailStr = "Access Denied"
				}
			}

			mu.Lock()
			defer mu.Unlock()
			items[idx] = types.CheckableItem{
				ID:        rCode,
				Name:      styles.GetFriendlyRegionName(rCode),
				IsDefault: rCode == cfg.Region,
				Selected:  err == nil,
				Disabled:  err != nil,
				Detail:    detailStr,
			}
		}(i, *reg.RegionName)
	}
	wg.Wait()

	sort.Slice(items, func(i, j int) bool {
		if items[i].Disabled != items[j].Disabled {
			return !items[i].Disabled
		}
		return items[i].ID < items[j].ID
	})

	return items
}

func LoadInstances(cfg aws.Config, selectedRegions []string) types.InstancesLoadedMsg {
	var mu sync.Mutex
	var wg sync.WaitGroup
	instanceMap := make(map[string]string)

	fallbackCandidates := map[string]string{
		"p3.16xlarge":   "8x V100 16GB",
		"p5.48xlarge":   "8x H100 80GB",
		"g5.48xlarge":   "8x A10G 24GB",
		"g6.48xlarge":   "8x L4 24GB",
	}

	for _, rCode := range selectedRegions {
		wg.Add(1)
		go func(region string) {
			defer wg.Done()
			client := ec2.NewFromConfig(cfg, func(o *ec2.Options) { o.Region = region })
			paginator := ec2.NewDescribeInstanceTypesPaginator(client, &ec2.DescribeInstanceTypesInput{
				Filters: []ec2types.Filter{{Name: aws.String("instance-category"), Values: []string{"accelerated-computing"}}},
			})

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			for paginator.HasMorePages() {
				page, err := paginator.NextPage(ctx)
				if err != nil {
					break
				}
				for _, it := range page.InstanceTypes {
					itype := string(it.InstanceType)
					mu.Lock()
					if _, exists := instanceMap[itype]; !exists {
						spec := "Accelerator"
						if it.GpuInfo != nil && len(it.GpuInfo.Gpus) > 0 {
							gpu := it.GpuInfo.Gpus[0]
							count := aws.ToInt32(gpu.Count)
							name := aws.ToString(gpu.Name)
							mem := aws.ToInt32(gpu.MemoryInfo.SizeInMiB) / 1024
							spec = fmt.Sprintf("%dx %s %dGB", count, name, mem)
						}
						instanceMap[itype] = spec
					}
					mu.Unlock()
				}
			}
		}(rCode)
	}
	wg.Wait()

	if len(instanceMap) == 0 {
		for k, v := range fallbackCandidates {
			instanceMap[k] = v
		}
	}

	var items []types.CheckableItem
	for id, detail := range instanceMap {
		selected := false
		prefixes := []string{"p4", "p5", "g5", "g6", "trn1", "inf2"}
		for _, p := range prefixes {
			if strings.HasPrefix(id, p) {
				selected = true
				break
			}
		}
		items = append(items, types.CheckableItem{ID: id, Detail: detail, Selected: selected})
	}

	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	return items
}

func CheckODCR(ctx context.Context, client *ec2.Client, instanceType string, az string) (status string, detail string) {
	input := &ec2.CreateCapacityReservationInput{
		InstanceType:          aws.String(instanceType),
		InstancePlatform:      ec2types.CapacityReservationInstancePlatformLinuxUnix,
		AvailabilityZone:      aws.String(az),
		InstanceCount:         aws.Int32(1),
		InstanceMatchCriteria: ec2types.InstanceMatchCriteriaTargeted,
		DryRun:                aws.Bool(true),
	}

	_, err := client.CreateCapacityReservation(ctx, input)
	if err != nil {
		var apiErr interface{ ErrorCode() string }
		if errors.As(err, &apiErr) {
			code := apiErr.ErrorCode()
			if code == "DryRunOperation" {
				// Success, proceed to real reservation
			} else if code == "InsufficientInstanceCapacity" {
				return "Insufficient Capacity", ""
			} else if code == "UnsupportedOperation" || code == "Unsupported" {
				return "Unsupported", ""
			} else if strings.Contains(code, "LimitExceeded") {
				return "Quota Exceeded", err.Error()
			} else {
				return "Error", code
			}
		} else {
			return "Error", err.Error()
		}
	}

	input.DryRun = aws.Bool(false)
	out, err := client.CreateCapacityReservation(ctx, input)
	if err != nil {
		var apiErr interface{ ErrorCode() string }
		if errors.As(err, &apiErr) && apiErr.ErrorCode() == "InsufficientInstanceCapacity" {
			return "Insufficient Capacity", ""
		}
		return "Error", err.Error()
	}

	crID := out.CapacityReservation.CapacityReservationId
	_, _ = client.CancelCapacityReservation(ctx, &ec2.CancelCapacityReservationInput{CapacityReservationId: crID})

	return "Confirmed Capacity", *crID
}

func CheckCapacityBlocks(ctx context.Context, client *ec2.Client, instanceType string) ([]types.CbOffering, string) {
	var offerings []types.CbOffering
	now := time.Now().UTC()
	nowTime := now

	for _, hours := range styles.CbDurationsHours {
		maxStart := now.Add(time.Hour * 24 * 7 * 20).Add(time.Duration(-hours) * time.Hour)
		if maxStart.Before(now) || maxStart.Equal(now) {
			continue
		}
		endRange := maxStart

		for attempt := 0; attempt < 4; attempt++ {
			resp, err := client.DescribeCapacityBlockOfferings(ctx, &ec2.DescribeCapacityBlockOfferingsInput{
				InstanceType:          aws.String(instanceType),
				InstanceCount:         aws.Int32(1),
				CapacityDurationHours: aws.Int32(int32(hours)),
				StartDateRange:        &nowTime,
				EndDateRange:          &endRange,
			})

			if err != nil {
				var apiErr interface{ ErrorCode() string }
				if errors.As(err, &apiErr) {
					code := apiErr.ErrorCode()
					msg := err.Error()
					if code == "RequestLimitExceeded" && attempt < 3 {
						time.Sleep(time.Duration(math.Pow(2, float64(attempt))) * time.Second)
						continue
					}
					if code == "InvalidAction" || code == "UnsupportedOperation" {
						return nil, "Not supported in this region"
					}
					if code == "PendingVerification" {
						return nil, "Org master account pending verification"
					}
					if code == "InvalidParameterValue" {
						if strings.Contains(msg, "not supported for Capacity Blocks") {
							return nil, fmt.Sprintf("CB not supported: %s", msg)
						}
						break
					}
					return nil, fmt.Sprintf("%s: %s", code, msg)
				}
				break
			}

			if len(resp.CapacityBlockOfferings) > 0 {
				sort.Slice(resp.CapacityBlockOfferings, func(i, j int) bool {
					return resp.CapacityBlockOfferings[i].StartDate.Before(*resp.CapacityBlockOfferings[j].StartDate)
				})
				earliest := resp.CapacityBlockOfferings[0]
				upfrontFee := 0.0
				if earliest.UpfrontFee != nil {
					fmt.Sscanf(*earliest.UpfrontFee, "%f", &upfrontFee)
				}
				offerings = append(offerings, types.CbOffering{
					DurationHours: hours,
					StartDate:     earliest.StartDate.Format("2006-01-02"),
					EndDate:       earliest.EndDate.Format("2006-01-02"),
					UpfrontFee:    upfrontFee,
					AZ:            aws.ToString(earliest.AvailabilityZone),
				})
			}
			break
		}
	}

	return offerings, ""
}

func FetchOndemandPrices(instanceTypes []string, regions []string) map[string]float64 {
	prices := make(map[string]float64)

	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return prices
	}

	pricingClient := pricing.NewFromConfig(cfg, func(o *pricing.Options) {
		o.Region = "us-east-1"
	})

	for _, region := range regions {
		location, ok := styles.RegionToPricingLocation[region]
		if !ok {
			continue
		}
		for _, itype := range instanceTypes {
			resp, err := pricingClient.GetProducts(context.TODO(), &pricing.GetProductsInput{
				ServiceCode: aws.String("AmazonEC2"),
				Filters: []pricingtypes.Filter{
					{Type: "TERM_MATCH", Field: aws.String("instanceType"), Value: aws.String(itype)},
					{Type: "TERM_MATCH", Field: aws.String("location"), Value: aws.String(location)},
					{Type: "TERM_MATCH", Field: aws.String("tenancy"), Value: aws.String("Shared")},
					{Type: "TERM_MATCH", Field: aws.String("operatingSystem"), Value: aws.String("Linux")},
					{Type: "TERM_MATCH", Field: aws.String("capacitystatus"), Value: aws.String("Used")},
					{Type: "TERM_MATCH", Field: aws.String("preInstalledSw"), Value: aws.String("NA")},
				},
				MaxResults: aws.Int32(1),
			})
			if err != nil || len(resp.PriceList) == 0 {
				continue
			}

			var product map[string]interface{}
			if err := json.Unmarshal([]byte(resp.PriceList[0]), &product); err != nil {
				continue
			}

			terms, ok := product["terms"].(map[string]interface{})
			if !ok {
				continue
			}
			onDemand, ok := terms["OnDemand"].(map[string]interface{})
			if !ok {
				continue
			}

			for _, termData := range onDemand {
				term, ok := termData.(map[string]interface{})
				if !ok {
					continue
				}
				priceDims, ok := term["priceDimensions"].(map[string]interface{})
				if !ok {
					continue
				}
				for _, dimData := range priceDims {
					dim, ok := dimData.(map[string]interface{})
					if !ok {
						continue
					}
					pricePerUnit, ok := dim["pricePerUnit"].(map[string]interface{})
					if !ok {
						continue
					}
					if usd, ok := pricePerUnit["USD"].(string); ok {
						var price float64
						fmt.Sscanf(usd, "%f", &price)
						if price > 0 {
							key := fmt.Sprintf("%s/%s", region, itype)
							prices[key] = price
						}
					}
				}
			}
		}
	}

	return prices
}

func FetchGpuSpecs(instanceTypes []string, region string) map[string]types.GpuSpec {
	specs := make(map[string]types.GpuSpec)

	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return specs
	}

	ec2Client := ec2.NewFromConfig(cfg, func(o *ec2.Options) {
		o.Region = region
	})

	for _, itype := range instanceTypes {
		resp, err := ec2Client.DescribeInstanceTypes(context.TODO(), &ec2.DescribeInstanceTypesInput{
			InstanceTypes: []ec2types.InstanceType{ec2types.InstanceType(itype)},
		})
		if err != nil {
			continue
		}
		for _, it := range resp.InstanceTypes {
			gpuInfo := it.GpuInfo
			if gpuInfo != nil && len(gpuInfo.Gpus) > 0 {
				gpu := gpuInfo.Gpus[0]
				specs[itype] = types.GpuSpec{
					InstanceType: itype,
					GpuCount:     int(aws.ToInt32(gpu.Count)),
					GpuName:      aws.ToString(gpu.Name),
					GpuMfr:       aws.ToString(gpu.Manufacturer),
					PerGpuMiB:    int(aws.ToInt32(gpu.MemoryInfo.SizeInMiB)),
					TotalGpuMiB:  int(aws.ToInt32(gpuInfo.TotalGpuMemoryInMiB)),
					Vcpus:        int(aws.ToInt32(it.VCpuInfo.DefaultVCpus)),
				}
			}
		}
	}

	for _, itype := range instanceTypes {
		if _, exists := specs[itype]; !exists {
			if fallback, ok := styles.FallbackGpuSpecs[itype]; ok {
				specs[itype] = types.GpuSpec{
					InstanceType: fallback.InstanceType,
					GpuCount:     fallback.GpuCount,
					GpuName:      fallback.GpuName,
					GpuMfr:       fallback.GpuMfr,
					PerGpuMiB:    fallback.PerGpuMiB,
					TotalGpuMiB:  fallback.TotalGpuMiB,
					Vcpus:        fallback.Vcpus,
				}
			}
		}
	}

	return specs
}
