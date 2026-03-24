package main

import (
	"database/sql"
	"fmt"
	"log"

	_ "modernc.org/sqlite"
)

func initDB(path string) *sql.DB {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		log.Fatal("open db:", err)
	}
	if err := db.Ping(); err != nil {
		log.Fatal("ping db:", err)
	}

	// Keep a single writer connection to avoid SQLite locking issues.
	db.SetMaxOpenConns(1)

	migrate(db)
	return db
}

func migrate(db *sql.DB) {
	migrations := []string{
		`CREATE TABLE IF NOT EXISTS words (
			id                INTEGER  PRIMARY KEY AUTOINCREMENT,
			word              TEXT     NOT NULL,
			reading           TEXT,
			part_of_speech    TEXT,
			meaning           TEXT,
			example_jp        TEXT,
			example_en        TEXT,
			audio_path        TEXT,
			drill_count       INTEGER  NOT NULL DEFAULT 0,
			drill_target      INTEGER  NOT NULL DEFAULT 1,
			incorrect_count   INTEGER  NOT NULL DEFAULT 0,
			created_at        DATETIME NOT NULL DEFAULT (datetime('now')),
			last_drilled_at   DATETIME,
			target_reached_at DATETIME
		)`,
		`CREATE TABLE IF NOT EXISTS drill_sessions (
			id         INTEGER  PRIMARY KEY AUTOINCREMENT,
			started_at DATETIME NOT NULL DEFAULT (datetime('now'))
		)`,
		`CREATE TABLE IF NOT EXISTS drill_answers (
			id          INTEGER  PRIMARY KEY AUTOINCREMENT,
			session_id  INTEGER  NOT NULL REFERENCES drill_sessions(id),
			word_id     INTEGER  NOT NULL REFERENCES words(id),
			correct     INTEGER  NOT NULL CHECK (correct IN (0, 1)),
			answered_at DATETIME NOT NULL DEFAULT (datetime('now'))
		)`,
	}

	for _, m := range migrations {
		if _, err := db.Exec(m); err != nil {
			log.Fatal("migrate:", err)
		}
	}
	log.Println("DB migration OK")
}

// resetDB drops all user tables and re-runs migrations, giving a clean slate.
func resetDB(db *sql.DB) error {
	tables, err := listTables(db)
	if err != nil {
		return err
	}
	for _, t := range tables {
		if _, err := db.Exec(fmt.Sprintf("DROP TABLE IF EXISTS %q", t)); err != nil {
			return fmt.Errorf("drop %s: %w", t, err)
		}
	}
	migrate(db)
	return nil
}

// tableInfo holds a table name and its current row count.
type tableInfo struct {
	Name string
	Rows int
}

// listTableInfos returns all user tables with their row counts.
func listTableInfos(db *sql.DB) ([]tableInfo, error) {
	tables, err := listTables(db)
	if err != nil {
		return nil, err
	}
	infos := make([]tableInfo, 0, len(tables))
	for _, t := range tables {
		var count int
		db.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM %q", t)).Scan(&count)
		infos = append(infos, tableInfo{Name: t, Rows: count})
	}
	return infos, nil
}

// listTables returns user-created table names (excludes sqlite internals).
func listTables(db *sql.DB) ([]string, error) {
	rows, err := db.Query(`
		SELECT name FROM sqlite_master
		WHERE type = 'table' AND name NOT LIKE 'sqlite_%'
		ORDER BY name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var name string
		rows.Scan(&name)
		tables = append(tables, name)
	}
	return tables, nil
}

// validTableName checks that the requested table actually exists, preventing SQL
// injection through the table name parameter (which cannot be parameterised in SQLite).
func validTableName(db *sql.DB, name string) bool {
	tables, err := listTables(db)
	if err != nil {
		return false
	}
	for _, t := range tables {
		if t == name {
			return true
		}
	}
	return false
}

// queryTable returns all rows of a table as string slices, newest first.
func queryTable(db *sql.DB, table string) (cols []string, rows [][]string, err error) {
	sqlRows, err := db.Query(fmt.Sprintf("SELECT * FROM %q ORDER BY rowid DESC", table))
	if err != nil {
		return nil, nil, err
	}
	defer sqlRows.Close()

	cols, err = sqlRows.Columns()
	if err != nil {
		return nil, nil, err
	}

	for sqlRows.Next() {
		vals := make([]interface{}, len(cols))
		ptrs := make([]interface{}, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		sqlRows.Scan(ptrs...)
		row := make([]string, len(cols))
		for i, v := range vals {
			if v == nil {
				row[i] = "NULL"
			} else {
				row[i] = fmt.Sprintf("%v", v)
			}
		}
		rows = append(rows, row)
	}
	return cols, rows, nil
}

