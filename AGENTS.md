# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with
code in this repository.

## What this is

`pg_explain` is a terminal (Bubble Tea) tool for exploring Postgres
`EXPLAIN`/`EXPLAIN ANALYZE` query plans. It can read a JSON plan from stdin,
run `EXPLAIN`/`EXPLAIN ANALYZE` against a live database for a given `.sql`
file, or replay a previously saved plan.

## Commands

````go build ./...          # build go vet ./...             # vet go test
./...             # run all tests go test ./... -run TestName   # run a single
test go run . <args>            # run without building (e.g. go run . explain
select_one.sql) ```

## Architecture

**Entry point / CLI (`main.go`)**: cobra-based CLI with subcommands `explain`,
`analyze` (alias for explain+analyze), and `version`, plus a bare root command
that auto-detects its input:
- stdin piped in → treat as a raw JSON explain plan (`SOURCE_STDIN`)
- one file arg → run `EXPLAIN` against that `.sql` file (`SOURCE_FILE`)
- no input/args → replay the most recent `.pgex` file (`SOURCE_PGEX`)

Connection config is resolved by `LoadSqlConfig`, layering (in order) an
optional `./pgex.conf` INI file (rendered through a Sprig-enabled Go template
first, so config files can use template functions), then CLI flags
(`--conn-string`, `--host`, `--port`, `--user`, `--password`, `--database`),
all ultimately producing a `pgx.ConnConfig` used for all DB connections.

**Bubble Tea app (`ui.go`, `context.go`)**: `Model` (ui.go) is the tea.Model;
`ProgramUIState` (context.go) holds view/display state (cursor position, which
stat is displayed — rows/buffers/cost/time —, styling, join view toggle, etc.)
separately from the `Model` itself. `RunProgram(source, runType)` constructs
the program for a given `Source`/`RunType` combination. Styling is built once
via `NormalStyles()` and derived into `CursorStyle`/`ChildCursorStyle` variants
(lipgloss).

**Query execution (`connection.go`, `query_run.go`)**: `Connection` wraps a
single `pgx.Conn` and applies Postgres settings (`SetSetting`) before running
`ExecuteExplain`. Only a fixed allowlist of settings (`allowedSettings` in
connection.go: `work_mem`, `join_collapse_limit`,
`max_parallel_workers_per_gather`, `random_page_cost`, `effective_cache_size`,
`cluster_name`) are read back via `ShowAll` and eligible to be
tweaked/replayed.

`QueryRun` (query_run.go) represents one query execution attempt — the SQL
text, the resulting explain JSON, the settings in effect, and cancellation
state. Every run is persisted as a `.pgex` file under a `_pgex` directory
(created relative to CWD via `CreatePgexDir`), named
`<timestamp>_<originalName>.pgex`. A `.pgex` file is plain text with two
divider markers (`sqlDivider`, `explainDivider` in query_run.go) separating
settings / SQL / explain JSON sections, optionally prefixed with a `CANCELLED
<duration>` marker if the query was cancelled mid-run.
`previousQueryRun`/`nextQueryRun`/`latestQueryRun` navigate the `_pgex` history
by sorted filename (timestamp-prefixed, so lexical sort == chronological).

**Plan parsing (`expjson.go`, `plannode.go`)**: `Convert` takes the raw JSON
explain output (as a string) and walks it recursively (`extractPlanNodes`) into
a flat `[]PlanNode` tree annotated with `Position` (id/level/parent, for
rendering indentation and gather/nested-loop-aware coloring). `PlanNode`
mirrors the fields Postgres's JSON explain format can emit (join type, cond
strings, sort/group keys, CTE/subplan/function names, analyzed stats, etc).
`ExplainPlan` wraps the parsed nodes plus whether the plan was ANALYZEd and its
total execution time.

**SQL splitting (`sqlsplit/sqlsplit.go`)**: standalone lexer-based package that
splits a string containing multiple SQL statements into individual statements
(handles quoted strings/comments so semicolons inside them aren't treated as
separators). Used by `NewQueryRun` to validate that a submitted `.sql` file
contains exactly one statement.

**Settings (`setting.go`)**: `Setting{name, setting}` pairs with
`Marshal`/`SettingUnmarshal` for the `.pgex` file's settings section, and a
fixed display order (`settingPositions`). `excludedFromNextSettings` (currently
`cluster_name`) lists settings that should not be carried forward into a
subsequent run's `SET` calls.

## Testing conventions

Tests use `testify/assert` and `t.Chdir(t.TempDir())` to sandbox
filesystem-touching tests (see `query_run_test.go`). `testdata/*.json` holds
sample Postgres explain-JSON fixtures used by
`expjson_test.go`/`plannode_test.go` to exercise specific plan features (CTEs,
incremental sort, merge joins, tid scans, etc.) — when adding support for a new
explain JSON field, add a corresponding fixture here rather than inlining large
JSON literals in the test file.
````
