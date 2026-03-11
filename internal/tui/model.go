package tui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/maplepreneur/chrono/internal/report"
	"github.com/maplepreneur/chrono/internal/service"
	"github.com/maplepreneur/chrono/internal/store/sqlite"
)

type mode int

const (
	modeMenu mode = iota
	modeDashboard
	modeInput
	modeReportView
	modeTypeForm
	modeConfirmDelete
	modeEditClient
	modeEditType
	modeSessionForm
)

type inputAction int

const (
	actionAddClient inputAction = iota
	actionAddType
	actionStartSession
	actionReport
	actionAddResource
)

type dashboardFocus int

const (
	focusClients dashboardFocus = iota
	focusTypes
	focusPaused
)

type tickMsg time.Time

type keyMap struct {
	quit         key.Binding
	menuUp       key.Binding
	menuDown     key.Binding
	selectItem   key.Binding
	addClient    key.Binding
	addType      key.Binding
	start        key.Binding
	stop         key.Binding
	resumeLatest key.Binding
	menu         key.Binding
	dashboard    key.Binding
	report       key.Binding
	addResource  key.Binding
	resumePaused key.Binding
	switchFocus  key.Binding
	toggleHelp   key.Binding
	back         key.Binding
}

func newKeyMap() keyMap {
	return keyMap{
		quit:         key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
		menuUp:       key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "menu up")),
		menuDown:     key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "menu down")),
		selectItem:   key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "select")),
		addClient:    key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "add client")),
		addType:      key.NewBinding(key.WithKeys("t"), key.WithHelp("t", "add type")),
		start:        key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "start session")),
		stop:         key.NewBinding(key.WithKeys("x"), key.WithHelp("x", "stop active")),
		resumeLatest: key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "resume latest")),
		menu:         key.NewBinding(key.WithKeys("m"), key.WithHelp("m", "management menu")),
		dashboard:    key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "dashboard")),
		report:       key.NewBinding(key.WithKeys("p"), key.WithHelp("p", "run report")),
		addResource:  key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "add resource cost")),
		resumePaused: key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "resume selected paused")),
		switchFocus:  key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "switch focus")),
		toggleHelp:   key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "overview/help")),
		back:         key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back")),
	}
}

func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.selectItem, k.addClient, k.start, k.dashboard, k.report, k.quit}
}

func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.menuUp, k.menuDown, k.selectItem, k.addClient, k.addType, k.start, k.stop},
		{k.resumeLatest, k.addResource, k.dashboard, k.menu, k.report, k.switchFocus, k.toggleHelp, k.back, k.quit},
		{k.resumePaused},
	}
}

type Model struct {
	store   *sqlite.Store
	service *service.TimerService

	mode   mode
	action inputAction
	input  textinput.Model
	help   help.Model
	keys   keyMap

	width  int
	height int
	focus  dashboardFocus

	message string

	active         *sqlite.SessionView
	activeResTotal float64
	clientNames    []string
	typeDetails    []sqlite.TrackingTypeView
	clientTotals   []sqlite.DurationAmountTotal
	typeTotals     []sqlite.DurationAmountTotal
	paused         []sqlite.PausedSessionView

	clientsTable table.Model
	typesTable   table.Model
	pausedTable  table.Model

	reportRows      []sqlite.ReportRow
	reportTotal     int64
	reportTimeTotal float64
	reportResTotal  float64
	reportGrand     float64
	reportViewport  viewport.Model

	reportFrom time.Time
	reportTo   time.Time

	resourceSessionID    int64
	resourceSessionLabel string

	showOverview bool
	menuCursor   int

	typeFormStep     int
	typeFormName     string
	typeFormBillable bool

	confirmMsg    string
	confirmYes    bool
	confirmAction func()

	editClientOldName string

	editTypeOldName string
	editTypeStep    int
	editTypeName    string
	editTypeBill    bool

	sessionFormStep   int
	sessionFormType   int // index into typeDetails
	sessionFormClient int // index into clientNames (0 = none, 1+ = clientNames[i-1])
}

var (
	brandRed   = lipgloss.AdaptiveColor{Light: "#CC0000", Dark: "#FF3333"}
	brandMuted = lipgloss.AdaptiveColor{Light: "#666666", Dark: "#999999"}
	brandFg    = lipgloss.AdaptiveColor{Light: "#1A1A1A", Dark: "#F0F0F0"}
	brandBdr   = lipgloss.AdaptiveColor{Light: "#AAAAAA", Dark: "#555555"}
	brandInfo  = lipgloss.AdaptiveColor{Light: "#FFFFFF", Dark: "#FFFFFF"}

	baseStyle  = lipgloss.NewStyle().Foreground(brandFg)
	titleStyle = lipgloss.NewStyle().Bold(true).Foreground(brandRed)
	panelStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(brandBdr).Padding(0, 1).Foreground(brandFg)
	mutedStyle = lipgloss.NewStyle().Foreground(brandMuted)
	infoStyle  = lipgloss.NewStyle().Foreground(brandInfo).Background(brandRed).Padding(0, 1)
	errStyle   = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#FFFFFF", Dark: "#FFFFFF"}).Background(lipgloss.AdaptiveColor{Light: "#AA0000", Dark: "#CC3333"}).Padding(0, 1)
	leafStyle  = lipgloss.NewStyle().Foreground(brandRed)
	logoText   = lipgloss.NewStyle().Foreground(brandFg).Bold(true)

	formLabelStyle    = lipgloss.NewStyle().Foreground(brandMuted)
	formActiveStyle   = lipgloss.NewStyle().Foreground(brandFg).Bold(true)
	formSelectedStyle = lipgloss.NewStyle().Foreground(brandRed).Bold(true)
	formAnswerStyle   = lipgloss.NewStyle().Foreground(brandRed)
)

func New(store *sqlite.Store, svc *service.TimerService) Model {
	ti := textinput.New()
	ti.Prompt = "> "
	ti.Focus()

	m := Model{
		store:          store,
		service:        svc,
		mode:           modeMenu,
		action:         actionAddClient,
		input:          ti,
		help:           help.New(),
		keys:           newKeyMap(),
		clientsTable:   newTable(),
		typesTable:     newTable(),
		pausedTable:    newTable(),
		reportViewport: viewport.New(20, 10),
	}
	m.help.ShowAll = false
	m.refreshDashboard()
	m.syncTables()
	return m
}

func newTable() table.Model {
	t := table.New(
		table.WithColumns([]table.Column{{Title: "Name", Width: 20}, {Title: "Duration", Width: 14}, {Title: "Billable", Width: 12}}),
		table.WithRows([]table.Row{}),
		table.WithFocused(true),
		table.WithHeight(7),
	)
	styles := table.DefaultStyles()
	styles.Header = styles.Header.Bold(true).BorderStyle(lipgloss.NormalBorder()).BorderBottom(true).Foreground(brandRed)
	styles.Selected = styles.Selected.Foreground(lipgloss.AdaptiveColor{Light: "#FFFFFF", Dark: "#000000"}).Background(brandRed).Bold(true)
	t.SetStyles(styles)
	return t
}

