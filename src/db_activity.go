package main

import (
	"database/sql"
	"encoding/json"
	"time"
)

// activityStats holds headline stats for the activity page.
type activityStats struct {
	LexiconSize     int `json:"lexiconSize"`
	ActiveWords     int `json:"activeWords"`
	ClearedLifetime int `json:"clearedLifetime"`
	DrillsCleared   int `json:"drillsCleared"`
	// close - mid - far = buckets of how many target drills remaining; close <= x drills, mid <= y drills
	DrillsClose int `json:"drillsClose"`
	DrillsMid   int `json:"drillsMid"`
	DrillsFar   int `json:"drillsFar"`
}

// activityWordEntry is one word entry within a calendar day section.
type activityWordEntry struct {
	Word    string `json:"word"`
	Reading string `json:"reading"`
	Meaning string `json:"meaning"`
	Knew    *bool  `json:"knew,omitempty"` // set only for drilled entries
}

// activityDay holds the drilled/added/cleared events for a single calendar day.
type activityDay struct {
	Drilled []activityWordEntry `json:"drilled"`
	Added   []activityWordEntry `json:"added"`
	Cleared []activityWordEntry `json:"cleared"`
}

// activityCalendar is the full response for the /api/activity/calendar endpoint.
type activityCalendar struct {
	Today        string                 `json:"today"`
	HistoryStart string                 `json:"historyStart"`
	Days         map[string]activityDay `json:"days"`
}

type drillSidebarItem struct {
	Word   wordJSON `json:"word"`
	Status string   `json:"status"`
}

type drillLastAnswered struct {
	Word wordJSON `json:"word"`
	Knew bool     `json:"knew"`
}

type drillSessionState struct {
	PoolSize      int                `json:"poolSize"`
	RoundSize     int                `json:"roundSize"`
	Round         int                `json:"round"`
	DoneCount     int                `json:"doneCount"`
	ActiveFilters []string           `json:"activeFilters"`
	Pool          []wordJSON         `json:"pool"`
	Redo          []wordJSON         `json:"redo"`
	Remaining     []wordJSON         `json:"remaining"`
	SidebarItems  []drillSidebarItem `json:"sidebarItems"`
	LastAnswered  *drillLastAnswered `json:"lastAnswered,omitempty"`
}

type drillSessionJSON struct {
	ID        int64             `json:"id"`
	StartedAt string            `json:"startedAt"`
	State     drillSessionState `json:"state"`
}

func normaliseDrillSessionState(state *drillSessionState) {
	if state.ActiveFilters == nil {
		state.ActiveFilters = []string{}
	}
	if state.Pool == nil {
		state.Pool = []wordJSON{}
	}
	if state.Redo == nil {
		state.Redo = []wordJSON{}
	}
	if state.Remaining == nil {
		state.Remaining = []wordJSON{}
	}
	if state.SidebarItems == nil {
		state.SidebarItems = []drillSidebarItem{}
	}
}

func sessionStateJSON(state drillSessionState) (string, error) {
	normaliseDrillSessionState(&state)
	b, err := json.Marshal(state)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func isDrillSessionComplete(state drillSessionState) bool {
	return len(state.Pool) == 0 && len(state.Redo) == 0 && len(state.Remaining) == 0
}

// createDrillSession closes any existing active drill and inserts a new session.
func createDrillSession(db *sql.DB, state drillSessionState) (int64, error) {
	stateJSON, err := sessionStateJSON(state)
	if err != nil {
		return 0, err
	}

	tx, err := db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`UPDATE drill_sessions SET state_json = '{}', completed_at = datetime('now') WHERE completed_at IS NULL`); err != nil {
		return 0, err
	}

	res, err := tx.Exec(`INSERT INTO drill_sessions (state_json) VALUES (?)`, stateJSON)
	if err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func getCurrentDrillSession(db *sql.DB) (*drillSessionJSON, error) {
	var s drillSessionJSON
	var stateJSON string
	err := db.QueryRow(`
		SELECT id, started_at, COALESCE(state_json, '{}')
		FROM drill_sessions
		WHERE completed_at IS NULL
		ORDER BY started_at DESC, id DESC
		LIMIT 1
	`).Scan(&s.ID, &s.StartedAt, &stateJSON)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(stateJSON), &s.State); err != nil {
		return nil, err
	}
	normaliseDrillSessionState(&s.State)
	return &s, nil
}

