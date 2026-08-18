package main

import (
	"context"
	"slices"
	"strings"

	pgx "github.com/jackc/pgx/v5"
)

type Connection struct {
	conn       *pgx.Conn
	connConfig pgx.ConnConfig
}

func (c *Connection) Connect() error {
	conn, err := pgx.ConnectConfig(context.Background(), &c.connConfig)
	if err != nil {
		return err
	}
	c.conn = conn
	return nil
}

func (c Connection) SetSetting(setting Setting) error {
	settingSql := setting.Sql()
	_, err := c.conn.Exec(context.Background(), settingSql)
	return err
}

func (c Connection) ExecuteExplain(query string, ctx context.Context) (string, error) {

	var explainResult string
	err := c.conn.QueryRow(ctx, query).Scan(&explainResult)
	if err != nil {
		return "", err
	}

	return explainResult, nil
}

func (c Connection) Close() error {
	return c.conn.Close(context.Background())
}

var allowedSettings = []string{"work_mem", "join_collapse_limit", "max_parallel_workers_per_gather", "random_page_cost", "effective_cache_size", "cluster_name"}

func (c Connection) ShowAll() ([]Setting, error) {
	rows, err := c.conn.Query(context.Background(), "show all")

	if err != nil {
		return nil, err
	}

	result := make([]Setting, 0, len(allowedSettings))
	for rows.Next() {
		var name, setting, description string

		err = rows.Scan(&name, &setting, &description)
		if err != nil {
			return nil, err
		}

		if slices.Contains(allowedSettings, name) {
			result = append(result, Setting{name: name, setting: setting})
		}
	}

	return result, nil
}

