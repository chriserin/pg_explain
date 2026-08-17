package main

import (
	"bytes"
	"os"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/acarl005/stripansi"
	"github.com/stretchr/testify/assert"
)

const (
	ROW_HEADER = iota
	ROW_STATUS
	ROW_EX_NODE_1
	ROW_EX_NODE_2
	ROW_EX_NODE_3
	ROW_EX_NODE_4
	ROW_BLANK
	ROW_DETAILS_TITLE
)

func TestStdinSource(t *testing.T) {

	var buf bytes.Buffer

	dat, err := os.ReadFile("./testdata/analyze_no_buffers.json")
	if err != nil {
		t.Fatal(err)
	}
	source := Source{sourceType: SOURCE_STDIN, input: string(dat)}
	p := RunProgram(source, RunExplainAnalyze, tea.WithOutput(&buf))

	cmds := []tea.Cmd{
		func() tea.Msg { return tea.WindowSizeMsg{Width: 80, Height: 24} },
		tea.Quit,
	}
	seq := tea.Sequence(cmds...)
	go p.Send(seq())

	if _, err := p.Run(); err != nil {
		t.Fatal(err)
	}

	rendered := strings.Split(buf.String(), "\n")

	assert.Contains(t, stripansi.Strip(rendered[ROW_HEADER]), "STDIN")
	assert.Contains(t, stripansi.Strip(rendered[ROW_STATUS]), "Time: 69.662ms")
	assert.Contains(t, stripansi.Strip(rendered[ROW_EX_NODE_1]), "Finalize Aggregate")
	assert.Contains(t, stripansi.Strip(rendered[ROW_DETAILS_TITLE]), "Details  Finalize Aggregate")
}

func TestCancelQueryKeyCancelsRunningQuery(t *testing.T) {
	t.Chdir(t.TempDir())

	m := InitModel(Source{sourceType: SOURCE_STDIN}, RunNothing)

	called := false
	m.explainCancelFn = func() { called = true }
	m.loading = true
	m.queryRun = QueryRun{query: "select 1", result: "some result"}

	updated, cmd := m.Update(tea.KeyPressMsg{Text: "C"})
	updatedModel := updated.(Model)

	assert.True(t, called, "expected the running query's cancel function to be called")
	assert.NotNil(t, cmd, "expected the stopwatch to be stopped and reset")
	assert.Nil(t, updatedModel.explainCancelFn, "explainCancelFn should be cleared after cancellation")
	assert.False(t, updatedModel.loading, "loading should be cleared after cancellation")

	entries, err := os.ReadDir("_pgex")
	if assert.NoError(t, err) {
		assert.Len(t, entries, 1, "expected the query run to be saved to the _pgex dir")
	}
}

func TestCancelQueryKeyNoOpWhenNoRunningQuery(t *testing.T) {
	m := InitModel(Source{sourceType: SOURCE_STDIN}, RunNothing)
	assert.Nil(t, m.explainCancelFn)

	assert.NotPanics(t, func() {
		_, cmd := m.Update(tea.KeyPressMsg{Text: "C"})
		assert.Nil(t, cmd)
	})
}
