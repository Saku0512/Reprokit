package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// FileStore はRunner専用ディレクトリにSessionと検査済みComposeを保存する。
type FileStore struct {
	directory string
}

type storedSession struct {
	Session
	Project            string `json:"project"`
	ManifestPath       string `json:"manifestPath"`
	RuntimeFile        string `json:"runtimeFile"`
	Manifest           []byte `json:"manifest"`
	SourceURL          string `json:"sourceUrl"`
	SourceSubdirectory string `json:"sourceSubdirectory"`
	SourceCompose      []byte `json:"sourceCompose"`
	Platform           string `json:"platform"`
	Compose            []byte `json:"compose"`
	Lockfile           []byte `json:"lockfile"`
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
	stored.Session.ManifestPath = stored.ManifestPath
	stored.Session.RuntimeFile = stored.RuntimeFile
	stored.Session.Manifest = stored.Manifest
	stored.Session.SourceURL = stored.SourceURL
	stored.Session.SourceSubdirectory = stored.SourceSubdirectory
	stored.Session.SourceCompose = stored.SourceCompose
	stored.Session.Platform = stored.Platform
	stored.Session.Compose = stored.Compose
	stored.Session.Lockfile = stored.Lockfile
	if stored.Session.ID != id || !projectPattern.MatchString(stored.Project) {
		return Session{}, errors.New("stored session identity is invalid")
	}
	return copySession(stored.Session), nil
}

// List はRunnerが所有するSessionだけを復元する。想定外のファイル名は
// stateディレクトリが破損または改ざんされた可能性として拒否する。
func (store *FileStore) List(ctx context.Context) ([]Session, error) {
	entries, err := os.ReadDir(store.directory)
	if err != nil {
		return nil, fmt.Errorf("list sessions: %w", err)
	}
	ids := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			return nil, errors.New("session directory contains an unexpected directory")
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".json") || !idPattern.MatchString(strings.TrimSuffix(name, ".json")) {
			return nil, errors.New("session directory contains an unexpected file")
		}
		ids = append(ids, strings.TrimSuffix(name, ".json"))
	}
	sort.Strings(ids)
	sessions := make([]Session, 0, len(ids))
	for _, id := range ids {
		session, err := store.Load(ctx, id)
		if err != nil {
			return nil, err
		}
		sessions = append(sessions, session)
	}
	return sessions, nil
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
	stored := storedSession{Session: copySession(session), Project: session.Project, ManifestPath: session.ManifestPath, RuntimeFile: session.RuntimeFile, Manifest: append([]byte(nil), session.Manifest...), SourceURL: session.SourceURL, SourceSubdirectory: session.SourceSubdirectory, SourceCompose: append([]byte(nil), session.SourceCompose...), Platform: session.Platform, Compose: append([]byte(nil), session.Compose...), Lockfile: append([]byte(nil), session.Lockfile...)}
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
