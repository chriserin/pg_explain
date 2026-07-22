package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"slices"
	"strconv"
	"text/template"

	sprig "github.com/Masterminds/sprig/v3"
	pgx "github.com/jackc/pgx/v5"
	"github.com/spf13/cobra"
	ini "github.com/vaughan0/go-ini"
)

const VERSION = "0.1.1-beta.3"

var cliOptions struct {
	connString  string
	host        string
	port        uint16
	user        string
	password    string
	database    string
	configPaths []string
}

var ConnConfig pgx.ConnConfig
var PGEnvvars map[string]string = make(map[string]string)
var ConnString string

func main() {
	var cmdExec = &cobra.Command{
		Use:   "analyze",
		Short: "Explain and Analyze the query in the sql file",
		Long:  "Explain and Analyze the query in the sql file",
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			if err := LoadSqlConfig(); err != nil {
				fmt.Println(err)
				os.Exit(1)
			}

			source := Source{sourceType: SOURCE_FILE, fileName: args[0]}

			if _, err := RunProgram(source, RunExplainAnalyze).Run(); err != nil {
				fmt.Println("Error running program:", err)
				os.Exit(1)
			} else {
				return
			}
		},
	}

	var cmdExplain = &cobra.Command{
		Use:   "explain",
		Short: "Get plan by explaining sql",
		Long:  "Get plan by explaining sql",
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			if err := LoadSqlConfig(); err != nil {
				fmt.Println(err)
				os.Exit(1)
			}

			source := Source{sourceType: SOURCE_FILE, fileName: args[0]}

			if _, err := RunProgram(source, RunExplain).Run(); err != nil {
				fmt.Println("Error running program:", err)
				os.Exit(1)
			} else {
				return
			}
		},
	}

	var rootCmd = &cobra.Command{
		Use:   "pg_explain",
		Short: "Read json from stdin, examine last pgex file, or explain a query",
		Long:  `Explain a query from the given file, read explain in json format from stdin, or if no inputs read last pgex file`,
		Args:  cobra.MinimumNArgs(0),
		Run: func(cmd *cobra.Command, args []string) {
			if err := LoadSqlConfig(); err != nil {
				fmt.Println(err)
				os.Exit(1)
			}

			var source Source

			stat, _ := os.Stdin.Stat()
			if (stat.Mode() & os.ModeCharDevice) == 0 {
				input, _ := io.ReadAll(os.Stdin)
				source = Source{sourceType: SOURCE_STDIN, input: string(input)}
			} else if len(args) == 1 {
				source := Source{sourceType: SOURCE_FILE, fileName: args[0]}

				if _, err := RunProgram(source, RunExplain).Run(); err != nil {
					fmt.Println("Error running program:", err)
					os.Exit(1)
				}
				return
			} else {
				source = Source{sourceType: SOURCE_PGEX}
			}

			if _, err := RunProgram(source, RunNothing).Run(); err != nil {
				fmt.Println("Error running program:", err)
				os.Exit(1)
			} else {
				return
			}

			cmd.Help()
			os.Exit(1)
		},
	}

	rootCmd.PersistentFlags().StringVarP(&cliOptions.connString, "conn-string", "", "", "database connection string (https://www.postgresql.org/docs/current/libpq-connect.html#LIBPQ-CONNSTRING)")
	rootCmd.PersistentFlags().StringVarP(&cliOptions.host, "host", "", "", "database host")
	rootCmd.PersistentFlags().Uint16VarP(&cliOptions.port, "port", "", 0, "database port")
	rootCmd.PersistentFlags().StringVarP(&cliOptions.user, "user", "", "", "database user")
	rootCmd.PersistentFlags().StringVarP(&cliOptions.password, "password", "", "", "database password")
	rootCmd.PersistentFlags().StringVarP(&cliOptions.database, "database", "", "", "database name")

	rootCmd.AddCommand(cmdExec)
	rootCmd.AddCommand(cmdExplain)

	cmdVersion := &cobra.Command{
		Use:   "version",
		Short: "Print version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("pg_explain v%s\n", VERSION)
		},
	}

	rootCmd.AddCommand(cmdVersion)

	rootCmd.Execute()
}

