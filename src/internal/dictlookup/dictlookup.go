package dictlookup

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

type KanjiInfo struct {
	Character string   `json:"character"`
	Reading   string   `json:"reading,omitempty"`
	Readings  []string `json:"readings"`
	Meanings  []string `json:"meanings"`
}

type WordInfo struct {
	Word         string      `json:"word"`
	Reading      string      `json:"reading"`
	PartOfSpeech string      `json:"part_of_speech"`
	Meaning      string      `json:"meaning"`
	Glosses      []string    `json:"glosses"`
	Kanji        []KanjiInfo `json:"kanji"`
}

type wordCandidate struct {
	WordID string
	Score  int
}

type kanaRow struct {
	Text           string
	Common         int
	AppliesToKanji []string
}

type senseRow struct {
	ID             int64
	PartOfSpeech   []string
	AppliesToKanji []string
	AppliesToKana  []string
}

type kanjiReadingCandidate struct {
	Display    string
	Normalized string
}

func LookupWordInDB(dictDB *sql.DB, word string) (*WordInfo, error) {
	word = strings.TrimSpace(word)
	if word == "" {
		return nil, nil
	}
	info, err := lookupWordFromTable(dictDB, word)
	if err == nil || !isMissingLookupTableErr(err) {
		return info, err
	}
	return lookupWordFromNormalizedTables(dictDB, word)
}

