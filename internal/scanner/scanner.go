package scanner

import (
	"context"
	"fmt"
	"sync"

	"github.com/ACM-Dev/gpu-finder/internal/api"
	"github.com/ACM-Dev/gpu-finder/internal/types"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	tea "github.com/charmbracelet/bubbletea"
)

func RunScanner(cfg aws.Config, regions []string, instances []string, p *tea.Program) {
	type job struct{ region, instance string }
	jobs := make(chan job, len(regions)*len(instances))

	for _, r := range regions {
		for _, i := range instances {
			jobs <- job{r, i}
		}
	}
	close(jobs)

	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range jobs {
				client := ec2.NewFromConfig(cfg, func(o *ec2.Options) { o.Region = j.region })

				azOut, err := client.DescribeInstanceTypeOfferings(context.TODO(), &ec2.DescribeInstanceTypeOfferingsInput{
					LocationType: ec2types.LocationTypeAvailabilityZone,
					Filters:      []ec2types.Filter{{Name: aws.String("instance-type"), Values: []string{j.instance}}},
				})

				if err == nil && len(azOut.InstanceTypeOfferings) > 0 {
					cbOfferings, cbError := api.CheckCapacityBlocks(context.TODO(), client, j.instance)

					for _, off := range azOut.InstanceTypeOfferings {
						az := *off.Location
						p.Send(types.ScanProgressMsg{JobName: fmt.Sprintf("Checking %s in %s", j.instance, az)})
						status, detail := api.CheckODCR(context.TODO(), client, j.instance, az)
						p.Send(types.ScanProgressMsg{Result: &types.CapacityResult{
							Region:      j.region,
							Instance:    j.instance,
							AZ:          az,
							Status:      status,
							Detail:      detail,
							CbOfferings: cbOfferings,
							CbError:     cbError,
						}})
					}
				} else {
					p.Send(types.ScanProgressMsg{JobName: fmt.Sprintf("%s / %s (No offerings)", j.region, j.instance)})
				}
				p.Send(types.ScanProgressMsg{})
			}
		}()
	}
	wg.Wait()
	p.Send(types.ScanDoneMsg{})
}
