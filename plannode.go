package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/x/ansi"
)

type Position struct {
	Id          int
	Level       int
	Parent      int
	Display     bool
	BelowGather bool
}

type PlanNode struct {
	// Explain Attributes
	NodeType           string
	PartialMode        string
	ParallelAware      bool
	Plans              []PlanNode
	PlanRows           int
	Position           Position
	JoinViewPosition   Position
	RelationName       string
	IsGather           bool
	PlannedWorkers     int
	StartupCost        float64
	TotalCost          float64
	IndexName          string
	IndexCond          string
	Filter             string
	JoinFilter         string
	HashCond           string
	RecheckCond        string
	GroupKey           []string
	SortKeys           []string
	PresortKeys        []string
	ParentRelationship string
	ParentIsNestedLoop bool
	Analyzed           Analyzed
	PlanWidth          int
	Strategy           string
	Command            string
	CteName            string
	Function           string
	FunctionName       string
	TableFunctionName  string
	TidCond            string
	Operation          string
	JoinType           string
	SubPlanName        string
}

type Analyzed struct {
	ActualRows        int
	SharedBuffersRead int
	SharedBuffersHit  int
	LaunchedWorkers   int
	StartupTime       float64
	TotalTime         float64
	ActualLoops       int
	TempReadBlocks    int
	TempWriteBlocks   int
}

func (node PlanNode) View(i int, ctx ProgramContext) string {

	var viewPosition Position
	if ctx.JoinView {
		viewPosition = node.JoinViewPosition
	} else {
		viewPosition = node.Position
	}

	var styles Styles
	if ctx.Cursor == i {
		styles = ctx.CursorStyle
	} else if ctx.SelectedNode.Position.Id == viewPosition.Parent {
		styles = ctx.ChildCursorStyle
	} else {
		styles = ctx.NormalStyle
	}

	var buf strings.Builder
	if ctx.DisplayNumbers {
		buf.WriteString(styles.Gutter.Render(fmt.Sprintf("%2d ", i+1)))
	} else {
		buf.WriteString("   ")
	}

	if ctx.DisplayParallel {
		if viewPosition.BelowGather {
			buf.WriteString(styles.Gutter.Render("┃┃ "))
		} else if node.IsGather {
			if node.Analyzed.LaunchedWorkers > 0 {
				buf.WriteString(styles.Workers.Render(fmt.Sprintf("%.2d ", node.Analyzed.LaunchedWorkers)))
			} else {
				buf.WriteString(styles.Workers.Render(" - "))
			}
		} else {
			buf.WriteString(styles.Everything.Render("   "))
		}
	}

	if ctx.Indent {
		buf.WriteString(styles.Everything.Render(strings.Repeat("  ", viewPosition.Level-1)))
	}

	if ctx.JoinView && node.RelationName != "" {
		buf.WriteString(styles.NodeName.Render(node.Name()))
		if ctx.DisplayRelations {
			buf.WriteString(styles.Relation.Render(" " + node.RelationName))
		}
	} else {
		buf.WriteString(styles.Workers.Render(node.label()))
		buf.WriteString(styles.NodeName.Render(node.Name()))
		if node.CteName != "" {
			buf.WriteString(styles.Relation.Render(" " + node.CteName))
		}
		if node.FunctionName != "" {
			buf.WriteString(styles.Relation.Render(" " + node.FunctionName))
		}
		if node.RelationName != "" && ctx.DisplayRelations {
			buf.WriteString(styles.Relation.Render(" " + node.RelationName))
		}
	}

	result := buf.String()

	needed := ctx.Width - ansi.StringWidth(result) - 2

	if ctx.StatDisplay == DisplayRows {
		buf.WriteString(node.rows(styles, needed, ctx))
	} else if ctx.StatDisplay == DisplayBuffers {
		buf.WriteString(node.buffers(styles, needed))
	} else if ctx.StatDisplay == DisplayCost {
		buf.WriteString(node.costs(styles, needed))
	} else if ctx.StatDisplay == DisplayTime {
		buf.WriteString(node.times(styles, needed))
	} else if ctx.StatDisplay == DisplayNothing {
		buf.WriteString(styles.Everything.Render(fmt.Sprintf("%*s", needed, "")))
	}

	buf.WriteString("\n")

	return buf.String()
}

func (node PlanNode) Display(ctx ProgramContext) bool {
	if ctx.JoinView {
		return node.JoinViewPosition.Display
	} else {
		return node.Position.Display
	}
}

func (node PlanNode) abbrevName() string {
	switch node.NodeType {
	case "Index Only Scan":
		return "IOS"
	case "Index Scan":
		return "IS"
	case "Seq Scan":
		return "SS"
	case "Bitmap Heap Scan":
		return "BHS"
	}
	return ""
}