// recordDrillAnswer inserts one row into drill_answers and updates the word's
// counts and timestamps. For a correct answer: drill_count++, last_drilled_at,
// and target_reached_at (first time drill_count reaches drill_target). For an
// incorrect answer: incorrect_count++, last_drilled_at.
func recordDrillAnswer(db *sql.DB, sessionID, wordID int64, correct bool, state drillSessionState) error {
	stateJSON, err := sessionStateJSON(state)
	if err != nil {
		return err
	}

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	correctInt := 0
	if correct {
		correctInt = 1
	}
	if _, err := tx.Exec(
		`INSERT INTO drill_answers (session_id, word_id, correct) VALUES (?, ?, ?)`,
		sessionID, wordID, correctInt,
	); err != nil {
		return err
	}

	if correct {
		_, err := tx.Exec(`
			UPDATE words SET
				drill_count     = drill_count + 1,
				last_drilled_at = datetime('now'),
				drill_target    = MAX(drill_target, drill_count + 1),
				target_reached_at = CASE
					WHEN target_reached_at IS NULL AND (drill_count + 1) >= drill_target
					THEN datetime('now')
					ELSE target_reached_at
				END
			WHERE id = ?
		`, wordID)
		if err != nil {
			return err
		}
	} else {
		_, err := tx.Exec(`
			UPDATE words SET
				incorrect_count = incorrect_count + 1,
				last_drilled_at = datetime('now')
			WHERE id = ?
		`, wordID)
		if err != nil {
			return err
		}
	}

	if isDrillSessionComplete(state) {
		if _, err := tx.Exec(`
			UPDATE drill_sessions
			SET state_json = '{}', completed_at = datetime('now')
			WHERE id = ?
		`, sessionID); err != nil {
			return err
		}
		return tx.Commit()
	}

	if _, err := tx.Exec(`
		UPDATE drill_sessions
		SET state_json = ?, completed_at = NULL
		WHERE id = ?
	`, stateJSON, sessionID); err != nil {
		return err
	}

	return tx.Commit()
}

// getActivityStats returns headline statistics computed from the words table.
func getActivityStats(db *sql.DB) (activityStats, error) {
	var s activityStats
	err := db.QueryRow(`
		SELECT
			COUNT(*),
			SUM(CASE WHEN drill_count < drill_target THEN 1 ELSE 0 END),
			SUM(CASE WHEN target_reached_at IS NOT NULL THEN 1 ELSE 0 END),
			SUM(CASE WHEN drill_count >= drill_target THEN 1 ELSE 0 END),
			SUM(CASE WHEN drill_count < drill_target AND (drill_target - drill_count) <= 4 THEN 1 ELSE 0 END),
			SUM(CASE WHEN drill_count < drill_target AND (drill_target - drill_count) > 4 AND (drill_target - drill_count) <= 8 THEN 1 ELSE 0 END),
			SUM(CASE WHEN drill_count < drill_target AND (drill_target - drill_count) > 8 THEN 1 ELSE 0 END)
		FROM words
		WHERE in_lexicon = 1
	`).Scan(&s.LexiconSize, &s.ActiveWords, &s.ClearedLifetime, &s.DrillsCleared, &s.DrillsClose, &s.DrillsMid, &s.DrillsFar)
	return s, err
}

