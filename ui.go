package main

import (
	"context"
	"fmt"
	"path"
	"slices"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/stopwatch"
)

// keyMap defines a set of keybindings. To work for help it must satisfy
// key.Map. It could also very easily be a map[string]key.Binding.
type keyMap struct {
	IndentToggle       key.Binding
	Up                 key.Binding
	Down               key.Binding
	SettingsUp         key.Binding
	SettingsDown       key.Binding
	ToggleSettingsType key.Binding
	ToggleSettings     key.Binding
	Help               key.Binding
	Quit               key.Binding
	JoinView           key.Binding
	ToggleDisplaySql   key.Binding
	NextStatDisplay    key.Binding
	PrevStatDisplay    key.Binding
	ToggleParallel     key.Binding
	ToggleNumbers      key.Binding
	ToggleRelations    key.Binding
	ReExplain          key.Binding
	ReAnalyze          key.Binding
	PrevQueryRun       key.Binding
	NextQueryRun       key.Binding
	SqlUp              key.Binding
	SqlDown            key.Binding
	SettingIncrement   key.Binding
	SettingDecrement   key.Binding
}

// ShortHelp returns keybindings to be shown in the mini help view. It's part
// of the key.Map interface.
func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Help, k.Quit}
}

func (k keyMap) SqlShortHelp() []key.Binding {
	return []key.Binding{k.ToggleDisplaySql, k.SqlUp, k.SqlDown}
}

// FullHelp returns keybindings for the expanded help view. It's part of the
// key.Map interface.
func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.ToggleParallel, k.ToggleNumbers, k.ToggleDisplaySql, k.ToggleRelations, k.ReExplain, k.ReAnalyze}, // first column
		{k.NextStatDisplay, k.PrevStatDisplay, k.SettingsUp, k.SettingsDown, k.SettingIncrement, k.SettingDecrement},
		{k.PrevQueryRun, k.NextQueryRun, k.Help, k.Quit}, // second column
	}
}

var keys = keyMap{
	IndentToggle: key.NewBinding(
		key.WithKeys("I"),
		key.WithHelp("I", "Indent toggle"),
	),
	Up: key.NewBinding(
		key.WithKeys("up", "k"),
		key.WithHelp("↑/k", "Move up"),
	),
	Down: key.NewBinding(
		key.WithKeys("down", "j"),
		key.WithHelp("↓/j", "Move down"),
	),
	SettingsUp: key.NewBinding(
		key.WithKeys("K"),
		key.WithHelp("K", "Settings up"),
	),
	SettingsDown: key.NewBinding(
		key.WithKeys("J"),
		key.WithHelp("J", "Settings down"),
	),
	Help: key.NewBinding(
		key.WithKeys("?"),
		key.WithHelp("?", "Toggle help"),
	),
	Quit: key.NewBinding(
		key.WithKeys("q", "esc", "ctrl+c"),
		key.WithHelp("q", "Quit"),
	),
	JoinView: key.NewBinding(
		key.WithKeys("O"),
		key.WithHelp("O", "Show Joins Only"),
	),
	NextStatDisplay: key.NewBinding(
		key.WithKeys("]"),
		key.WithHelp("]", "Next Stat Display"),
	),
	PrevStatDisplay: key.NewBinding(
		key.WithKeys("["),
		key.WithHelp("[", "Prev Stat Display"),
	),
	ToggleParallel: key.NewBinding(
		key.WithKeys("P"),
		key.WithHelp("P", "Toggle Parallel"),
	),
	ToggleNumbers: key.NewBinding(
		key.WithKeys("N"),
		key.WithHelp("N", "Toggle Numbers"),
	),
	ToggleRelations: key.NewBinding(
		key.WithKeys("R"),
		key.WithHelp("R", "Toggle Relations"),
	),
	ToggleDisplaySql: key.NewBinding(
		key.WithKeys("D"),
		key.WithHelp("D", "Toggle Display SQL"),
	),
	ReExplain: key.NewBinding(
		key.WithKeys("X"),
		key.WithHelp("X", "ReExplain Query"),
	),
	ReAnalyze: key.NewBinding(
		key.WithKeys("A"),
		key.WithHelp("A", "ReExecute Query"),
	),
	PrevQueryRun: key.NewBinding(
		key.WithKeys("{"),
		key.WithHelp("{", "Prev Query Run"),
	),
	NextQueryRun: key.NewBinding(
		key.WithKeys("}"),
		key.WithHelp("}", "Next Query Run"),
	),
	SqlUp: key.NewBinding(
		key.WithKeys("ctrl+p"),
		key.WithHelp("ctrl+p", "SQL Up"),
	),
	SqlDown: key.NewBinding(
		key.WithKeys("ctrl+n"),
		key.WithHelp("ctrl+n", "SQL Down"),
	),
	ToggleSettings: key.NewBinding(
		key.WithKeys("S"),
		key.WithHelp("S", "Toggle Settings"),
	),
	ToggleSettingsType: key.NewBinding(
		key.WithKeys("s"),
		key.WithHelp("s", "Toggle Settings Type"),
	),
	SettingIncrement: key.NewBinding(
		key.WithKeys("+"),
		key.WithHelp("+", "Increment Setting"),
	),
	SettingDecrement: key.NewBinding(
		key.WithKeys("-"),
		key.WithHelp("-", "Decrement Setting"),
	),
}

