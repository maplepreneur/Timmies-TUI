package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/maplepreneur/chrono/internal/report"
	"github.com/maplepreneur/chrono/internal/service"
	"github.com/maplepreneur/chrono/internal/store/sqlite"
)

type mode int

const (
	modeDashboard mode = iota
	modeAddClient
	modeAddType
	modeStart
	modeReport
)

type tickMsg time.Time

type Model struct {
	store   *sqlite.Store
	service *service.TimerService

	mode    mode
	input   textinput.Model
	message string

	active *sqlite.SessionView
	report []sqlite.ReportRow
	total  int64
}

func New(store *sqlite.Store, svc *service.TimerService) Model {
	ti := textinput.New()
	ti.Prompt = "> "
	ti.Focus()
	return Model{store: store, service: svc, mode: modeDashboard, input: ti}
}

func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m Model) Init() tea.Cmd {
	active, _ := m.service.Status()
	m.active = active
	return tickCmd()
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tickMsg:
		active, _ := m.service.Status()
		m.active = active
		return m, tickCmd()
	case tea.KeyMsg:
		switch m.mode {
		case modeDashboard:
			switch msg.String() {
			case "q", "ctrl+c":
				return m, tea.Quit
			case "a":
				m.mode = modeAddClient
				m.input.SetValue("")
				m.input.Placeholder = "client name"
				m.message = ""
			case "k":
				m.mode = modeAddType
				m.input.SetValue("")
				m.input.Placeholder = "tracking type name"
				m.message = ""
			case "s":
				m.mode = modeStart
				m.input.SetValue("")
				m.input.Placeholder = "@client type note..."
				m.message = ""
			case "x":
				if _, err := m.service.Stop(); err != nil {
					m.message = err.Error()
				} else {
					m.message = "stopped active session"
				}
			case "r":
				if _, err := m.service.Resume(); err != nil {
					m.message = err.Error()
				} else {
					m.message = "resumed latest session as new segment"
				}
			case "p":
				m.mode = modeReport
				m.input.SetValue("")
				m.input.Placeholder = "@client YYYY-MM-DD YYYY-MM-DD"
			}
		default:
			switch msg.String() {
			case "esc":
				m.mode = modeDashboard
				m.input.SetValue("")
				return m, nil
			case "enter":
				return m.submitInput()
			default:
				var cmd tea.Cmd
				m.input, cmd = m.input.Update(msg)
				return m, cmd
			}
		}
	}
	return m, nil
}

func (m Model) submitInput() (tea.Model, tea.Cmd) {
	v := strings.TrimSpace(m.input.Value())
	switch m.mode {
	case modeAddClient:
		if v == "" {
			m.message = "client name is required"
			return m, nil
		}
		if err := m.store.AddClient(v); err != nil {
			m.message = err.Error()
		} else {
			m.message = "client created"
		}
	case modeAddType:
		if v == "" {
			m.message = "tracking type name is required"
			return m, nil
		}
		if err := m.store.AddTrackingType(v); err != nil {
			m.message = err.Error()
		} else {
			m.message = "tracking type created"
		}
	case modeStart:
		parts := strings.Fields(v)
		if len(parts) < 2 || !strings.HasPrefix(parts[0], "@") {
			m.message = "use: @client type note..."
			return m, nil
		}
		client := strings.TrimPrefix(parts[0], "@")
		trackingType := parts[1]
		note := ""
		if len(parts) > 2 {
			note = strings.Join(parts[2:], " ")
		}
		if _, err := m.service.Start(client, trackingType, note); err != nil {
			m.message = err.Error()
		} else {
			m.message = "started session"
		}
	case modeReport:
		parts := strings.Fields(v)
		if len(parts) != 3 || !strings.HasPrefix(parts[0], "@") {
			m.message = "use: @client YYYY-MM-DD YYYY-MM-DD"
			return m, nil
		}
		client := strings.TrimPrefix(parts[0], "@")
		from, to, err := report.ParseDateRange(parts[1], parts[2])
		if err != nil {
			m.message = err.Error()
			return m, nil
		}
		rows, total, err := m.service.Report(client, from, to)
		if err != nil {
			m.message = err.Error()
			return m, nil
		}
		m.report = rows
		m.total = total
		m.message = fmt.Sprintf("report loaded: %d sessions", len(rows))
	}
	m.mode = modeDashboard
	m.input.SetValue("")
	active, _ := m.service.Status()
	m.active = active
	return m, nil
}

func (m Model) View() string {
	title := lipgloss.NewStyle().Bold(true).Render("chrono")
	active := "none"
	if m.active != nil {
		elapsed := int64(time.Since(m.active.StartedAt).Seconds())
		if elapsed < 0 {
			elapsed = 0
		}
		active = fmt.Sprintf("@%s | %s | %s | %s", m.active.ClientName, m.active.TrackingTypeName, report.HumanDuration(elapsed), m.active.Note)
	}

	base := fmt.Sprintf("%s\n\nActive: %s\n\n[a] add client  [k] add type  [s] start  [x] stop  [r] resume  [p] report  [q] quit\n", title, active)
	if m.message != "" {
		base += "\n" + m.message + "\n"
	}
	if m.mode != modeDashboard {
		return base + "\n" + m.input.View() + "\n(enter to submit, esc to cancel)"
	}

	if len(m.report) > 0 {
		base += "\nReport:\n"
		for _, r := range m.report {
			base += fmt.Sprintf("- %s %s %s (%s)\n", r.StartedAt.Local().Format("2006-01-02 15:04"), r.TrackingTypeName, report.HumanDuration(r.ComputedDurationS), r.Note)
		}
		base += fmt.Sprintf("Total: %s\n", report.HumanDuration(m.total))
	}
	return base
}

func Run(store *sqlite.Store, svc *service.TimerService) error {
	p := tea.NewProgram(New(store, svc))
	_, err := p.Run()
	return err
}
