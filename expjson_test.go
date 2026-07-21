package main

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConvertNoBuffers(t *testing.T) {
	data, err := os.ReadFile("./testdata/analyze_no_buffers.json")
	if err != nil {
		t.Fatal(err)
	}
	plan := Convert(string(data))
	assert.Equal(t, plan.executionTime, 69.662)
}

func TestConvertNoWriteBuffers(t *testing.T) {
	data, err := os.ReadFile("./testdata/analyze_buffers.json")
	if err != nil {
		t.Fatal(err)
	}
	plan := Convert(string(data))
	assert.Equal(t, plan.nodes[0].Analyzed.TempWriteBlocks, 0)
}

func TestSubPlanNameProperty(t *testing.T) {
	data, err := os.ReadFile("./testdata/subplanname.json")
	if err != nil {
		t.Fatal(err)
	}
	plan := Convert(string(data))
	assert.Equal(t, plan.nodes[1].SubPlanName, "SubPlan 1")
}

func TestJoinTypeProperty(t *testing.T) {
	data, err := os.ReadFile("./testdata/jointype.json")
	if err != nil {
		t.Fatal(err)
	}
	plan := Convert(string(data))
	assert.Equal(t, plan.nodes[0].JoinType, "Semi")
}

func TestJoinFilterProperty(t *testing.T) {
	data, err := os.ReadFile("./testdata/jointype.json")
	if err != nil {
		t.Fatal(err)
	}
	plan := Convert(string(data))
	assert.Equal(t, plan.nodes[0].JoinFilter, "x = 1 and y = 2")
}

func TestOperationProperty(t *testing.T) {
	data, err := os.ReadFile("./testdata/merge.json")
	if err != nil {
		t.Fatal(err)
	}
	plan := Convert(string(data))
	assert.Equal(t, plan.nodes[0].Operation, "Merge")
}

func TestTIDCondProperty(t *testing.T) {
	data, err := os.ReadFile("./testdata/tidcond.json")
	if err != nil {
		t.Fatal(err)
	}
	plan := Convert(string(data))
	assert.Equal(t, plan.nodes[0].TidCond, "(ctid = '(0,1)'::tid)")
}

func TestTableFuncNameProperty(t *testing.T) {
	data, err := os.ReadFile("./testdata/tablefunctionscan.json")
	if err != nil {
		t.Fatal(err)
	}
	plan := Convert(string(data))
	assert.Equal(t, plan.nodes[0].TableFunctionName, "xmltable")
}

func TestCTENAMEProperty(t *testing.T) {
	data, err := os.ReadFile("./testdata/ctename.json")
	if err != nil {
		t.Fatal(err)
	}
	plan := Convert(string(data))
	assert.Equal(t, plan.nodes[0].CteName, "source")
}

func TestStrategyProperty(t *testing.T) {
	data, err := os.ReadFile("./testdata/strategy.json")
	if err != nil {
		t.Fatal(err)
	}
	plan := Convert(string(data))
	assert.Equal(t, plan.nodes[0].NodeType, "SetOp")
	assert.Equal(t, plan.nodes[0].Strategy, "Hashed")
	assert.Equal(t, plan.nodes[0].Command, "Except")
}

func TestFunctionNameProperty(t *testing.T) {
	data, err := os.ReadFile("./testdata/functionname.json")
	if err != nil {
		t.Fatal(err)
	}
	plan := Convert(string(data))
	assert.Equal(t, plan.nodes[0].FunctionName, "generate_series")
}

func TestIncrementalSortProperty(t *testing.T) {
	data, err := os.ReadFile("./testdata/incrementalsort.json")
	if err != nil {
		t.Fatal(err)
	}
	plan := Convert(string(data))
	assert.Equal(t, []string{"x"}, plan.nodes[0].PresortKeys)
	assert.Equal(t, []string{"x", "y"}, plan.nodes[0].SortKeys)
}

func TestParallelAwareProperty(t *testing.T) {
	data, err := os.ReadFile("./testdata/analyze_no_buffers.json")
	if err != nil {
		t.Fatal(err)
	}
	plan := Convert(string(data))
	assert.Equal(t, true, plan.nodes[3].ParallelAware)
}
