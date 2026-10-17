package runtime

import (
	"context"
	"os"
	"strings"
	"testing"
)

func TestFileStorePersistsRunnerOnlyFieldsWithPrivatePermissions(t *testing.T) {
	store, err := NewFileStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	session := Session{
		ID:             "01234567-89ab-4def-8123-456789abcdef",
		Project:        "reprokit_0123456789ab",
		Compose:        []byte("services: {}\n"),
		LabID:          "vulhub/filebrowser-cve-2026-54088",
		SourceRevision: strings.Repeat("a", 40),
		State:          Prepared,
	}
	if err := store.Save(context.Background(), session); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.Load(context.Background(), session.ID)
	if err != nil || loaded.Project != session.Project || string(loaded.Compose) != string(session.Compose) {
		t.Fatalf("Load = %#v, %v", loaded, err)
	}
	info, err := os.Stat(store.path(session.ID))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("permissions = %o", info.Mode().Perm())
	}
}

func TestFileStoreRejectsUnsafeSessionID(t *testing.T) {
	store, err := NewFileStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Load(context.Background(), "../session"); err != ErrNotFound {
		t.Fatalf("Load error = %v", err)
	}
}
