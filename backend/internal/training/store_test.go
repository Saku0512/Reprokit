package training

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStoreRejectsUnknownFieldsAndUnsafeDirectory(t *testing.T) {
	unsafe := filepath.Join(t.TempDir(), "unsafe")
	if err := os.Mkdir(unsafe, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := NewStore(unsafe); err == nil {
		t.Fatal("store accepted a directory readable by other users")
	}

	root := filepath.Join(t.TempDir(), "training")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	state := `{"cohorts":{},"assignments":{},"submissions":{},"completions":{},"auditEvents":{},"unexpected":true}`
	if err := os.WriteFile(filepath.Join(root, "training.json"), []byte(state), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := NewStore(root); err == nil {
		t.Fatal("store accepted a state file with unknown fields")
	}
}

func TestStoreWritesPrivateStateFile(t *testing.T) {
	service := testService(t)
	if _, err := service.CreateCohort(CreateCohortInput{OrganizationID: "acme", ID: "new-hires", Name: "新入社員", LearnerIDs: []string{"alice"}}); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(filepath.Join(service.store.root, "training.json"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("state file permissions = %o", info.Mode().Perm())
	}
}