// getActivityCalendar builds the date-keyed calendar data from drill_answers,
// words.created_at, and words.target_reached_at.
func getActivityCalendar(db *sql.DB) (activityCalendar, error) {
	days := make(map[string]activityDay)

	// Drilled entries — one entry per (word, date), marked wrong if any answer
	// that day was wrong (MIN(correct) = 0 if any incorrect answer exists).
	rows, err := db.Query(`
		SELECT w.word, COALESCE(w.reading,''), COALESCE(w.meaning,''),
		       MIN(da.correct), DATE(da.answered_at)
		FROM drill_answers da
		JOIN words w ON w.id = da.word_id
		GROUP BY w.id, DATE(da.answered_at)
		ORDER BY MIN(da.answered_at)
	`)
	if err != nil {
		return activityCalendar{}, err
	}
	for rows.Next() {
		var word, reading, meaning, dateStr string
		var correct int
		if err := rows.Scan(&word, &reading, &meaning, &correct, &dateStr); err != nil {
			rows.Close()
			return activityCalendar{}, err
		}
		knew := correct == 1
		d := days[dateStr]
		d.Drilled = append(d.Drilled, activityWordEntry{Word: word, Reading: reading, Meaning: meaning, Knew: &knew})
		days[dateStr] = d
	}
	if err := rows.Close(); err != nil {
		return activityCalendar{}, err
	}

	// Added entries — one entry per word on its creation date.
	rows2, err := db.Query(`
		SELECT word, COALESCE(reading,''), COALESCE(meaning,''), DATE(created_at)
		FROM words
		WHERE in_lexicon = 1
		ORDER BY created_at
	`)
	if err != nil {
		return activityCalendar{}, err
	}
	for rows2.Next() {
		var word, reading, meaning, dateStr string
		if err := rows2.Scan(&word, &reading, &meaning, &dateStr); err != nil {
			rows2.Close()
			return activityCalendar{}, err
		}
		d := days[dateStr]
		d.Added = append(d.Added, activityWordEntry{Word: word, Reading: reading, Meaning: meaning})
		days[dateStr] = d
	}
	if err := rows2.Close(); err != nil {
		return activityCalendar{}, err
	}

	// Cleared entries — words that first reached their drill target on a given date.
	rows3, err := db.Query(`
		SELECT word, COALESCE(reading,''), COALESCE(meaning,''), DATE(target_reached_at)
		FROM words
		WHERE in_lexicon = 1 AND target_reached_at IS NOT NULL
		ORDER BY target_reached_at
	`)
	if err != nil {
		return activityCalendar{}, err
	}
	for rows3.Next() {
		var word, reading, meaning, dateStr string
		if err := rows3.Scan(&word, &reading, &meaning, &dateStr); err != nil {
			rows3.Close()
			return activityCalendar{}, err
		}
		d := days[dateStr]
		d.Cleared = append(d.Cleared, activityWordEntry{Word: word, Reading: reading, Meaning: meaning})
		days[dateStr] = d
	}
	if err := rows3.Close(); err != nil {
		return activityCalendar{}, err
	}

	// Ensure every day's slices are non-nil so they encode as [] not null.
	for k, v := range days {
		if v.Drilled == nil {
			v.Drilled = []activityWordEntry{}
		}
		if v.Added == nil {
			v.Added = []activityWordEntry{}
		}
		if v.Cleared == nil {
			v.Cleared = []activityWordEntry{}
		}
		days[k] = v
	}

	today := time.Now().UTC().Format("2006-01-02")

	// historyStart is the Sunday of the week containing the earliest activity date.
	historyStart := today
	for dateStr := range days {
		if dateStr < historyStart {
			historyStart = dateStr
		}
	}
	historyStart = calendarWeekSunday(historyStart)

	return activityCalendar{Today: today, HistoryStart: historyStart, Days: days}, nil
}

// calendarWeekSunday returns the Sunday of the week that contains dateStr (YYYY-MM-DD).
func calendarWeekSunday(dateStr string) string {
	d, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return dateStr
	}
	dayOfWeek := int(d.Weekday()) // 0 = Sunday
	sun := d.AddDate(0, 0, -dayOfWeek)
	return sun.Format("2006-01-02")
}
