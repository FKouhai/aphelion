package main

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/lipgloss"

	"aphelion/pkg/config"
	"aphelion/pkg/connect"
	"aphelion/pkg/qmp"
)

var (
	styleBold   = lipgloss.NewStyle().Bold(true)
	styleGreen  = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	styleRed    = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	styleYellow = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	styleDim    = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
)

type vmRow struct {
	host       config.Host
	name       string
	state      string
	memBytes   uint64
	cpuPct     float64
	hasMetrics bool
}

type tuiModel struct {
	cfg        *config.Config
	rows       []vmRow
	cursor     int
	loading    bool
	statusLine string
}

type (
	fetchDoneMsg  struct{ rows []vmRow }
	actionDoneMsg struct {
		name, action string
		err          error
	}
	attachDoneMsg struct{ err error }
	tickMsg       time.Time
)

func newTUIModel(cfg *config.Config) tuiModel {
	return tuiModel{cfg: cfg, loading: true}
}

func (m tuiModel) Init() tea.Cmd {
	return tea.Batch(fetchAllCmd(m.cfg), tickCmd())
}

func fetchAllCmd(cfg *config.Config) tea.Cmd {
	return func() tea.Msg {
		results := make([][]vmRow, len(cfg.Hosts))
		var wg sync.WaitGroup
		for i, host := range cfg.Hosts {
			i, host := i, host
			wg.Add(1)
			go func() {
				defer wg.Done()
				results[i] = fetchHostRows(host)
			}()
		}
		wg.Wait()
		var rows []vmRow
		for _, r := range results {
			rows = append(rows, r...)
		}
		return fetchDoneMsg{rows}
	}
}

func fetchHostRows(host config.Host) []vmRow {
	conn, err := connect.Dial(host)
	if err != nil {
		return []vmRow{{host: host, name: "-", state: "unreachable"}}
	}
	defer conn.Close()

	ag, err := conn.Agent(agentPort)
	if err != nil {
		return []vmRow{{host: host, name: "-", state: "agent error"}}
	}
	defer ag.Close()

	ctx := context.Background()

	vms, err := ag.ListVMs(ctx)
	if err != nil {
		return []vmRow{{host: host, name: "-", state: "list error"}}
	}

	metrics, _ := ag.Metrics(ctx)

	rows := make([]vmRow, 0, len(vms))
	for _, vmName := range vms {
		client := qmp.New(ag, vmName)
		status, err := client.QueryStatus(ctx)
		state := "unknown"
		if err == nil {
			state = string(status.Status)
		}

		row := vmRow{host: host, name: vmName, state: state}
		if m, ok := metrics[vmName]; ok {
			row.memBytes = m.MemoryBytes
			row.cpuPct = m.CPUPercent
			row.hasMetrics = true
		}
		rows = append(rows, row)
	}
	return rows
}