type Model struct {
	keys                 keyMap
	help                 help.Model
	sqlHelp              help.Model
	nodes                []PlanNode
	uiState              ProgramUIState
	DisplayNodes         []PlanNode
	StatusLine           StatusLine
	detailsViewport      Section
	sqlViewport          Section
	thisSettingsViewport Section
	nextSettingsViewport Section
	source               Source
	queryRun             QueryRun
	spinner              spinner.Model
	stopwatch            stopwatch.Model
	loading              bool
	nextRunSettings      []Setting
	error                error
	errorViewport        Section
	explainCancelFn      context.CancelFunc
	runType              RunType
}

func InitModel(source Source, runType RunType) Model {
	ctx := InitProgramUIState()
	nextRunSettings := NewSection("Settings", 80, 7)
	thisRunSettings := NewSection("Settings", 80, 7)
	nextRunSettings.subtitle = ctx.SettingsStyles.SelectedSettingsType.Render(" Next Run ")
	thisRunSettings.subtitle = ctx.SettingsStyles.SelectedSettingsType.Render(" This Run ")
	sqlViewport := NewSection("SQL", 80, 10)

	return Model{
		uiState:              ctx,
		keys:                 keys,
		help:                 help.New(),
		sqlHelp:              help.New(),
		detailsViewport:      NewSection("Details", 80, 10),
		sqlViewport:          sqlViewport,
		nextSettingsViewport: nextRunSettings,
		thisSettingsViewport: thisRunSettings,
		source:               source,
		spinner:              initialSpinner(),
		errorViewport:        NewSection("!Error!", 80, 7),
		runType:              runType,
	}
}

func initialSpinner() spinner.Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	return s
}

func (m *Model) UpdateModel(explainPlan ExplainPlan) {
	m.nodes = explainPlan.nodes
	m.SetDisplayNodes(displayedNodes(explainPlan.nodes, m.uiState))
	m.StatusLine = NewStatusLine(explainPlan)
}

func (m *Model) SetDisplayNodes(nodes []PlanNode) {
	m.DisplayNodes = displayedNodes(nodes, m.uiState)
	m.setSqlViewHeight()
}

func (m *Model) setSqlViewHeight() {
	m.sqlViewport.SetDimensions(m.uiState.Width-1, m.uiState.Height-len(m.DisplayNodes)-13)
}

type SourceType int

const (
	SOURCE_STDIN SourceType = iota
	SOURCE_FILE
	SOURCE_PGEX
)

type Source struct {
	sourceType SourceType
	fileName   string
	input      string
}

func (s Source) DisplayName() string {
	_, file := path.Split(s.fileName)
	return file
}

func (s Source) FileDate() string {
	_, file := path.Split(s.fileName)
	parts := strings.Split(file, "_")
	pgex_datetime, err := time.Parse(PGEX_DATE_FORMAT, parts[0])
	if err != nil {
		return ""
	}
	return pgex_datetime.Format(time.DateTime)
}

