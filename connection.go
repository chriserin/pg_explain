package main

import (
	"context"
	"fmt"
	"os"
	"slices"

	pgx "github.com/jackc/pgx/v5"
)

type Connection struct {
	databaseUrl string
	conn        *pgx.Conn
}

func (c *Connection) Connect() {
	conn, err := pgx.Connect(context.Background(), c.databaseUrl)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to connect to database: %v\n", err)
		os.Exit(1)
	}
	c.conn = conn
}

func (c Connection) ExecuteExplain(query string) string {

	var explainResult string
	err := c.conn.QueryRow(context.Background(), query).Scan(&explainResult)
	if err != nil {
		fmt.Fprintf(os.Stderr, "QueryRow failed: %v\n", err)
		os.Exit(1)
	}

	return explainResult
}

func (c Connection) Close() {
	c.conn.Close(context.Background())
}

type Setting struct {
	name    string
	setting string
}

var allowedSettings = []string{"work_mem", "join_collapse_limit", "max_parallel_workers_per_gather", "random_page_cost", "effective_cache_size"}

func (c Connection) ShowAll() []Setting {
	var result []Setting
	rows, err := c.conn.Query(context.Background(), "show all")

	if err != nil {
		fmt.Fprintf(os.Stderr, "Show all failed: %v\n", err)
		os.Exit(1)
	}

	for rows.Next() {
		var name, setting, description string
		rows.Scan(&name, &setting, &description)
		if slices.Contains(allowedSettings, name) {
			result = append(result, Setting{name: name, setting: setting})
		}
	}

	return result
}
