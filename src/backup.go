package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
)

const backupFormatVersion = 1

var (
	backupIDPattern     = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}_\d{2}-\d{2}-\d{2}(?:_\d+)?$`)
	backupTableOrder    = []string{"kanji", "words", "drill_sessions", "drill_answers", "user_settings", "stories", "story_sentences", "tutor_prompts", "activity_events", "token_usage"}
	backupClearOrder    = []string{"drill_answers", "story_sentences", "drill_sessions", "stories", "activity_events", "token_usage", "user_settings", "words", "kanji", "tutor_prompts"}
	backupSummaryTables = []string{"words", "stories", "drill_sessions", "drill_answers", "tutor_prompts", "activity_events"}
)

type backupManifest struct {
	FormatVersion  int            `json:"formatVersion"`
	CreatedAt      string         `json:"createdAt"`
	BackupID       string         `json:"backupID"`
	IncludesImages bool           `json:"includesImages"`
	Counts         map[string]int `json:"counts"`
}

type backupListItem struct {
	ID             string         `json:"id"`
	CreatedAt      string         `json:"createdAt"`
	FormatVersion  int            `json:"formatVersion"`
	IncludesImages bool           `json:"includesImages"`
	Counts         map[string]int `json:"counts"`
}

type backupData map[string][]map[string]any

func backupsDir() string {
	return filepath.Join(backupRootDir(), "backups")
}

func backupRootDir() string {
	wd, err := os.Getwd()
	if err != nil {
		return "."
	}
	if filepath.Base(wd) != "src" {
		return "."
	}
	if _, err := os.Stat(filepath.Join(wd, "go.mod")); err != nil {
		return "."
	}
	return filepath.Dir(wd)
}

func backupDirByID(id string) string {
	return filepath.Join(backupsDir(), id)
}

func wordImagesDir() string {
	return filepath.Join("static", "images", "words")
}

func createBackupID() string {
	base := time.Now().Format("2006-01-02_15-04-05")
	id := base
	for suffix := 2; ; suffix++ {
		if _, err := os.Stat(backupDirByID(id)); errors.Is(err, os.ErrNotExist) {
			return id
		}
		id = base + "_" + strconv.Itoa(suffix)
	}
}

func ensureValidBackupID(id string) error {
	if !backupIDPattern.MatchString(id) {
		return errors.New("invalid backup id")
	}
	return nil
}

func listBackups() ([]backupListItem, error) {
	if err := os.MkdirAll(backupsDir(), 0o755); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(backupsDir())
	if err != nil {
		return nil, err
	}
	out := make([]backupListItem, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() || ensureValidBackupID(entry.Name()) != nil {
			continue
		}
		manifest, err := readBackupManifest(entry.Name())
		if err != nil {
			continue
		}
		out = append(out, backupListItem{
			ID:             manifest.BackupID,
			CreatedAt:      manifest.CreatedAt,
			FormatVersion:  manifest.FormatVersion,
			IncludesImages: manifest.IncludesImages,
			Counts:         manifest.Counts,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].ID > out[j].ID
	})
	return out, nil
}

func readBackupManifest(id string) (*backupManifest, error) {
	if err := ensureValidBackupID(id); err != nil {
		return nil, err
	}
	path := filepath.Join(backupDirByID(id), "manifest.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var manifest backupManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, err
	}
	if manifest.BackupID == "" {
		manifest.BackupID = id
	}
	return &manifest, nil
}

func createBackup(db *sql.DB) (*backupManifest, error) {
	if err := os.MkdirAll(backupsDir(), 0o755); err != nil {
		return nil, err
	}
	id := createBackupID()
	backupDir := backupDirByID(id)
	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		return nil, err
	}

	data, err := exportBackupData(db)
	if err != nil {
		return nil, err
	}

	if err := writeBackupDataFile(backupDir, data); err != nil {
		return nil, err
	}
	if err := copyBackupImages(backupDir, data); err != nil {
		return nil, err
	}

	manifest := &backupManifest{
		FormatVersion:  backupFormatVersion,
		CreatedAt:      time.Now().UTC().Format(time.RFC3339),
		BackupID:       id,
		IncludesImages: true,
		Counts:         backupCounts(data),
	}
	if err := writeBackupManifestFile(backupDir, manifest); err != nil {
		return nil, err
	}
	return manifest, nil
}

func writeBackupManifestFile(dir string, manifest *backupManifest) error {
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "manifest.json"), append(data, '\n'), 0o644)
}

func writeBackupDataFile(dir string, data backupData) error {
	payload, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "data.json"), append(payload, '\n'), 0o644)
}

func exportBackupData(db *sql.DB) (backupData, error) {
	out := make(backupData, len(backupTableOrder))
	for _, table := range backupTableOrder {
		rows, err := exportTableRows(db, table)
		if err != nil {
			return nil, err
		}
		out[table] = rows
	}
	return out, nil
}

func exportTableRows(db *sql.DB, table string) ([]map[string]any, error) {
	rows, err := db.Query(fmt.Sprintf(`SELECT * FROM %q ORDER BY rowid`, table))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	var out []map[string]any
	for rows.Next() {
		values := make([]any, len(cols))
		scanArgs := make([]any, len(cols))
		for i := range values {
			scanArgs[i] = &values[i]
		}
		if err := rows.Scan(scanArgs...); err != nil {
			return nil, err
		}
		row := make(map[string]any, len(cols))
		for i, col := range cols {
			row[col] = normalizeBackupValue(values[i])
		}
		out = append(out, row)
	}
	if out == nil {
		out = []map[string]any{}
	}
	return out, rows.Err()
}

func normalizeBackupValue(v any) any {
	switch value := v.(type) {
	case nil:
		return nil
	case []byte:
		return string(value)
	case time.Time:
		return value.UTC().Format(time.RFC3339)
	default:
		return value
	}
}

func backupCounts(data backupData) map[string]int {
	counts := make(map[string]int, len(backupSummaryTables))
	for _, table := range backupSummaryTables {
		counts[table] = len(data[table])
	}
	return counts
}

func backupImagePaths(data backupData) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, row := range data["words"] {
		raw, ok := row["image_path"].(string)
		if !ok {
			continue
		}
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			continue
		}
		if !strings.HasPrefix(trimmed, "images/words/") {
			continue
		}
		if _, exists := seen[trimmed]; exists {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	slices.Sort(out)
	return out
}

func copyBackupImages(backupDir string, data backupData) error {
	for _, relPath := range backupImagePaths(data) {
		src := filepath.Join("static", filepath.FromSlash(relPath))
		dst := filepath.Join(backupDir, filepath.FromSlash(relPath))
		if err := copyFile(src, dst); err != nil {
			return fmt.Errorf("copy backup image %s: %w", relPath, err)
		}
	}
	return nil
}

func readBackupDataFile(id string) (backupData, error) {
	if err := ensureValidBackupID(id); err != nil {
		return nil, err
	}
	file, err := os.Open(filepath.Join(backupDirByID(id), "data.json"))
	if err != nil {
		return nil, err
	}
	defer file.Close()

	dec := json.NewDecoder(file)
	dec.UseNumber()
	var data backupData
	if err := dec.Decode(&data); err != nil {
		return nil, err
	}
	return data, nil
}

func validateBackup(id string, data backupData, manifest *backupManifest) error {
	if manifest.FormatVersion != backupFormatVersion {
		return fmt.Errorf("unsupported backup format version %d", manifest.FormatVersion)
	}
	for _, table := range backupTableOrder {
		if _, ok := data[table]; !ok {
			return fmt.Errorf("backup missing table %s", table)
		}
	}
	for _, relPath := range backupImagePaths(data) {
		if !strings.HasPrefix(relPath, "images/words/") {
			return fmt.Errorf("invalid image path %q", relPath)
		}
		if _, err := os.Stat(filepath.Join(backupDirByID(id), filepath.FromSlash(relPath))); err != nil {
			return fmt.Errorf("backup missing image %s", relPath)
		}
	}
	return nil
}

func restoreBackup(db *sql.DB, id string, createSafetyBackup bool) error {
	if err := ensureValidBackupID(id); err != nil {
		return err
	}
	manifest, err := readBackupManifest(id)
	if err != nil {
		return err
	}
	data, err := readBackupDataFile(id)
	if err != nil {
		return err
	}
	if err := validateBackup(id, data, manifest); err != nil {
		return err
	}

	if createSafetyBackup {
		if _, err := createBackup(db); err != nil {
			return fmt.Errorf("create safety backup: %w", err)
		}
	}

	columnMap := make(map[string][]string, len(backupTableOrder))
	for _, table := range backupTableOrder {
		cols, err := tableColumnNames(db, table)
		if err != nil {
			return err
		}
		columnMap[table] = cols
	}

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err := clearUserData(tx); err != nil {
		return err
	}
	for _, table := range backupTableOrder {
		if err := importTableRows(tx, table, columnMap[table], data[table]); err != nil {
			return fmt.Errorf("restore %s: %w", table, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	if err := markBlacklistedWordsUntracked(db); err != nil {
		return err
	}

	if err := restoreBackupImages(id); err != nil {
		return err
	}
	return nil
}

func tableColumnNames(db *sql.DB, table string) ([]string, error) {
	cols, err := listColumns(db, table)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(cols))
	for _, col := range cols {
		names = append(names, col.Name)
	}
	return names, nil
}

func clearUserData(tx *sql.Tx) error {
	for _, table := range backupClearOrder {
		if _, err := tx.Exec(fmt.Sprintf(`DELETE FROM %q`, table)); err != nil {
			return err
		}
	}
	for _, table := range backupClearOrder {
		if _, err := tx.Exec(`DELETE FROM sqlite_sequence WHERE name = ?`, table); err != nil {
			if strings.Contains(err.Error(), "no such table: sqlite_sequence") {
				return nil
			}
			return err
		}
	}
	return nil
}

func importTableRows(tx *sql.Tx, table string, columns []string, rows []map[string]any) error {
	if len(rows) == 0 {
		return nil
	}
	quotedCols := make([]string, len(columns))
	placeholders := make([]string, len(columns))
	for i, col := range columns {
		quotedCols[i] = fmt.Sprintf(`%q`, col)
		placeholders[i] = "?"
	}
	stmt, err := tx.Prepare(fmt.Sprintf(
		`INSERT INTO %q (%s) VALUES (%s)`,
		table,
		strings.Join(quotedCols, ", "),
		strings.Join(placeholders, ", "),
	))
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, row := range rows {
		args := make([]any, len(columns))
		for i, col := range columns {
			args[i] = normalizeImportedValue(row[col])
		}
		if _, err := stmt.Exec(args...); err != nil {
			return err
		}
	}
	return nil
}

func normalizeImportedValue(v any) any {
	switch value := v.(type) {
	case nil:
		return nil
	case json.Number:
		if i, err := value.Int64(); err == nil {
			return i
		}
		if f, err := value.Float64(); err == nil {
			return f
		}
		return value.String()
	default:
		return value
	}
}

func restoreBackupImages(id string) error {
	if err := os.RemoveAll(wordImagesDir()); err != nil {
		return err
	}
	srcRoot := filepath.Join(backupDirByID(id), "images", "words")
	if _, err := os.Stat(srcRoot); errors.Is(err, os.ErrNotExist) {
		return os.MkdirAll(wordImagesDir(), 0o755)
	} else if err != nil {
		return err
	}
	return filepath.WalkDir(srcRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(srcRoot, path)
		if err != nil {
			return err
		}
		dst := filepath.Join(wordImagesDir(), rel)
		if d.IsDir() {
			return os.MkdirAll(dst, 0o755)
		}
		return copyFile(path, dst)
	})
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o644)
}

func apiListBackups() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		backups, err := listBackups()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]any{"backups": backups})
	}
}

func apiCreateBackup(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		manifest, err := createBackup(db)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSONStatus(w, http.StatusCreated, manifest)
	}
}

func apiRestoreBackup(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimSpace(chi.URLParam(r, "id"))
		if err := ensureValidBackupID(id); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		var body struct {
			CreateSafetyBackup bool `json:"createSafetyBackup"`
		}
		if r.Body != nil {
			defer r.Body.Close()
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil && !errors.Is(err, io.EOF) {
				http.Error(w, "bad request", http.StatusBadRequest)
				return
			}
		}
		if err := restoreBackup(db, id, body.CreateSafetyBackup); err != nil {
			status := http.StatusInternalServerError
			if isBackupValidationError(err) {
				status = http.StatusBadRequest
			}
			http.Error(w, err.Error(), status)
			return
		}
		writeJSON(w, map[string]any{"ok": true})
	}
}

func isBackupValidationError(err error) bool {
	if errors.Is(err, os.ErrNotExist) {
		return true
	}
	var syntaxErr *json.SyntaxError
	if errors.As(err, &syntaxErr) {
		return true
	}
	var typeErr *json.UnmarshalTypeError
	if errors.As(err, &typeErr) {
		return true
	}
	msg := err.Error()
	return strings.Contains(msg, "invalid backup id") ||
		strings.Contains(msg, "unsupported backup format version") ||
		strings.Contains(msg, "backup missing") ||
		strings.Contains(msg, "invalid image path")
}