func (s Source) View(ctx ProgramUIState) string {
	switch s.sourceType {
	case SOURCE_FILE:
		return ctx.StatusStyles.AltNormal.Render(fmt.Sprintf("FILE - %s", s.DisplayName()))
	case SOURCE_PGEX:
		return ctx.StatusStyles.AltNormal.UnsetBackground().Render(fmt.Sprintf("PGEX - %s - %s", s.FileDate(), s.DisplayName()))
	default:
		return "STDIN"
	}
}

type RunType int

const (
	RunNothing RunType = iota
	RunExplain
	RunExplainAnalyze
)

func RunProgram(source Source, runType RunType, teaOpts ...tea.ProgramOption) *tea.Program {
	model := InitModel(source, runType)

	switch source.sourceType {
	case SOURCE_STDIN:
		explainPlan := Convert(source.input)
		model.UpdateModel(explainPlan)
		model.uiState.ResetUIState(explainPlan, model)
	case SOURCE_FILE:
		model.queryRun = NewQueryRun(source.fileName)
	}

	program := tea.NewProgram(
		model,
		teaOpts...,
	)

	return program
}

type newQueryRunMsg struct{ queryRun QueryRun }

func PreviousQueryRun(queryRun QueryRun) tea.Cmd {
	return func() tea.Msg {
		newQueryRun, err := queryRun.previousQueryRun()
		if err != nil {
			return errorMsg{error: err}
		}
		return newQueryRunMsg{queryRun: newQueryRun}
	}
}

func NextQueryRun(queryRun QueryRun) tea.Cmd {
	return func() tea.Msg {
		newQueryRun, err := queryRun.nextQueryRun()
		if err != nil {
			return errorMsg{error: err}
		}
		return newQueryRunMsg{queryRun: newQueryRun}
	}
}

func LatestQueryRun() tea.Cmd {
	return tea.Batch(func() tea.Msg {
		newQueryRun, err := latestQueryRun()
		if err != nil {
			return errorMsg{error: err}
		}
		return newQueryRunMsg{queryRun: newQueryRun}
	}, ShowAllCmd)
}

type executeQueryMsg struct {
	queryRun QueryRun
}

type errorMsg struct {
	error error
}

func ExecuteAnalyzeQueryCmd(queryRun QueryRun, settings []Setting, ctx context.Context) tea.Cmd {
	return func() tea.Msg {
		queryWithExplain := queryRun.WithExplainAnalyze()
		var queryRunSettings = make([]Setting, len(settings))
		copy(queryRunSettings, settings)
		queryRun.settings = queryRunSettings
		result, err := ExecuteExplain(queryWithExplain, settings, ctx)
		if err != nil {
			return errorMsg{error: err}
		}
		queryRun.SetResult(result)
		return executeQueryMsg{queryRun: queryRun}
	}
}

type executeExplainQueryMsg struct {
	queryRun QueryRun
}

func ExecuteExplainQueryCmd(queryRun QueryRun, settings []Setting, ctx context.Context) tea.Cmd {
	return func() tea.Msg {
		queryWithExplain := queryRun.WithExplain()
		var queryRunSettings = make([]Setting, len(settings))
		copy(queryRunSettings, settings)
		queryRun.settings = queryRunSettings
		result, err := ExecuteExplain(queryWithExplain, settings, ctx)
		if err != nil {
			return errorMsg{error: err}
		}
		queryRun.SetResult(result)
		return executeExplainQueryMsg{queryRun: queryRun}
	}
}

type showAllMsg struct {
	settings []Setting
}

func ShowAllCmd() tea.Msg {
	settings, err := ShowAll()
	if err != nil {
		return errorMsg{error: err}
	}
	slices.SortFunc(settings, SettingCompare)
	return showAllMsg{settings: settings}
}

func ShowAll() ([]Setting, error) {
	pgConn := Connection{
		connConfig: ConnConfig,
	}
	err := pgConn.Connect()
	if err != nil {
		return nil, err
	}
	defer pgConn.Close()
	return pgConn.ShowAll()
}

