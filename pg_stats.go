package main

import (
	"fmt"
	"strings"
	"time"
)

// PgColumnStat mirrors a subset of columns from Postgres's pg_stats view —
// the per-column planner statistics computed by ANALYZE, scoped (by the
// caller) to columns actually referenced in a plan's filter/join/sort/
// group conditions. Nested under a PgTableStats entry, so schema/table
// identity isn't repeated per column.
type PgColumnStat struct {
	AttName         string
	NullFrac        float64
	AvgWidth        int
	NDistinct       float64
	Correlation     *float64
	MostCommonVals  *string
	MostCommonFreqs *string
	HistogramBounds *string
}

// PgRelationStat mirrors the pg_class fields the planner's cost formulas
// scale against, for the table itself (nested under its PgTableStats entry).
type PgRelationStat struct {
	RelKind       string
	Reltuples     float64
	Relpages      int
	Relallvisible int
}

// PgIndexStat mirrors pg_class fields for an index belonging to the
// enclosing table.
type PgIndexStat struct {
	IndexName     string
	RelKind       string
	Reltuples     float64
	Relpages      int
	Relallvisible int
}

// PgExtendedStat mirrors a pg_statistic_ext/pg_statistic_ext_data entry —
// multivariate statistics created via CREATE STATISTICS.
type PgExtendedStat struct {
	StatName     string
	Columns      string
	NDistinct    *string
	Dependencies *string
	HasMCV       bool
}

// PgTableActivity mirrors staleness/trust fields from pg_stat_user_tables —
// not a cost-model input itself, but context for whether the captured
// stats above are trustworthy.
type PgTableActivity struct {
	NLiveTup        int64
	NDeadTup        int64
	LastVacuum      *time.Time
	LastAutovacuum  *time.Time
	LastAnalyze     *time.Time
	LastAutoanalyze *time.Time
}

// PgTableStats groups every captured catalog fact for a single table —
// its own pg_class row, relevant pg_stats columns, its indexes' pg_class
// rows, any extended statistics objects, and staleness/activity info — so
// a reader doesn't have to cross-reference separate lists by table name.
type PgTableStats struct {
	SchemaName string
	TableName  string
	Relation   *PgRelationStat
	Columns    []PgColumnStat
	Indexes    []PgIndexStat
	Extended   []PgExtendedStat
	Activity   *PgTableActivity
}

// PgStatsSnapshot bundles all captured planner-statistics catalog data for
// a single live EXPLAIN/EXPLAIN ANALYZE run, grouped by table.
type PgStatsSnapshot struct {
	Tables []PgTableStats
}

func (s PgStatsSnapshot) IsEmpty() bool {
	return len(s.Tables) == 0
}

// String renders the snapshot as a plain-text report, one section per
// table, meant to be read directly rather than parsed back.
func (s PgStatsSnapshot) String() string {
	var buf strings.Builder
	for i, t := range s.Tables {
		if i > 0 {
			buf.WriteString("\n")
		}
		t.writeTo(&buf)
	}
	return strings.TrimRight(buf.String(), "\n")
}

const tableDividerWidth = 60

func tableDivider(label string) string {
	padded := " " + label + " "
	fill := tableDividerWidth - len(padded)
	if fill < 4 {
		fill = 4
	}
	left := fill / 2
	right := fill - left
	return strings.Repeat("=", left) + padded + strings.Repeat("=", right)
}

const columnDividerWidth = 40

func columnDivider(label string) string {
	padded := " " + label + " "
	fill := columnDividerWidth - len(padded)
	if fill < 4 {
		fill = 4
	}
	left := fill / 2
	right := fill - left
	return strings.Repeat("-", left) + padded + strings.Repeat("-", right)
}