// FetchPgStats gathers Postgres catalog statistics relevant to the cost
// model for the given tables, indexes, and columns, grouped by table so a
// reader isn't left cross-referencing separate lists by name.
// tableNames/indexNames come from PlanNode.RelationName/IndexName;
// columnNames from ExplainPlan.RelevantColumns(). Matching is by bare
// (unqualified) name, since EXPLAIN JSON never schema-qualifies
// relation/column names.
func (c Connection) FetchPgStats(tableNames, indexNames, columnNames []string) (PgStatsSnapshot, error) {
	groups := make(map[string]*PgTableStats)
	group := func(schema, table string) *PgTableStats {
		key := schema + "." + table
		g, ok := groups[key]
		if !ok {
			g = &PgTableStats{SchemaName: schema, TableName: table}
			groups[key] = g
		}
		return g
	}

	if len(tableNames) > 0 && len(columnNames) > 0 {
		// Primary key columns are excluded: null_frac is always 0, n_distinct
		// is always -1, and there's no MCV skew to explain, since every value
		// is unique — capturing them is noise, not diagnostic signal.
		rows, err := c.conn.Query(context.Background(),
			`SELECT ps.schemaname, ps.tablename, ps.attname, ps.null_frac, ps.avg_width, ps.n_distinct,
			        ps.correlation, ps.most_common_vals::text, ps.most_common_freqs::text,
			        ps.histogram_bounds::text
			 FROM pg_stats ps
			 WHERE ps.tablename = ANY($1) AND ps.attname = ANY($2)
			   AND NOT EXISTS (
			     SELECT 1
			     FROM pg_index i
			     JOIN pg_class c ON c.oid = i.indrelid
			     JOIN pg_namespace n ON n.oid = c.relnamespace
			     JOIN pg_attribute a ON a.attrelid = i.indrelid AND a.attnum = ANY(i.indkey)
			     WHERE i.indisprimary
			       AND c.relname = ps.tablename
			       AND n.nspname = ps.schemaname
			       AND a.attname = ps.attname
			   )`,
			tableNames, columnNames)
		if err != nil {
			return PgStatsSnapshot{}, err
		}
		for rows.Next() {
			var schema, table string
			var stat PgColumnStat
			if err := rows.Scan(&schema, &table, &stat.AttName,
				&stat.NullFrac, &stat.AvgWidth, &stat.NDistinct,
				&stat.Correlation, &stat.MostCommonVals, &stat.MostCommonFreqs,
				&stat.HistogramBounds); err != nil {
				return PgStatsSnapshot{}, err
			}
			g := group(schema, table)
			g.Columns = append(g.Columns, stat)
		}
	}

	if len(tableNames) > 0 {
		activityRows, err := c.conn.Query(context.Background(),
			`SELECT schemaname, relname, n_live_tup, n_dead_tup,
			        last_vacuum, last_autovacuum, last_analyze, last_autoanalyze
			 FROM pg_stat_user_tables
			 WHERE relname = ANY($1)`,
			tableNames)
		if err != nil {
			return PgStatsSnapshot{}, err
		}
		for activityRows.Next() {
			var schema, table string
			var activity PgTableActivity
			if err := activityRows.Scan(&schema, &table,
				&activity.NLiveTup, &activity.NDeadTup, &activity.LastVacuum,
				&activity.LastAutovacuum, &activity.LastAnalyze,
				&activity.LastAutoanalyze); err != nil {
				return PgStatsSnapshot{}, err
			}
			g := group(schema, table)
			g.Activity = &activity
		}

		extRows, err := c.conn.Query(context.Background(),
			`SELECT n.nspname, c.relname, se.stxname,
			        array_to_string(ARRAY(
			          SELECT a.attname FROM pg_attribute a
			          WHERE a.attrelid = se.stxrelid AND a.attnum = ANY(se.stxkeys)
			        ), ', ') AS columns,
			        sed.stxdndistinct::text, sed.stxddependencies::text,
			        sed.stxdmcv IS NOT NULL AS has_mcv
			 FROM pg_statistic_ext se
			 JOIN pg_class c ON c.oid = se.stxrelid
			 JOIN pg_namespace n ON n.oid = c.relnamespace
			 LEFT JOIN pg_statistic_ext_data sed ON sed.stxoid = se.oid
			 WHERE c.relname = ANY($1)`,
			tableNames)
		if err != nil {
			return PgStatsSnapshot{}, err
		}
		for extRows.Next() {
			var schema, table string
			var ext PgExtendedStat
			if err := extRows.Scan(&schema, &table, &ext.StatName,
				&ext.Columns, &ext.NDistinct, &ext.Dependencies, &ext.HasMCV); err != nil {
				return PgStatsSnapshot{}, err
			}
			g := group(schema, table)
			g.Extended = append(g.Extended, ext)
		}
	}

	allRelNames := make([]string, 0, len(tableNames)+len(indexNames))
	allRelNames = append(allRelNames, tableNames...)
	allRelNames = append(allRelNames, indexNames...)
	if len(allRelNames) > 0 {
		// owner_relname is non-null for index rows (resolved via pg_index),
		// letting an index's pg_class stats nest under its owning table.
		relRows, err := c.conn.Query(context.Background(),
			`SELECT n.nspname, c.relname, c.relkind, c.reltuples, c.relpages, c.relallvisible,
			        owner.relname AS owner_relname
			 FROM pg_class c
			 JOIN pg_namespace n ON n.oid = c.relnamespace
			 LEFT JOIN pg_index i ON i.indexrelid = c.oid
			 LEFT JOIN pg_class owner ON owner.oid = i.indrelid
			 WHERE c.relname = ANY($1)`,
			allRelNames)
		if err != nil {
			return PgStatsSnapshot{}, err
		}
		for relRows.Next() {
			var schema, relname, relkind string
			var reltuples float64
			var relpages, relallvisible int
			var ownerRelname *string
			if err := relRows.Scan(&schema, &relname, &relkind,
				&reltuples, &relpages, &relallvisible, &ownerRelname); err != nil {
				return PgStatsSnapshot{}, err
			}
			if ownerRelname != nil {
				g := group(schema, *ownerRelname)
				g.Indexes = append(g.Indexes, PgIndexStat{
					IndexName: relname, RelKind: relkind, Reltuples: reltuples,
					Relpages: relpages, Relallvisible: relallvisible,
				})
			} else {
				g := group(schema, relname)
				g.Relation = &PgRelationStat{
					RelKind: relkind, Reltuples: reltuples,
					Relpages: relpages, Relallvisible: relallvisible,
				}
			}
		}
	}

	tables := make([]PgTableStats, 0, len(groups))
	for _, g := range groups {
		tables = append(tables, *g)
	}
	slices.SortFunc(tables, func(a, b PgTableStats) int {
		if a.SchemaName != b.SchemaName {
			return strings.Compare(a.SchemaName, b.SchemaName)
		}
		return strings.Compare(a.TableName, b.TableName)
	})

	return PgStatsSnapshot{Tables: tables}, nil
}