func LoadSqlConfig() error {
	if len(cliOptions.configPaths) == 0 {
		if _, err := os.Stat("./pgex.conf"); err == nil {
			cliOptions.configPaths = append(cliOptions.configPaths, "./pgex.conf")
		}
	}

	for _, configFile := range cliOptions.configPaths {
		err := appendConfigFromFile(configFile)
		if err != nil {
			return err
		}
	}

	if cliOptions.connString != "" {
		ConnString = cliOptions.connString
		if _, err := pgx.ParseConfig(cliOptions.connString); err != nil {
			return fmt.Errorf("error while parsing conn-string argument: %w", err)
		}
	}
	if cliOptions.host != "" {
		PGEnvvars["PGHOST"] = cliOptions.host
	}
	if cliOptions.port != 0 {
		PGEnvvars["PGPORT"] = strconv.FormatUint(uint64(cliOptions.port), 10)
	}
	if cliOptions.database != "" {
		PGEnvvars["PGDATABASE"] = cliOptions.database
	}
	if cliOptions.user != "" {
		PGEnvvars["PGUSER"] = cliOptions.user
	}
	if cliOptions.password != "" {
		PGEnvvars["PGPASSWORD"] = cliOptions.password
	}

	for key, value := range PGEnvvars {
		if err := os.Setenv(key, value); err != nil {
			return fmt.Errorf("error setting PostgreSQL environment variables from config: %s: %w", key, err)
		}
	}

	if connConfig, err := pgx.ParseConfig(ConnString); err == nil {
		ConnConfig = *connConfig
	} else {
		return err
	}

	return nil
}

func appendConfigFromFile(path string) error {

	fileBytes, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	confTemplate, err := template.New("conf").Funcs(sprig.TxtFuncMap()).Parse(string(fileBytes))
	if err != nil {
		return err
	}

	var buf bytes.Buffer
	err = confTemplate.Execute(&buf, map[string]interface{}{})
	if err != nil {
		return err
	}

	file, err := ini.Load(&buf)
	if err != nil {
		return err
	}

	if connString, ok := file.Get("database", "conn_string"); ok {
		ConnString = connString
		if _, err := pgx.ParseConfig(connString); err != nil {
			return fmt.Errorf("error while parsing conn_string property: %w", err)
		}
	}

	if host, ok := file.Get("database", "host"); ok {
		PGEnvvars["PGHOST"] = host
	}

	// For backwards compatibility if host isn't set look for socket.
	if PGEnvvars["PGHOST"] == "" {
		if socket, ok := file.Get("database", "socket"); ok {
			PGEnvvars["PGHOST"] = socket
		}
	}

	if p, ok := file.Get("database", "port"); ok {
		_, err := strconv.ParseUint(p, 10, 16)
		if err != nil {
			return err
		}
		PGEnvvars["PGPORT"] = p
	}

	if database, ok := file.Get("database", "database"); ok {
		PGEnvvars["PGDATABASE"] = database
	}

	if user, ok := file.Get("database", "user"); ok {
		PGEnvvars["PGUSER"] = user
	}
	if password, ok := file.Get("database", "password"); ok {
		PGEnvvars["PGPASSWORD"] = password
	}
	return nil
}

func ExecuteExplain(query string, settings []Setting, ctx context.Context) (string, error) {
	pgConn := Connection{
		connConfig: ConnConfig,
	}

	err := pgConn.Connect()
	if err != nil {
		return "", err
	}

	defer pgConn.Close()

	for _, setting := range settings {
		if !slices.Contains(excludedFromNextSettings, setting.name) {
			err := pgConn.SetSetting(setting)
			if err != nil {
				return "", err
			}
		}
	}
	return pgConn.ExecuteExplain(query, ctx)
}
