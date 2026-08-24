package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"os/user"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/chriserin/pg_explain/sqlsplit"

	"github.com/charmbracelet/x/ansi"
)

var extension string = ".pgex"
var defaultPgexDir = "_pgex"

type QueryRun struct {
	query            string
	result           string
	originalFilename string
	pgexPointer      string
	sourcePath       string
	settings         []Setting
	pgStats          PgStatsSnapshot
	cancelled        bool
	cancelledElapsed time.Duration
	isFromCmdLine    bool
	ranAt            time.Time
}

func CreatePgexDir() (string, error) {
	workingDir, _ := os.Getwd()
	dirPath := filepath.Join(workingDir, defaultPgexDir)
	err := os.MkdirAll(dirPath, 0755)
	return dirPath, err
}

var ErrNoPreviousQueryRun = errors.New("no previous query run")

// currentQueryRunIndex locates q within the real chronological _pgex
// history by exact absolute path (sourcePath), not by filename. A
// substring/basename match would misidentify a query run loaded from
// outside the _pgex dir (e.g. via --pgex pointing at a copy or a
// non-timestamped bookmark file) as whichever real history entry happens
// to share that name, making it impossible to navigate back to the actual
// file that was loaded. found is false when q isn't part of the real
// history at all.
func currentQueryRunIndex(q QueryRun, pgexFiles []string) (index int, found bool) {
	index = -1
	for i, pgexFile := range pgexFiles {
		if pgexFile == q.sourcePath {
			index, found = i, true
		}
	}
	return index, found
}

// isInRealHistory reports whether q corresponds to a file in the real
// chronological _pgex history (as opposed to a --pgex bookmark file loaded
// from outside it).
func (q QueryRun) isInRealHistory() (bool, error) {
	pgexFiles, err := getQueryRunEntries()
	if err != nil {
		return false, err
	}

	_, found := currentQueryRunIndex(q, pgexFiles)
	return found, nil
}

func (q QueryRun) previousQueryRun() (QueryRun, bool, error) {
	pgexFiles, err := getQueryRunEntries()
	if err != nil {
		return QueryRun{}, false, err
	}

	currentIndex, found := currentQueryRunIndex(q, pgexFiles)

	if found && currentIndex-1 >= 0 {
		qr, err := loadQueryRun(pgexFiles[currentIndex-1])
		return qr, true, err
	} else {
		return q, false, nil
	}
}

var ErrNoNextQueryRun = errors.New("no next query run")

func (q QueryRun) nextQueryRun() (QueryRun, bool, error) {
	pgexFiles, err := getQueryRunEntries()
	if err != nil {
		return QueryRun{}, false, err
	}

	currentIndex, found := currentQueryRunIndex(q, pgexFiles)

	if found && currentIndex+1 < len(pgexFiles) {
		qr, err := loadQueryRun(pgexFiles[currentIndex+1])
		return qr, true, err
	} else {
		return q, false, nil
	}
}

func latestQueryRun() (QueryRun, error) {
	pgexFiles, err := getQueryRunEntries()
	if err != nil {
		return QueryRun{}, err
	}

	return loadQueryRun(pgexFiles[len(pgexFiles)-1])
}

func loadQueryRun(pgexFile string) (QueryRun, error) {
	body, err := os.ReadFile(pgexFile)
	if err != nil {
		return QueryRun{}, err
	}

	sourcePath, err := filepath.Abs(pgexFile)
	if err != nil {
		return QueryRun{}, err
	}

	contents := string(body)
	if !strings.Contains(contents, sqlDivider) {
		return QueryRun{}, errors.New("wrong pgex format: no settings-above divider")
	}

	if !strings.Contains(contents, explainDivider) {
		return QueryRun{}, errors.New("wrong pgex format: no sql-above divider")
	}

	var ranAt time.Time
	if rest, found := strings.CutPrefix(contents, ranAtMarker+" "); found {
		markerLine, remainder, _ := strings.Cut(rest, "\n")
		if t, err := time.Parse(time.RFC3339, strings.TrimSpace(markerLine)); err == nil {
			ranAt = t
			contents = strings.TrimLeft(remainder, "\n")
		}
	}

	settingsAbove := strings.Split(contents, sqlDivider)

	settingsContent := settingsAbove[0]

	settingsStrings := strings.Split(settingsContent, "\n")

	settings := make([]Setting, 0, len(settingsStrings))
	for _, settingStr := range settingsStrings {
		if ansi.StringWidth(strings.Trim(settingStr, " ")) > 0 {
			settings = append(settings, SettingUnmarshal(settingStr))
		}
	}

	sqlAbove := strings.Split(settingsAbove[1], explainDivider)

	sql := sqlAbove[0]
	plan := sqlAbove[1]

	var cancelled bool
	var cancelledElapsed time.Duration
	trimmedPlan := strings.TrimLeft(plan, "\n")
	if rest, found := strings.CutPrefix(trimmedPlan, cancelledMarker+" "); found {
		markerLine, remainder, _ := strings.Cut(rest, "\n")
		if d, err := time.ParseDuration(strings.TrimSpace(markerLine)); err == nil {
			cancelled = true
			cancelledElapsed = d
			plan = strings.TrimLeft(remainder, "\n")
		}
	}

	result := plan
	if resultPart, _, found := strings.Cut(plan, statsDivider); found {
		// The PG_STATS section is a human-readable report, not re-parsed
		// back into structured data — nothing in the app consumes it after
		// load, so it's dropped here, only the explain result is kept.
		result = resultPart
	}

	_, file := path.Split(pgexFile)
	_, name, _ := strings.Cut(file, "_")
	originalFilename := strings.Replace(name, ".pgex", "", 1) + ".sql"
	return QueryRun{
		query:            sql,
		result:           result,
		pgexPointer:      file,
		sourcePath:       sourcePath,
		settings:         settings,
		originalFilename: originalFilename,
		cancelled:        cancelled,
		cancelledElapsed: cancelledElapsed,
		ranAt:            ranAt,
	}, nil
}