func (node PlanNode) Name() string {
	if node.NodeType == "SetOp" {
		var strategy string
		if node.Strategy == "Hashed" {
			strategy = "Hash"
		}
		return strings.Trim(fmt.Sprintf("%s%s %s", strategy, "SetOp", node.Command), " ")
	}

	var joinType string
	var nodeName = node.NodeType
	if node.JoinType != "" && node.JoinType != "Inner" {
		joinType = node.JoinType + " Join"
		nodeName = strings.Replace(node.NodeType, " Join", "", 1)
	}
	if node.NodeType == "ModifyTable" {
		nodeName = node.Operation
	} else {
		nodeName = nodeName
	}
	return strings.ReplaceAll(strings.Trim(fmt.Sprintf("%s %s %s", node.PartialMode, nodeName, joinType), " "), "  ", " ")
}

func (node PlanNode) label() string {
	if node.SubPlanName != "" {
		return fmt.Sprintf("%s: ", node.SubPlanName)
	}
	return ""
}

func (node PlanNode) buffers(styles Styles, space int) string {
	var buf strings.Builder
	totalBuffers := formatUnderscores(node.Analyzed.SharedBuffersRead + node.Analyzed.SharedBuffersHit)
	readBuffers := formatUnderscores(node.Analyzed.SharedBuffersRead)

	if false {
		buf.WriteString(styles.Bracket.Render(" ["))
		buf.WriteString(styles.Everything.Render("total="))
		buf.WriteString(totalBuffers)
		buf.WriteString(styles.Everything.Render(" read="))
		buf.WriteString(readBuffers)
		buf.WriteString(styles.Bracket.Render("]"))
	} else {
		columns := fmt.Sprintf("%5s%15s", totalBuffers, readBuffers)
		buf.WriteString(styles.Value.Render(fmt.Sprintf("%*s", space, columns)))
	}

	return buf.String()
}

func (node PlanNode) costs(styles Styles, space int) string {
	startupCost := formatUnderscoresFloat(node.StartupCost)
	totalCost := formatUnderscoresFloat(node.TotalCost)

	var buf strings.Builder
	if false {
		buf.WriteString(styles.Bracket.Render(" ["))
		buf.WriteString(styles.Everything.Render("startup="))
		buf.WriteString(styles.Value.Render(startupCost))
		buf.WriteString(styles.Everything.Render(" total="))
		buf.WriteString(styles.Value.Render(totalCost))
		buf.WriteString(styles.Bracket.Render("]"))
	} else {
		columns := fmt.Sprintf("%5s%15s", startupCost, totalCost)
		buf.WriteString(styles.Value.Render(fmt.Sprintf("%*s", space, columns)))
	}

	return buf.String()
}

func (node PlanNode) times(styles Styles, space int) string {
	startupTime := formatUnderscoresFloat(node.Analyzed.StartupTime)
	totalTime := formatUnderscoresFloat(node.Analyzed.TotalTime)

	var buf strings.Builder
	if false {
		buf.WriteString(styles.Bracket.Render(" ["))
		buf.WriteString(styles.Everything.Render("startup="))
		buf.WriteString(styles.Value.Render(startupTime))
		buf.WriteString(styles.Everything.Render(" total="))
		buf.WriteString(styles.Value.Render(totalTime))
		buf.WriteString(styles.Bracket.Render("]"))
	} else {
		if node.ParentIsNestedLoop && node.ParentRelationship == "Inner" {
			totalTime = fmt.Sprintf("(%s)→%s", formatUnderscores(node.Analyzed.ActualLoops), totalTime)
		}
		columns := fmt.Sprintf("%5s%15s", startupTime, totalTime)
		buf.WriteString(styles.Value.Render(fmt.Sprintf("%*s", space, columns)))
	}

	return buf.String()
}

func (node PlanNode) rows(styles Styles, space int, ctx ProgramContext) string {

	separatedPlanRows := formatUnderscores(node.PlanRows)
	separatedActualRows := formatUnderscores(node.Analyzed.ActualRows)

	var buf strings.Builder

	if ctx.Analyzed {
		if node.ParentIsNestedLoop && node.ParentRelationship == "Inner" {
			separatedActualRows = fmt.Sprintf("(%s) → %s", formatUnderscores(node.Analyzed.ActualLoops), separatedActualRows)
		} else if node.Analyzed.ActualLoops > 1 {
			separatedActualRows = fmt.Sprintf("%s(%s)", separatedActualRows, formatUnderscores(node.Analyzed.ActualLoops))
		}
	} else {
		separatedActualRows = "- "
	}

	columns := fmt.Sprintf("%5s%15s", separatedPlanRows, separatedActualRows)
	buf.WriteString(styles.Value.Render(fmt.Sprintf("%*s", space, columns)))

	return buf.String()
}

func getRowStatus(percentOfActual float32, styles Styles) string {
	if percentOfActual < 10 {
		return styles.Warning.Render(fmt.Sprintf(" %.1f%%", percentOfActual))
	} else if percentOfActual < 50 {
		return styles.Caution.Render(fmt.Sprintf(" %.1f%%", percentOfActual))
	} else {
		return styles.Everything.Render(fmt.Sprintf(" %.1f%%", percentOfActual))
	}
}