func (m Model) Init() tea.Cmd {
	switch m.source.sourceType {
	case SOURCE_STDIN:
		return nil
	case SOURCE_PGEX:
		return LatestQueryRun()
	default:
		return ShowAllCmd
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, m.keys.Quit):
			if m.explainCancelFn != nil {
				m.explainCancelFn()
			}
			return m, tea.Quit
		case key.Matches(msg, m.keys.IndentToggle):
			m.uiState.Indent = !m.uiState.Indent
		case key.Matches(msg, m.keys.Up):
			if m.uiState.Cursor-1 >= 0 {
				m.uiState.Cursor = m.uiState.Cursor - 1
				m.uiState.SelectedNode = m.DisplayNodes[m.uiState.Cursor]
			}
		case key.Matches(msg, m.keys.Down):
			if m.uiState.Cursor+1 < len(m.DisplayNodes) {
				m.uiState.Cursor = m.uiState.Cursor + 1
				m.uiState.SelectedNode = m.DisplayNodes[m.uiState.Cursor]
			}
		case key.Matches(msg, m.keys.SettingsUp):
			if m.uiState.SettingsCursor-1 >= 0 {
				m.uiState.SettingsCursor = m.uiState.SettingsCursor - 1
			}
		case key.Matches(msg, m.keys.SettingsDown):
			if m.uiState.SettingsCursor+1 < len(m.nextRunSettings) {
				m.uiState.SettingsCursor = m.uiState.SettingsCursor + 1
			}
		case key.Matches(msg, m.keys.Help):
			m.help.ShowAll = !m.help.ShowAll
		case key.Matches(msg, m.keys.JoinView):
			m.uiState.JoinView = !m.uiState.JoinView
			m.SetDisplayNodes(displayedNodes(m.nodes, m.uiState))
			m.uiState.Cursor = 0
			if len(m.DisplayNodes) > 0 {
				m.uiState.SelectedNode = m.DisplayNodes[m.uiState.Cursor]
			} else {
				m.uiState.SelectedNode = PlanNode{}
			}
		case key.Matches(msg, m.keys.NextStatDisplay):
			m.uiState.StatDisplay = nextStatDisplay(m.uiState)
		case key.Matches(msg, m.keys.PrevStatDisplay):
			m.uiState.StatDisplay = prevStatDisplay(m.uiState)
		case key.Matches(msg, m.keys.ToggleParallel):
			m.uiState.DisplayParallel = !m.uiState.DisplayParallel
		case key.Matches(msg, m.keys.ToggleNumbers):
			m.uiState.DisplayNumbers = !m.uiState.DisplayNumbers
		case key.Matches(msg, m.keys.ToggleDisplaySql):
			m.uiState.DisplaySql = !m.uiState.DisplaySql
		case key.Matches(msg, m.keys.ToggleRelations):
			m.uiState.DisplayRelations = !m.uiState.DisplayRelations
		case key.Matches(msg, m.keys.ReAnalyze):
			m.loading = true
			m.stopwatch = stopwatch.New(stopwatch.WithInterval(time.Millisecond * 100))
			explainContext, cancelFunc := context.WithCancel(context.Background())
			m.explainCancelFn = cancelFunc
			m.runType = RunExplainAnalyze
			return m, tea.Batch(m.stopwatch.Init(), m.spinner.Tick, ExecuteAnalyzeQueryCmd(m.queryRun, m.nextRunSettings, explainContext))
		case key.Matches(msg, m.keys.ReExplain):
			m.loading = true
			m.stopwatch = stopwatch.New(stopwatch.WithInterval(time.Millisecond * 100))
			explainContext, cancelFunc := context.WithCancel(context.Background())
			m.explainCancelFn = cancelFunc
			m.runType = RunExplain
			return m, tea.Batch(m.stopwatch.Init(), m.spinner.Tick, ExecuteExplainQueryCmd(m.queryRun, m.nextRunSettings, explainContext))
		case key.Matches(msg, m.keys.PrevQueryRun):
			return m, PreviousQueryRun(m.queryRun)
		case key.Matches(msg, m.keys.NextQueryRun):
			return m, NextQueryRun(m.queryRun)
		case key.Matches(msg, m.keys.SqlUp):
			m.sqlViewport.LineUp(1)
		case key.Matches(msg, m.keys.SqlDown):
			m.sqlViewport.LineDown(1)
		case key.Matches(msg, m.keys.SettingIncrement):
			m.nextRunSettings[m.uiState.SettingsCursor].IncrementSetting()
		case key.Matches(msg, m.keys.SettingDecrement):
			m.nextRunSettings[m.uiState.SettingsCursor].DecrementSetting()
		default:
			return m, nil
		}
	case showAllMsg:
		m.nextRunSettings = msg.settings
		if m.runType == RunNothing {
			return m, nil
		}
		m.loading = true
		explainContext, cancelFunc := context.WithCancel(context.Background())
		m.explainCancelFn = cancelFunc
		return m, tea.Batch(m.spinner.Tick, ExecuteExplainQueryCmd(m.queryRun, m.nextRunSettings, explainContext))
	case executeExplainQueryMsg:
		UpdateModel(&m, msg.queryRun)
		if m.runType == RunExplain {
			SaveQueryRun(msg.queryRun)
			m.loading = false
			m.explainCancelFn = nil
			return m, tea.Batch(m.stopwatch.Stop(), m.stopwatch.Reset())
		}
		m.loading = true
		m.stopwatch = stopwatch.New(stopwatch.WithInterval(time.Millisecond * 100))
		explainContext, cancelFunc := context.WithCancel(context.Background())
		m.explainCancelFn = cancelFunc
		return m, tea.Batch(m.stopwatch.Init(), ExecuteAnalyzeQueryCmd(m.queryRun, m.nextRunSettings, explainContext))
	case executeQueryMsg:
		UpdateModel(&m, msg.queryRun)
		m.loading = false
		m.explainCancelFn = nil
		SaveQueryRun(msg.queryRun)
		return m, tea.Batch(m.stopwatch.Stop(), m.stopwatch.Reset())
	case newQueryRunMsg:
		newQueryRun := msg.queryRun
		UpdateModel(&m, newQueryRun)
		m.source = Source{sourceType: SOURCE_PGEX, fileName: newQueryRun.pgexPointer}
		return m, nil
	case errorMsg:
		m.error = msg.error
		m.errorViewport.SetContent(msg.error.Error())
		m.loading = false
		return m, m.stopwatch.Stop()
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		if m.loading {
			return m, cmd
		}
	case stopwatch.StartStopMsg:
		var cmd tea.Cmd
		m.stopwatch, cmd = m.stopwatch.Update(msg)
		return m, cmd
	case stopwatch.TickMsg:
		var cmd tea.Cmd
		m.stopwatch, cmd = m.stopwatch.Update(msg)
		if m.loading {
			return m, cmd
		}
	case tea.WindowSizeMsg:
		m.uiState.Width = msg.Width
		m.uiState.Height = msg.Height
		m.setSqlViewHeight()
		m.detailsViewport.SetDimensions(m.uiState.Width-1, 10)
		m.thisSettingsViewport.SetDimensions((m.uiState.Width-1)/2, len(allowedSettings)+2)
		var nextSettingsWidth int
		if m.uiState.Width%2 == 1 {
			nextSettingsWidth = (m.uiState.Width-1)/2 - 1
		} else {
			nextSettingsWidth = (m.uiState.Width - 1) / 2
		}
		m.nextSettingsViewport.SetDimensions(nextSettingsWidth, len(allowedSettings)+2)
	}

	return m, nil
}