func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m Model) Init() tea.Cmd {
	return tickCmd()
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tickMsg:
		m.refreshDashboard()
		m.syncTables()
		return m, tickCmd()
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.applyResponsiveLayout()
		return m, nil
	case tea.KeyMsg:
		switch m.mode {
		case modeMenu:
			return m.updateMenuKey(msg)
		case modeDashboard:
			return m.updateDashboardKey(msg)
		case modeInput:
			return m.updateInputKey(msg)
		case modeReportView:
			return m.updateReportKey(msg)
		case modeTypeForm:
			return m.updateTypeFormKey(msg)
		case modeConfirmDelete:
			return m.updateConfirmDeleteKey(msg)
		case modeEditClient:
			return m.updateEditClientKey(msg)
		case modeEditType:
			return m.updateEditTypeKey(msg)
		case modeSessionForm:
			return m.updateSessionFormKey(msg)
		}
	}
	return m, nil
}

func (m Model) updateMenuKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "up", "k":
		if m.menuCursor > 0 {
			m.menuCursor--
		}
		return m, nil
	case "down", "j":
		if m.menuCursor < 6 {
			m.menuCursor++
		}
		return m, nil
	case "enter":
		return m.selectMenuItem()
	case "a":
		m.enterInput(actionAddClient, "client name")
		return m, nil
	case "t":
		m.enterTypeForm()
		return m, nil
	case "s":
		m.enterSessionForm()
		return m, nil
	case "x":
		m.stopActiveSession()
		return m, nil
	case "r":
		m.resumeLatestSession()
		return m, nil
	case "d":
		m.mode = modeDashboard
		return m, nil
	case "p":
		m.enterInput(actionReport, "@client YYYY-MM-DD YYYY-MM-DD | @client last N days | @client last N weeks | @client this year")
		return m, nil
	case "?":
		m.showOverview = !m.showOverview
		return m, nil
	}
	return m, nil
}

func (m Model) selectMenuItem() (tea.Model, tea.Cmd) {
	switch m.menuCursor {
	case 0:
		m.enterInput(actionAddClient, "client name")
	case 1:
		m.enterTypeForm()
	case 2:
		m.enterSessionForm()
	case 3:
		m.stopActiveSession()
	case 4:
		m.resumeLatestSession()
	case 5:
		m.mode = modeDashboard
	case 6:
		m.enterInput(actionReport, "@client YYYY-MM-DD YYYY-MM-DD | @client last N days | @client last N weeks | @client this year")
	}
	return m, nil
}

func (m *Model) stopActiveSession() {
	if _, err := m.service.Stop(); err != nil {
		m.message = err.Error()
	} else {
		m.message = "stopped active session"
	}
	m.refreshDashboard()
	m.syncTables()
}

func (m *Model) resumeLatestSession() {
	if _, err := m.service.Resume(); err != nil {
		m.message = err.Error()
	} else {
		m.message = "resumed latest stopped session as new segment"
	}
	m.refreshDashboard()
	m.syncTables()
}

func (m Model) updateDashboardKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "m", "esc":
		m.mode = modeMenu
		return m, nil
	case "d":
		return m, nil
	case "a":
		m.enterInput(actionAddClient, "client name")
		return m, nil
	case "t":
		m.enterTypeForm()
		return m, nil
	case "s":
		m.enterSessionForm()
		return m, nil
	case "x":
		m.stopActiveSession()
		return m, nil
	case "r":
		m.resumeLatestSession()
		return m, nil
	case "p":
		m.enterInput(actionReport, "@client YYYY-MM-DD YYYY-MM-DD | @client last N days | @client last N weeks | @client this year")
		return m, nil
	case "c":
		return m.enterResourceInput()
	case "tab":
		m.focus = (m.focus + 1) % 3
		return m, nil
	case "?":
		m.showOverview = !m.showOverview
		return m, nil
	case "e":
		return m.startEditFromDashboard()
	case "D":
		return m.startDeleteFromDashboard()
	case "enter":
		if m.focus == focusPaused {
			return m.resumeSelectedPaused()
		}
	}

	switch m.focus {
	case focusClients:
		var cmd tea.Cmd
		m.clientsTable, cmd = m.clientsTable.Update(msg)
		return m, cmd
	case focusTypes:
		var cmd tea.Cmd
		m.typesTable, cmd = m.typesTable.Update(msg)
		return m, cmd
	default:
		var cmd tea.Cmd
		m.pausedTable, cmd = m.pausedTable.Update(msg)
		return m, cmd
	}
}

func (m Model) updateInputKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = modeMenu
		m.input.SetValue("")
		m.resourceSessionID = 0
		m.resourceSessionLabel = ""
		return m, nil
	case "enter":
		return m.submitInput()
	default:
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	}
}

func (m Model) updateReportKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "esc":
		m.mode = modeDashboard
		return m, nil
	}
	var cmd tea.Cmd
	m.reportViewport, cmd = m.reportViewport.Update(msg)
	return m, cmd
}

func (m *Model) enterInput(action inputAction, placeholder string) {
	m.mode = modeInput
	m.action = action
	m.input.SetValue("")
	m.input.Placeholder = placeholder
	m.input.Focus()
	m.message = ""
}

func (m *Model) enterTypeForm() {
	m.mode = modeTypeForm
	m.typeFormStep = 0
	m.typeFormName = ""
	m.typeFormBillable = false
	m.input.SetValue("")
	m.input.Placeholder = "type name"
	m.input.Focus()
	m.message = ""
}

func (m *Model) enterSessionForm() {
	m.refreshDashboard()
	if len(m.typeDetails) == 0 {
		m.message = "create a tracking type first"
		return
	}
	m.mode = modeSessionForm
	m.sessionFormStep = 0
	m.sessionFormType = 0
	m.sessionFormClient = 0
	m.input.SetValue("")
	m.input.Placeholder = "optional note"
	m.message = ""
}

func (m Model) updateTypeFormKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.typeFormStep {
	case 0:
		switch msg.String() {
		case "esc":
			m.mode = modeMenu
			m.input.SetValue("")
			return m, nil
		case "enter":
			name := strings.TrimSpace(m.input.Value())
			if name == "" {
				m.message = "type name is required"
				return m, nil
			}
			m.typeFormName = name
			m.typeFormStep = 1
			m.typeFormBillable = false
			m.input.SetValue("")
			m.message = ""
			return m, nil
		default:
			var cmd tea.Cmd
			m.input, cmd = m.input.Update(msg)
			return m, cmd
		}
	case 1:
		switch msg.String() {
		case "esc":
			m.mode = modeMenu
			m.input.SetValue("")
			return m, nil
		case "up", "down", "k", "j", "y", "n":
			m.typeFormBillable = !m.typeFormBillable
			return m, nil
		case "enter":
			if m.typeFormBillable {
				m.typeFormStep = 2
				m.input.SetValue("")
				m.input.Placeholder = "hourly rate"
				m.input.Focus()
				m.message = ""
			} else {
				return m.submitTypeForm(0)
			}
			return m, nil
		}
		return m, nil
	case 2:
		switch msg.String() {
		case "esc":
			m.mode = modeMenu
			m.input.SetValue("")
			return m, nil
		case "enter":
			rateStr := strings.TrimSpace(m.input.Value())
			if rateStr == "" {
				m.message = "hourly rate is required"
				return m, nil
			}
			rate, err := strconv.ParseFloat(rateStr, 64)
			if err != nil {
				m.message = "invalid rate — enter a number"
				return m, nil
			}
			if rate <= 0 {
				m.message = "rate must be greater than 0"
				return m, nil
			}
			return m.submitTypeForm(rate)
		default:
			var cmd tea.Cmd
			m.input, cmd = m.input.Update(msg)
			return m, cmd
		}
	}
	return m, nil
}