func (t PgTableStats) writeTo(buf *strings.Builder) {
	fmt.Fprintf(buf, "%s\n\n", tableDivider(t.SchemaName+"."+t.TableName))

	if t.Relation != nil {
		writeAttrTopLevel(buf, "tuples", formatUnderscores(int(t.Relation.Reltuples)))
		writeAttrTopLevel(buf, "pages", formatUnderscores(t.Relation.Relpages))
		writeAttrTopLevel(buf, "visible", formatUnderscores(t.Relation.Relallvisible))
	}

	if t.Activity != nil {
		writeAttrTopLevel(buf, "live", formatUnderscores(int(t.Activity.NLiveTup)))
		writeAttrTopLevel(buf, "dead", formatUnderscores(int(t.Activity.NDeadTup)))
		writeAttrTopLevel(buf, "last_vacuum", formatTimePtr(t.Activity.LastVacuum))
		writeAttrTopLevel(buf, "last_autovacuum", formatTimePtr(t.Activity.LastAutovacuum))
		writeAttrTopLevel(buf, "last_analyze", formatTimePtr(t.Activity.LastAnalyze))
		writeAttrTopLevel(buf, "last_autoanalyze", formatTimePtr(t.Activity.LastAutoanalyze))
	}

	if len(t.Columns) > 0 {
		buf.WriteString("\nCOLUMNS\n")
		for i, c := range t.Columns {
			if i > 0 {
				buf.WriteString("\n")
			}
			fmt.Fprintf(buf, "  %s\n", columnDivider(c.AttName))
			writeAttr(buf, "n_distinct", formatFloat(c.NDistinct))
			writeAttr(buf, "avg_width", formatUnderscores(c.AvgWidth))
			writeAttr(buf, "correlation", formatCorrelation(c.Correlation))
			writeAttr(buf, "null_frac", formatFloat(c.NullFrac))
			// Variable-length arrays get their own line rather than a table
			// column — a single wide value (e.g. 50 most-common values)
			// would otherwise stretch every row's alignment.
			if c.MostCommonVals != nil {
				writeAttr(buf, "most_common_vals", *c.MostCommonVals)
			}
			if c.MostCommonFreqs != nil {
				writeAttr(buf, "most_common_freqs", *c.MostCommonFreqs)
			}
			if c.HistogramBounds != nil {
				writeAttr(buf, "histogram_bounds", *c.HistogramBounds)
			}
		}
	}

	if len(t.Indexes) > 0 {
		buf.WriteString("\nINDEXES\n")
		for i, idx := range t.Indexes {
			if i > 0 {
				buf.WriteString("\n")
			}
			fmt.Fprintf(buf, "  %s\n", columnDivider(idx.IndexName))
			writeAttr(buf, "tuples", formatUnderscores(int(idx.Reltuples)))
			writeAttr(buf, "pages", formatUnderscores(idx.Relpages))
			writeAttr(buf, "visible", formatUnderscores(idx.Relallvisible))
		}
	}

	if len(t.Extended) > 0 {
		buf.WriteString("\nEXTENDED STATS\n")
		for _, e := range t.Extended {
			fmt.Fprintf(buf, "  %s on (%s):\n", e.StatName, e.Columns)
			writeAttr(buf, "has_mcv", fmt.Sprintf("%v", e.HasMCV))
			if e.NDistinct != nil {
				writeAttr(buf, "ndistinct", *e.NDistinct)
			}
			if e.Dependencies != nil {
				writeAttr(buf, "dependencies", *e.Dependencies)
			}
		}
	}

}

// writeAttr writes a single "label: value" line, aligned so values line up
// regardless of label length, nested under a divider/header line.
func writeAttr(buf *strings.Builder, label, value string) {
	writeAttrIndent(buf, "    ", label, value)
}

// writeAttrTopLevel writes a "label: value" line with no indentation, for
// attributes that sit directly under the table divider rather than nested
// under a section header.
func writeAttrTopLevel(buf *strings.Builder, label, value string) {
	writeAttrIndent(buf, "", label, value)
}

func writeAttrIndent(buf *strings.Builder, indent, label, value string) {
	fmt.Fprintf(buf, "%s%-19s%s\n", indent, label+":", value)
}

func formatFloat(f float64) string {
	return fmt.Sprintf("%g", f)
}

// formatCorrelation rounds to 4 decimal places — correlation is a
// statistical estimate in [-1, 1], not a value where full float precision
// carries any meaning.
func formatCorrelation(f *float64) string {
	if f == nil {
		return "-"
	}
	return fmt.Sprintf("%.4f", *f)
}

func formatTimePtr(t *time.Time) string {
	if t == nil {
		return "-"
	}
	return t.Format("2006-01-02 15:04:05")
}
