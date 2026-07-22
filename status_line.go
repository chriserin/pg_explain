package main

import (
	"fmt"
	"strings"
	"time"
)

type StatusLine struct {
	ExecutionTime float64
	TotalBuffers  int
	TotalRows     int
}

func NewStatusLine(explainPlan ExplainPlan) StatusLine {
	return StatusLine{
		ExecutionTime: explainPlan.executionTime,
		TotalBuffers:  explainPlan.TotalBuffers(),
		TotalRows:     explainPlan.TotalRows(),
	}
}

func (s StatusLine) View(m Model) string {
	styles := m.uiState.StatusStyles

	if m.uiState.Width < 30 {
		return ""
	}

	var buf strings.Builder

	buf.WriteString(styles.AltNormal.Render("  "))
	buf.WriteString(styles.Normal.Render(""))
	buf.WriteString(styles.Normal.Render(" Time:"))
	buf.WriteString(styles.Value.Render(" %.3fms "))
	buf.WriteString(styles.AltNormal.Render(""))
	buf.WriteString(styles.Normal.Render(""))
	buf.WriteString(styles.Normal.Render(" Buffers:"))
	buf.WriteString(styles.Value.Render(" %s "))
	buf.WriteString(styles.AltNormal.Render(""))
	buf.WriteString(styles.Normal.Render(""))
	buf.WriteString(styles.Normal.Render(" Rows:"))
	buf.WriteString(styles.Value.Render(" %s "))
	buf.WriteString(styles.AltNormal.Render(" "))

	var executionTime float64
	if m.loading {
		executionTime = float64(int64(m.stopwatch.Elapsed() / time.Millisecond))
	} else {
		executionTime = s.ExecutionTime
	}
	result := fmt.Sprintf(buf.String(),
		executionTime,
		formatUnderscores(s.TotalBuffers),
		formatUnderscores(s.TotalRows),
	)

	return result
}
