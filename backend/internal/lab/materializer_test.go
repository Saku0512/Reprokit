package lab

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestLocalMaterializerUsesCommittedObjectsWithoutNetwork(t *testing.T) {
	repo := t.TempDir()
	git := func(args ...string) string {
		t.Helper()
		command := exec.Command("git", args...)
		command.Dir = repo
		command.Env = append(os.Environ(), "GIT_AUTHOR_NAME=Test", "GIT_AUTHOR_EMAIL=test@example.invalid", "GIT_COMMITTER_NAME=Test", "GIT_COMMITTER_EMAIL=test@example.invalid")
		out, err := command.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	git("init", "--quiet")
	dir := filepath.Join(repo, "images", "CVE-2026-54088")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	compose := []byte("services: {}\n")
	file := filepath.Join(dir, "docker-compose.yml")
	if err := os.WriteFile(file, compose, 0o600); err != nil {
		t.Fatal(err)
	}
	git("add", "images/CVE-2026-54088/docker-compose.yml")
	git("commit", "--quiet", "-m", "fixture")
	revision := git("rev-parse", "HEAD")
	if err := os.WriteFile(file, []byte("modified\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	resolved := ResolvedLab{Source: Source{Type: "git", URL: "https://github.com/Saku0512/Reprokit.git", Revision: revision, Subdirectory: "images/CVE-2026-54088"}, Runtime: Runtime{File: "docker-compose.yml"}}
	result, err := NewLocalMaterializer(repo).Materialize(context.Background(), resolved, filepath.Join(t.TempDir(), "source"))
	if err != nil || string(result.RuntimeCompose) != string(compose) {
		t.Fatalf("Materialize = %#v, %v", result, err)
	}
	if _, err := os.Stat(repo); err != nil {
		t.Fatalf("source repository was removed: %v", err)
	}
	resolved.Source.Revision = strings.Repeat("0", 40)
	if _, err := NewLocalMaterializer(repo).Materialize(context.Background(), resolved, filepath.Join(t.TempDir(), "source")); err == nil {
		t.Fatal("missing commit was accepted")
	}
}

func TestMaterializerCopiesOnlySelectedSubtreeAndHashesFiles(t *testing.T) {
	compose := []byte("services: {}\n")
	readme := []byte("reviewed lab\n")
	snapshot := &fakeSnapshot{
		entries: []treeEntry{
			{Mode: "100644", Type: "blob", OID: "compose", Path: "images/CVE-2026-54088/docker-compose.yml"},
			{Mode: "100644", Type: "blob", OID: "readme", Path: "images/CVE-2026-54088/README.md"},
		},
		blobs: map[string][]byte{
			"compose": compose,
			"readme":  readme,
		},
	}
	backend := &fakeBackend{snapshot: snapshot}
	materializer := newMaterializerForTest(backend)
	destination := filepath.Join(t.TempDir(), "source")
	resolved := testResolvedLab()

	result, err := materializer.Materialize(context.Background(), resolved, destination)
	if err != nil {
		t.Fatalf("Materialize() error = %v", err)
	}
	if backend.source != resolved.Source {
		t.Fatalf("backend source = %#v, want %#v", backend.source, resolved.Source)
	}
	if snapshot.closed != 1 {
		t.Fatalf("snapshot Close() calls = %d, want 1", snapshot.closed)
	}
	actualCompose, err := os.ReadFile(filepath.Join(destination, "docker-compose.yml"))
	if err != nil {
		t.Fatal(err)
	}
	if string(actualCompose) != string(compose) {
		t.Fatalf("compose contents = %q", actualCompose)
	}
	wantDigest := sha256.Sum256(compose)
	if result.ComposeSHA256 != hex.EncodeToString(wantDigest[:]) {
		t.Fatalf("ComposeSHA256 = %q", result.ComposeSHA256)
	}
	if string(result.RuntimeCompose) != string(compose) {
		t.Fatalf("RuntimeCompose = %q", result.RuntimeCompose)
	}
	if len(result.Files) != 2 || result.Files[0].Path != "README.md" || result.Files[1].Path != "docker-compose.yml" {
		t.Fatalf("Files = %#v", result.Files)
	}
	if _, err := os.Stat(filepath.Join(destination, "..", "other")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("unexpected out-of-scope file: %v", err)
	}
}

func TestMaterializerRejectsOutOfScopeEntry(t *testing.T) {
	tests := []struct {
		name  string
		entry treeEntry
		want  string
	}{
		{
			name:  "different subtree",
			entry: treeEntry{Mode: "100644", Type: "blob", OID: "bad", Path: "other/CVE/file"},
			want:  "out-of-scope path",
		},
		{
			name:  "parent traversal",
			entry: treeEntry{Mode: "100644", Type: "blob", OID: "bad", Path: "images/CVE-2026-54088/../../escape"},
			want:  "unsafe source entry",
		},
		{
			name:  "symbolic link",
			entry: treeEntry{Mode: "120000", Type: "blob", OID: "bad", Path: "images/CVE-2026-54088/link"},
			want:  "not a regular file",
		},
		{
			name:  "submodule",
			entry: treeEntry{Mode: "160000", Type: "commit", OID: "bad", Path: "images/CVE-2026-54088/nested"},
			want:  "not a regular file",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			snapshot := &fakeSnapshot{
				entries: []treeEntry{test.entry},
				blobs:   map[string][]byte{"bad": []byte("bad")},
			}
			materializer := newMaterializerForTest(&fakeBackend{snapshot: snapshot})
			destination := filepath.Join(t.TempDir(), "source")
			_, err := materializer.Materialize(context.Background(), testResolvedLab(), destination)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("Materialize() error = %v, want %q", err, test.want)
			}
			if _, statErr := os.Stat(destination); !errors.Is(statErr, os.ErrNotExist) {
				t.Fatalf("destination must not remain after failure: %v", statErr)
			}
		})
	}
}

func TestMaterializerRejectsMissingComposeFile(t *testing.T) {
	snapshot := &fakeSnapshot{
		entries: []treeEntry{
			{Mode: "100644", Type: "blob", OID: "readme", Path: "images/CVE-2026-54088/README.md"},
		},
		blobs: map[string][]byte{"readme": []byte("readme")},
	}
	destination := filepath.Join(t.TempDir(), "source")
	_, err := newMaterializerForTest(&fakeBackend{snapshot: snapshot}).Materialize(
		context.Background(), testResolvedLab(), destination,
	)
	if err == nil || !strings.Contains(err.Error(), "runtime file") {
		t.Fatalf("Materialize() error = %v, want missing runtime file", err)
	}
}

func TestParseTreeEntries(t *testing.T) {
	entries, err := parseTreeEntries([]byte("100644 blob abcdef\tdir/file.yml\x00"))
	if err != nil {
		t.Fatalf("parseTreeEntries() error = %v", err)
	}
	if len(entries) != 1 || entries[0].Path != "dir/file.yml" || entries[0].OID != "abcdef" {
		t.Fatalf("entries = %#v", entries)
	}
	if _, err := parseTreeEntries([]byte("100644 blob abcdef\tdir/file.yml")); err == nil {
		t.Fatal("parseTreeEntries() accepted non-NUL-terminated output")
	}
}

func testResolvedLab() ResolvedLab {
	return ResolvedLab{
		ID: "reprokit/httpd-cve-2021-41773",
		Source: Source{
			Type:         "git",
			URL:          "https://github.com/Saku0512/Reprokit.git",
			Revision:     "a94285e057faee8c7232e100116dc750cf17e257",
			Subdirectory: "images/CVE-2026-54088",
		},
		Runtime: Runtime{
			Type:     "docker-compose",
			File:     "docker-compose.yml",
			Platform: "linux/amd64",
		},
	}
}

type fakeBackend struct {
	source   Source
	snapshot gitSnapshot
	err      error
}

func (backend *fakeBackend) Fetch(_ context.Context, source Source) (gitSnapshot, error) {
	backend.source = source
	return backend.snapshot, backend.err
}

type fakeSnapshot struct {
	entries []treeEntry
	blobs   map[string][]byte
	closed  int
}

func (snapshot *fakeSnapshot) Entries(context.Context, string) ([]treeEntry, error) {
	return snapshot.entries, nil
}

func (snapshot *fakeSnapshot) BlobSize(_ context.Context, oid string) (int64, error) {
	contents, found := snapshot.blobs[oid]
	if !found {
		return 0, errors.New("missing blob")
	}
	return int64(len(contents)), nil
}

func (snapshot *fakeSnapshot) ReadBlob(_ context.Context, oid string) ([]byte, error) {
	contents, found := snapshot.blobs[oid]
	if !found {
		return nil, errors.New("missing blob")
	}
	return contents, nil
}

func (snapshot *fakeSnapshot) Close() error {
	snapshot.closed++
	return nil
}
