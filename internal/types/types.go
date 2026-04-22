package types

import (
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/table"
)

type GpuSpec struct {
	InstanceType string
	GpuCount     int
	GpuName      string
	GpuMfr       string
	PerGpuMiB    int
	TotalGpuMiB  int
	Vcpus        int
}

func (g GpuSpec) Summary() string {
	return fmt.Sprintf("%dx %s", g.GpuCount, g.GpuName)
}

func (g GpuSpec) SummaryFull() string {
	perGB := float64(g.PerGpuMiB) / 1024
	totalGB := float64(g.TotalGpuMiB) / 1024
	return fmt.Sprintf("%dx %s %.0fGB (%.0fGB total) %dvCPU", g.GpuCount, g.GpuName, perGB, totalGB, g.Vcpus)
}

type CbOffering struct {
	DurationHours int
	StartDate     string
	EndDate       string
	UpfrontFee    float64
	AZ            string
}

type CapacityResult struct {
	Region      string
	Instance    string
	AZ          string
	Status      string
	Detail      string
	CbOfferings []CbOffering
	CbError     string
}

type CheckableItem struct {
	ID        string
	Name      string
	Selected  bool
	Disabled  bool
	Detail    string
	IsDefault bool
}

type AppState int

const (
	StateWelcome AppState = iota
	StateInitializing
	StateRegionSelect
	StateLoadingInstances
	StateInstanceSelect
	StateScanning
	StateDone
	StateAuthDone
)

type Model struct {
	State           AppState
	AwsCfg          aws.Config
	AccountID       string
	Arn             string
	OrgID           string
	OrgMasterID     string
	OrgMasterEmail  string
	ErrorMsg        string

	Regions   []CheckableItem
	Instances []CheckableItem
	Cursor    int

	TotalJobs   int
	Completed   int
	Results     []CapacityResult
	Progress    progress.Model
	Spinner     spinner.Model
	ScanningMsg string

	Table           table.Model
	FilterConfirmed bool
	SaveStatus      string
	DetailVisible   bool
	SelectedRow     *CapacityResult
	GpuSpecs        map[string]GpuSpec
	Prices          map[string]float64

	ShowQuitConfirm bool
	Mode            string // "normal", "auth", "headless"
	AllMode         bool   // when true, select all P/G series in headless mode
	ShowSavePrompt  bool
}

type AuthMsg struct {
	Cfg           aws.Config
	AccountID     string
	Arn           string
	OrgID         string
	OrgMasterID   string
	OrgMasterEmail string
	Err           error
}

type RegionsLoadedMsg []CheckableItem
type InstancesLoadedMsg []CheckableItem

type ScanProgressMsg struct {
	JobName string
	Result  *CapacityResult
}

type ScanDoneMsg struct{}
type SaveDoneMsg string
type ErrMsg struct{ Err error }
