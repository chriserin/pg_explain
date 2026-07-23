package main

import (
	"context"
	"slices"

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
