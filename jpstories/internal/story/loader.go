package story

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type Summary struct {
	ID       string
	Title    string
	FilePath string
}

func LoadFile(path string) (Story, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Story{}, err
	}

	var s Story
	if err := json.Unmarshal(data, &s); err != nil {
		return Story{}, fmt.Errorf("decode story %s: %w", path, err)
	}
	if err := Validate(s); err != nil {
		return Story{}, fmt.Errorf("validate story %s: %w", path, err)
	}
	return s, nil
}

func SaveFile(path string, s Story) error {
	if err := Validate(s); err != nil {
		return fmt.Errorf("validate story %s: %w", path, err)
	}

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("encode story %s: %w", path, err)
	}
	data = append(data, '\n')

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return err
	}
	return nil
}

func ListDir(dir string) ([]Summary, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var summaries []Summary
	for _, entry := range entries {
		if entry.IsDir() {
			path := filepath.Join(dir, entry.Name(), entry.Name()+".json")
			s, err := LoadFile(path)
			if err != nil {
				if os.IsNotExist(err) {
					continue
				}
				return nil, err
			}
			summaries = append(summaries, Summary{
				ID:       s.ID,
				Title:    s.Title,
				FilePath: path,
			})
			continue
		}
		if !strings.EqualFold(filepath.Ext(entry.Name()), ".json") {
			continue
		}

		path := filepath.Join(dir, entry.Name())
		s, err := LoadFile(path)
		if err != nil {
			return nil, err
		}
		summaries = append(summaries, Summary{
			ID:       s.ID,
			Title:    s.Title,
			FilePath: path,
		})
	}

	sort.Slice(summaries, func(i, j int) bool {
		if summaries[i].Title == summaries[j].Title {
			return summaries[i].ID < summaries[j].ID
		}
		return summaries[i].Title < summaries[j].Title
	})
	return summaries, nil
}

func LoadByID(dir, id string) (Story, string, error) {
	summaries, err := ListDir(dir)
	if err != nil {
		return Story{}, "", err
	}

	for _, summary := range summaries {
		if summary.ID != id {
			continue
		}
		s, err := LoadFile(summary.FilePath)
		if err != nil {
			return Story{}, "", err
		}
		return s, summary.FilePath, nil
	}

	return Story{}, "", os.ErrNotExist
}
