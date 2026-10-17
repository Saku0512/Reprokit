package lesson

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func testStore(t *testing.T, catalog *Catalog) *SessionStore {
	t.Helper()
	store, err := NewSessionStore(filepath.Join(t.TempDir(), "study"), catalog)
	if err != nil {
		t.Fatal(err)
	}
	return store
}

func createSession(t *testing.T, store *SessionStore) StudySession {
	t.Helper()
	value := store.catalog.List()[0]
	session, err := store.Create(value.ID, value.Version)
	if err != nil {
		t.Fatal(err)
	}
	return session
}

func TestSessionPersistsAnswersProgressAndHints(t *testing.T) {
	catalog := testCatalog(t)
	store := testStore(t, catalog)
	value := testLesson(t)
	session := createSession(t, store)
	if session.StepIndex != 0 || len(store.view(session).Hints) != 0 {
		t.Fatal("new session exposes progress or hints")
	}
	for session.StepIndex < len(value.Spec.Steps) {
		step := value.Spec.Steps[session.StepIndex]
		for count := range step.Hints {
			next, err := store.RevealHint(session.ID, step.ID)
			if err != nil || next.RevealedHints[step.ID] != count+1 || len(store.view(next).Hints[step.ID]) != count+1 {
				t.Fatalf("hint: %v %#v", err, next)
			}
		}
		if _, err := store.RevealHint(session.ID, step.ID); !errors.Is(err, ErrInvalid) {
			t.Fatal("hint beyond limit accepted")
		}
		next, err := store.Advance(session.ID)
		if err != nil {
			t.Fatal(err)
		}
		session = next
	}
	if _, err := store.Advance(session.ID); !errors.Is(err, ErrInvalid) {
		t.Fatal("advance beyond last step accepted")
	}
	question := value.Spec.Assessment[0].ID
	if _, err := store.SaveAnswer(session.ID, question, "受講者の回答"); err != nil {
		t.Fatal(err)
	}
	reopened, err := NewSessionStore(store.root, catalog)
	if err != nil {
		t.Fatal(err)
	}
	restored, err := reopened.Get(session.ID)
	if err != nil || restored.StepIndex != len(value.Spec.Steps) || restored.Answers[question] != "受講者の回答" {
		t.Fatalf("restore: %v %#v", err, restored)
	}
	if len(reopened.view(restored).Hints) != len(store.view(restored).Hints) {
		t.Fatal("hints not persisted")
	}
	info, err := os.Stat(filepath.Join(store.root, session.ID+".json"))
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("state permissions: %v %v", info, err)
	}
}

func TestSessionRejectsInvalidOperations(t *testing.T) {
	store := testStore(t, testCatalog(t))
	value := testLesson(t)
	session := createSession(t, store)
	if _, err := store.Get("../../outside"); !errors.Is(err, ErrInvalid) {
		t.Fatal("unsafe id accepted")
	}
	if _, err := store.Get(strings.Repeat("a", 32)); !errors.Is(err, ErrNotFound) {
		t.Fatal("missing session accepted")
	}
	if _, err := store.Create(value.Metadata.ID, "9.9.9"); !errors.Is(err, ErrNotFound) {
		t.Fatal("unpublished version accepted")
	}
	if len(value.Spec.Steps) > 1 {
		if _, err := store.RevealHint(session.ID, value.Spec.Steps[1].ID); !errors.Is(err, ErrInvalid) {
			t.Fatal("future hint accepted")
		}
	}
	if _, err := store.SaveAnswer(session.ID, "unknown", "value"); !errors.Is(err, ErrInvalid) {
		t.Fatal("unknown question accepted")
	}
	if _, err := store.SaveAnswer(session.ID, value.Spec.Assessment[0].ID, strings.Repeat("x", maxAnswerBytes+1)); !errors.Is(err, ErrInvalid) {
		t.Fatal("oversized answer accepted")
	}
	reference, err := store.LabBinding(session.ID, value.Spec.Labs[0].Ref)
	if err != nil || reference.LessonID != session.LessonID || reference.LessonVersion != session.LessonVersion || reference.LabRef != value.Spec.Labs[0].Ref {
		t.Fatal("binding not pinned")
	}
	if _, err := store.LabBinding(session.ID, "https://invalid.test"); !errors.Is(err, ErrInvalid) {
		t.Fatal("arbitrary target accepted")
	}
}

func TestSessionRejectsCorruptState(t *testing.T) {
	for _, name := range []string{"step", "hint", "answer", "trailing", "symlink"} {
		t.Run(name, func(t *testing.T) {
			store := testStore(t, testCatalog(t))
			session := createSession(t, store)
			switch name {
			case "step":
				session.StepIndex = -1
			case "hint":
				session.RevealedHints["unknown"] = 1
			case "answer":
				session.Answers["unknown"] = "value"
			}
			data, err := json.Marshal(session)
			if err != nil {
				t.Fatal(err)
			}
			if name == "trailing" {
				data = append(data, []byte(` {}`)...)
			}
			path := filepath.Join(store.root, session.ID+".json")
			if name == "symlink" {
				other := filepath.Join(t.TempDir(), "state.json")
				if err := os.WriteFile(other, data, 0600); err != nil {
					t.Fatal(err)
				}
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(other, path); err != nil {
					t.Fatal(err)
				}
			} else if err := os.WriteFile(path, data, 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := store.Get(session.ID); err == nil {
				t.Fatal("corrupt state accepted")
			}
		})
	}
}

func TestConcurrentAnswerSavesDoNotLoseOtherAnswers(t *testing.T) {
	store := testStore(t, testCatalog(t))
	session := createSession(t, store)
	value := testLesson(t)
	var wait sync.WaitGroup
	for _, question := range value.Spec.Assessment {
		wait.Add(1)
		go func(id string) {
			defer wait.Done()
			if _, err := store.SaveAnswer(session.ID, id, "saved"); err != nil {
				t.Error(err)
			}
		}(question.ID)
	}
	wait.Wait()
	restored, err := store.Get(session.ID)
	if err != nil || len(restored.Answers) != len(value.Spec.Assessment) {
		t.Fatalf("lost answers: %v %#v", err, restored.Answers)
	}
}