func (m Model) submitTypeForm(hourlyRate float64) (tea.Model, tea.Cmd) {
	if err := m.store.AddTrackingTypeWithBilling(m.typeFormName, m.typeFormBillable, hourlyRate); err != nil {
		m.message = err.Error()
	} else {
		if m.typeFormBillable {
			m.message = fmt.Sprintf("type created: %s (billable @ $%.0f/h)", m.typeFormName, hourlyRate)
		} else {
			m.message = fmt.Sprintf("type created: %s (non-billable)", m.typeFormName)
		}
	}
	m.mode = modeMenu
	m.input.SetValue("")
	m.refreshDashboard()
	m.syncTables()
	return m, nil
}

func (m Model) updateSessionFormKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.sessionFormStep {
	case 0: // select tracking type
		switch msg.String() {
		case "esc":
			m.mode = modeMenu
			return m, nil
		case "up", "k":
			if m.sessionFormType > 0 {
				m.sessionFormType--
			}
			return m, nil
		case "down", "j":
			if m.sessionFormType < len(m.typeDetails)-1 {
				m.sessionFormType++
			}
			return m, nil
		case "enter":
			m.sessionFormStep = 1
			m.sessionFormClient = 0
			return m, nil
		}
		return m, nil
	case 1: // select client
		clientCount := len(m.clientNames) + 1 // +1 for "(none)"
		switch msg.String() {
		case "esc":
			m.mode = modeMenu
			return m, nil
		case "up", "k":
			if m.sessionFormClient > 0 {
				m.sessionFormClient--
			}
			return m, nil
		case "down", "j":
			if m.sessionFormClient < clientCount-1 {
				m.sessionFormClient++
			}
			return m, nil
		case "enter":
			m.sessionFormStep = 2
			m.input.SetValue("")
			m.input.Placeholder = "optional note"
			m.input.Focus()
			return m, nil
		}
		return m, nil
	case 2: // note input
		switch msg.String() {
		case "esc":
			m.mode = modeMenu
			m.input.SetValue("")
			return m, nil
		case "enter":
			note := strings.TrimSpace(m.input.Value())
			typeName := m.typeDetails[m.sessionFormType].Name
			clientName := ""
			if m.sessionFormClient > 0 {
				clientName = m.clientNames[m.sessionFormClient-1]
			}
			if clientName == "" {
				m.message = "a client is required to start a session"
				return m, nil
			}
			if _, err := m.service.Start(clientName, typeName, note); err != nil {
				m.message = err.Error()
			} else {
				m.message = fmt.Sprintf("started session: @%s · %s", clientName, typeName)
			}
			m.mode = modeMenu
			m.input.SetValue("")
			m.refreshDashboard()
			m.syncTables()
			return m, nil
		default:
			var cmd tea.Cmd
			m.input, cmd = m.input.Update(msg)
			return m, cmd
		}
	}
	return m, nil
}

func (m Model) submitInput() (tea.Model, tea.Cmd) {
	value := strings.TrimSpace(m.input.Value())
	switch m.action {
	case actionAddClient:
		if value == "" {
			m.message = "client name is required"
			return m, nil
		}
		if err := m.store.AddClient(value); err != nil {
			m.message = err.Error()
		} else {
			m.message = "client created"
		}
	case actionAddType:
		if value == "" {
			m.message = "tracking type name is required"
			return m, nil
		}
		typeName, isBillable, hourlyRate, err := parseTrackingTypeInput(value)
		if err != nil {
			m.message = err.Error()
			return m, nil
		}
		if err := m.store.AddTrackingTypeWithBilling(typeName, isBillable, hourlyRate); err != nil {
			m.message = err.Error()
		} else {
			if isBillable {
				m.message = fmt.Sprintf("tracking type created (%s @ $%.2f/h)", typeName, hourlyRate)
			} else {
				m.message = "tracking type created"
			}
		}
	case actionStartSession:
		parts := strings.Fields(value)
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
	case actionReport:
		parts := strings.Fields(value)
		if len(parts) < 2 || !strings.HasPrefix(parts[0], "@") {
			m.message = "use: @client YYYY-MM-DD YYYY-MM-DD | @client last N days | @client last N weeks | @client this year"
			return m, nil
		}
		client := strings.TrimPrefix(parts[0], "@")
		period, err := parseReportPeriod(parts[1:])
		if err != nil {
			m.message = err.Error()
			return m, nil
		}
		from, to, err := report.ResolveDateRange(period, time.Now())
		if err != nil {
			m.message = err.Error()
			return m, nil
		}
		rows, summary, err := m.service.Report(client, from, to)
		if err != nil {
			m.message = err.Error()
			return m, nil
		}
		m.reportRows = rows
		m.reportTotal = summary.DurationSec
		m.reportTimeTotal = summary.TimeBillableTotal
		m.reportResTotal = summary.ResourceCostTotal
		m.reportGrand = summary.MonetaryTotal
		m.reportFrom = from
		m.reportTo = to
		m.message = fmt.Sprintf("report loaded: %d sessions", len(rows))
		m.mode = modeReportView
		m.refreshReportViewport(client)
		return m, nil
	case actionAddResource:
		if m.resourceSessionID <= 0 {
			m.message = "resource target session is not set"
			return m, nil
		}
		parts := strings.Fields(value)
		if len(parts) < 2 {
			m.message = "use: resource_name cost"
			return m, nil
		}
		resourceName := strings.TrimSpace(strings.Join(parts[:len(parts)-1], " "))
		if resourceName == "" {
			m.message = "resource name is required"
			return m, nil
		}
		cost, err := strconv.ParseFloat(parts[len(parts)-1], 64)
		if err != nil {
			m.message = "invalid cost; use: resource_name cost"
			return m, nil
		}
		if cost < 0 {
			m.message = "cost must be zero or greater"
			return m, nil
		}
		if err := m.service.AddSessionResource(m.resourceSessionID, resourceName, cost); err != nil {
			m.message = err.Error()
		} else {
			m.message = fmt.Sprintf("added resource to session %d: %s ($%.2f)", m.resourceSessionID, resourceName, cost)
		}
	}

	m.mode = modeMenu
	m.input.SetValue("")
	m.resourceSessionID = 0
	m.resourceSessionLabel = ""
	m.refreshDashboard()
	m.syncTables()
	return m, nil
}

