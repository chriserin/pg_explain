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
		key.WithKeys("ctrl+k"),
		key.WithHelp("ctrl+k", "Settings up"),
	),
	SettingsDown: key.NewBinding(
		key.WithKeys("ctrl+j"),
		key.WithHelp("ctrl+j", "Settings down"),
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
		key.WithKeys("J"),
		key.WithHelp("J", "Join"),
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
	ctx                  ProgramContext
	DisplayNodes         []PlanNode
	StatusLine           StatusLine
	detailsViewport      Section
	sqlViewport          Section
	thisSettingsViewport Section
	nextSettingsViewport Section
	source               Source
	originalSource       Source
	queryRun             QueryRun
	spinner              spinner.Model
	stopwatch            stopwatch.Model
	sqlChannel           chan QueryRun
	loading              bool
	pgexPointer          string
	nextRunSettings      []Setting
	error                error
	errorViewport        Section
	explainCancelFn      context.CancelFunc
	runType              RunType
}

func InitModel(source Source, runType RunType) Model {
	ctx := InitProgramContext()
	nextRunSettings := NewSection("Settings", 80, 7)
	thisRunSettings := NewSection("Settings", 80, 7)
	nextRunSettings.subtitle = ctx.SettingsStyles.SelectedSettingsType.Render(" Next Run ")
	thisRunSettings.subtitle = ctx.SettingsStyles.SelectedSettingsType.Render(" This Run ")
	sqlViewport := NewSection("SQL", 80, 10)

	return Model{
		ctx:                  ctx,
		keys:                 keys,
		help:                 help.New(),
		sqlHelp:              help.New(),
		detailsViewport:      NewSection("Details", 80, 10),
		sqlViewport:          sqlViewport,
		nextSettingsViewport: nextRunSettings,
		thisSettingsViewport: thisRunSettings,
		source:               source,
		originalSource:       source,
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
	m.SetDisplayNodes(displayedNodes(explainPlan.nodes, m.ctx))
	m.StatusLine = NewStatusLine(explainPlan)
}

func (m *Model) SetDisplayNodes(nodes []PlanNode) {
	m.DisplayNodes = displayedNodes(nodes, m.ctx)
	m.setSqlViewHeight()
}

func (m *Model) setSqlViewHeight() {
	m.sqlViewport.SetDimensions(m.ctx.Width-1, m.ctx.Height-len(m.DisplayNodes)-13)
}

type SourceType int

const (
	SOURCE_ZERO SourceType = iota
	SOURCE_STDIN
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

func (s Source) View(ctx ProgramContext) string {
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
	RunExplain RunType = iota
	RunExplainAnalyze
)

func RunProgram(source Source, runType RunType, teaOpts ...tea.ProgramOption) *tea.Program {
	model := InitModel(source, runType)

	if source.sourceType == SOURCE_STDIN {
		explainPlan := Convert(source.input)
		model.UpdateModel(explainPlan)
		model.ctx.ResetContext(explainPlan, model)
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
	return func() tea.Msg {
		newQueryRun, err := latestQueryRun()
		if err != nil {
			return errorMsg{error: err}
		}
		return newQueryRunMsg{queryRun: newQueryRun}
	}
}

type executeQueryMsg struct {
	queryRun QueryRun
}

type errorMsg struct {
	error error
}

func ExecuteAnalyzeQueryCmd(fileName string, settings []Setting, ctx context.Context) tea.Cmd {
	return func() tea.Msg {
		queryRun := NewQueryRun(fileName)
		queryWithExplain := queryRun.WithExplainAnalyze()
		var queryRunSettings = make([]Setting, len(settings), len(settings))
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

func ExecuteExplainQueryCmd(fileName string, settings []Setting, ctx context.Context) tea.Cmd {
	return func() tea.Msg {
		queryRun := NewQueryRun(fileName)
		queryWithExplain := queryRun.WithExplain()
		var queryRunSettings = make([]Setting, len(settings), len(settings))
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
	if m.source.sourceType == SOURCE_STDIN {
		return nil
	} else if m.source.sourceType == SOURCE_PGEX {
		return LatestQueryRun()
	} else {
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
			m.ctx.Indent = !m.ctx.Indent
		case key.Matches(msg, m.keys.Up):
			if m.ctx.Cursor-1 >= 0 {
				m.ctx.Cursor = m.ctx.Cursor - 1
				m.ctx.SelectedNode = m.DisplayNodes[m.ctx.Cursor]
			}
		case key.Matches(msg, m.keys.Down):
			if m.ctx.Cursor+1 < len(m.DisplayNodes) {
				m.ctx.Cursor = m.ctx.Cursor + 1
				m.ctx.SelectedNode = m.DisplayNodes[m.ctx.Cursor]
			}
		case key.Matches(msg, m.keys.SettingsUp):
			if m.ctx.SettingsCursor-1 >= 0 {
				m.ctx.SettingsCursor = m.ctx.SettingsCursor - 1
			}
		case key.Matches(msg, m.keys.SettingsDown):
			if m.ctx.SettingsCursor+1 < len(m.nextRunSettings) {
				m.ctx.SettingsCursor = m.ctx.SettingsCursor + 1
			}
		case key.Matches(msg, m.keys.Help):
			m.help.ShowAll = !m.help.ShowAll
		case key.Matches(msg, m.keys.JoinView):
			m.ctx.JoinView = !m.ctx.JoinView
			m.SetDisplayNodes(displayedNodes(m.nodes, m.ctx))
			m.ctx.Cursor = 0
			if len(m.DisplayNodes) > 0 {
				m.ctx.SelectedNode = m.DisplayNodes[m.ctx.Cursor]
			} else {
				m.ctx.SelectedNode = PlanNode{}
			}
		case key.Matches(msg, m.keys.NextStatDisplay):
			m.ctx.StatDisplay = nextStatDisplay(m.ctx)
		case key.Matches(msg, m.keys.PrevStatDisplay):
			m.ctx.StatDisplay = prevStatDisplay(m.ctx)
		case key.Matches(msg, m.keys.ToggleParallel):
			m.ctx.DisplayParallel = !m.ctx.DisplayParallel
		case key.Matches(msg, m.keys.ToggleNumbers):
			m.ctx.DisplayNumbers = !m.ctx.DisplayNumbers
		case key.Matches(msg, m.keys.ToggleDisplaySql):
			m.ctx.DisplaySql = !m.ctx.DisplaySql
		case key.Matches(msg, m.keys.ToggleRelations):
			m.ctx.DisplayRelations = !m.ctx.DisplayRelations
		case key.Matches(msg, m.keys.ReAnalyze):
			if m.originalSource.sourceType == SOURCE_FILE {
				m.loading = true
				m.stopwatch = stopwatch.New(stopwatch.WithInterval(time.Millisecond * 100))
				explainContext, cancelFunc := context.WithCancel(context.Background())
				m.explainCancelFn = cancelFunc
				m.runType = RunExplainAnalyze
				return m, tea.Batch(m.stopwatch.Init(), m.spinner.Tick, ExecuteAnalyzeQueryCmd(m.originalSource.fileName, m.nextRunSettings, explainContext))
			}
		case key.Matches(msg, m.keys.ReAnalyze):
			if m.originalSource.sourceType == SOURCE_FILE {
				m.loading = true
				m.stopwatch = stopwatch.New(stopwatch.WithInterval(time.Millisecond * 100))
				explainContext, cancelFunc := context.WithCancel(context.Background())
				m.explainCancelFn = cancelFunc
				m.runType = RunExplain
				return m, tea.Batch(m.stopwatch.Init(), m.spinner.Tick, ExecuteExplainQueryCmd(m.originalSource.fileName, m.nextRunSettings, explainContext))
			}
		case key.Matches(msg, m.keys.PrevQueryRun):
			return m, PreviousQueryRun(m.queryRun)
		case key.Matches(msg, m.keys.NextQueryRun):
			return m, NextQueryRun(m.queryRun)
		case key.Matches(msg, m.keys.SqlUp):
			m.sqlViewport.LineUp(1)
		case key.Matches(msg, m.keys.SqlDown):
			m.sqlViewport.LineDown(1)
		case key.Matches(msg, m.keys.SettingIncrement):
			m.nextRunSettings[m.ctx.SettingsCursor].IncrementSetting()
		case key.Matches(msg, m.keys.SettingDecrement):
			m.nextRunSettings[m.ctx.SettingsCursor].DecrementSetting()
		default:
			return m, tea.Println(msg)
		}
	case showAllMsg:
		m.nextRunSettings = msg.settings
		m.loading = true
		explainContext, cancelFunc := context.WithCancel(context.Background())
		m.explainCancelFn = cancelFunc
		return m, tea.Batch(m.spinner.Tick, ExecuteExplainQueryCmd(m.source.fileName, m.nextRunSettings, explainContext))
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
		return m, tea.Batch(m.stopwatch.Init(), ExecuteAnalyzeQueryCmd(m.source.fileName, m.nextRunSettings, explainContext))
	case executeQueryMsg:
		UpdateModel(&m, msg.queryRun)
		m.loading = false
		m.explainCancelFn = nil
		SaveQueryRun(msg.queryRun)
		return m, tea.Batch(m.stopwatch.Stop(), m.stopwatch.Reset())
	case newQueryRunMsg:
		newQueryRun := msg.queryRun
		if newQueryRun.pgexPointer != m.queryRun.pgexPointer {
			UpdateModel(&m, newQueryRun)
			m.source = Source{sourceType: SOURCE_PGEX, fileName: newQueryRun.pgexPointer}
		}
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
		m.ctx.Width = msg.Width
		m.ctx.Height = msg.Height
		m.setSqlViewHeight()
		m.detailsViewport.SetDimensions(m.ctx.Width-1, 10)
		m.thisSettingsViewport.SetDimensions((m.ctx.Width-1)/2, len(allowedSettings)+2)
		var nextSettingsWidth int
		if m.ctx.Width%2 == 1 {
			nextSettingsWidth = (m.ctx.Width-1)/2 - 1
		} else {
			nextSettingsWidth = (m.ctx.Width - 1) / 2
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
	queryRun.WritePgexFile(pgexDir)
	if err != nil {
		fmt.Println("Error", err)
	}
}

func UpdateModel(m *Model, queryRun QueryRun) {
	m.queryRun = queryRun
	explainPlan := Convert(queryRun.result)
	m.UpdateModel(explainPlan)
	m.ctx.ResetContext(explainPlan, *m)
	m.ctx.SelectedNode = m.DisplayNodes[0]
	wrappedSql := ansi.Wordwrap(queryRun.query, m.ctx.Width-10, "") + "\n"
	m.sqlViewport.SetContent(wrappedSql)
}

func prevStatDisplay(ctx ProgramContext) StatView {
	newStatDisplay := ctx.StatDisplay

	for true {
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

func nextStatDisplay(ctx ProgramContext) StatView {
	newStatDisplay := ctx.StatDisplay

	for true {
		newStatDisplay = (newStatDisplay + 1) % 5

		if ctx.Analyzed {
			break
		} else if slices.Contains([]StatView{DisplayRows, DisplayCost, DisplayNothing}, newStatDisplay) {
			break
		}
	}
	return newStatDisplay
}

func displayedNodes(nodes []PlanNode, ctx ProgramContext) []PlanNode {
	resultNodes := make([]PlanNode, 0, len(nodes))

	for _, node := range nodes {
		if node.Display(ctx) {
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
	sourceView := m.source.View(m.ctx)
	buf.WriteString(sourceView)

	spaceAvailable := m.ctx.Width - ansi.StringWidth(sourceView)

	buf.WriteString(fmt.Sprintf("%*s%*s", spaceAvailable-10, m.ctx.StatDisplay.String(), 10, ""))
	buf.WriteString("\n")

	statusLine := m.StatusLine.View(m)
	buf.WriteString(statusLine)
	buf.WriteString(lipgloss.NewStyle().Background(lipgloss.Color("#1E2030")).Render(HeadersView(m.ctx, m.ctx.Width-ansi.StringWidth(statusLine)-1)))
	buf.WriteString("\n")

	for i, node := range m.DisplayNodes {
		buf.WriteString(node.View(i, m.ctx))
	}

	buf.WriteString("\n")
	if m.error != nil {
		buf.WriteString(m.errorViewport.View())
	} else if m.ctx.DisplaySql {
		buf.WriteString(m.sqlViewport.View())
		buf.WriteString("\n")
		buf.WriteString(m.sqlHelp.ShortHelpView(keys.SqlShortHelp()))
	} else {
		m.detailsViewport.SetContent(m.ctx.SelectedNode.Content(m.ctx))
		m.detailsViewport.subtitle = m.ctx.NormalStyle.NodeName.Render(m.ctx.SelectedNode.Name())
		buf.WriteString(m.detailsViewport.View())
		buf.WriteString("\n")
		if slices.Contains([]SourceType{SOURCE_PGEX, SOURCE_FILE}, m.source.sourceType) {
			m.thisSettingsViewport.SetContent(SettingsView(m.queryRun.settings, m.ctx, false))
			m.nextSettingsViewport.SetContent(SettingsView(m.nextRunSettings, m.ctx, true))
			buf.WriteString(lipgloss.JoinHorizontal(1, m.thisSettingsViewport.View(), " ", m.nextSettingsViewport.View()))
		}
		buf.WriteString("\n")
		buf.WriteString(m.help.View(m.keys))
	}
	buf.WriteString("\n")

	return buf.String()
}

func HeadersView(ctx ProgramContext, spaceAvailable int) string {
	var headers string
	if ctx.StatDisplay == DisplayTime {
		headers = fmt.Sprintf("%10s%15s ", "Startup", "Total")
	} else if ctx.StatDisplay == DisplayCost {
		headers = fmt.Sprintf("%10s%15s ", "Startup", "Total")
	} else if ctx.StatDisplay == DisplayBuffers {
		headers = fmt.Sprintf("%10s%15s ", "Total", "Read")
	} else if ctx.StatDisplay == DisplayRows {
		headers = fmt.Sprintf("%10s%15s ", "Planned", "Actual")
	} else if ctx.StatDisplay == DisplayNothing {
		headers = ""
	}
	return fmt.Sprintf("%*s", spaceAvailable, headers)
}

func SettingsView(settings []Setting, ctx ProgramContext, nextSettings bool) string {
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
