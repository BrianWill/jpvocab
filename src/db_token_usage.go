package main

import (
	"database/sql"
	"time"
)

// tokenUsageEntry is one row from the token_usage table.
type tokenUsageEntry struct {
	ID           int64     `json:"id"`
	Provider     string    `json:"provider"`
	Model        string    `json:"model"`
	Operation    string    `json:"operation"`
	InputTokens  int       `json:"input_tokens"`
	OutputTokens int       `json:"output_tokens"`
	CalledAt     time.Time `json:"called_at"`
}

// tokenUsageSummaryRow is an aggregate row grouped by provider+model.
type tokenUsageSummaryRow struct {
	Provider     string `json:"provider"`
	Model        string `json:"model"`
	TotalCalls   int    `json:"total_calls"`
	InputTokens  int    `json:"total_input_tokens"`
	OutputTokens int    `json:"total_output_tokens"`
}

// insertTokenUsage records one AI API call's token usage.
// Errors are logged but not fatal — token tracking is best-effort.
func insertTokenUsage(db *sql.DB, provider, model, operation string, inputTokens, outputTokens int) {
	if db == nil {
		return
	}
	db.Exec(
		`INSERT INTO token_usage (provider, model, operation, input_tokens, output_tokens) VALUES (?, ?, ?, ?, ?)`,
		provider, model, operation, inputTokens, outputTokens,
	)
}

// getTokenUsageSummary returns aggregated totals grouped by provider + model, most-used first.
func getTokenUsageSummary(db *sql.DB) ([]tokenUsageSummaryRow, error) {
	rows, err := db.Query(`
		SELECT provider, model,
		       COUNT(*) AS total_calls,
		       SUM(input_tokens) AS total_input,
		       SUM(output_tokens) AS total_output
		FROM token_usage
		GROUP BY provider, model
		ORDER BY total_calls DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []tokenUsageSummaryRow
	for rows.Next() {
		var r tokenUsageSummaryRow
		if err := rows.Scan(&r.Provider, &r.Model, &r.TotalCalls, &r.InputTokens, &r.OutputTokens); err != nil {
			return nil, err
		}
		result = append(result, r)
	}
	return result, nil
}

// getTokenUsageLog returns the most recent token_usage rows, newest first.
func getTokenUsageLog(db *sql.DB, limit int) ([]tokenUsageEntry, error) {
	rows, err := db.Query(`
		SELECT id, provider, model, operation, input_tokens, output_tokens, called_at
		FROM token_usage
		ORDER BY id DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []tokenUsageEntry
	for rows.Next() {
		var e tokenUsageEntry
		if err := rows.Scan(&e.ID, &e.Provider, &e.Model, &e.Operation, &e.InputTokens, &e.OutputTokens, &e.CalledAt); err != nil {
			return nil, err
		}
		result = append(result, e)
	}
	return result, nil
}

// getTokenUsageTotals returns the grand total across all providers.
func getTokenUsageTotals(db *sql.DB) (totalCalls, totalInput, totalOutput int, err error) {
	err = db.QueryRow(`
		SELECT COUNT(*), COALESCE(SUM(input_tokens),0), COALESCE(SUM(output_tokens),0)
		FROM token_usage
	`).Scan(&totalCalls, &totalInput, &totalOutput)
	return
}
