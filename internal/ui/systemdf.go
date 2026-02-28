package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/fpbpi/podman-tui/internal/models"
	"github.com/fpbpi/podman-tui/internal/podman"
)

// SystemDFModel fetches disk-usage and machine info and renders them as a header bar.
type SystemDFModel struct {
	spinner     spinner.Model
	loading     bool
	err         error
	info        *models.SystemDFInfo
	machineInfo *models.MachineInfo
	service     *podman.Service
}

func newSystemDFModel(svc *podman.Service) SystemDFModel {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("15"))

	return SystemDFModel{
		spinner: sp,
		service: svc,
		loading: true,
	}
}

func (m SystemDFModel) fetchDF() tea.Cmd {
	return func() tea.Msg {
		info, err := m.service.GetSystemDF()
		return SystemDFLoadedMsg{Info: info, Err: err}
	}
}

func (m SystemDFModel) fetchMachineInfo() tea.Cmd {
	return func() tea.Msg {
		info, err := m.service.GetMachineInfo()
		return MachineInfoLoadedMsg{Info: info, Err: err}
	}
}

func (m SystemDFModel) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, m.fetchDF(), m.fetchMachineInfo())
}

func (m SystemDFModel) Update(msg tea.Msg) (SystemDFModel, tea.Cmd) {
	switch msg := msg.(type) {
	case spinner.TickMsg:
		if m.loading {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
	case SystemDFLoadedMsg:
		m.loading = false
		m.err = msg.Err
		m.info = msg.Info
	case MachineInfoLoadedMsg:
		m.machineInfo = msg.Info
	}
	return m, nil
}

// Refresh triggers a new background fetch of system df data.
func (m *SystemDFModel) Refresh() tea.Cmd {
	m.loading = true
	return tea.Batch(m.spinner.Tick, m.fetchDF())
}

// ---- styles ----

var (
	headerBarStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("15"))
	headerAppStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("15")).Bold(true).Padding(0, 1)
	headerSepStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("62"))
	headerLabelStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("189"))
	headerValueStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("15")).Bold(true)
	headerTimeStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("189")).Padding(0, 1)
)

// formatMachineMem converts a MiB value to a human-readable string.
func formatMachineMem(mb int64) string {
	if mb >= 1024 {
		return fmt.Sprintf("%.1f GiB", float64(mb)/1024.0)
	}
	return fmt.Sprintf("%d MiB", mb)
}

// HeaderView renders a multi-line header with two equal columns:
//
//	podman-tui                                               15:04:05
//	 Machine: podman-machine-default │  Images: 4 (294.3MB)
//	 CPU: 4                          │  Containers: 2 (1.2MB)
//	 Mem: 2.0 GiB                    │  Volumes: 1 (100MB)
//	 Disk: 100 GiB                   │  Reclaimable: 154.9 MB
func (m SystemDFModel) HeaderView(width int) string {
	stat := func(label, val string) string {
		return headerLabelStyle.Render(label+": ") + headerValueStyle.Render(val)
	}
	indent := headerBarStyle.Render(" ")

	// ── title bar: app name left, timestamp right ──────────────────────────
	title := headerAppStyle.Render("podman-tui")
	timestamp := headerTimeStyle.Render(time.Now().Format("15:04:05"))
	titlePad := width - lipgloss.Width(title) - lipgloss.Width(timestamp)
	if titlePad < 0 {
		titlePad = 0
	}
	titleRow := title + headerBarStyle.Render(strings.Repeat(" ", titlePad)) + timestamp

	// ── two-column body ────────────────────────────────────────────────────
	colDiv := headerSepStyle.Render("│")
	divW := lipgloss.Width(colDiv)
	leftW := (width - divW) / 2
	rightW := width - leftW - divW

	// row pads both sides to their column width and joins them with the divider.
	row := func(leftContent, rightContent string) string {
		lPad := leftW - lipgloss.Width(leftContent)
		if lPad < 0 {
			lPad = 0
		}
		rPad := rightW - lipgloss.Width(rightContent)
		if rPad < 0 {
			rPad = 0
		}
		l := leftContent + headerBarStyle.Render(strings.Repeat(" ", lPad))
		r := rightContent + headerBarStyle.Render(strings.Repeat(" ", rPad))
		return l + colDiv + r
	}

	// Left column: machine info (one item per line).
	var left [4]string
	if m.machineInfo != nil {
		mi := m.machineInfo
		left[0] = indent + stat("Machine", mi.Name)
		left[1] = indent + stat("CPU", fmt.Sprintf("%d", mi.CPUs))
		left[2] = indent + stat("Mem", formatMachineMem(mi.MemoryMB))
		left[3] = indent + stat("Disk", fmt.Sprintf("%d GiB", mi.DiskGB))
	}

	// Right column: system df (one item per line).
	var right [4]string
	if m.loading {
		right[0] = indent + headerBarStyle.Render(fmt.Sprintf("%s loading…", m.spinner.View()))
	} else if m.err != nil {
		right[0] = indent + headerBarStyle.Render("system df unavailable")
	} else if m.info != nil {
		right[0] = indent + stat("Images", fmt.Sprintf("%d (%s)", m.info.ImagesCount, m.info.ImagesSize))
		right[1] = indent + stat("Containers", fmt.Sprintf("%d (%s)", m.info.ContainersCount, m.info.ContainersSize))
		right[2] = indent + stat("Volumes", fmt.Sprintf("%d (%s)", m.info.VolumesCount, m.info.VolumesSize))
		right[3] = indent + stat("Reclaimable", m.info.TotalReclaimable)
	}

	divider := headerSepStyle.Render(strings.Repeat("─", width))

	return strings.Join([]string{
		titleRow,
		divider,
		row(left[0], right[0]),
		row(left[1], right[1]),
		row(left[2], right[2]),
		row(left[3], right[3]),
	}, "\n")
}

// renderDFInfo is kept for potential future use (e.g. a detail popup).
func renderDFInfo(info *models.SystemDFInfo) string {
	row := func(label, value string) string {
		return dfLabelStyle.Render(label) + dfValueStyle.Render(value)
	}
	return strings.Join([]string{
		row("  Images:      ", fmt.Sprintf("%d  (%s)", info.ImagesCount, info.ImagesSize)),
		row("  Containers:  ", fmt.Sprintf("%d  (%s)", info.ContainersCount, info.ContainersSize)),
		row("  Volumes:     ", fmt.Sprintf("%d  (%s)", info.VolumesCount, info.VolumesSize)),
		row("  Reclaimable: ", info.TotalReclaimable),
	}, "\n")
}

var (
	dfLabelStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Width(22)
	dfValueStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("15")).Bold(true)
)
