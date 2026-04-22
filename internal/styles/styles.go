package styles

import "github.com/charmbracelet/lipgloss"

const MaxConcurrentWorkers = 5

var CbDurationsHours = []int{168, 336, 672, 1344, 3360}

var RegionNames = map[string]string{
	"us-east-1": "N. Virginia", "us-east-2": "Ohio",
	"us-west-1": "N. California", "us-west-2": "Oregon",
	"ap-south-1": "Mumbai", "ap-south-2": "Hyderabad",
	"ap-northeast-1": "Tokyo", "ap-northeast-2": "Seoul", "ap-northeast-3": "Osaka",
	"ap-southeast-1": "Singapore", "ap-southeast-2": "Sydney", "ap-southeast-3": "Jakarta",
	"ap-southeast-4": "Melbourne", "ap-southeast-7": "Bangkok",
	"ca-central-1": "Central", "ca-west-1": "Calgary",
	"eu-central-1": "Frankfurt", "eu-central-2": "Zurich",
	"eu-west-1": "Ireland", "eu-west-2": "London", "eu-west-3": "Paris",
	"eu-south-1": "Milan", "eu-south-2": "Spain",
	"eu-north-1": "Stockholm", "me-south-1": "Bahrain",
	"me-central-1": "UAE", "sa-east-1": "São Paulo",
	"af-south-1": "Cape Town", "ap-east-1": "Hong Kong",
	"il-central-1": "Tel Aviv",
}

var RegionToPricingLocation = map[string]string{
	"us-east-1":      "US East (N. Virginia)",
	"us-east-2":      "US East (Ohio)",
	"us-west-1":      "US West (N. California)",
	"us-west-2":      "US West (Oregon)",
	"ap-southeast-1": "Asia Pacific (Singapore)",
	"ap-southeast-2": "Asia Pacific (Sydney)",
	"ap-southeast-3": "Asia Pacific (Jakarta)",
	"ap-southeast-7": "Asia Pacific (Thailand)",
	"ap-northeast-1": "Asia Pacific (Tokyo)",
	"ap-northeast-2": "Asia Pacific (Seoul)",
	"ap-south-1":     "Asia Pacific (Mumbai)",
}

var PSeries = []string{
	"p3.16xlarge", "p3dn.24xlarge", "p4d.24xlarge", "p4de.24xlarge",
	"p5.48xlarge", "p5en.48xlarge", "p5e.48xlarge",
}

var GSeries = []string{"g5.48xlarge", "g6.48xlarge", "g6e.48xlarge"}

var FallbackGpuSpecs = map[string]GpuSpecFallback{
	"p3.16xlarge":   {InstanceType: "p3.16xlarge", GpuCount: 8, GpuName: "V100", GpuMfr: "NVIDIA", PerGpuMiB: 16384, TotalGpuMiB: 131072, Vcpus: 64},
	"p3dn.24xlarge": {InstanceType: "p3dn.24xlarge", GpuCount: 8, GpuName: "V100", GpuMfr: "NVIDIA", PerGpuMiB: 16384, TotalGpuMiB: 131072, Vcpus: 96},
	"p4d.24xlarge":  {InstanceType: "p4d.24xlarge", GpuCount: 8, GpuName: "A100", GpuMfr: "NVIDIA", PerGpuMiB: 40960, TotalGpuMiB: 327680, Vcpus: 96},
	"p4de.24xlarge": {InstanceType: "p4de.24xlarge", GpuCount: 8, GpuName: "A100", GpuMfr: "NVIDIA", PerGpuMiB: 81920, TotalGpuMiB: 655360, Vcpus: 96},
	"p5.48xlarge":   {InstanceType: "p5.48xlarge", GpuCount: 8, GpuName: "H100", GpuMfr: "NVIDIA", PerGpuMiB: 81920, TotalGpuMiB: 655360, Vcpus: 192},
	"p5en.48xlarge": {InstanceType: "p5en.48xlarge", GpuCount: 8, GpuName: "H200", GpuMfr: "NVIDIA", PerGpuMiB: 144896, TotalGpuMiB: 1159168, Vcpus: 192},
	"p5e.48xlarge":  {InstanceType: "p5e.48xlarge", GpuCount: 8, GpuName: "H200", GpuMfr: "NVIDIA", PerGpuMiB: 144896, TotalGpuMiB: 1159168, Vcpus: 192},
	"g5.48xlarge":   {InstanceType: "g5.48xlarge", GpuCount: 8, GpuName: "A10G", GpuMfr: "NVIDIA", PerGpuMiB: 24576, TotalGpuMiB: 196608, Vcpus: 192},
	"g6.48xlarge":   {InstanceType: "g6.48xlarge", GpuCount: 8, GpuName: "L4", GpuMfr: "NVIDIA", PerGpuMiB: 24576, TotalGpuMiB: 196608, Vcpus: 192},
	"g6e.48xlarge":  {InstanceType: "g6e.48xlarge", GpuCount: 8, GpuName: "L40S", GpuMfr: "NVIDIA", PerGpuMiB: 49152, TotalGpuMiB: 393216, Vcpus: 192},
	"trn1.32xlarge": {InstanceType: "trn1.32xlarge", GpuCount: 16, GpuName: "Trainium", GpuMfr: "AWS", PerGpuMiB: 32768, TotalGpuMiB: 524288, Vcpus: 128},
	"inf2.48xlarge": {InstanceType: "inf2.48xlarge", GpuCount: 12, GpuName: "Inferentia2", GpuMfr: "AWS", PerGpuMiB: 32768, TotalGpuMiB: 393216, Vcpus: 192},
}

type GpuSpecFallback struct {
	InstanceType string
	GpuCount     int
	GpuName      string
	GpuMfr       string
	PerGpuMiB    int
	TotalGpuMiB  int
	Vcpus        int
}

var (
	BaseStyle      = lipgloss.NewStyle().Padding(1, 2)
	TitleStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF")).Background(lipgloss.Color("#FF9900")).Padding(0, 1).Bold(true)
	SubTitleStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF9900")).Bold(true)
	HighlightStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#00D7FF")).Bold(true)
	SuccessStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#00FF00"))
	ErrorStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF0000"))
	DimStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("#777777"))
	WarnStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFA500"))
	FooterStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF")).Background(lipgloss.Color("#FF9900")).Padding(0, 1).Bold(true)
	DetailStyle    = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("#4472C4")).Padding(1, 2).Width(80)
)

func GetFriendlyRegionName(code string) string {
	if name, ok := RegionNames[code]; ok {
		return name
	}
	return "AWS Region"
}
