package main

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestPgexFileRoundTripsCancelledState(t *testing.T) {
	t.Chdir(t.TempDir())

	queryRun := QueryRun{
		query:            "select 1",
		result:           `{"some": "plan"}`,
		originalFilename: "select.sql",
		cancelled:        true,
		cancelledElapsed: 2300 * time.Millisecond,
	}

	pgexDir, err := CreatePgexDir()
	assert.NoError(t, err)

	err = queryRun.WritePgexFile(pgexDir)
	assert.NoError(t, err)

	loaded, err := loadQueryRun(pgexDir + "/" + queryRun.pgexPointer)
	assert.NoError(t, err)

	assert.True(t, loaded.cancelled)
	assert.Equal(t, 2300*time.Millisecond, loaded.cancelledElapsed)
	assert.Equal(t, "select 1", strings.TrimSpace(loaded.query))
	assert.Equal(t, `{"some": "plan"}`, strings.TrimSpace(loaded.result))
}

func TestPgexFileRoundTripsUncancelledState(t *testing.T) {
	t.Chdir(t.TempDir())

	queryRun := QueryRun{
		query:            "select 1",
		result:           `{"some": "plan"}`,
		originalFilename: "select.sql",
	}

	pgexDir, err := CreatePgexDir()
	assert.NoError(t, err)

	err = queryRun.WritePgexFile(pgexDir)
	assert.NoError(t, err)

	loaded, err := loadQueryRun(pgexDir + "/" + queryRun.pgexPointer)
	assert.NoError(t, err)

	assert.False(t, loaded.cancelled)
	assert.Equal(t, time.Duration(0), loaded.cancelledElapsed)
	assert.Equal(t, `{"some": "plan"}`, strings.TrimSpace(loaded.result))
	assert.True(t, loaded.pgStats.IsEmpty())
}

func TestPgexFileRoundTripsPgStats(t *testing.T) {
	t.Chdir(t.TempDir())

	correlation := 0.85
	mcv := "{1,2,3}"
	mcf := "{0.5,0.3,0.2}"
	ndistinct := `{"1, 2": 5000}`
	queryRun := QueryRun{
		query:            "select 1",
		result:           `{"some": "plan"}`,
		originalFilename: "select.sql",
		pgStats: PgStatsSnapshot{
			Tables: []PgTableStats{
				{
					SchemaName: "public",
					TableName:  "widgets",
					Relation:   &PgRelationStat{RelKind: "r", Reltuples: 1000, Relpages: 10, Relallvisible: 10},
					Columns: []PgColumnStat{
						{
							AttName:         "status",
							NullFrac:        0.0,
							AvgWidth:        8,
							NDistinct:       -1,
							Correlation:     &correlation,
							MostCommonVals:  &mcv,
							MostCommonFreqs: &mcf,
						},
						{AttName: "description", AvgWidth: 32},
					},
					Indexes: []PgIndexStat{
						{IndexName: "widgets_status_idx", RelKind: "i", Reltuples: 1000, Relpages: 3},
					},
					Extended: []PgExtendedStat{
						{StatName: "widgets_stat", Columns: "status, description", NDistinct: &ndistinct, HasMCV: true},
					},
					Activity: &PgTableActivity{NLiveTup: 1000, NDeadTup: 5},
				},
			},
		},
	}

	pgexDir, err := CreatePgexDir()
	assert.NoError(t, err)

	err = queryRun.WritePgexFile(pgexDir)
	assert.NoError(t, err)

	// The PG_STATS section is a human-readable report, not JSON — assert
	// on the raw file content rather than re-parsing it back into structs.
	rawContent, err := os.ReadFile(pgexDir + "/" + queryRun.pgexPointer)
	assert.NoError(t, err)
	content := string(rawContent)
	assert.Contains(t, content, statsDivider)
	assert.Contains(t, content, "=== public.widgets ===")
	assert.Contains(t, content, "status")
	assert.Contains(t, content, "widgets_status_idx")
	assert.Contains(t, content, "widgets_stat")

	loaded, err := loadQueryRun(pgexDir + "/" + queryRun.pgexPointer)
	assert.NoError(t, err)

	assert.Equal(t, `{"some": "plan"}`, strings.TrimSpace(loaded.result))
	assert.True(t, loaded.pgStats.IsEmpty())
}
