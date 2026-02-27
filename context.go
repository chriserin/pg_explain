package main

import (
	"charm.land/lipgloss/v2"
)

type StatView int

const (
	DisplayNothing StatView = iota
	DisplayTime
	DisplayRows
	DisplayBuffers
	DisplayCost
)

func (s StatView) String() string {
	switch s {
	case DisplayNothing:
		return ""
	case DisplayRows:
		return "Rows"
	case DisplayBuffers:
		return "Buffers"
	case DisplayCost:
		return "Cost"
	case DisplayTime:
		return "Time"
	}
	return ""
}

type ProgramContext struct {
	Indent           bool
	Cursor           int
	SettingsCursor   int
	JoinView         bool
	StatDisplay      StatView
	DisplayParallel  bool
	DisplayNumbers   bool
	NormalStyle      Styles
	CursorStyle      Styles
	ChildCursorStyle Styles
	StatusStyles     StatusStyles
	DetailStyles     DetailStyles
	SettingsStyles   SettingsStyles
	SelectedNode     PlanNode
	Width            int
	Height           int
	Analyzed         bool
	DisplaySql       bool
	DisplayRelations bool
}

type Styles struct {
	Gutter     lipgloss.Style
	NodeName   lipgloss.Style
	Everything lipgloss.Style
	Relation   lipgloss.Style
	Bracket    lipgloss.Style
	Value      lipgloss.Style
	Warning    lipgloss.Style
	Caution    lipgloss.Style
	Workers    lipgloss.Style
}

type StatusStyles struct {
	Value     lipgloss.Style
	Normal    lipgloss.Style
	AltNormal lipgloss.Style
}

type DetailStyles struct {
	Label   lipgloss.Style
	Warning lipgloss.Style
}

type SettingsStyles struct {
	Cursor               lipgloss.Style
	SelectedSettingsType lipgloss.Style
}

func InitProgramContext() ProgramContext {
	normal := NormalStyles()

	return ProgramContext{
		Cursor:           0,
		Indent:           true,
		JoinView:         false,
		NormalStyle:      normal,
		CursorStyle:      CursorStyle(normal),
		ChildCursorStyle: ChildCursorStyle(normal),
		StatusStyles:     StatusLineStyles(),
		DetailStyles:     DetailViewStyles(),
		SettingsStyles:   SettingsViewStyles(),
		StatDisplay:      DisplayRows,
		DisplayNumbers:   true,
		DisplayParallel:  true,
	}
}

func (ctx *ProgramContext) ResetContext(explainPlan ExplainPlan, model Model) {
	ctx.Cursor = 0
	ctx.SettingsCursor = 0
	ctx.SelectedNode = model.DisplayNodes[ctx.SettingsCursor]
	ctx.Analyzed = explainPlan.analyzed
}

func StatusLineStyles() StatusStyles {

	color_a := lipgloss.Color("#9999bb")
	color_c := lipgloss.Color("#1E2030")

	value := lipgloss.NewStyle().Background(color_a).Foreground(color_c)
	normal := lipgloss.NewStyle().Background(color_a).Foreground(color_c)
	altNormal := lipgloss.NewStyle().Foreground(color_a).Background(color_c)

	return StatusStyles{
		Value:     value,
		Normal:    normal,
		AltNormal: altNormal,
	}
}

func DetailViewStyles() DetailStyles {
	return DetailStyles{
		Label:   lipgloss.NewStyle().Bold(true),
		Warning: lipgloss.NewStyle().Background(lipgloss.Color("#880000")),
	}
}

func SettingsViewStyles() SettingsStyles {
	return SettingsStyles{
		Cursor:               lipgloss.NewStyle().Background(lipgloss.Color("#452297")),
		SelectedSettingsType: lipgloss.NewStyle().Background(lipgloss.Color("#65bcff")).Foreground(lipgloss.Color("#000000")),
	}
}

func NormalStyles() Styles {
	gutterStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#777"))
	nodeNameStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#65bcff"))
	everythingStyle := lipgloss.NewStyle()
	relationStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#c099ff"))
	bracketStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#828bb8"))
	valueStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#c3e88d"))
	warningStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#c53b53"))
	cautionStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#ffc777"))
	workersStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#fca7ea"))

	return Styles{
		Gutter:     gutterStyle,
		NodeName:   nodeNameStyle,
		Everything: everythingStyle,
		Relation:   relationStyle,
		Bracket:    bracketStyle,
		Value:      valueStyle,
		Warning:    warningStyle,
		Caution:    cautionStyle,
		Workers:    workersStyle,
	}
}

func CursorStyle(style Styles) Styles {
	background := lipgloss.Color("#222277")

	return Styles{
		Gutter:     style.Gutter.Foreground(lipgloss.Color("#ff966c")).Background(background),
		NodeName:   style.NodeName.Background(background),
		Everything: style.Everything.Background(background),
		Relation:   style.Relation.Background(background),
		Bracket:    style.Bracket.Background(background),
		Value:      style.Value.Background(background),
		Warning:    style.Warning.Background(background),
		Caution:    style.Caution.Background(background),
		Workers:    style.Workers.Background(background),
	}
}

func ChildCursorStyle(style Styles) Styles {
	background := lipgloss.Color("#2f334d")

	return Styles{
		Gutter:     style.Gutter.Background(background),
		NodeName:   style.NodeName.Background(background),
		Everything: style.Everything.Background(background),
		Relation:   style.Relation.Background(background),
		Bracket:    style.Bracket.Background(background),
		Value:      style.Value.Background(background),
		Warning:    style.Warning.Background(background),
		Caution:    style.Caution.Background(background),
		Workers:    style.Workers.Background(background),
	}
}