func SaveQueryRun(queryRun QueryRun) {
	pgexDir, err := CreatePgexDir()
	if err != nil {
		fmt.Println("Error", err)
	}
	err = queryRun.WritePgexFile(pgexDir)
	if err != nil {
		fmt.Println("Error", err)
	}
}

func UpdateModel(m *Model, queryRun QueryRun) {
	m.queryRun = queryRun
	explainPlan := Convert(queryRun.result)
	m.UpdateModel(explainPlan)
	m.uiState.ResetUIState(explainPlan, *m)
	m.uiState.SelectedNode = m.DisplayNodes[0]
	wrappedSql := ansi.Wordwrap(queryRun.query, m.uiState.Width-10, "") + "\n"
	m.sqlViewport.SetContent(wrappedSql)
}

func prevStatDisplay(ctx ProgramUIState) StatView {
	newStatDisplay := ctx.StatDisplay

	for {
		if newStatDisplay == 0 {
			newStatDisplay = DisplayCost
		} else {
			newStatDisplay = (newStatDisplay - 1) % 5
		}

		if ctx.Analyzed {
			break
		} else if slices.Contains([]StatView{DisplayRows, DisplayCost, DisplayNothing}, newStatDisplay) {
			break
		}
	}
	return newStatDisplay
}

func nextStatDisplay(ctx ProgramUIState) StatView {
	newStatDisplay := ctx.StatDisplay

	for {
		newStatDisplay = (newStatDisplay + 1) % 5

		if ctx.Analyzed {
			break
		} else if slices.Contains([]StatView{DisplayRows, DisplayCost, DisplayNothing}, newStatDisplay) {
			break
		}
	}
	return newStatDisplay
}