func (node PlanNode) Content(ctx ProgramContext) string {
	if node.NodeType == "" {
		return "No Node Selected"
	}

	var buf strings.Builder

	if node.Analyzed.TempReadBlocks > 0 {
		buf.WriteString(ctx.DetailStyles.Label.Render("Temp Read Blocks: "))
		buf.WriteString(ctx.DetailStyles.Warning.Render(formatUnderscores(node.Analyzed.TempReadBlocks)))
		buf.WriteString("\n")
	}
	if node.Analyzed.TempWriteBlocks > 0 {
		buf.WriteString(ctx.DetailStyles.Label.Render("Temp Write Blocks: "))
		buf.WriteString(ctx.DetailStyles.Warning.Render(formatUnderscores(node.Analyzed.TempWriteBlocks)))
		buf.WriteString("\n")
	}
	if node.ParallelAware {
		buf.WriteString(ctx.DetailStyles.Label.Render("Parallel Aware: "))
		buf.WriteString(ctx.NormalStyle.Workers.Render("true"))
		buf.WriteString("\n")
	}
	if ctx.Analyzed {
		buf.WriteString(ctx.DetailStyles.Label.Render("Actual Loops: "))
		loops := formatUnderscores(node.Analyzed.ActualLoops)
		buf.WriteString(ctx.NormalStyle.Everything.Render(loops))
		if node.Analyzed.ActualLoops > 1 {
			if node.ParentIsNestedLoop {
				buf.WriteString(fmt.Sprintf(" Loops each returning an avg of %s rows", formatUnderscores(node.Analyzed.ActualRows)))
			} else {
				buf.WriteString(fmt.Sprintf(" Parallel workers each processing an avg of %s rows", formatUnderscores(node.Analyzed.ActualRows)))
			}
		}
		buf.WriteString("\n")
	}
	if node.RelationName != "" {
		buf.WriteString(ctx.DetailStyles.Label.Render("Relation Name: "))
		buf.WriteString(ctx.NormalStyle.Relation.Render(node.RelationName))
		buf.WriteString("\n")
	}
	if node.CteName != "" {
		buf.WriteString(ctx.DetailStyles.Label.Render("CTE Name: "))
		buf.WriteString(ctx.NormalStyle.Relation.Render(node.CteName))
		buf.WriteString("\n")
	}
	if node.FunctionName != "" {
		buf.WriteString(ctx.DetailStyles.Label.Render("Function Name: "))
		buf.WriteString(ctx.NormalStyle.Relation.Render(node.FunctionName))
		buf.WriteString("\n")
	}
	if node.IndexName != "" {
		buf.WriteString(ctx.DetailStyles.Label.Render("Index Name: "))
		buf.WriteString(ctx.NormalStyle.Everything.Render(node.IndexName))
		buf.WriteString("\n")
	}
	if node.IndexCond != "" {
		buf.WriteString(ctx.DetailStyles.Label.Render("Index Cond: "))
		buf.WriteString(ctx.NormalStyle.Everything.Render(node.IndexCond))
		buf.WriteString("\n")
	}
	if node.HashCond != "" {
		buf.WriteString(ctx.DetailStyles.Label.Render("Hash Cond: "))
		buf.WriteString(ctx.NormalStyle.Everything.Render(node.HashCond))
		buf.WriteString("\n")
	}
	if node.RecheckCond != "" {
		buf.WriteString(ctx.DetailStyles.Label.Render("Recheck Cond: "))
		buf.WriteString(ctx.NormalStyle.Everything.Render(node.RecheckCond))
		buf.WriteString("\n")
	}
	if node.GroupKey != nil {
		buf.WriteString(ctx.DetailStyles.Label.Render("Group Keys: "))
		buf.WriteString(ctx.NormalStyle.Everything.Render(strings.Join(node.GroupKey, ", ")))
		buf.WriteString("\n")
	}
	if node.PresortKeys != nil {
		buf.WriteString(ctx.DetailStyles.Label.Render("Presort Keys: "))
		buf.WriteString(ctx.NormalStyle.Everything.Render(strings.Join(node.PresortKeys, ", ")))
		buf.WriteString("\n")
	}
	if node.SortKeys != nil {
		buf.WriteString(ctx.DetailStyles.Label.Render("Sort Keys: "))
		buf.WriteString(ctx.NormalStyle.Everything.Render(strings.Join(node.SortKeys, ", ")))
		buf.WriteString("\n")
	}
	if node.Filter != "" {
		buf.WriteString(ctx.DetailStyles.Label.Render("Filter: "))
		buf.WriteString(ctx.NormalStyle.Everything.Render(node.Filter))
		buf.WriteString("\n")
	}
	if node.JoinFilter != "" {
		buf.WriteString(ctx.DetailStyles.Label.Render("Join Filter: "))
		buf.WriteString(ctx.NormalStyle.Everything.Render(node.JoinFilter))
		buf.WriteString("\n")
	}
	if node.PlanWidth > 0 {
		buf.WriteString(ctx.DetailStyles.Label.Render("Plan Width: "))
		buf.WriteString(ctx.NormalStyle.Everything.Render(formatUnderscores(node.PlanWidth)))
		buf.WriteString("\n")
	}

	return buf.String()
}