func (m Model) enterResourceInput() (tea.Model, tea.Cmd) {
	if m.focus == focusPaused && len(m.paused) > 0 {
		idx := m.pausedTable.Cursor()
		if idx >= 0 && idx < len(m.paused) {
			p := m.paused[idx]
			m.resourceSessionID = p.ID
			m.resourceSessionLabel = fmt.Sprintf("paused session %d (@%s · %s)", p.ID, p.ClientName, p.TrackingTypeName)
			m.enterInput(actionAddResource, "resource_name cost")
			return m, nil
		}
	}
	if m.active != nil {
		m.resourceSessionID = m.active.ID
		m.resourceSessionLabel = fmt.Sprintf("active session %d (@%s · %s)", m.active.ID, m.active.ClientName, m.active.TrackingTypeName)
		m.enterInput(actionAddResource, "resource_name cost")
		return m, nil
	}
	m.message = "no active or selected paused session to attach resources"
	return m, nil
}

func (m Model) resumeSelectedPaused() (tea.Model, tea.Cmd) {
	if len(m.paused) == 0 {
		m.message = "no paused sessions to resume"
		return m, nil
	}
	idx := m.pausedTable.Cursor()
	if idx < 0 || idx >= len(m.paused) {
		m.message = "select a paused session first"
		return m, nil
	}
	selected := m.paused[idx]
	if _, err := m.service.ResumeSession(selected.ID); err != nil {
		m.message = err.Error()
		return m, nil
	}
	m.message = fmt.Sprintf("resumed paused session from @%s / %s", selected.ClientName, selected.TrackingTypeName)
	m.refreshDashboard()
	m.syncTables()
	return m, nil
}

func (m Model) startEditFromDashboard() (tea.Model, tea.Cmd) {
	switch m.focus {
	case focusClients:
		rows := m.clientsTable.Rows()
		idx := m.clientsTable.Cursor()
		if idx < 0 || idx >= len(rows) {
			m.message = "select a client to edit"
			return m, nil
		}
		oldName := rows[idx][0]
		m.editClientOldName = oldName
		m.mode = modeEditClient
		m.input.SetValue(oldName)
		m.input.Placeholder = "new client name"
		m.input.Focus()
		m.message = ""
		return m, nil
	case focusTypes:
		rows := m.typesTable.Rows()
		idx := m.typesTable.Cursor()
		if idx < 0 || idx >= len(rows) {
			m.message = "select a type to edit"
			return m, nil
		}
		oldName := rows[idx][0]
		m.editTypeOldName = oldName
		m.editTypeStep = 0
		// pre-fill from typeDetails
		for _, td := range m.typeDetails {
			if td.Name == oldName {
				m.editTypeName = td.Name
				m.editTypeBill = td.IsBillable
				break
			}
		}
		m.mode = modeEditType
		m.input.SetValue(oldName)
		m.input.Placeholder = "type name"
		m.input.Focus()
		m.message = ""
		return m, nil
	}
	m.message = "edit is available for Clients and Types tables"
	return m, nil
}

func (m Model) startDeleteFromDashboard() (tea.Model, tea.Cmd) {
	switch m.focus {
	case focusClients:
		rows := m.clientsTable.Rows()
		idx := m.clientsTable.Cursor()
		if idx < 0 || idx >= len(rows) {
			m.message = "select a client to delete"
			return m, nil
		}
		name := rows[idx][0]
		count, err := m.store.CountSessionsByClient(name)
		if err != nil {
			m.message = err.Error()
			return m, nil
		}
		m.confirmMsg = fmt.Sprintf("⚠ Delete client '%s' and %d session(s)?", name, count)
		m.confirmYes = false
		m.confirmAction = func() {
			if err := m.store.DeleteClient(name); err != nil {
				m.message = err.Error()
			} else {
				m.message = fmt.Sprintf("deleted client: %s", name)
			}
		}
		m.mode = modeConfirmDelete
		return m, nil
	case focusTypes:
		rows := m.typesTable.Rows()
		idx := m.typesTable.Cursor()
		if idx < 0 || idx >= len(rows) {
			m.message = "select a type to delete"
			return m, nil
		}
		name := rows[idx][0]
		count, err := m.store.CountSessionsByTrackingType(name)
		if err != nil {
			m.message = err.Error()
			return m, nil
		}
		m.confirmMsg = fmt.Sprintf("⚠ Delete tracking type '%s' and %d session(s)?", name, count)
		m.confirmYes = false
		m.confirmAction = func() {
			if err := m.store.DeleteTrackingType(name); err != nil {
				m.message = err.Error()
			} else {
				m.message = fmt.Sprintf("deleted type: %s", name)
			}
		}
		m.mode = modeConfirmDelete
		return m, nil
	case focusPaused:
		if len(m.paused) == 0 {
			m.message = "no sessions to delete"
			return m, nil
		}
		idx := m.pausedTable.Cursor()
		if idx < 0 || idx >= len(m.paused) {
			m.message = "select a session to delete"
			return m, nil
		}
		p := m.paused[idx]
		m.confirmMsg = fmt.Sprintf("⚠ Delete session %d (@%s · %s)?", p.ID, p.ClientName, p.TrackingTypeName)
		m.confirmYes = false
		m.confirmAction = func() {
			if err := m.store.DeleteSession(p.ID); err != nil {
				m.message = err.Error()
			} else {
				m.message = fmt.Sprintf("deleted session %d", p.ID)
			}
		}
		m.mode = modeConfirmDelete
		return m, nil
	}
	return m, nil
}

func (m Model) updateConfirmDeleteKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "esc":
		m.mode = modeDashboard
		m.message = ""
		return m, nil
	case "up", "down", "k", "j", "left", "right", "h", "l", "tab":
		m.confirmYes = !m.confirmYes
		return m, nil
	case "y":
		m.confirmYes = true
		return m, nil
	case "n":
		m.confirmYes = false
		return m, nil
	case "enter":
		if m.confirmYes && m.confirmAction != nil {
			m.confirmAction()
		} else {
			m.message = "cancelled"
		}
		m.mode = modeDashboard
		m.confirmAction = nil
		m.refreshDashboard()
		m.syncTables()
		return m, nil
	}
	return m, nil
}

func (m Model) updateEditClientKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = modeDashboard
		m.input.SetValue("")
		m.message = ""
		return m, nil
	case "enter":
		newName := strings.TrimSpace(m.input.Value())
		if newName == "" {
			m.message = "client name is required"
			return m, nil
		}
		if err := m.store.RenameClient(m.editClientOldName, newName); err != nil {
			m.message = err.Error()
		} else {
			m.message = fmt.Sprintf("renamed client: %s → %s", m.editClientOldName, newName)
		}
		m.mode = modeDashboard
		m.input.SetValue("")
		m.refreshDashboard()
		m.syncTables()
		return m, nil
	default:
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	}
}

