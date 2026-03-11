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
	modeDashboard mode = iota
	modeInput
	modeReportView
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
	addClient    key.Binding
	addType      key.Binding
	start        key.Binding
	stop         key.Binding
	resumeLatest key.Binding
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
		addClient:    key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "add client")),
		addType:      key.NewBinding(key.WithKeys("k"), key.WithHelp("k", "add type")),
		start:        key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "start session")),
		stop:         key.NewBinding(key.WithKeys("x"), key.WithHelp("x", "stop active")),
		resumeLatest: key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "resume latest")),
		report:       key.NewBinding(key.WithKeys("p"), key.WithHelp("p", "run report")),
		addResource:  key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "add resource cost")),
		resumePaused: key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "resume selected paused")),
		switchFocus:  key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "switch focus")),
		toggleHelp:   key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "overview/help")),
		back:         key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back")),
	}
}

func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.addClient, k.start, k.addResource, k.report, k.quit}
}

func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.addClient, k.addType, k.start, k.stop, k.resumeLatest, k.addResource, k.report},
		{k.resumePaused, k.switchFocus, k.toggleHelp, k.back, k.quit},
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
}

var (
	titleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))
	panelStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1)
	mutedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("242"))
	infoStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("86"))
	errStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
)

func New(store *sqlite.Store, svc *service.TimerService) Model {
	ti := textinput.New()
	ti.Prompt = "> "
	ti.Focus()

	m := Model{
		store:          store,
		service:        svc,
		mode:           modeDashboard,
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
	styles.Header = styles.Header.Bold(true).BorderStyle(lipgloss.NormalBorder()).BorderBottom(true).Foreground(lipgloss.Color("99"))
	styles.Selected = styles.Selected.Foreground(lipgloss.Color("230")).Background(lipgloss.Color("62")).Bold(true)
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
		case modeDashboard:
			return m.updateDashboardKey(msg)
		case modeInput:
			return m.updateInputKey(msg)
		case modeReportView:
			return m.updateReportKey(msg)
		}
	}
	return m, nil
}

func (m Model) updateDashboardKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "a":
		m.enterInput(actionAddClient, "client name")
		return m, nil
	case "k":
		m.enterInput(actionAddType, "tracking type name")
		return m, nil
	case "s":
		m.enterInput(actionStartSession, "@client type note...")
		return m, nil
	case "x":
		if _, err := m.service.Stop(); err != nil {
			m.message = err.Error()
		} else {
			m.message = "stopped active session"
		}
		m.refreshDashboard()
		m.syncTables()
		return m, nil
	case "r":
		if _, err := m.service.Resume(); err != nil {
			m.message = err.Error()
		} else {
			m.message = "resumed latest stopped session as new segment"
		}
		m.refreshDashboard()
		m.syncTables()
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
		m.mode = modeDashboard
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
		if err := m.store.AddTrackingType(value); err != nil {
			m.message = err.Error()
		} else {
			m.message = "tracking type created"
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

	m.mode = modeDashboard
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

	from, to := currentMonthRange(time.Now().UTC())
	clients, err := m.store.DashboardTotalsByClient(from, to)
	if err != nil {
		m.message = err.Error()
		return
	}
	types, err := m.store.DashboardTotalsByTrackingType(from, to)
	if err != nil {
		m.message = err.Error()
		return
	}
	paused, err := m.store.ListPausedSessions(8)
	if err != nil {
		m.message = err.Error()
		return
	}
	m.clientTotals = clients
	m.typeTotals = types
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

	sectionHeight := maxInt(4, (maxInt(18, m.height)-9)/3)
	m.clientsTable.SetHeight(sectionHeight)
	m.typesTable.SetHeight(sectionHeight)
	m.pausedTable.SetHeight(maxInt(6, sectionHeight*2+1))

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
	if m.mode == modeInput {
		return m.viewInput()
	}
	if m.mode == modeReportView {
		return m.viewReport()
	}
	return m.viewDashboard()
}

func (m Model) viewDashboard() string {
	header := titleStyle.Render("Timmies TUI") + "  " + mutedStyle.Render("dashboard")
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
			"### Overview\n\n- Use **Tab** to move focus between dashboard sections.\n- Press **c** to add a resource cost to the active session or selected paused row.\n- In **Paused/stopped sessions**, press **Enter** to resume the selected row.\n- Use **p** for reports with explicit dates (`@client 2026-01-01 2026-01-31`) or relative periods (`@client last 2 weeks`, `@client this year`).",
			maxInt(40, m.width-6),
		)
		footer = panelStyle.Width(maxInt(40, m.width-4)).Render(overview) + "\n" + footer
	}

	return lipgloss.JoinVertical(lipgloss.Left, header, activePanel, body, footer)
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
	return titleStyle.Render("Timmies TUI") + "\n" + panel + msg
}

func (m Model) viewReport() string {
	header := titleStyle.Render("Timmies TUI report") + "\n" + mutedStyle.Render("Esc to dashboard · ↑/↓/PgUp/PgDn to scroll")
	panel := panelStyle.Width(maxInt(40, m.width-4)).Render(m.reportViewport.View())
	return lipgloss.JoinVertical(lipgloss.Left, header, panel)
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

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
