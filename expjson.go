package main

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"slices"
	"strings"
)

type ExplainPlan struct {
	nodes         []PlanNode
	analyzed      bool
	executionTime float64
}

func (ep ExplainPlan) TotalBuffers() int {
	return ep.nodes[0].Analyzed.SharedBuffersHit + ep.nodes[0].Analyzed.SharedBuffersRead
}

func (ep ExplainPlan) TotalRows() int {
	return ep.nodes[0].Analyzed.ActualRows
}

// RelationNames returns the deduplicated, sorted set of non-empty
// RelationName values across all nodes in the plan.
func (ep ExplainPlan) RelationNames() []string {
	return collectNonEmpty(ep.nodes, func(n PlanNode) string { return n.RelationName })
}

// IndexNames returns the deduplicated, sorted set of non-empty
// IndexName values across all nodes in the plan.
func (ep ExplainPlan) IndexNames() []string {
	return collectNonEmpty(ep.nodes, func(n PlanNode) string { return n.IndexName })
}

func collectNonEmpty(nodes []PlanNode, extract func(PlanNode) string) []string {
	seen := make(map[string]struct{})
	for _, node := range nodes {
		if v := extract(node); v != "" {
			seen[v] = struct{}{}
		}
	}
	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

var stringLiteralPattern = regexp.MustCompile(`'[^']*'`)
var castSuffixPattern = regexp.MustCompile(`::[A-Za-z_][A-Za-z0-9_]*(\[\])?`)
var identifierPattern = regexp.MustCompile(`[A-Za-z_][A-Za-z0-9_$]*`)

var conditionKeywords = map[string]struct{}{
	"AND": {}, "OR": {}, "NOT": {}, "IS": {}, "NULL": {}, "TRUE": {}, "FALSE": {},
	"ANY": {}, "ALL": {}, "IN": {}, "LIKE": {}, "ILIKE": {}, "BETWEEN": {}, "EXISTS": {},
	"CASE": {}, "WHEN": {}, "THEN": {}, "ELSE": {}, "END": {}, "ASC": {}, "DESC": {},
	"NULLS": {}, "FIRST": {}, "LAST": {}, "ARRAY": {}, "DISTINCT": {},
}

// extractIdentifiers pulls candidate column-name tokens out of a Postgres
// deparsed condition/key fragment. It is a heuristic (regex-based, not a
// real SQL parser): it may over-include tokens (function names, table
// qualifiers) that don't correspond to real columns, but those are dropped
// later when matched against actual column names in the database.
func extractIdentifiers(s string) []string {
	s = stringLiteralPattern.ReplaceAllString(s, " ")
	s = castSuffixPattern.ReplaceAllString(s, "")
	var out []string
	for _, m := range identifierPattern.FindAllString(s, -1) {
		if _, isKeyword := conditionKeywords[strings.ToUpper(m)]; !isKeyword {
			out = append(out, m)
		}
	}
	return out
}

// RelevantColumns returns the deduplicated, sorted set of candidate column
// names referenced across every node's Filter/IndexCond/JoinFilter/
// HashCond/RecheckCond/TidCond/GroupKey/SortKeys/PresortKeys fields — the
// fields that actually feed the planner's selectivity estimates.
func (ep ExplainPlan) RelevantColumns() []string {
	seen := make(map[string]struct{})
	for _, node := range ep.nodes {
		for _, s := range []string{node.Filter, node.IndexCond, node.JoinFilter,
			node.HashCond, node.RecheckCond, node.TidCond} {
			for _, id := range extractIdentifiers(s) {
				seen[id] = struct{}{}
			}
		}
		for _, keys := range [][]string{node.GroupKey, node.SortKeys, node.PresortKeys} {
			for _, k := range keys {
				for _, id := range extractIdentifiers(k) {
					seen[id] = struct{}{}
				}
			}
		}
	}
	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

type ParseContext struct {
	Id               *int
	Nodes            *[]PlanNode
	BelowGather      bool
	ParentNestedLoop bool
	Analyzed         bool
	HasBuffers       bool
}

func Convert(explainJson string) ExplainPlan {
	decoded, executionTime, analyzed := decodeJson(explainJson)
	nodes := make([]PlanNode, 0, 1)
	id := 0

	_, hasBuffers := decoded["Shared Read Blocks"]

	extractPlanNodes(decoded,
		Position{Id: 0, Level: 0, Parent: 0},
		Position{Id: 0, Level: 0, Parent: 0},
		ParseContext{Id: &id, Nodes: &nodes, Analyzed: analyzed, HasBuffers: hasBuffers},
	)

	return ExplainPlan{
		nodes:         nodes,
		analyzed:      analyzed,
		executionTime: executionTime,
	}
}

func decodeJson(data string) (map[string]any, float64, bool) {
	var decoded any

	err := json.Unmarshal([]byte(data), &decoded)

	if err != nil {
		fmt.Fprintln(os.Stderr, "Error parsing json:", err)
		os.Exit(1)
	}

	planJson, ok := decoded.([]any)
	if !ok && len(planJson) != 1 {
		fmt.Fprintf(os.Stderr, "Unexpected value in json, expected array: %v\n", decoded)
		os.Exit(1)
	}

	planObject, ok := planJson[0].(map[string]any)
	if !ok {
		fmt.Fprintf(os.Stderr, "Unexpected value in json, expected object: %v\n", planJson[0])
		os.Exit(1)
	}

	plan, ok := planObject["Plan"].(map[string]any)
	if !ok {
		fmt.Fprintf(os.Stderr, "Unexpected value in json, expected 'Plan' attribute: %v\n", planObject)
		os.Exit(1)
	}

	executionTime, analyzed := planObject["Execution Time"].(float64)

	return plan, executionTime, analyzed
}

func extractPlanNodes(plan map[string]any, parentPosition Position, parentJoinPosition Position, parseContext ParseContext) PlanNode {
	nodeType := plan["Node Type"].(string)
	planRows := plan["Plan Rows"].(float64)

	parallelAware := plan["Parallel Aware"].(bool)

	partialMode, ok := plan["Partial Mode"].(string)
	if !ok {
		partialMode = ""
	}

	relationName, ok := plan["Relation Name"].(string)

	if !ok {
		relationName = ""
	}

	indexName, ok := plan["Index Name"].(string)
	if !ok {
		indexName = ""
	}

	indexCond, ok := plan["Index Cond"].(string)
	if !ok {
		indexCond = ""
	}

	filter, ok := plan["Filter"].(string)
	if !ok {
		filter = ""
	}

	joinFilter, ok := plan["Join Filter"].(string)
	if !ok {
		joinFilter = ""
	}

	strategy, ok := plan["Strategy"].(string)
	if !ok {
		strategy = ""
	}

	command, ok := plan["Command"].(string)
	if !ok {
		command = ""
	}

	ctename, ok := plan["CTE Name"].(string)
	if !ok {
		ctename = ""
	}

	functionName, ok := plan["Function Name"].(string)
	if !ok {
		functionName = ""
	}

	tablefunctionname, ok := plan["Table Function Name"].(string)
	if !ok {
		tablefunctionname = ""
	}

	tidcond, ok := plan["TID Cond"].(string)
	if !ok {
		tidcond = ""
	}

	operation, ok := plan["Operation"].(string)
	if !ok {
		operation = ""
	}

	jointype, ok := plan["Join Type"].(string)
	if !ok {
		jointype = ""
	}

	subplanname, ok := plan["Subplan Name"].(string)
	if !ok {
		subplanname = ""
	}

	hashcond, ok := plan["Hash Cond"].(string)
	if !ok {
		hashcond = ""
	}
	recheckcond, ok := plan["Recheck Cond"].(string)
	if !ok {
		recheckcond = ""
	}

	var groupkeys []string
	groupkeyI, ok := plan["Group Key"].([]any)
	if ok {
		for _, gi := range groupkeyI {
			groupkeys = append(groupkeys, gi.(string))
		}
	}

	var sortkeys []string
	sortkeyI, ok := plan["Sort Key"].([]any)
	if ok {
		for _, gi := range sortkeyI {
			sortkeys = append(sortkeys, gi.(string))
		}
	}

	var presortedkeys []string
	presortedkeyI, ok := plan["Presorted Key"].([]any)
	if ok {
		for _, gi := range presortedkeyI {
			presortedkeys = append(presortedkeys, gi.(string))
		}
	}

	planWidth := plan["Plan Width"].(float64)

	parentRelationship, ok := plan["Parent Relationship"].(string)
	if !ok {
		parentRelationship = ""
	}

	startupCost := plan["Startup Cost"].(float64)
	totalCost := plan["Total Cost"].(float64)
	workersPlanned, ok := plan["Workers Planned"].(float64)

	plans := plan["Plans"]

	id := parseContext.Id
	*id = *id + 1

	isGather := strings.Contains(nodeType, "Gather")

	var workersPlannedInt int
	if isGather && ok {
		workersPlannedInt = int(workersPlanned) + 1
	} else {
		workersPlannedInt = 0
	}

	newPosition := Position{
		Id:          *id,
		Level:       parentPosition.Level + 1,
		Parent:      parentPosition.Id,
		Display:     true,
		BelowGather: parseContext.BelowGather,
	}

	var joinViewPosition Position
	if isJoinType(nodeType) || relationName != "" || isGather {
		joinViewPosition = Position{
			Id:          *id,
			Level:       parentJoinPosition.Level + 1,
			Parent:      parentJoinPosition.Id,
			Display:     true,
			BelowGather: parseContext.BelowGather,
		}
	} else {
		joinViewPosition = parentJoinPosition
		joinViewPosition.Display = false
	}

	extractedNode := PlanNode{
		NodeType:           nodeType,
		PlanRows:           int(planRows),
		PartialMode:        partialMode,
		ParallelAware:      parallelAware,
		Position:           newPosition,
		JoinViewPosition:   joinViewPosition,
		RelationName:       relationName,
		IsGather:           isGather,
		StartupCost:        startupCost,
		TotalCost:          totalCost,
		PlannedWorkers:     workersPlannedInt,
		IndexName:          indexName,
		IndexCond:          indexCond,
		Filter:             filter,
		JoinFilter:         joinFilter,
		HashCond:           hashcond,
		RecheckCond:        recheckcond,
		GroupKey:           groupkeys,
		SortKeys:           sortkeys,
		PresortKeys:        presortedkeys,
		ParentRelationship: parentRelationship,
		ParentIsNestedLoop: parseContext.ParentNestedLoop,
		PlanWidth:          int(planWidth),
		Strategy:           strategy,
		Command:            command,
		CteName:            ctename,
		FunctionName:       functionName,
		TableFunctionName:  tablefunctionname,
		TidCond:            tidcond,
		Operation:          operation,
		JoinType:           jointype,
		SubPlanName:        subplanname,
	}

	if parseContext.Analyzed {
		actualRows := plan["Actual Rows"].(float64)
		startupTime := plan["Actual Startup Time"].(float64)
		totalTime := plan["Actual Total Time"].(float64)
		workersLaunched, _ := plan["Workers Launched"].(float64)
		actualLoops := plan["Actual Loops"].(float64)

		var workersLaunchedInt int
		if isGather {
			workersLaunchedInt = int(workersLaunched) + 1
		} else {
			workersLaunchedInt = 0
		}

		analyzed := Analyzed{
			LaunchedWorkers: workersLaunchedInt,
			StartupTime:     startupTime,
			TotalTime:       totalTime,
			ActualLoops:     int(actualLoops),
			ActualRows:      int(actualRows),
		}

		if parseContext.HasBuffers {
			tempReadBlocks := plan["Temp Read Blocks"].(float64)
			tempWriteBlocks := plan["Temp Written Blocks"].(float64)
			sharedReadBlocks := plan["Shared Read Blocks"].(float64)
			sharedHitBlocks := plan["Shared Hit Blocks"].(float64)
			analyzed.TempReadBlocks = int(tempReadBlocks)
			analyzed.TempWriteBlocks = int(tempWriteBlocks)
			analyzed.SharedBuffersHit = int(sharedHitBlocks)
			analyzed.SharedBuffersRead = int(sharedReadBlocks)
		}

		extractedNode.Analyzed = analyzed
	}

	nodes := parseContext.Nodes

	*nodes = append(*nodes, extractedNode)

	newParseContext := ParseContext{
		Id:               id,
		Nodes:            nodes,
		BelowGather:      isGather || parseContext.BelowGather,
		ParentNestedLoop: nodeType == "Nested Loop",
		Analyzed:         parseContext.Analyzed,
		HasBuffers:       parseContext.HasBuffers,
	}

	if plans != nil {
		for _, plan := range plans.([]any) {
			if plan != nil {
				extractPlanNodes(
					plan.(map[string]any),
					newPosition,
					joinViewPosition,
					newParseContext,
				)
			}
		}
	}

	return extractedNode
}

func isJoinType(nodeType string) bool {
	return slices.Contains([]string{"Nested Loop", "Hash Join", "Merge Join"}, nodeType)
}