func displayedNodes(nodes []PlanNode, uiState ProgramUIState) []PlanNode {
	resultNodes := make([]PlanNode, 0, len(nodes))

	for _, node := range nodes {
		if node.Display(uiState) {
			resultNodes = append(resultNodes, node)
		}
	}

	return resultNodes
}

func (m Model) View() tea.View {
	content := m.renderView()
	view := tea.NewView(content)
	view.AltScreen = true
	return view
}

func (m Model) renderView() string {
	var buf strings.Builder

	var spinnerView string
	if m.loading {
		spinnerView = m.spinner.View()
	} else {
		spinnerView = "  "
	}
	buf.WriteString(spinnerView)
	sourceView := m.source.View(m.uiState)
	buf.WriteString(sourceView)

	spaceAvailable := m.uiState.Width - ansi.StringWidth(sourceView)

	fmt.Fprintf(&buf, "%*s%*s", spaceAvailable-10, m.uiState.StatDisplay.String(), 10, "")
	buf.WriteString("\n")

	statusLine := m.StatusLine.View(m)
	buf.WriteString(statusLine)
	buf.WriteString(lipgloss.NewStyle().Background(lipgloss.Color("#1E2030")).Render(HeadersView(m.uiState, m.uiState.Width-ansi.StringWidth(statusLine)-1)))
	buf.WriteString("\n")

	for i, node := range m.DisplayNodes {
		buf.WriteString(node.View(i, m.uiState))
	}

	buf.WriteString("\n")
	if m.error != nil {
		buf.WriteString(m.errorViewport.View())
	} else if m.uiState.DisplaySql {
		buf.WriteString(m.sqlViewport.View())
		buf.WriteString("\n")
		buf.WriteString(m.sqlHelp.ShortHelpView(keys.SqlShortHelp()))
	} else {
		m.detailsViewport.SetContent(m.uiState.SelectedNode.Content(m.uiState))
		m.detailsViewport.subtitle = m.uiState.NormalStyle.NodeName.Render(m.uiState.SelectedNode.Name())
		buf.WriteString(m.detailsViewport.View())
		buf.WriteString("\n")
		if m.source.sourceType != SOURCE_STDIN {
			m.thisSettingsViewport.SetContent(SettingsView(m.queryRun.settings, m.uiState, false))
			m.nextSettingsViewport.SetContent(SettingsView(m.nextRunSettings, m.uiState, true))
			buf.WriteString(lipgloss.JoinHorizontal(1, m.thisSettingsViewport.View(), " ", m.nextSettingsViewport.View()))
		}
		buf.WriteString("\n")
		buf.WriteString(m.help.View(m.keys))
	}
	buf.WriteString("\n")

	return buf.String()
}

func HeadersView(ctx ProgramUIState, spaceAvailable int) string {
	var headers string
	switch ctx.StatDisplay {
	case DisplayTime:
		headers = fmt.Sprintf("%10s%15s ", "Startup", "Total")
	case DisplayCost:
		headers = fmt.Sprintf("%10s%15s ", "Startup", "Total")
	case DisplayBuffers:
		headers = fmt.Sprintf("%10s%15s ", "Total", "Read")
	case DisplayRows:
		headers = fmt.Sprintf("%10s%15s ", "Planned", "Actual")
	case DisplayNothing:
		headers = ""
	}
	return fmt.Sprintf("%*s", spaceAvailable, headers)
}

func SettingsView(settings []Setting, ctx ProgramUIState, nextSettings bool) string {
	var buf strings.Builder

	for i, setting := range settings {
		if i == ctx.SettingsCursor && nextSettings {
			buf.WriteString(ctx.SettingsStyles.SelectedSettingsType.Render(setting.View()))
		} else {
			buf.WriteString(ctx.NormalStyle.Everything.Render(setting.View()))
		}
		buf.WriteString("\n")
	}
	return buf.String()
}
