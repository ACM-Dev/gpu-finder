package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/ACM-Dev/gpu-finder/cmd"
	"github.com/ACM-Dev/gpu-finder/internal/api"
	"github.com/ACM-Dev/gpu-finder/internal/scanner"
	"github.com/ACM-Dev/gpu-finder/internal/styles"
	"github.com/ACM-Dev/gpu-finder/internal/tui"
	"github.com/ACM-Dev/gpu-finder/internal/types"

	"github.com/aws/aws-sdk-go-v2/aws"
	tea "github.com/charmbracelet/bubbletea"
)

type wrappedModel struct {
	model types.Model
	p     *tea.Program
}

func (w *wrappedModel) Init() tea.Cmd {
	if w.model.Mode == "auth" {
		return tea.Batch(checkAuthCmd(), w.model.Spinner.Tick)
	}
	if w.model.Mode == "headless" {
		w.model.State = types.StateInitializing
		w.model.Spinner = tui.NewSpinner()
		return tea.Batch(checkAuthCmd(), w.model.Spinner.Tick)
	}
	return nil
}

func (w *wrappedModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	newModel, c := tui.Update(w.model, msg)
	w.model = newModel

	if w.model.State == types.StateScanning && w.model.Completed == 0 && w.model.ScanningMsg == "" {
		w.model.ScanningMsg = "Starting workers..."
		var regs, insts []string
		for _, r := range w.model.Regions {
			if r.Selected && !r.Disabled {
				regs = append(regs, r.ID)
			}
		}
		for _, i := range w.model.Instances {
			if i.Selected {
				insts = append(insts, i.ID)
			}
		}
		go runScanner(w.model.AwsCfg, regs, insts, w.p)
	}

	if w.model.Mode == "headless" && w.model.State == types.StateRegionSelect {
		var sel []string
		for _, r := range w.model.Regions {
			if r.Selected && !r.Disabled {
				sel = append(sel, r.ID)
			}
		}
		if len(sel) > 0 {
			w.model.State = types.StateLoadingInstances
			w.model.Spinner = tui.NewSpinner()
			return w, tea.Batch(tui.LoadInstancesCmd(w.model.AwsCfg, sel), w.model.Spinner.Tick)
		}
	}

	if w.model.Mode == "headless" && w.model.State == types.StateInstanceSelect {
		var sel []string
		if w.model.AllMode {
			// Select P and G series only
			pSeries := map[string]bool{"p3.16xlarge": true, "p3dn.24xlarge": true, "p4d.24xlarge": true, "p4de.24xlarge": true, "p5.48xlarge": true, "p5en.48xlarge": true, "p5e.48xlarge": true}
			gSeries := map[string]bool{"g5.48xlarge": true, "g6.48xlarge": true, "g6e.48xlarge": true}
			for _, i := range w.model.Instances {
				if pSeries[i.ID] || gSeries[i.ID] {
					sel = append(sel, i.ID)
				}
			}
		} else {
			for _, i := range w.model.Instances {
				if i.Selected {
					sel = append(sel, i.ID)
				}
			}
		}
		if len(sel) > 0 {
			rCount := 0
			for _, r := range w.model.Regions {
				if r.Selected && !r.Disabled {
					rCount++
				}
			}
			w.model.TotalJobs = rCount * len(sel)
			w.model.State = types.StateScanning
			w.model.ScanningMsg = "Starting workers..."

			var regs []string
			for _, r := range w.model.Regions {
				if r.Selected && !r.Disabled {
					regs = append(regs, r.ID)
				}
			}
			go runScanner(w.model.AwsCfg, regs, sel, w.p)
		}
	}

	return w, c
}

func (w *wrappedModel) View() string { return tui.View(w.model) }

func runScanner(cfg aws.Config, regions []string, instances []string, p *tea.Program) {
	scanner.RunScanner(cfg, regions, instances, p)
}

func checkAuthCmd() tea.Cmd {
	return func() tea.Msg {
		return api.CheckAuth()
	}
}

func main() {
	flags := cmd.ParseFlags()

	if flags.Auth {
		m := tui.InitialModel("auth")
		wm := &wrappedModel{model: m}
		p := tea.NewProgram(wm, tea.WithAltScreen())
		wm.p = p
		if _, err := p.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	if flags.Headless || flags.All {
		m := tui.InitialModel("headless")
		m.AllMode = flags.All
		wm := &wrappedModel{model: m}
		p := tea.NewProgram(wm, tea.WithAltScreen())
		wm.p = p
		if _, err := p.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	m := tui.InitialModel("normal")
	wm := &wrappedModel{model: m}
	p := tea.NewProgram(wm, tea.WithAltScreen())
	wm.p = p

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func renderAuthView(m types.Model) string {
	s := strings.Builder{}
	s.WriteString(styles.TitleStyle.Render(" AWS GPU Capacity Finder ") + "\n\n")
	s.WriteString(styles.SuccessStyle.Render("AWS Authentication Successful") + "\n\n")
	s.WriteString(styles.SubTitleStyle.Render("Account Details:") + "\n\n")
	s.WriteString(fmt.Sprintf("  Account : %s\n", styles.HighlightStyle.Render(m.AccountID)))
	s.WriteString(fmt.Sprintf("  ARN     : %s\n", styles.DimStyle.Render(m.Arn)))
	if m.OrgID != "" {
		s.WriteString(fmt.Sprintf("  Org     : %s\n", styles.DimStyle.Render(fmt.Sprintf("%s (master: %s / %s)", m.OrgID, m.OrgMasterID, m.OrgMasterEmail))))
	} else {
		s.WriteString(fmt.Sprintf("  Org     : %s\n", styles.DimStyle.Render("(no organizations access)")))
	}
	s.WriteString("\n" + styles.DimStyle.Render("Press ENTER or q to exit") + "\n")
	s.WriteString("\n" + styles.FooterStyle.Render(" enter: exit • q: quit • made by acuitmeshdev "))
	return styles.BaseStyle.Render(s.String())
}
