package main

import (
	"encoding/json"
	"fmt"
	"os"
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

func decodeJson(data string) (map[string]interface{}, float64, bool) {
	var decoded any

	err := json.Unmarshal([]byte(data), &decoded)

	if err != nil {
		fmt.Fprintln(os.Stderr, "Error parsing json:", err)
		os.Exit(1)
	}

	planJson, ok := decoded.([]interface{})
	if !ok && len(planJson) != 1 {
		fmt.Fprintf(os.Stderr, "Unexpected value in json, expected array: %v\n", decoded)
		os.Exit(1)
	}

	planObject, ok := planJson[0].(map[string]interface{})
	if !ok {
		fmt.Fprintf(os.Stderr, "Unexpected value in json, expected object: %v\n", planJson[0])
		os.Exit(1)
	}

	plan, ok := planObject["Plan"].(map[string]interface{})
	if !ok {
		fmt.Fprintf(os.Stderr, "Unexpected value in json, expected 'Plan' attribute: %v\n", planObject)
		os.Exit(1)
	}

	executionTime, analyzed := planObject["Execution Time"].(float64)

	return plan, executionTime, analyzed
}

func extractPlanNodes(plan map[string]interface{}, parentPosition Position, parentJoinPosition Position, parseContext ParseContext) PlanNode {
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

	var groupkeys []string
	groupkeyI, ok := plan["Group Key"].([]interface{})
	if ok {
		for _, gi := range groupkeyI {
			groupkeys = append(groupkeys, gi.(string))
		}
	}

	var sortkeys []string
	sortkeyI, ok := plan["Sort Key"].([]interface{})
	if ok {
		for _, gi := range sortkeyI {
			sortkeys = append(sortkeys, gi.(string))
		}
	}

	var presortedkeys []string
	presortedkeyI, ok := plan["Presorted Key"].([]interface{})
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
	if isGather {
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
		for _, plan := range plans.([]interface{}) {
			if plan != nil {
				extractPlanNodes(
					plan.(map[string]interface{}),
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
