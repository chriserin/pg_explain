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
	settings         []Setting
}

func CreatePgexDir() (string, error) {
	workingDir, _ := os.Getwd()
	dirPath := filepath.Join(workingDir, defaultPgexDir)
	err := os.MkdirAll(dirPath, 0755)
	return dirPath, err
}

func (q QueryRun) previousQueryRun() (QueryRun, error) {
	pgexFiles, err := getQueryRunEntries()
	if err != nil {
		return QueryRun{}, err
	}

	var currentIndex int
	for i, pgexFile := range pgexFiles {
		if strings.Contains(pgexFile, q.pgexPointer) {
			currentIndex = i
		}
	}

	if currentIndex-1 >= 0 {
		return loadQueryRun(pgexFiles[currentIndex-1])
	} else {
		return q, nil
	}
}

func (q QueryRun) nextQueryRun() (QueryRun, error) {
	pgexFiles, err := getQueryRunEntries()
	if err != nil {
		return QueryRun{}, err
	}

	var currentIndex int
	for i, pgexFile := range pgexFiles {
		if strings.Contains(pgexFile, q.pgexPointer) {
			currentIndex = i
		}
	}

	if currentIndex+1 < len(pgexFiles) {
		return loadQueryRun(pgexFiles[currentIndex+1])
	} else {
		return q, nil
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

	contents := string(body)
	if !strings.Contains(contents, sqlDivider) {
		return QueryRun{}, errors.New("Wrong pgex format: no settings-above divider")
	}

	if !strings.Contains(contents, explainDivider) {
		return QueryRun{}, errors.New("Wrong pgex format: no sql-above divider")
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

	_, file := path.Split(pgexFile)
	_, name, _ := strings.Cut(file, "_")
	originalFilename := strings.Replace(name, ".pgex", "", 1) + ".sql"
	return QueryRun{query: sql, result: plan, pgexPointer: file, settings: settings, originalFilename: originalFilename}, nil
}

func getQueryRunEntries() ([]string, error) {
	dirEntries, err := os.ReadDir(defaultPgexDir)
	if err != nil {
		return []string{}, errors.New("_pgex dir does not exist, use the exec command to create a .pgex file in a _pgex dir")
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

func (q *QueryRun) SetResult(result string) {
	q.result = result
}

func (q *QueryRun) WritePgexFile(pgexDir string) error {
	fileName := q.pgexFilename()
	fullFilePath := filepath.Join(pgexDir, fileName)
	contentBytes := []byte(q.pgexFileContent())

	err := os.WriteFile(fullFilePath, contentBytes, 0666)
	q.pgexPointer = fileName

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

	formattedNow := time.Now().Format(PGEX_DATE_FORMAT)
	return fmt.Sprintf("%s_%s%s", formattedNow, name, extension)
}

var explainDivider = "---------------- SQL ABOVE / EXPLAIN JSON BELOW ----------------"
var sqlDivider = "---------------- SETTINGS ABOVE / SQL BELOW ----------------"

func (q QueryRun) pgexFileContent() string {
	var buf strings.Builder
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
	buf.WriteString(q.result)
	buf.WriteString("\n\n")
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
