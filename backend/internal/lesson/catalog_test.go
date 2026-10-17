package lesson

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fixture(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile("../../../lessons/filebrowser-auth-hook.json")
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestSchemaValidFixtureAccepted(t *testing.T) {
	data, err := os.ReadFile("../../../lessons/fixtures/valid.json")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := decodeLesson(data); err != nil {
		t.Fatal(err)
	}
}

func testCatalog(t *testing.T) *Catalog {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "lesson.json"), fixture(t), 0600); err != nil {
		t.Fatal(err)
	}
	catalog, issues, err := LoadCatalog(root, func(string) bool { return true })
	if err != nil || len(issues) != 0 {
		t.Fatalf("load: %v %v", err, issues)
	}
	return catalog
}

func testLesson(t *testing.T) Lesson {
	t.Helper()
	value, err := decodeLesson(fixture(t))
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func mutateFixture(t *testing.T, change func(map[string]any)) []byte {
	t.Helper()
	var value map[string]any
	if err := json.Unmarshal(fixture(t), &value); err != nil {
		t.Fatal(err)
	}
	change(value)
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestCatalogRejectsInvalidFixtures(t *testing.T) {
	files, err := filepath.Glob("../../../lessons/fixtures/invalid/*.json")
	if err != nil || len(files) == 0 {
		t.Fatal("missing invalid fixtures")
	}
	for _, file := range files {
		t.Run(filepath.Base(file), func(t *testing.T) {
			data, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := decodeLesson(data); err == nil {
				t.Fatal("invalid fixture accepted")
			}
		})
	}
}

func TestStrictLessonShape(t *testing.T) {
	changes := map[string]func(map[string]any){
		"missing hints": func(v map[string]any) {
			delete(v["spec"].(map[string]any)["steps"].([]any)[0].(map[string]any), "hints")
		},
		"null hints":             func(v map[string]any) { v["spec"].(map[string]any)["steps"].([]any)[0].(map[string]any)["hints"] = nil },
		"wrong key case":         func(v map[string]any) { v["Metadata"] = v["metadata"]; delete(v, "metadata") },
		"unknown key":            func(v map[string]any) { v["spec"].(map[string]any)["command"] = "unsafe" },
		"empty optional ref":     func(v map[string]any) { v["spec"].(map[string]any)["steps"].([]any)[0].(map[string]any)["labRef"] = "" },
		"duplicate prerequisite": func(v map[string]any) { v["spec"].(map[string]any)["prerequisites"] = []string{"same", "same"} },
		"null optional cves":     func(v map[string]any) { v["spec"].(map[string]any)["evidence"].(map[string]any)["cves"] = nil },
	}
	for name, change := range changes {
		t.Run(name, func(t *testing.T) {
			if _, err := decodeLesson(mutateFixture(t, change)); err == nil {
				t.Fatal("invalid shape accepted")
			}
		})
	}
	for _, data := range [][]byte{
		[]byte(strings.Replace(string(fixture(t)), `"kind": "Lesson"`, `"kind": "Lab", "kind": "Lesson"`, 1)),
		append(fixture(t), []byte(` {}`)...),
	} {
		if _, err := decodeLesson(data); err == nil {
			t.Fatal("duplicate/trailing JSON accepted")
		}
	}
}

func TestCatalogExcludesUnapprovedBrokenAndDuplicateLessons(t *testing.T) {
	root := t.TempDir()
	for name, data := range map[string][]byte{"a.json": fixture(t), "b.json": fixture(t), "broken.json": []byte(`{`)} {
		if err := os.WriteFile(filepath.Join(root, name), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	catalog, issues, err := LoadCatalog(root, func(string) bool { return true })
	if err != nil || len(catalog.List()) != 0 || len(issues) != 2 {
		t.Fatalf("duplicates: %v, %v, %#v", err, issues, catalog.List())
	}
	root = t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.json"), fixture(t), 0600); err != nil {
		t.Fatal(err)
	}
	catalog, issues, err = LoadCatalog(root, func(string) bool { return false })
	if err != nil || len(catalog.List()) != 0 || len(issues) == 0 {
		t.Fatal("unapproved Lab published")
	}
	if _, _, err := LoadCatalog(root, nil); err == nil {
		t.Fatal("nil allowlist accepted")
	}
}

func TestCatalogSkipsSymlinksAndOversizedFiles(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(t.TempDir(), "lesson.json")
	if err := os.WriteFile(source, fixture(t), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(source, filepath.Join(root, "link.json")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "large.json"), make([]byte, maxLessonBytes+1), 0600); err != nil {
		t.Fatal(err)
	}
	catalog, issues, err := LoadCatalog(root, func(string) bool { return true })
	if err != nil || len(catalog.List()) != 0 || len(issues) != 2 {
		t.Fatalf("%v %v", err, issues)
	}
}

func TestCatalogCopiesAndLearnerView(t *testing.T) {
	catalog := testCatalog(t)
	original := testLesson(t)
	value, exists := catalog.Get(original.Metadata.ID, original.Metadata.Version)
	if !exists {
		t.Fatal("missing lesson")
	}
	value.Spec.Steps[0].Title = "changed"
	catalog.List()[0].Title = "changed"
	again, _ := catalog.Get(original.Metadata.ID, original.Metadata.Version)
	if again.Spec.Steps[0].Title != original.Spec.Steps[0].Title || catalog.List()[0].Title != original.Metadata.Title {
		t.Fatal("catalog mutated")
	}
	data, err := json.Marshal(learnerView(again))
	if err != nil {
		t.Fatal(err)
	}
	for _, private := range []string{`"hints"`, `"answerExample"`, `"criteria"`, again.Spec.Assessment[0].AnswerExample} {
		if strings.Contains(string(data), private) {
			t.Fatalf("private data leaked: %s", private)
		}
	}
	for index, step := range learnerView(again).Spec.Steps {
		if step.ID != original.Spec.Steps[index].ID || step.HintCount != len(original.Spec.Steps[index].Hints) {
			t.Fatal("step order or hint count changed")
		}
	}
}