func RebuildLookupTable(dictDB *sql.DB) error {
	if _, err := dictDB.Exec(`
		DROP TABLE IF EXISTS dict_word_lookup;
	`); err != nil {
		return err
	}
	if _, err := dictDB.Exec(`
		CREATE TABLE dict_word_lookup (
			lookup_text    TEXT PRIMARY KEY,
			word           TEXT NOT NULL,
			reading        TEXT NOT NULL,
			part_of_speech TEXT NOT NULL,
			meaning        TEXT NOT NULL,
			glosses_json   TEXT NOT NULL,
			kanji_json     TEXT NOT NULL
		);
	`); err != nil {
		return err
	}

	lookupTexts, err := allLookupTexts(dictDB)
	if err != nil {
		return err
	}

	type lookupRow struct {
		LookupText   string
		Word         string
		Reading      string
		PartOfSpeech string
		Meaning      string
		GlossesJSON  string
		KanjiJSON    string
	}
	rows := make([]lookupRow, 0, len(lookupTexts))
	for _, lookupText := range lookupTexts {
		info, err := lookupWordFromNormalizedTables(dictDB, lookupText)
		if err != nil {
			return err
		}
		if info == nil {
			continue
		}
		glossesJSON, err := json.Marshal(info.Glosses)
		if err != nil {
			return err
		}
		kanjiJSON, err := json.Marshal(info.Kanji)
		if err != nil {
			return err
		}
		rows = append(rows, lookupRow{
			LookupText:   lookupText,
			Word:         info.Word,
			Reading:      info.Reading,
			PartOfSpeech: info.PartOfSpeech,
			Meaning:      info.Meaning,
			GlossesJSON:  string(glossesJSON),
			KanjiJSON:    string(kanjiJSON),
		})
	}

	tx, err := dictDB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT INTO dict_word_lookup (
			lookup_text, word, reading, part_of_speech, meaning, glosses_json, kanji_json
		) VALUES (?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, row := range rows {
		if _, err := stmt.Exec(
			row.LookupText,
			row.Word,
			row.Reading,
			row.PartOfSpeech,
			row.Meaning,
			row.GlossesJSON,
			row.KanjiJSON,
		); err != nil {
			return err
		}
	}

	if _, err := tx.Exec(`CREATE INDEX idx_dict_word_lookup_reading ON dict_word_lookup(reading)`); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return RebuildKanjiLookupTable(dictDB)
}

func RebuildKanjiLookupTable(dictDB *sql.DB) error {
	if _, err := dictDB.Exec(`
		DROP TABLE IF EXISTS dict_kanji_lookup;
	`); err != nil {
		return err
	}
	if _, err := dictDB.Exec(`
		CREATE TABLE dict_kanji_lookup (
			literal       TEXT PRIMARY KEY,
			meanings_json TEXT NOT NULL,
			readings_json TEXT NOT NULL
		);
	`); err != nil {
		return err
	}

	rows, err := dictDB.Query(`
		SELECT literal
		FROM kanjidic
		ORDER BY literal
	`)
	if err != nil {
		return err
	}
	defer rows.Close()

	var literals []string
	for rows.Next() {
		var literal string
		if err := rows.Scan(&literal); err != nil {
			return err
		}
		literals = append(literals, literal)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	type kanjiRow struct {
		Literal      string
		MeaningsJSON string
		ReadingsJSON string
	}
	values := make([]kanjiRow, 0, len(literals))
	for _, literal := range literals {
		info, _, err := loadDictionaryKanjiInfoFromNormalizedTables(dictDB, literal)
		if err != nil {
			return err
		}
		meaningsJSON, err := json.Marshal(info.Meanings)
		if err != nil {
			return err
		}
		readingsJSON, err := json.Marshal(info.Readings)
		if err != nil {
			return err
		}
		values = append(values, kanjiRow{
			Literal:      literal,
			MeaningsJSON: string(meaningsJSON),
			ReadingsJSON: string(readingsJSON),
		})
	}

	tx, err := dictDB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT INTO dict_kanji_lookup (literal, meanings_json, readings_json)
		VALUES (?, ?, ?)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, row := range values {
		if _, err := stmt.Exec(row.Literal, row.MeaningsJSON, row.ReadingsJSON); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func allLookupTexts(dictDB *sql.DB) ([]string, error) {
	rows, err := dictDB.Query(`
		SELECT text FROM jmdict_kanji
		UNION
		SELECT text FROM jmdict_kana
		ORDER BY text
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var text string
		if err := rows.Scan(&text); err != nil {
			return nil, err
		}
		text = strings.TrimSpace(text)
		if text != "" {
			out = append(out, text)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func lookupWordFromTable(dictDB *sql.DB, word string) (*WordInfo, error) {
	var info WordInfo
	var glossesJSON string
	var kanjiJSON string
	err := dictDB.QueryRow(`
		SELECT word, reading, part_of_speech, meaning, glosses_json, kanji_json
		FROM dict_word_lookup
		WHERE lookup_text = ?
	`, word).Scan(
		&info.Word,
		&info.Reading,
		&info.PartOfSpeech,
		&info.Meaning,
		&glossesJSON,
		&kanjiJSON,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(glossesJSON), &info.Glosses); err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(kanjiJSON), &info.Kanji); err != nil {
		return nil, err
	}
	if info.Glosses == nil {
		info.Glosses = []string{}
	}
	if info.Kanji == nil {
		info.Kanji = []KanjiInfo{}
	}
	return &info, nil
}

func isMissingLookupTableErr(err error) bool {
	return err != nil && strings.Contains(strings.ToLower(err.Error()), "no such table: dict_word_lookup")
}

func lookupWordFromNormalizedTables(dictDB *sql.DB, word string) (*WordInfo, error) {
	candidate, matchedByKanji, err := bestDictionaryCandidate(dictDB, word)
	if err != nil {
		return nil, err
	}
	if candidate == nil {
		return nil, nil
	}

	reading, err := selectDictionaryReading(dictDB, candidate.WordID, word, matchedByKanji)
	if err != nil {
		return nil, err
	}
	if reading == "" {
		reading = word
	}

	partOfSpeech, glosses, err := selectDictionarySenseData(dictDB, candidate.WordID, word, reading, matchedByKanji)
	if err != nil {
		return nil, err
	}

	kanji, err := lookupDictionaryKanjiInfo(dictDB, word, reading)
	if err != nil {
		return nil, err
	}

	return &WordInfo{
		Word:         word,
		Reading:      reading,
		PartOfSpeech: partOfSpeech,
		Meaning:      strings.Join(glosses, "; "),
		Glosses:      glosses,
		Kanji:        kanji,
	}, nil
}

func bestDictionaryCandidate(dictDB *sql.DB, word string) (*wordCandidate, bool, error) {
	candidates := map[string]*wordCandidate{}
	matchedByKanji := map[string]bool{}

	kanjiRows, err := dictDB.Query(`
		SELECT word_id, MAX(common)
		FROM jmdict_kanji
		WHERE text = ?
		GROUP BY word_id
	`, word)
	if err != nil {
		return nil, false, err
	}
	for kanjiRows.Next() {
		var wordID string
		var common int
		if err := kanjiRows.Scan(&wordID, &common); err != nil {
			kanjiRows.Close()
			return nil, false, err
		}
		candidates[wordID] = &wordCandidate{WordID: wordID, Score: 300 + common*10}
		matchedByKanji[wordID] = true
	}
	if err := kanjiRows.Err(); err != nil {
		kanjiRows.Close()
		return nil, false, err
	}
	kanjiRows.Close()

	kanaRows, err := dictDB.Query(`
		SELECT word_id, MAX(common)
		FROM jmdict_kana
		WHERE text = ?
		GROUP BY word_id
	`, word)
	if err != nil {
		return nil, false, err
	}
	for kanaRows.Next() {
		var wordID string
		var common int
		if err := kanaRows.Scan(&wordID, &common); err != nil {
			kanaRows.Close()
			return nil, false, err
		}
		score := 200 + common*10
		if cur, ok := candidates[wordID]; ok {
			if score > cur.Score {
				cur.Score = score
			}
			continue
		}
		candidates[wordID] = &wordCandidate{WordID: wordID, Score: score}
	}
	if err := kanaRows.Err(); err != nil {
		kanaRows.Close()
		return nil, false, err
	}
	kanaRows.Close()

	if len(candidates) == 0 {
		return nil, false, nil
	}

	var best *wordCandidate
	for _, c := range candidates {
		if best == nil || c.Score > best.Score || (c.Score == best.Score && c.WordID < best.WordID) {
			best = c
		}
	}
	return best, matchedByKanji[best.WordID], nil
}

func selectDictionaryReading(dictDB *sql.DB, wordID, word string, matchedByKanji bool) (string, error) {
	rows, err := dictDB.Query(`
		SELECT text, common, applies_to_kanji
		FROM jmdict_kana
		WHERE word_id = ?
		ORDER BY common DESC, text
	`, wordID)
	if err != nil {
		return "", err
	}
	defer rows.Close()

	var best string
	bestScore := -1
	for rows.Next() {
		var row kanaRow
		var appliesJSON string
		if err := rows.Scan(&row.Text, &row.Common, &appliesJSON); err != nil {
			return "", err
		}
		if err := json.Unmarshal([]byte(appliesJSON), &row.AppliesToKanji); err != nil {
			row.AppliesToKanji = nil
		}
		score := row.Common * 10
		if row.Text == word {
			score += 20
		}
		if matchedByKanji && jsonListApplies(row.AppliesToKanji, word) {
			score += 50
		}
		if !matchedByKanji {
			score += 5
		}
		if score > bestScore {
			best = row.Text
			bestScore = score
		}
	}
	if err := rows.Err(); err != nil {
		return "", err
	}
	return best, nil
}

func selectDictionarySenseData(dictDB *sql.DB, wordID, word, reading string, matchedByKanji bool) (string, []string, error) {
	rows, err := dictDB.Query(`
		SELECT id, pos, applies_to_kanji, applies_to_kana
		FROM jmdict_senses
		WHERE word_id = ?
		ORDER BY id
	`, wordID)
	if err != nil {
		return "", nil, err
	}
	defer rows.Close()

	var senses []senseRow
	var fallbackSenses []senseRow
	for rows.Next() {
		var row senseRow
		var posJSON string
		var appliesKanjiJSON string
		var appliesKanaJSON string
		if err := rows.Scan(&row.ID, &posJSON, &appliesKanjiJSON, &appliesKanaJSON); err != nil {
			return "", nil, err
		}
		if err := json.Unmarshal([]byte(posJSON), &row.PartOfSpeech); err != nil {
			row.PartOfSpeech = nil
		}
		if err := json.Unmarshal([]byte(appliesKanjiJSON), &row.AppliesToKanji); err != nil {
			row.AppliesToKanji = nil
		}
		if err := json.Unmarshal([]byte(appliesKanaJSON), &row.AppliesToKana); err != nil {
			row.AppliesToKana = nil
		}
		fallbackSenses = append(fallbackSenses, row)
		if matchedByKanji {
			if jsonListApplies(row.AppliesToKanji, word) && jsonListApplies(row.AppliesToKana, reading) {
				senses = append(senses, row)
			}
			continue
		}
		if jsonListApplies(row.AppliesToKana, reading) {
			senses = append(senses, row)
		}
	}
	if err := rows.Err(); err != nil {
		return "", nil, err
	}
	if len(senses) == 0 {
		senses = fallbackSenses
	}

	senseIDs := make([]int64, 0, len(senses))
	var posTags []string
	for _, sense := range senses {
		senseIDs = append(senseIDs, sense.ID)
		posTags = append(posTags, sense.PartOfSpeech...)
	}
	glosses, err := loadSenseGlosses(dictDB, senseIDs)
	if err != nil {
		return "", nil, err
	}
	return CanonicalPartOfSpeech(tagsLower(posTags)), glosses, nil
}

func loadSenseGlosses(dictDB *sql.DB, senseIDs []int64) ([]string, error) {
	seen := map[string]struct{}{}
	var glosses []string
	for _, senseID := range senseIDs {
		rows, err := dictDB.Query(`
			SELECT text
			FROM jmdict_glosses
			WHERE sense_id = ?
			ORDER BY rowid
		`, senseID)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var gloss string
			if err := rows.Scan(&gloss); err != nil {
				rows.Close()
				return nil, err
			}
			gloss = strings.TrimSpace(gloss)
			if gloss == "" {
				continue
			}
			if _, ok := seen[gloss]; ok {
				continue
			}
			seen[gloss] = struct{}{}
			glosses = append(glosses, gloss)
			if len(glosses) >= 3 {
				rows.Close()
				return glosses, nil
			}
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return nil, err
		}
		rows.Close()
	}
	return glosses, nil
}

func lookupDictionaryKanjiInfo(dictDB *sql.DB, word, reading string) ([]KanjiInfo, error) {
	wordRunes := []rune(word)
	kanjiIdxs := make([]int, 0, len(wordRunes))
	for i, r := range wordRunes {
		if isDictionaryKanjiRune(r) {
			kanjiIdxs = append(kanjiIdxs, i)
		}
	}
	if len(kanjiIdxs) == 0 {
		return []KanjiInfo{}, nil
	}

	out := make([]KanjiInfo, 0, len(kanjiIdxs))
	candidates := make([][]kanjiReadingCandidate, 0, len(kanjiIdxs))
	for _, idx := range kanjiIdxs {
		info, readingCandidates, err := LookupKanjiInDB(dictDB, string(wordRunes[idx]))
		if err != nil {
			return nil, err
		}
		out = append(out, info)
		candidates = append(candidates, readingCandidates)
	}

	if inferred, ok := inferDictionaryKanjiReadings(wordRunes, kanaToHiragana(reading), kanjiIdxs, candidates); ok {
		for i := range out {
			out[i].Reading = inferred[i]
		}
	}
	return out, nil
}

func LookupKanjiInDB(dictDB *sql.DB, character string) (KanjiInfo, []kanjiReadingCandidate, error) {
	info, candidates, err := lookupKanjiFromTable(dictDB, character)
	if err == nil || !isMissingKanjiLookupTableErr(err) {
		return info, candidates, err
	}
	return loadDictionaryKanjiInfoFromNormalizedTables(dictDB, character)
}

func lookupKanjiFromTable(dictDB *sql.DB, character string) (KanjiInfo, []kanjiReadingCandidate, error) {
	info := KanjiInfo{
		Character: strings.TrimSpace(character),
		Readings:  []string{},
		Meanings:  []string{},
	}
	if info.Character == "" {
		return info, nil, nil
	}

	var meaningsJSON string
	var readingsJSON string
	err := dictDB.QueryRow(`
		SELECT meanings_json, readings_json
		FROM dict_kanji_lookup
		WHERE literal = ?
	`, info.Character).Scan(&meaningsJSON, &readingsJSON)
	if err == sql.ErrNoRows {
		return info, nil, nil
	}
	if err != nil {
		return info, nil, err
	}
	if err := json.Unmarshal([]byte(meaningsJSON), &info.Meanings); err != nil {
		return info, nil, err
	}
	if err := json.Unmarshal([]byte(readingsJSON), &info.Readings); err != nil {
		return info, nil, err
	}
	if info.Meanings == nil {
		info.Meanings = []string{}
	}
	if info.Readings == nil {
		info.Readings = []string{}
	}
	candidates := make([]kanjiReadingCandidate, 0, len(info.Readings))
	for _, display := range info.Readings {
		normalized := kanaToHiragana(display)
		if normalized == "" {
			continue
		}
		candidates = append(candidates, kanjiReadingCandidate{
			Display:    display,
			Normalized: normalized,
		})
	}
	return info, candidates, nil
}

func isMissingKanjiLookupTableErr(err error) bool {
	return err != nil && strings.Contains(strings.ToLower(err.Error()), "no such table: dict_kanji_lookup")
}

func loadDictionaryKanjiInfoFromNormalizedTables(dictDB *sql.DB, character string) (KanjiInfo, []kanjiReadingCandidate, error) {
	info := KanjiInfo{
		Character: character,
		Readings:  []string{},
		Meanings:  []string{},
	}

	readingRows, err := dictDB.Query(`
		SELECT value
		FROM kanjidic_readings
		WHERE literal = ?
		ORDER BY type, value
	`, character)
	if err != nil {
		return info, nil, err
	}
	defer readingRows.Close()

	var candidates []kanjiReadingCandidate
	seenReadings := map[string]struct{}{}
	for readingRows.Next() {
		var raw string
		if err := readingRows.Scan(&raw); err != nil {
			return info, nil, err
		}
		display := dictionaryReadingDisplayForm(raw)
		if display == "" {
			continue
		}
		if _, ok := seenReadings[display]; !ok {
			seenReadings[display] = struct{}{}
			info.Readings = append(info.Readings, display)
		}
		normalized := kanaToHiragana(display)
		if normalized != "" {
			candidates = append(candidates, kanjiReadingCandidate{
				Display:    display,
				Normalized: normalized,
			})
		}
	}
	if err := readingRows.Err(); err != nil {
		return info, nil, err
	}

	meaningRows, err := dictDB.Query(`
		SELECT value
		FROM kanjidic_meanings
		WHERE literal = ?
		ORDER BY value
	`, character)
	if err != nil {
		return info, nil, err
	}
	defer meaningRows.Close()

	seenMeanings := map[string]struct{}{}
	for meaningRows.Next() {
		var meaning string
		if err := meaningRows.Scan(&meaning); err != nil {
			return info, nil, err
		}
		meaning = strings.TrimSpace(meaning)
		if meaning == "" {
			continue
		}
		if _, ok := seenMeanings[meaning]; ok {
			continue
		}
		seenMeanings[meaning] = struct{}{}
		info.Meanings = append(info.Meanings, meaning)
	}
	if err := meaningRows.Err(); err != nil {
		return info, nil, err
	}

	sort.Strings(info.Meanings)
	return info, candidates, nil
}

func inferDictionaryKanjiReadings(wordRunes []rune, reading string, kanjiIdxs []int, candidates [][]kanjiReadingCandidate) ([]string, bool) {
	readingRunes := []rune(reading)
	memo := map[string]bool{}
	var walk func(wordPos, kanjiPos, readingPos int) ([]string, bool)
	walk = func(wordPos, kanjiPos, readingPos int) ([]string, bool) {
		key := fmt.Sprintf("%d:%d:%d", wordPos, kanjiPos, readingPos)
		if memo[key] {
			return nil, false
		}
		if wordPos == len(wordRunes) {
			if readingPos == len(readingRunes) && kanjiPos == len(kanjiIdxs) {
				return []string{}, true
			}
			memo[key] = true
			return nil, false
		}

		r := wordRunes[wordPos]
		if !isDictionaryKanjiRune(r) {
			kana := []rune(kanaToHiragana(string(r)))
			if len(kana) == 0 || readingPos+len(kana) > len(readingRunes) {
				memo[key] = true
				return nil, false
			}
			for i := range kana {
				if readingRunes[readingPos+i] != kana[i] {
					memo[key] = true
					return nil, false
				}
			}
			return walk(wordPos+1, kanjiPos, readingPos+len(kana))
		}

		if kanjiPos >= len(candidates) {
			memo[key] = true
			return nil, false
		}

		for _, candidate := range candidates[kanjiPos] {
			candidateRunes := []rune(candidate.Normalized)
			if len(candidateRunes) == 0 {
				continue
			}
			maxPrefix := len(candidateRunes)
			if remaining := len(readingRunes) - readingPos; maxPrefix > remaining {
				maxPrefix = remaining
			}
			for prefixLen := maxPrefix; prefixLen >= 1; prefixLen-- {
				matched := true
				for i := 0; i < prefixLen; i++ {
					if readingRunes[readingPos+i] != candidateRunes[i] {
						matched = false
						break
					}
				}
				if !matched {
					continue
				}
				tail, ok := walk(wordPos+1, kanjiPos+1, readingPos+prefixLen)
				if !ok {
					continue
				}
				return append([]string{firstRunes(candidate.Display, prefixLen)}, tail...), true
			}
		}

		memo[key] = true
		return nil, false
	}
	return walk(0, 0, 0)
}

func CanonicalPartOfSpeech(tags []string) string {
	for _, tag := range tags {
		if tag == "v1" || strings.Contains(tag, "ichidan verb") {
			return "ichidan-verb"
		}
	}
	for _, tag := range tags {
		if strings.HasPrefix(tag, "v5") || strings.Contains(tag, "godan verb") {
			return "godan-verb"
		}
	}
	for _, tag := range tags {
		if tag == "adj-na" || strings.Contains(tag, "na-adjective") || strings.Contains(tag, "adjectival noun") {
			return "na-adjective"
		}
	}
	for _, tag := range tags {
		if tag == "adj-i" || strings.Contains(tag, "i-adjective") {
			return "i-adjective"
		}
	}
	for _, tag := range tags {
		if tag == "adv" || strings.Contains(tag, "adverb") {
			return "adverb"
		}
	}
	for _, tag := range tags {
		if tag == "n" || tag == "pn" || strings.Contains(tag, "noun") || strings.Contains(tag, "pronoun") {
			return "noun"
		}
	}
	if len(tags) == 0 {
		return ""
	}
	return "other"
}

func tagsLower(tags []string) []string {
	lowered := make([]string, 0, len(tags))
	for _, tag := range tags {
		tag = strings.ToLower(strings.TrimSpace(tag))
		if tag != "" {
			lowered = append(lowered, tag)
		}
	}
	return lowered
}

func jsonListApplies(values []string, target string) bool {
	if len(values) == 0 {
		return true
	}
	for _, v := range values {
		if v == "*" || v == target {
			return true
		}
	}
	return false
}

func dictionaryReadingDisplayForm(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if i := strings.IndexRune(raw, '.'); i >= 0 {
		raw = raw[:i]
	}
	raw = strings.ReplaceAll(raw, "-", "")
	return strings.TrimSpace(raw)
}

func firstRunes(s string, count int) string {
	runes := []rune(s)
	if count >= len(runes) {
		return s
	}
	return string(runes[:count])
}

func kanaToHiragana(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= 0x30A1 && r <= 0x30F6 {
			b.WriteRune(r - 0x60)
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func isDictionaryKanjiRune(r rune) bool {
	return r >= 0x4E00 && r <= 0x9FFF
}
