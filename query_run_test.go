package main

import (
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
}
