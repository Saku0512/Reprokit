package lesson

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const maxLessonBytes = 1 << 20

// #9のPrepare本文の上限に収まらない参照は公開前に除外する。
const maxRunnerPrepareBytes = 4096

// LabAllowedは承認済みRegistryに存在するLab IDだけを許可する。
type LabAllowed func(labID string) bool

type Summary struct {
	ID              string `json:"id"`
	Version         string `json:"version"`
	Title           string `json:"title"`
	Audience        string `json:"audience"`
	DurationMinutes int    `json:"durationMinutes"`
	Product         string `json:"product"`
}

// Catalogは検証済みのLesson Versionだけを保持する。
type Catalog struct {
	lessons   map[string][]byte
	summaries []Summary
}

func lessonKey(id, version string) string { return id + "@" + version }

// LoadCatalogは不正な教材を除外し、理由をissuesに返す。
func LoadCatalog(root string, allowed LabAllowed) (*Catalog, []error, error) {
	if allowed == nil {
		return nil, nil, errors.New("Lab allowlist is required")
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, nil, err
	}
	catalog := &Catalog{lessons: make(map[string][]byte)}
	var issues []error
	duplicate := make(map[string]bool)
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		if !entry.Type().IsRegular() {
			issues = append(issues, fmt.Errorf("%s: not a regular file", entry.Name()))
			continue
		}
		data, err := readRegularFile(filepath.Join(root, entry.Name()), maxLessonBytes)
		if err != nil {
			issues = append(issues, fmt.Errorf("%s: %w", entry.Name(), err))
			continue
		}
		value, err := decodeLesson(data)
		if err != nil {
			issues = append(issues, fmt.Errorf("%s: %w", entry.Name(), err))
			continue
		}
		approved := true
		for _, lab := range value.Spec.Labs {
			if !allowed(lab.LabID) {
				issues = append(issues, fmt.Errorf("%s: unapproved Lab ID %q", entry.Name(), lab.LabID))
				approved = false
			}
		}
		if !approved {
			continue
		}
		key := lessonKey(value.Metadata.ID, value.Metadata.Version)
		if _, exists := catalog.lessons[key]; exists || duplicate[key] {
			delete(catalog.lessons, key)
			duplicate[key] = true
			issues = append(issues, fmt.Errorf("%s: duplicate Lesson Version %q", entry.Name(), key))
			continue
		}
		catalog.lessons[key] = data
	}
	for _, data := range catalog.lessons {
		value, _ := decodeLesson(data)
		catalog.summaries = append(catalog.summaries, Summary{
			ID: value.Metadata.ID, Version: value.Metadata.Version, Title: value.Metadata.Title,
			Audience: value.Spec.Audience, DurationMinutes: value.Spec.DurationMinutes, Product: value.Spec.Case.Product,
		})
	}
	sort.Slice(catalog.summaries, func(i, j int) bool {
		left, right := catalog.summaries[i], catalog.summaries[j]
		if left.ID == right.ID {
			return left.Version < right.Version
		}
		return left.ID < right.ID
	})
	return catalog, issues, nil
}

func decodeLesson(data []byte) (Lesson, error) {
	var value Lesson
	if err := decodeStrict(data, &value); err != nil {
		return Lesson{}, err
	}
	if err := Validate(value); err != nil {
		return Lesson{}, err
	}
	for _, lab := range value.Spec.Labs {
		reference, err := json.Marshal(LabReference{value.Metadata.ID, value.Metadata.Version, lab.Ref})
		if err != nil || len(reference) > maxRunnerPrepareBytes {
			return Lesson{}, errors.New("Lab reference exceeds Runner Prepare request limit")
		}
	}
	return value, nil
}

func readRegularFile(path string, limit int64) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > limit {
		return nil, errors.New("not a regular file or exceeds size limit")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, errors.New("file exceeds size limit")
	}
	return data, nil
}

func (catalog *Catalog) List() []Summary {
	return append([]Summary{}, catalog.summaries...)
}

func (catalog *Catalog) Get(id, version string) (Lesson, bool) {
	data, exists := catalog.lessons[lessonKey(id, version)]
	if !exists {
		return Lesson{}, false
	}
	value, err := decodeLesson(data)
	return value, err == nil
}