func getQueryRunEntries() ([]string, error) {
	dirEntries, err := os.ReadDir(defaultPgexDir)
	if err != nil {
		return []string{}, errors.New("_pgex dir does not yet exist, the explain command will create a .pgex file in a _pgex dir")
	}

	pgexFiles := make([]string, 0, len(dirEntries))
	wd, _ := os.Getwd()
	for _, d := range dirEntries {
		pgexFile := regexp.MustCompile(`[0-9]{14}_.*\.pgex`)
		if pgexFile.Match([]byte(d.Name())) {
			result := filepath.Join(wd, defaultPgexDir, d.Name())
			pgexFiles = append(pgexFiles, result)
		}
	}

	return pgexFiles, nil
}

func NewQueryRun(filename string) QueryRun {
	if _, err := os.Stat(filename); err == nil {
		body, err := os.ReadFile(filename)
		if err != nil {
			log.Fatal(err)
		}

		sqls := sqlsplit.Split(string(body))

		if len(sqls) > 1 {
			fmt.Println(sqls)
			log.Fatal("too many sql statements in provided file")
		} else if len(sqls) == 0 {
			log.Fatal("no sql statements in provided file")
		} else {
			return QueryRun{
				query:            sqls[0],
				originalFilename: filename,
			}
		}
	} else {
		log.Fatal(err)
	}
	return QueryRun{}
}

// NewReQueryRun returns a fresh QueryRun for re-running the same query,
// carrying over only the query text and its originating filename.
func NewReQueryRun(previous QueryRun) QueryRun {
	return QueryRun{
		query:            previous.query,
		originalFilename: previous.originalFilename,
	}
}

func (q *QueryRun) SetSettings(settings []Setting) {
	queryRunSettings := make([]Setting, len(settings))
	copy(queryRunSettings, settings)
	q.settings = queryRunSettings
}

func (q *QueryRun) SetResult(result string) {
	q.result = result
	q.ClearCancelled()
}

func (q *QueryRun) SetPgStats(pgStats PgStatsSnapshot) {
	q.pgStats = pgStats
}

func (q *QueryRun) Cancel(elapsed time.Duration) {
	q.cancelled = true
	q.cancelledElapsed = elapsed
}

func (q *QueryRun) ClearCancelled() {
	q.cancelled = false
	q.cancelledElapsed = 0
}

func (q *QueryRun) WritePgexFile(pgexDir string) error {
	if q.ranAt.IsZero() {
		q.ranAt = time.Now()
	}
	fileName := q.pgexFilename()
	fullFilePath := filepath.Join(pgexDir, fileName)
	contentBytes := []byte(q.pgexFileContent())

	err := os.WriteFile(fullFilePath, contentBytes, 0666)
	q.pgexPointer = fileName
	if absPath, absErr := filepath.Abs(fullFilePath); absErr == nil {
		q.sourcePath = absPath
	}

	return err
}

func (q QueryRun) DisplayName() string {
	_, file := path.Split(q.originalFilename)
	return file
}

var PGEX_DATE_FORMAT = "20060102150405"

func (q QueryRun) pgexFilename() string {
	user, _ := user.Current()
	filePath := strings.Replace(q.originalFilename, "~", user.HomeDir, 1)

	_, file := path.Split(filePath)
	name, _, _ := strings.Cut(file, ".")

	formattedNow := q.ranAt.Format(PGEX_DATE_FORMAT)
	return fmt.Sprintf("%s_%s%s", formattedNow, name, extension)
}

var explainDivider = "---------------- SQL ABOVE / EXPLAIN JSON BELOW ----------------"
var sqlDivider = "---------------- SETTINGS ABOVE / SQL BELOW ----------------"
var cancelledMarker = "CANCELLED"
var statsDivider = "---------------- EXPLAIN JSON ABOVE / PG_STATS BELOW ----------------"
var ranAtMarker = "RAN_AT"

func (q QueryRun) pgexFileContent() string {
	var buf strings.Builder
	if !q.ranAt.IsZero() {
		buf.WriteString(fmt.Sprintf("%s %s\n\n", ranAtMarker, q.ranAt.Format(time.RFC3339)))
	}
	for _, setting := range q.settings {
		buf.WriteString(setting.Marshal())
		buf.WriteString("\n")
	}
	buf.WriteString("\n\n")
	buf.WriteString(sqlDivider)
	buf.WriteString("\n\n")
	buf.WriteString(q.query)
	buf.WriteString("\n\n")
	buf.WriteString(explainDivider)
	buf.WriteString("\n\n")
	if q.cancelled {
		buf.WriteString(fmt.Sprintf("%s %s\n\n", cancelledMarker, q.cancelledElapsed))
	}
	buf.WriteString(q.result)
	buf.WriteString("\n\n")
	if !q.pgStats.IsEmpty() {
		buf.WriteString(statsDivider)
		buf.WriteString("\n\n")
		buf.WriteString(q.pgStats.String())
		buf.WriteString("\n")
	}
	return buf.String()
}

func (q QueryRun) WithExplain() string {
	explainSegment := `explain (
		format json
	) `

	return explainSegment + q.query
}

func (q QueryRun) WithExplainAnalyze() string {
	explainSegment := `explain (
		settings,
		format json,
		buffers,
		analyze
	) `

	return explainSegment + q.query
}