func (m Model) updateEditTypeKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.editTypeStep {
	case 0: // name
		switch msg.String() {
		case "esc":
			m.mode = modeDashboard
			m.input.SetValue("")
			return m, nil
		case "enter":
			name := strings.TrimSpace(m.input.Value())
			if name == "" {
				m.message = "type name is required"
				return m, nil
			}
			m.editTypeName = name
			m.editTypeStep = 1
			m.message = ""
			return m, nil
		default:
			var cmd tea.Cmd
			m.input, cmd = m.input.Update(msg)
			return m, cmd
		}
	case 1: // billable toggle
		switch msg.String() {
		case "esc":
			m.mode = modeDashboard
			return m, nil
		case "up", "down", "k", "j", "y", "n":
			m.editTypeBill = !m.editTypeBill
			return m, nil
		case "enter":
			if m.editTypeBill {
				m.editTypeStep = 2
				m.input.SetValue("")
				m.input.Placeholder = "hourly rate"
				m.input.Focus()
				m.message = ""
			} else {
				return m.submitEditType(0)
			}
			return m, nil
		}
		return m, nil
	case 2: // rate
		switch msg.String() {
		case "esc":
			m.mode = modeDashboard
			m.input.SetValue("")
			return m, nil
		case "enter":
			rateStr := strings.TrimSpace(m.input.Value())
			if rateStr == "" {
				m.message = "hourly rate is required"
				return m, nil
			}
			rate, err := strconv.ParseFloat(rateStr, 64)
			if err != nil {
				m.message = "invalid rate — enter a number"
				return m, nil
			}
			if rate <= 0 {
				m.message = "rate must be greater than 0"
				return m, nil
			}
			return m.submitEditType(rate)
		default:
			var cmd tea.Cmd
			m.input, cmd = m.input.Update(msg)
			return m, cmd
		}
	}
	return m, nil
}

func (m Model) submitEditType(hourlyRate float64) (tea.Model, tea.Cmd) {
	if err := m.store.UpdateTrackingType(m.editTypeOldName, m.editTypeName, m.editTypeBill, hourlyRate); err != nil {
		m.message = err.Error()
	} else {
		m.message = fmt.Sprintf("updated type: %s", m.editTypeName)
	}
	m.mode = modeDashboard
	m.input.SetValue("")
	m.refreshDashboard()
	m.syncTables()
	return m, nil
}

func (m *Model) refreshDashboard() {
	active, err := m.service.Status()
	if err != nil {
		m.message = err.Error()
		return
	}
	m.active = active
	m.activeResTotal = 0
	if m.active != nil {
		resources, err := m.service.ListSessionResources(m.active.ID)
		if err != nil {
			m.message = err.Error()
			return
		}
		for _, r := range resources {
			m.activeResTotal += r.CostAmount
		}
	}

	clients, err := m.store.ListClients()
	if err != nil {
		m.message = err.Error()
		return
	}
	typeDetails, err := m.store.ListTrackingTypeDetails()
	if err != nil {
		m.message = err.Error()
		return
	}
	m.clientNames = clients
	m.typeDetails = typeDetails

	from, to := currentMonthRange(time.Now().UTC())
	clientTotals, err := m.store.DashboardTotalsByClient(from, to)
	if err != nil {
		m.message = err.Error()
		return
	}
	typeTotals, err := m.store.DashboardTotalsByTrackingType(from, to)
	if err != nil {
		m.message = err.Error()
		return
	}
	paused, err := m.store.ListPausedSessions(8)
	if err != nil {
		m.message = err.Error()
		return
	}
	m.clientTotals = clientTotals
	m.typeTotals = typeTotals
	m.paused = paused
}

func currentMonthRange(now time.Time) (time.Time, time.Time) {
	n := now.UTC()
	from := time.Date(n.Year(), n.Month(), 1, 0, 0, 0, 0, time.UTC)
	return from, n
}

func (m *Model) syncTables() {
	clientRows := make([]table.Row, 0, len(m.clientTotals))
	for _, t := range m.clientTotals {
		clientRows = append(clientRows, table.Row{
			t.Name,
			report.HumanDuration(t.DurationSec),
			fmt.Sprintf("$%.2f", t.AmountTotal),
		})
	}
	typeRows := make([]table.Row, 0, len(m.typeTotals))
	for _, t := range m.typeTotals {
		typeRows = append(typeRows, table.Row{
			t.Name,
			report.HumanDuration(t.DurationSec),
			fmt.Sprintf("$%.2f", t.AmountTotal),
		})
	}
	pausedRows := make([]table.Row, 0, len(m.paused))
	for _, p := range m.paused {
		pausedRows = append(pausedRows, table.Row{
			p.StoppedAt.Local().Format("2006-01-02 15:04"),
			p.ClientName,
			p.TrackingTypeName,
			report.HumanDuration(p.DurationSec),
			fmt.Sprintf("$%.2f", p.ResourceCostTotal),
			p.Note,
		})
	}

	m.clientsTable.SetRows(clientRows)
	m.typesTable.SetRows(typeRows)
	m.pausedTable.SetRows(pausedRows)
	m.applyResponsiveLayout()
}

func (m *Model) applyResponsiveLayout() {
	if m.width <= 0 {
		return
	}
	contentWidth := maxInt(40, m.width-4)
	leftWidth := (contentWidth - 1) / 2
	rightWidth := contentWidth - leftWidth - 1

	m.clientsTable.SetColumns([]table.Column{
		{Title: "Client", Width: maxInt(14, leftWidth-30)},
		{Title: "Duration", Width: 12},
		{Title: "Billable", Width: 12},
	})
	m.typesTable.SetColumns([]table.Column{
		{Title: "Type", Width: maxInt(14, leftWidth-30)},
		{Title: "Duration", Width: 12},
		{Title: "Billable", Width: 12},
	})
	m.pausedTable.SetColumns([]table.Column{
		{Title: "Stopped", Width: 16},
		{Title: "Client", Width: 14},
		{Title: "Type", Width: 14},
		{Title: "Duration", Width: 12},
		{Title: "Resources", Width: 12},
		{Title: "Note", Width: maxInt(12, rightWidth-72)},
	})

	// header (2) + active panel border+content (4) + footer (2) + border chrome (~6)
	chrome := 14
	availHeight := maxInt(12, m.height-chrome)
	sectionHeight := maxInt(3, availHeight/3)
	m.clientsTable.SetHeight(sectionHeight)
	m.typesTable.SetHeight(sectionHeight)
	m.pausedTable.SetHeight(maxInt(4, sectionHeight))

	m.reportViewport.Width = maxInt(20, m.width-6)
	m.reportViewport.Height = maxInt(8, m.height-8)
}

func (m *Model) refreshReportViewport(client string) {
	markdown := renderReportMarkdown(client, m.reportFrom, m.reportTo, m.reportRows, m.reportTotal, m.reportTimeTotal, m.reportResTotal, m.reportGrand)
	rendered := renderMarkdown(markdown, maxInt(40, m.reportViewport.Width))
	m.reportViewport.SetContent(rendered)
	m.reportViewport.GotoTop()
}

