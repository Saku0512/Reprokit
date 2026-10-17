package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// FileStore はRunner専用ディレクトリにSessionと検査済みComposeを保存する。
type FileStore struct {
	directory string
}

type storedSession struct {
	Session
	Project string `json:"project"`
	Compose []byte `json:"compose"`
}

func NewFileStore(directory string) (*FileStore, error) {
	abs, err := filepath.Abs(directory)
	if err != nil {
		return nil, fmt.Errorf("resolve session directory: %w", err)
	}
	if err := os.MkdirAll(abs, 0o700); err != nil {
		return nil, fmt.Errorf("create session directory: %w", err)
	}
	return &FileStore{directory: abs}, nil
}

func (store *FileStore) Load(_ context.Context, id string) (Session, error) {
	if !idPattern.MatchString(id) {
		return Session{}, ErrNotFound
	}
	file, err := os.Open(store.path(id))
	if errors.Is(err, os.ErrNotExist) {
		return Session{}, ErrNotFound
	}
	if err != nil {
		return Session{}, fmt.Errorf("open session: %w", err)
	}
	defer file.Close()
	decoder := json.NewDecoder(io.LimitReader(file, 4<<20))
	decoder.DisallowUnknownFields()
	var stored storedSession
	if err := decoder.Decode(&stored); err != nil {
		return Session{}, fmt.Errorf("decode session: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return Session{}, errors.New("session contains trailing data")
	}
	stored.Session.Project = stored.Project
	stored.Session.Compose = stored.Compose
	if stored.Session.ID != id || !projectPattern.MatchString(stored.Project) {
		return Session{}, errors.New("stored session identity is invalid")
	}
	return copySession(stored.Session), nil
}

func (store *FileStore) Save(_ context.Context, session Session) error {
	if !idPattern.MatchString(session.ID) || !projectPattern.MatchString(session.Project) {
		return errors.New("invalid session identity")
	}
	staging, err := os.CreateTemp(store.directory, ".session-*")
	if err != nil {
		return fmt.Errorf("create session staging file: %w", err)
	}
	stagingName := staging.Name()
	defer os.Remove(stagingName)
	if err := staging.Chmod(0o600); err != nil {
		staging.Close()
		return fmt.Errorf("set session permissions: %w", err)
	}
	stored := storedSession{Session: copySession(session), Project: session.Project, Compose: append([]byte(nil), session.Compose...)}
	if err := json.NewEncoder(staging).Encode(stored); err != nil {
		staging.Close()
		return fmt.Errorf("encode session: %w", err)
	}
	if err := staging.Sync(); err != nil {
		staging.Close()
		return fmt.Errorf("sync session: %w", err)
	}
	if err := staging.Close(); err != nil {
		return fmt.Errorf("close session: %w", err)
	}
	if err := os.Rename(stagingName, store.path(session.ID)); err != nil {
		return fmt.Errorf("install session: %w", err)
	}
	return nil
}

func (store *FileStore) path(id string) string {
	return filepath.Join(store.directory, id+".json")
}
