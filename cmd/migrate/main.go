package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/tyha2404/nexo-app-api/internal/config"
	"github.com/tyha2404/nexo-app-api/internal/logger"
)

const (
	migrationsDir = "internal/migration/schema"
)

func main() {
	flag.Usage = func() {
		fmt.Println("Usage: go run cmd/migrate/main.go <subcommand> [args]")
		fmt.Println("Subcommands:")
		fmt.Println("  create <name> sql  Create a new SQL migration file")
		fmt.Println("  up                 Apply all pending migrations")
		fmt.Println("  down               Roll back the last migration")
		fmt.Println("  status             Show migration status")
		fmt.Println("  redo               Roll back the last migration, then apply it again")
	}

	flag.Parse()

	args := flag.Args()
	if len(args) == 0 {
		flag.Usage()
		os.Exit(1)
	}

	command := args[0]

	// 1. For "create" command, we don't necessarily need a database connection
	if command == "create" {
		if len(args) < 2 {
			log.Fatal("Migration name is required. Usage: go run cmd/migrate/main.go create <name> [sql|go]")
		}
		migrationName := args[1]
		migrationType := "sql"
		if len(args) >= 3 {
			migrationType = args[2]
		}

		// Ensure directory exists
		if err := os.MkdirAll(migrationsDir, 0755); err != nil {
			log.Fatalf("failed to create migrations directory: %v", err)
		}

		if err := goose.Run("create", nil, migrationsDir, migrationName, migrationType); err != nil {
			log.Fatalf("goose create failed: %v", err)
		}
		return
	}

	// 2. For database commands, load config and connect to DB
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	logg, err := logger.New(cfg.LogLevel)
	if err != nil {
		log.Fatalf("failed to init logger: %v", err)
	}
	defer logg.Sync()

	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=%s default_query_exec_mode=simple_protocol",
		cfg.DBHost, cfg.DBUser, cfg.DBPass, cfg.DBName, cfg.DBPort, cfg.DBSSL,
	)

	connConfig, err := pgx.ParseConfig(dsn)
	if err != nil {
		log.Fatalf("failed to parse DB config: %v", err)
	}
	connConfig.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol

	sqlDB := stdlib.OpenDB(*connConfig)
	defer sqlDB.Close()

	if err := sqlDB.Ping(); err != nil {
		log.Fatalf("failed to ping database: %v", err)
	}

	if err := goose.SetDialect("postgres"); err != nil {
		log.Fatalf("failed to set goose dialect: %v", err)
	}

	// Ensure migrations directory exists before running DB migrations
	if err := os.MkdirAll(migrationsDir, 0755); err != nil {
		log.Fatalf("failed to create migrations directory: %v", err)
	}

	// Resync goose_db_version sequence if needed
	_, _ = sqlDB.Exec(`SELECT setval(pg_get_serial_sequence('goose_db_version', 'id'), COALESCE((SELECT MAX(id) FROM goose_db_version), 1));`)

	log.Printf("Executing goose command: %s", command)
	var gooseArgs []string
	if len(args) > 1 {
		gooseArgs = args[1:]
	}

	if err := goose.Run(command, sqlDB, migrationsDir, gooseArgs...); err != nil {
		log.Fatalf("goose run %s failed: %v", command, err)
	}
}