func renderReportMarkdown(client string, from, to time.Time, rows []sqlite.ReportRow, totalDuration int64, timeTotal, resourceTotal, monetaryTotal float64) string {
	var b strings.Builder
	fmt.Fprintf(&b, "## Report for @%s\n\n", client)
	fmt.Fprintf(&b, "**Range:** %s → %s  \n", from.Local().Format("2006-01-02"), to.Local().Format("2006-01-02"))
	fmt.Fprintf(&b, "**Totals:** %s · time **$%.2f** · resources **$%.2f** · combined **$%.2f**\n\n", report.HumanDuration(totalDuration), timeTotal, resourceTotal, monetaryTotal)

	if len(rows) == 0 {
		b.WriteString("_No sessions found for this period._\n")
		return b.String()
	}

	b.WriteString("| Start | Type | Duration | Billable | Time | Resources | Total | Note |\n")
	b.WriteString("| --- | --- | ---: | :---: | ---: | ---: | ---: | --- |\n")
	for _, r := range rows {
		fmt.Fprintf(
			&b,
			"| %s | %s | %s | %t | $%.2f | $%.2f | $%.2f | %s |\n",
			r.StartedAt.Local().Format("2006-01-02 15:04"),
			r.TrackingTypeName,
			report.HumanDuration(r.ComputedDurationS),
			r.IsBillable,
			r.BillableAmount,
			r.ResourceCostTotal,
			r.MonetaryTotal,
			strings.ReplaceAll(r.Note, "|", "\\|"),
		)
	}
	return b.String()
}

func renderMarkdown(markdown string, width int) string {
	renderer, err := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(width),
	)
	if err != nil {
		return markdown
	}
	out, err := renderer.Render(markdown)
	if err != nil {
		return markdown
	}
	return out
}

func (m Model) View() string {
	switch m.mode {
	case modeMenu:
		return m.viewMenu()
	case modeInput:
		return m.viewInput()
	case modeReportView:
		return m.viewReport()
	case modeTypeForm:
		return m.viewTypeForm()
	case modeConfirmDelete:
		return m.viewConfirmDelete()
	case modeEditClient:
		return m.viewEditClient()
	case modeEditType:
		return m.viewEditType()
	case modeSessionForm:
		return m.viewSessionForm()
	default:
		return m.viewDashboard()
	}
}

func (m Model) viewMenu() string {
	header := titleStyle.Render("Timmies TUI") + "  " + mutedStyle.Render("management menu")
	activeLine := mutedStyle.Render("Active session: none")
	if m.active != nil {
		elapsed := int64(time.Since(m.active.StartedAt).Seconds())
		if elapsed < 0 {
			elapsed = 0
		}
		activeLine = fmt.Sprintf("Active session: @%s · %s · %s", m.active.ClientName, m.active.TrackingTypeName, report.HumanDuration(elapsed))
	}

	menuItems := []string{
		"Add client",
		"Add tracking type",
		"Start session",
		"Stop session",
		"Resume latest",
		"Open dashboard",
		"Run report",
	}
	var menuLines []string
	for i, item := range menuItems {
		prefix := "  "
		if i == m.menuCursor {
			prefix = "> "
			item = titleStyle.Render(item)
		}
		menuLines = append(menuLines, prefix+item)
	}
	menuPanel := panelStyle.Width(maxInt(40, m.width-4)).Render(
		titleStyle.Render("Management actions") + "\n" +
			strings.Join(menuLines, "\n") + "\n\n" +
			mutedStyle.Render("Use ↑/↓ (or j/k), Enter to select."),
	)

	var clientsBuilder strings.Builder
	for _, c := range m.clientNames {
		fmt.Fprintf(&clientsBuilder, "- @%s\n", c)
	}
	if len(m.clientNames) == 0 {
		clientsBuilder.WriteString("- (none)\n")
	}

	var typesBuilder strings.Builder
	for _, t := range m.typeDetails {
		if t.IsBillable {
			fmt.Fprintf(&typesBuilder, "- %s (billable @ $%.2f/h)\n", t.Name, t.HourlyRate)
		} else {
			fmt.Fprintf(&typesBuilder, "- %s (non-billable)\n", t.Name)
		}
	}
	if len(m.typeDetails) == 0 {
		typesBuilder.WriteString("- (none)\n")
	}

	dbPanel := lipgloss.JoinHorizontal(
		lipgloss.Top,
		panelStyle.Width(maxInt(20, (maxInt(40, m.width-5))/2)).Render(titleStyle.Render("Clients")+"\n"+clientsBuilder.String()),
		" ",
		panelStyle.Width(maxInt(20, (maxInt(40, m.width-5))/2)).Render(titleStyle.Render("Tracking types")+"\n"+typesBuilder.String()),
	)
	if m.width > 0 && m.width < 110 {
		dbPanel = lipgloss.JoinVertical(
			lipgloss.Left,
			panelStyle.Width(maxInt(40, m.width-4)).Render(titleStyle.Render("Clients")+"\n"+clientsBuilder.String()),
			panelStyle.Width(maxInt(40, m.width-4)).Render(titleStyle.Render("Tracking types")+"\n"+typesBuilder.String()),
		)
	}

	footer := m.help.View(m.keys)
	if m.message != "" {
		if strings.Contains(strings.ToLower(m.message), "error") || strings.Contains(strings.ToLower(m.message), "invalid") {
			footer = errStyle.Render(m.message) + "\n" + footer
		} else {
			footer = infoStyle.Render(m.message) + "\n" + footer
		}
	}
	if m.showOverview {
		overview := renderMarkdown(
			"### Management menu\n\n- Create clients and tracking types from this page.\n- Tracking types now use a guided form: press **t** to add a type.\n- Start sessions with **s**, stop with **x**, resume latest with **r**.\n- Press **d** for dashboard, **p** for reports, **m** to return here.\n- In the dashboard, press **D** on a paused session to delete it.\n\n---\n_Created with ❤️ by Voxel North Technologies Inc. · O'Saasy License_",
			maxInt(40, m.width-6),
		)
		footer = panelStyle.Width(maxInt(40, m.width-4)).Render(overview) + "\n" + footer
	}

	out := lipgloss.JoinVertical(lipgloss.Left, renderTimmiesLogo(), header, mutedStyle.Render(activeLine), menuPanel, dbPanel, footer)
	return baseStyle.Render(out)
}

