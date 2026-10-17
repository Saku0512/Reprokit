package lesson

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sync"
)

const maxAnswerBytes = 16 << 10
const maxSessionBytes = 1 << 20

var sessionIDPattern = regexp.MustCompile(`^[0-9a-f]{32}$`)

var (
	ErrNotFound = errors.New("study session not found")
	ErrInvalid  = errors.New("invalid study operation")
)

// StudySessionはLessonの版に固定されたローカルの学習状態を表す。
type StudySession struct {
	ID            string            `json:"id"`
	LessonID      string            `json:"lessonId"`
	LessonVersion string            `json:"lessonVersion"`
	StepIndex     int               `json:"stepIndex"`
	RevealedHints map[string]int    `json:"revealedHints"`
	Answers       map[string]string `json:"answers"`
}

// SessionStoreは受講状態だけを保存し、Labの実行状態を保持しない。
type SessionStore struct {
	root    string
	catalog *Catalog
	mu      sync.Mutex
}

func NewSessionStore(root string, catalog *Catalog) (*SessionStore, error) {
	if catalog == nil {
		return nil, errors.New("catalog is required")
	}
	if err := os.MkdirAll(root, 0700); err != nil {
		return nil, err
	}
	info, err := os.Lstat(root)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() || info.Mode().Perm()&0077 != 0 {
		return nil, errors.New("study directory must be private (0700)")
	}
	return &SessionStore{root: root, catalog: catalog}, nil
}

func (store *SessionStore) Create(lessonID, version string) (StudySession, error) {
	if _, exists := store.catalog.Get(lessonID, version); !exists {
		return StudySession{}, ErrNotFound
	}
	idBytes := make([]byte, 16)
	if _, err := rand.Read(idBytes); err != nil {
		return StudySession{}, err
	}
	session := StudySession{
		ID: hex.EncodeToString(idBytes), LessonID: lessonID, LessonVersion: version,
		RevealedHints: make(map[string]int), Answers: make(map[string]string),
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	if err := store.write(session); err != nil {
		return StudySession{}, err
	}
	return session, nil
}

func (store *SessionStore) Get(id string) (StudySession, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	return store.read(id)
}

func (store *SessionStore) Advance(id string) (StudySession, error) {
	return store.update(id, func(session *StudySession, lesson Lesson) error {
		if session.StepIndex >= len(lesson.Spec.Steps) {
			return ErrInvalid
		}
		session.StepIndex++
		return nil
	})
}

func (store *SessionStore) RevealHint(id, stepID string) (StudySession, error) {
	return store.update(id, func(session *StudySession, lesson Lesson) error {
		for index, step := range lesson.Spec.Steps {
			if step.ID != stepID {
				continue
			}
			if index > session.StepIndex || session.RevealedHints[stepID] >= len(step.Hints) {
				return ErrInvalid
			}
			session.RevealedHints[stepID]++
			return nil
		}
		return ErrInvalid
	})
}

func (store *SessionStore) SaveAnswer(id, questionID, answer string) (StudySession, error) {
	if len(answer) > maxAnswerBytes {
		return StudySession{}, ErrInvalid
	}
	return store.update(id, func(session *StudySession, lesson Lesson) error {
		for _, question := range lesson.Spec.Assessment {
			if question.ID == questionID {
				session.Answers[questionID] = answer
				return nil
			}
		}
		return ErrInvalid
	})
}

type LabReference struct {
	LessonID      string `json:"lessonId"`
	LessonVersion string `json:"lessonVersion"`
	LabRef        string `json:"labRef"`
}

// LabBindingは受講中のLessonに固定された論理参照だけを返す。
func (store *SessionStore) LabBinding(id, ref string) (LabReference, error) {
	session, err := store.Get(id)
	if err != nil {
		return LabReference{}, err
	}
	lesson, exists := store.catalog.Get(session.LessonID, session.LessonVersion)
	if !exists {
		return LabReference{}, ErrNotFound
	}
	for _, lab := range lesson.Spec.Labs {
		if lab.Ref == ref {
			return LabReference{session.LessonID, session.LessonVersion, lab.Ref}, nil
		}
	}
	return LabReference{}, ErrInvalid
}

func (store *SessionStore) update(id string, change func(*StudySession, Lesson) error) (StudySession, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	session, err := store.read(id)
	if err != nil {
		return StudySession{}, err
	}
	lesson, exists := store.catalog.Get(session.LessonID, session.LessonVersion)
	if !exists {
		return StudySession{}, ErrNotFound
	}
	if err := change(&session, lesson); err != nil {
		return StudySession{}, err
	}
	if err := store.write(session); err != nil {
		return StudySession{}, err
	}
	return session, nil
}

func (store *SessionStore) read(id string) (StudySession, error) {
	if !sessionIDPattern.MatchString(id) {
		return StudySession{}, ErrInvalid
	}
	data, err := readRegularFile(filepath.Join(store.root, id+".json"), maxSessionBytes)
	if errors.Is(err, os.ErrNotExist) {
		return StudySession{}, ErrNotFound
	}
	if err != nil {
		return StudySession{}, err
	}
	var session StudySession
	if err := decodeStrict(data, &session); err != nil {
		return StudySession{}, errors.New("corrupt study session")
	}
	if session.ID != id {
		return StudySession{}, errors.New("corrupt study session")
	}
	value, exists := store.catalog.Get(session.LessonID, session.LessonVersion)
	if !exists {
		return StudySession{}, ErrNotFound
	}
	if session.StepIndex < 0 || session.StepIndex > len(value.Spec.Steps) {
		return StudySession{}, errors.New("corrupt study progress")
	}
	steps := make(map[string]int)
	for index, step := range value.Spec.Steps {
		if index <= session.StepIndex {
			steps[step.ID] = len(step.Hints)
		}
	}
	for step, count := range session.RevealedHints {
		maximum, ok := steps[step]
		if !ok || count < 0 || count > maximum {
			return StudySession{}, errors.New("corrupt hint progress")
		}
	}
	questions := make(map[string]bool)
	for _, question := range value.Spec.Assessment {
		questions[question.ID] = true
	}
	for id, answer := range session.Answers {
		if !questions[id] || len(answer) > maxAnswerBytes {
			return StudySession{}, errors.New("corrupt answer")
		}
	}
	return session, nil
}

func (store *SessionStore) write(session StudySession) error {
	data, err := json.Marshal(session)
	if err != nil {
		return err
	}
	if len(data) > maxSessionBytes {
		return ErrInvalid
	}
	file, err := os.CreateTemp(store.root, ".study-*")
	if err != nil {
		return err
	}
	name := file.Name()
	defer file.Close()
	defer os.Remove(name)
	if err := file.Chmod(0600); err != nil {
		return err
	}
	if _, err := file.Write(data); err != nil {
		return err
	}
	if err := file.Sync(); err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(name, filepath.Join(store.root, fmt.Sprintf("%s.json", session.ID)))
}