func tickCmd() tea.Cmd {
	return tea.Tick(10*time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func vmActionCmd(row vmRow, action string) tea.Cmd {
	return func() tea.Msg {
		conn, err := connect.Dial(row.host)
		if err != nil {
			return actionDoneMsg{row.name, action, err}
		}
		defer conn.Close()

		ag, err := conn.Agent(agentPort)
		if err != nil {
			return actionDoneMsg{row.name, action, err}
		}
		defer ag.Close()

		client := qmp.New(ag, row.name)
		ctx := context.Background()
		switch action {
		case "stop":
			err = client.Stop(ctx)
		case "restart":
			err = client.Reset(ctx)
		case "resume":
			err = client.Resume(ctx)
		}
		return actionDoneMsg{row.name, action, err}
	}
}

func (m tuiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		// reserved for future responsive layout

	case fetchDoneMsg:
		m.loading = false
		m.rows = msg.rows
		if m.cursor >= len(m.rows) {
			m.cursor = max(0, len(m.rows)-1)
		}

	case tickMsg:
		return m, tea.Batch(fetchAllCmd(m.cfg), tickCmd())

	case actionDoneMsg:
		if msg.err != nil {
			m.statusLine = fmt.Sprintf("✗ %s %s: %v", msg.action, msg.name, msg.err)
		} else {
			m.statusLine = fmt.Sprintf("✓ %s: %s ok", msg.name, msg.action)
		}
		return m, fetchAllCmd(m.cfg)

	case attachDoneMsg:
		if msg.err != nil {
			m.statusLine = fmt.Sprintf("✗ attach: %v", msg.err)
		}
		return m, fetchAllCmd(m.cfg)

	case tea.KeyPressMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.rows)-1 {
				m.cursor++
			}
		case "enter", "a":
			if row, ok := m.actionable(); ok {
				return m, tea.Exec(newAttachExec(row.host, row.name), func(err error) tea.Msg {
					return attachDoneMsg{err}
				})
			}
		case "s":
			if row, ok := m.actionable(); ok {
				m.statusLine = fmt.Sprintf("stopping %s…", row.name)
				return m, vmActionCmd(row, "stop")
			}
		case "r":
			if row, ok := m.actionable(); ok {
				m.statusLine = fmt.Sprintf("restarting %s…", row.name)
				return m, vmActionCmd(row, "restart")
			}
		case "R":
			if row, ok := m.actionable(); ok {
				m.statusLine = fmt.Sprintf("resuming %s…", row.name)
				return m, vmActionCmd(row, "resume")
			}
		case "ctrl+r":
			return m, fetchAllCmd(m.cfg)
		}
	}
	return m, nil
}

func (m tuiModel) actionable() (vmRow, bool) {
	if len(m.rows) == 0 || m.rows[m.cursor].name == "-" {
		return vmRow{}, false
	}
	return m.rows[m.cursor], true
}

func colorState(state string) string {
	switch state {
	case "running":
		return styleGreen.Render(state)
	case "stopped", "paused":
		return styleRed.Render(state)
	default:
		return styleYellow.Render(state)
	}
}

func fmtMem(bytes uint64) string {
	if bytes == 0 {
		return styleDim.Render("—")
	}
	gib := float64(bytes) / (1 << 30)
	if gib >= 1 {
		return fmt.Sprintf("%.1f GiB", gib)
	}
	return fmt.Sprintf("%.0f MiB", float64(bytes)/(1<<20))
}

func fmtCPU(pct float64, has bool) string {
	if !has {
		return styleDim.Render("—")
	}
	return fmt.Sprintf("%.1f%%", pct)
}

func (m tuiModel) render() string {
	if m.loading && len(m.rows) == 0 {
		return "\nLoading…\n"
	}

	var b strings.Builder

	b.WriteString(styleBold.Render("Aphelion") + "\n\n")
	b.WriteString(styleBold.Render(fmt.Sprintf("  %-16s %-16s %-10s %-9s %s", "HOST", "VM", "STATE", "MEM", "CPU")) + "\n")
	b.WriteString("  " + strings.Repeat("─", 58) + "\n")

	for i, row := range m.rows {
		prefix := "  "
		if i == m.cursor {
			prefix = styleBold.Render("▶ ")
		}
		fmt.Fprintf(&b, "%s%-16s %-16s %-10s %-9s %s\n",
			prefix,
			row.host.DisplayName,
			row.name,
			colorState(row.state),
			fmtMem(row.memBytes),
			fmtCPU(row.cpuPct, row.hasMetrics),
		)
	}

	b.WriteString("\n")
	if m.statusLine != "" {
		b.WriteString(styleDim.Render(m.statusLine) + "\n")
	}
	b.WriteString(styleDim.Render("[a/↵] attach  [s] stop  [r] restart  [R] resume  [^r] refresh  [q] quit") + "\n")

	return b.String()
}

func (m tuiModel) View() tea.View {
	v := tea.NewView(m.render())
	v.AltScreen = true
	return v
}

func runTUI() error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	p := tea.NewProgram(newTUIModel(cfg))
	_, err = p.Run()
	return err
}