func (m Model) viewDashboard() string {
	header := titleStyle.Render("🍁 Timmies") + "  " + mutedStyle.Render("dashboard")
	monthLabel := time.Now().UTC().Format("January 2006")
	header = lipgloss.JoinVertical(lipgloss.Left, header, mutedStyle.Render("Current-month totals: "+monthLabel))

	activeLine := "none"
	if m.active != nil {
		elapsed := int64(time.Since(m.active.StartedAt).Seconds())
		if elapsed < 0 {
			elapsed = 0
		}
		activeLine = fmt.Sprintf("@%s · %s · %s · resources $%.2f · %s", m.active.ClientName, m.active.TrackingTypeName, report.HumanDuration(elapsed), m.activeResTotal, m.active.Note)
	}
	activePanel := panelStyle.Width(maxInt(40, m.width-4)).Render(
		titleStyle.Render("Active session") + "\n" + activeLine,
	)

	clientsTitle := "Clients"
	typesTitle := "Tracking types"
	pausedTitle := "Paused/stopped sessions"
	if m.focus == focusClients {
		clientsTitle += " ●"
	}
	if m.focus == focusTypes {
		typesTitle += " ●"
	}
	if m.focus == focusPaused {
		pausedTitle += " ●"
	}

	left := lipgloss.JoinVertical(
		lipgloss.Left,
		panelStyle.Render(titleStyle.Render(clientsTitle)+"\n"+m.clientsTable.View()),
		panelStyle.Render(titleStyle.Render(typesTitle)+"\n"+m.typesTable.View()),
	)
	right := panelStyle.Render(titleStyle.Render(pausedTitle) + "\n" + m.pausedTable.View())

	body := lipgloss.JoinHorizontal(lipgloss.Top, left, " ", right)
	if m.width > 0 && m.width < 110 {
		body = lipgloss.JoinVertical(lipgloss.Left, left, right)
	}

	footer := m.help.View(m.keys)
	if m.message != "" {
		if strings.Contains(strings.ToLower(m.message), "error") || strings.Contains(strings.ToLower(m.message), "invalid") {
			footer = errStyle.Render(m.message) + "\n" + footer
		} else {
			footer = infoStyle.Render(m.message) + "\n" + footer
		}
	}

	if m.showOverview {
		overview := renderMarkdown(
			"### Overview\n\n- Use **Tab** to move focus between dashboard sections.\n- Press **x** to stop the active session, **r** to resume the latest stopped session.\n- Press **e** to edit the selected client or type. Press **D** (shift-D) to delete.\n- Press **c** to add a resource cost to the active session or selected paused row.\n- In **Paused/stopped sessions**, press **Enter** to resume the selected row, **D** to delete it.\n- Use **p** for reports with explicit dates (`@client 2026-01-01 2026-01-31`) or relative periods (`@client last 2 weeks`, `@client this year`).\n\n---\n_Created with ❤️ by Voxel North Technologies Inc. · O'Saasy License_",
			maxInt(40, m.width-6),
		)
		footer = panelStyle.Width(maxInt(40, m.width-4)).Render(overview) + "\n" + footer
	}

	return baseStyle.Render(lipgloss.JoinVertical(lipgloss.Left, header, activePanel, body, footer))
}

func (m Model) viewInput() string {
	label := "Input"
	switch m.action {
	case actionAddClient:
		label = "Add client"
	case actionAddType:
		label = "Add tracking type"
	case actionStartSession:
		label = "Start session"
	case actionReport:
		label = "Run report"
	case actionAddResource:
		label = "Add session resource"
	}
	targetLine := ""
	if m.action == actionAddResource && m.resourceSessionLabel != "" {
		targetLine = mutedStyle.Render("Target: " + m.resourceSessionLabel)
	}
	panel := panelStyle.Width(maxInt(40, m.width-4)).Render(
		titleStyle.Render(label) + "\n" +
			m.input.View() + "\n" +
			targetLine + "\n" +
			mutedStyle.Render("Enter to submit · Esc to cancel"),
	)
	msg := ""
	if m.message != "" {
		msg = "\n" + infoStyle.Render(m.message)
	}
	return baseStyle.Render(renderTimmiesLogo() + "\n" + titleStyle.Render("Timmies TUI") + "\n" + panel + msg)
}

func (m Model) viewTypeForm() string {
	w := maxInt(40, m.width-4)

	stepName := formLabelStyle.Render("  1. Name")
	stepBillable := formLabelStyle.Render("  2. Billable")
	stepRate := formLabelStyle.Render("  3. Hourly rate")

	switch m.typeFormStep {
	case 0:
		stepName = formActiveStyle.Render("▸ 1. Name")
		stepBillable = formLabelStyle.Render("  2. Billable")
		stepRate = formLabelStyle.Render("  3. Hourly rate")
	case 1:
		stepName = formLabelStyle.Render("  1. ") + formAnswerStyle.Render(m.typeFormName)
		stepBillable = formActiveStyle.Render("▸ 2. Billable")
		stepRate = formLabelStyle.Render("  3. Hourly rate")
	case 2:
		stepName = formLabelStyle.Render("  1. ") + formAnswerStyle.Render(m.typeFormName)
		stepBillable = formLabelStyle.Render("  2. ") + formAnswerStyle.Render("Yes")
		stepRate = formActiveStyle.Render("▸ 3. Hourly rate")
	}

	steps := lipgloss.JoinVertical(lipgloss.Left, stepName, stepBillable, stepRate)

	var inputArea string
	switch m.typeFormStep {
	case 0:
		inputArea = m.input.View()
	case 1:
		yes := "  Yes"
		no := "  No"
		if m.typeFormBillable {
			yes = formSelectedStyle.Render("▸ Yes")
			no = "  No"
		} else {
			yes = "  Yes"
			no = formSelectedStyle.Render("▸ No")
		}
		inputArea = lipgloss.JoinVertical(lipgloss.Left, yes, no) + "\n" +
			mutedStyle.Render("↑/↓ to toggle · Enter to confirm")
	case 2:
		inputArea = m.input.View()
	}

	panel := panelStyle.Width(w).Render(
		titleStyle.Render("Add tracking type") + "\n\n" +
			steps + "\n\n" +
			inputArea + "\n" +
			mutedStyle.Render("Esc to cancel"),
	)

	msg := ""
	if m.message != "" {
		msg = "\n" + errStyle.Render(m.message)
	}
	return baseStyle.Render(renderTimmiesLogo() + "\n" + titleStyle.Render("Timmies TUI") + "\n" + panel + msg)
}

func (m Model) viewConfirmDelete() string {
	w := maxInt(40, m.width/2)

	noLabel := formSelectedStyle.Render("▸ No")
	yesLabel := formLabelStyle.Render("  Yes")
	if m.confirmYes {
		yesLabel = formSelectedStyle.Render("▸ Yes")
		noLabel = formLabelStyle.Render("  No")
	}

	panel := panelStyle.Width(w).Render(
		titleStyle.Render("Confirm delete") + "\n\n" +
			m.confirmMsg + "\n\n" +
			lipgloss.JoinHorizontal(lipgloss.Top, yesLabel, "   ", noLabel) + "\n\n" +
			mutedStyle.Render("←/→ or y/n to toggle · Enter to confirm · Esc to cancel"),
	)

	return baseStyle.Render(renderTimmiesLogo() + "\n" + panel)
}

func (m Model) viewEditClient() string {
	w := maxInt(40, m.width-4)
	panel := panelStyle.Width(w).Render(
		titleStyle.Render("Edit client") + "\n\n" +
			formLabelStyle.Render("Current: ") + formAnswerStyle.Render(m.editClientOldName) + "\n\n" +
			formActiveStyle.Render("New name:") + "\n" +
			m.input.View() + "\n\n" +
			mutedStyle.Render("Enter to save · Esc to cancel"),
	)
	msg := ""
	if m.message != "" {
		msg = "\n" + errStyle.Render(m.message)
	}
	return baseStyle.Render(renderTimmiesLogo() + "\n" + panel + msg)
}

