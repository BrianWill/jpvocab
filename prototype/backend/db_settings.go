package main

import (
	"database/sql"
	"encoding/json"
)

type drillSettings struct {
	MaxWords      int      `json:"maxWords"`
	RoundSize     int      `json:"roundSize"`
	WordTypes     []string `json:"wordTypes"`
	NewWordTarget int      `json:"newWordTarget"`
}

func getDrillSettings(db *sql.DB) (drillSettings, error) {
	s := drillSettings{
		MaxWords:      100,
		RoundSize:     10,
		WordTypes:     []string{"katakana", "verbs", "nouns", "other"},
		NewWordTarget: 8,
	}
	rows, err := db.Query(`
		SELECT key, value FROM user_settings
		WHERE key IN ('drill_max_words', 'drill_round_size', 'drill_word_types', 'drill_new_word_target')
	`)
	if err != nil {
		return s, err
	}
	defer rows.Close()
	for rows.Next() {
		var k, v string
		rows.Scan(&k, &v)
		switch k {
		case "drill_max_words":
			var n int
			if json.Unmarshal([]byte(v), &n) == nil && n > 0 {
				s.MaxWords = n
			}
		case "drill_round_size":
			var n int
			if json.Unmarshal([]byte(v), &n) == nil && n > 0 {
				s.RoundSize = n
			}
		case "drill_word_types":
			var types []string
			if json.Unmarshal([]byte(v), &types) == nil {
				s.WordTypes = types
			}
		case "drill_new_word_target":
			var n int
			if json.Unmarshal([]byte(v), &n) == nil && n > 0 {
				s.NewWordTarget = n
			}
		}
	}
	return s, rows.Err()
}

func putDrillSettings(db *sql.DB, s drillSettings) error {
	upsert := func(key string, val any) error {
		b, err := json.Marshal(val)
		if err != nil {
			return err
		}
		_, err = db.Exec(`
			INSERT INTO user_settings (key, value) VALUES (?, ?)
			ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
			key, string(b))
		return err
	}

	if s.MaxWords > 0 {
		if err := upsert("drill_max_words", s.MaxWords); err != nil {
			return err
		}
	} else {
		if _, err := db.Exec(`DELETE FROM user_settings WHERE key = 'drill_max_words'`); err != nil {
			return err
		}
	}
	if err := upsert("drill_round_size", s.RoundSize); err != nil {
		return err
	}
	if err := upsert("drill_word_types", s.WordTypes); err != nil {
		return err
	}
	return upsert("drill_new_word_target", s.NewWordTarget)
}
