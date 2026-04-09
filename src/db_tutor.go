package main

import (
	"database/sql"
	"fmt"
)

type tutorPromptJSON struct {
	ID           int64  `json:"id"`
	Label        string `json:"label"`
	SystemPrompt string `json:"system_prompt"`
	Greeting     string `json:"greeting"`
	LangInput    string `json:"lang_input"`
	CanRemove    bool   `json:"can_remove"`
}

func listTutorPrompts(db *sql.DB) ([]tutorPromptJSON, error) {
	rows, err := db.Query(`
		SELECT id, label, system_prompt, greeting, lang_input, can_remove
		FROM tutor_prompts ORDER BY id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []tutorPromptJSON{}
	for rows.Next() {
		var p tutorPromptJSON
		var canRemove int
		if err := rows.Scan(&p.ID, &p.Label, &p.SystemPrompt, &p.Greeting, &p.LangInput, &canRemove); err != nil {
			return nil, err
		}
		p.CanRemove = canRemove == 1
		out = append(out, p)
	}
	return out, rows.Err()
}

func insertTutorPrompt(db *sql.DB, label, systemPrompt, greeting, langInput string) (int64, error) {
	if langInput == "" {
		langInput = "en"
	}
	res, err := db.Exec(`
		INSERT INTO tutor_prompts (label, system_prompt, greeting, lang_input, can_remove)
		VALUES (?, ?, ?, ?, 1)
	`, label, systemPrompt, greeting, langInput)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func updateTutorPrompt(db *sql.DB, id int64, label, systemPrompt, greeting, langInput string) error {
	if langInput == "" {
		langInput = "en"
	}
	res, err := db.Exec(`
		UPDATE tutor_prompts
		SET label = ?, system_prompt = ?, greeting = ?, lang_input = ?
		WHERE id = ? AND can_remove = 1
	`, label, systemPrompt, greeting, langInput, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("prompt not found or cannot be edited")
	}
	return nil
}

func deleteTutorPrompt(db *sql.DB, id int64) error {
	var canRemove int
	err := db.QueryRow(`SELECT can_remove FROM tutor_prompts WHERE id = ?`, id).Scan(&canRemove)
	if err == sql.ErrNoRows {
		return fmt.Errorf("prompt not found")
	}
	if err != nil {
		return err
	}
	if canRemove == 0 {
		return fmt.Errorf("cannot delete built-in prompt")
	}
	_, err = db.Exec(`DELETE FROM tutor_prompts WHERE id = ?`, id)
	return err
}

// tutorSystemPromptByID returns the stored system prompt for the given
// tutor_prompts row ID. The prefix is already baked in (prepended at seed/insert
// time), so this returns the value as-is. Falls back to the first prompt.
func tutorSystemPromptByID(db *sql.DB, id int64) string {
	var prompt string
	err := db.QueryRow(`SELECT system_prompt FROM tutor_prompts WHERE id = ?`, id).Scan(&prompt)
	if err != nil {
		db.QueryRow(`SELECT system_prompt FROM tutor_prompts ORDER BY id LIMIT 1`).Scan(&prompt)
	}
	return prompt
}