func (m Model) viewEditType() string {
	w := maxInt(40, m.width-4)

	stepName := formLabelStyle.Render("  1. Name")
	stepBillable := formLabelStyle.Render("  2. Billable")
	stepRate := formLabelStyle.Render("  3. Hourly rate")

	switch m.editTypeStep {
	case 0:
		stepName = formActiveStyle.Render("▸ 1. Name")
	case 1:
		stepName = formLabelStyle.Render("  1. ") + formAnswerStyle.Render(m.editTypeName)
		stepBillable = formActiveStyle.Render("▸ 2. Billable")
	case 2:
		stepName = formLabelStyle.Render("  1. ") + formAnswerStyle.Render(m.editTypeName)
		stepBillable = formLabelStyle.Render("  2. ") + formAnswerStyle.Render("Yes")
		stepRate = formActiveStyle.Render("▸ 3. Hourly rate")
	}

	steps := lipgloss.JoinVertical(lipgloss.Left, stepName, stepBillable, stepRate)

	var inputArea string
	switch m.editTypeStep {
	case 0:
		inputArea = m.input.View()
	case 1:
		yes := "  Yes"
		no := "  No"
		if m.editTypeBill {
			yes = formSelectedStyle.Render("▸ Yes")
		} else {
			no = formSelectedStyle.Render("▸ No")
		}
		inputArea = lipgloss.JoinVertical(lipgloss.Left, yes, no) + "\n" +
			mutedStyle.Render("↑/↓ to toggle · Enter to confirm")
	case 2:
		inputArea = m.input.View()
	}

	panel := panelStyle.Width(w).Render(
		titleStyle.Render("Edit tracking type") + "\n" +
			formLabelStyle.Render("Editing: ") + formAnswerStyle.Render(m.editTypeOldName) + "\n\n" +
			steps + "\n\n" +
			inputArea + "\n" +
			mutedStyle.Render("Esc to cancel"),
	)

	msg := ""
	if m.message != "" {
		msg = "\n" + errStyle.Render(m.message)
	}
	return baseStyle.Render(renderTimmiesLogo() + "\n" + panel + msg)
}

func (m Model) viewSessionForm() string {
	w := maxInt(40, m.width-4)

	stepType := formLabelStyle.Render("  1. Tracking type")
	stepClient := formLabelStyle.Render("  2. Client")
	stepNote := formLabelStyle.Render("  3. Note")

	var inputArea string

	switch m.sessionFormStep {
	case 0:
		stepType = formActiveStyle.Render("▸ 1. Tracking type")
		var lines []string
		for i, td := range m.typeDetails {
			label := td.Name
			if td.IsBillable {
				label += fmt.Sprintf(" ($%.0f/h)", td.HourlyRate)
			}
			if i == m.sessionFormType {
				lines = append(lines, formSelectedStyle.Render("  ▸ "+label))
			} else {
				lines = append(lines, formLabelStyle.Render("    "+label))
			}
		}
		inputArea = strings.Join(lines, "\n") + "\n" +
			mutedStyle.Render("↑/↓ to select · Enter to confirm")
	case 1:
		selType := m.typeDetails[m.sessionFormType]
		stepType = formLabelStyle.Render("  1. ") + formAnswerStyle.Render(selType.Name)
		stepClient = formActiveStyle.Render("▸ 2. Client")
		options := []string{"(none)"}
		for _, c := range m.clientNames {
			options = append(options, "@"+c)
		}
		var lines []string
		for i, opt := range options {
			if i == m.sessionFormClient {
				lines = append(lines, formSelectedStyle.Render("  ▸ "+opt))
			} else {
				lines = append(lines, formLabelStyle.Render("    "+opt))
			}
		}
		inputArea = strings.Join(lines, "\n") + "\n" +
			mutedStyle.Render("↑/↓ to select · Enter to confirm")
	case 2:
		selType := m.typeDetails[m.sessionFormType]
		stepType = formLabelStyle.Render("  1. ") + formAnswerStyle.Render(selType.Name)
		clientLabel := "(none)"
		if m.sessionFormClient > 0 {
			clientLabel = "@" + m.clientNames[m.sessionFormClient-1]
		}
		stepClient = formLabelStyle.Render("  2. ") + formAnswerStyle.Render(clientLabel)
		stepNote = formActiveStyle.Render("▸ 3. Note")
		inputArea = m.input.View()
	}

	steps := lipgloss.JoinVertical(lipgloss.Left, stepType, stepClient, stepNote)

	panel := panelStyle.Width(w).Render(
		titleStyle.Render("Start session") + "\n\n" +
			steps + "\n\n" +
			inputArea + "\n" +
			mutedStyle.Render("Esc to cancel"),
	)

	msg := ""
	if m.message != "" {
		msg = "\n" + errStyle.Render(m.message)
	}
	return baseStyle.Render(renderTimmiesLogo() + "\n" + panel + msg)
}

func (m Model) viewReport() string {
	header := titleStyle.Render("Timmies TUI report") + "\n" + mutedStyle.Render("Esc to dashboard · ↑/↓/PgUp/PgDn to scroll")
	panel := panelStyle.Width(maxInt(40, m.width-4)).Render(m.reportViewport.View())
	return baseStyle.Render(lipgloss.JoinVertical(lipgloss.Left, renderTimmiesLogo(), header, panel))
}

func Run(store *sqlite.Store, svc *service.TimerService) error {
	p := tea.NewProgram(New(store, svc), tea.WithAltScreen())
	_, err := p.Run()
	return err
}

func parseReportPeriod(parts []string) (report.PeriodOptions, error) {
	if len(parts) == 2 {
		return report.PeriodOptions{FromDate: parts[0], ToDate: parts[1]}, nil
	}
	return report.ParseRelativePeriod(parts)
}

func parseTrackingTypeInput(value string) (name string, isBillable bool, hourlyRate float64, err error) {
	parts := strings.Fields(value)
	if len(parts) == 0 {
		return "", false, 0, fmt.Errorf("tracking type name is required")
	}
	name = parts[0]
	if len(parts) == 1 {
		return name, false, 0, nil
	}
	if len(parts) != 3 || strings.ToLower(parts[1]) != "billable" {
		return "", false, 0, fmt.Errorf("use: type_name | type_name billable hourly_rate")
	}
	rate, parseErr := strconv.ParseFloat(parts[2], 64)
	if parseErr != nil {
		return "", false, 0, fmt.Errorf("invalid hourly rate; use: type_name billable hourly_rate")
	}
	if rate <= 0 {
		return "", false, 0, fmt.Errorf("hourly rate must be greater than 0")
	}
	return name, true, rate, nil
}

func renderTimmiesLogo() string {
	return leafStyle.Render("🍁") + " " + logoText.Render("TIMMIES")
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
