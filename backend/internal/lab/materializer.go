package lab

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/jphacks/sd_2601/internal/network"
)

const (
	defaultMaxFiles      = 10_000
	defaultMaxFileBytes  = 32 << 20
	defaultMaxTotalBytes = 256 << 20
)

// Materializer copies only regular files in the selected Git subtree into an
// isolated destination. It does not invoke Docker or execute fetched files.
type Materializer struct {
	backend       gitBackend
	maxFiles      int
	maxFileBytes  int64
	maxTotalBytes int64
}

// NewMaterializer creates a source materializer backed by the git executable.
func NewMaterializer() Materializer {
	materializer, err := NewMaterializerWithNetwork(network.Config{})
	if err != nil {
		panic(err)
	}
	return materializer
}

// NewMaterializerWithNetwork は明示された企業ネットワーク設定だけでGitを取得する。
func NewMaterializerWithNetwork(config network.Config) (Materializer, error) {
	if err := config.Validate(); err != nil {
		return Materializer{}, err
	}
	backend, err := newGitBackend(config)
	if err != nil {
		return Materializer{}, err
	}
	return Materializer{
		backend:       backend,
		maxFiles:      defaultMaxFiles,
		maxFileBytes:  defaultMaxFileBytes,
		maxTotalBytes: defaultMaxTotalBytes,
	}, nil
}

func newMaterializerForTest(backend gitBackend) Materializer {
	materializer := NewMaterializer()
	materializer.backend = backend
	return materializer
}

// Materialize fetches and verifies the exact source revision, then atomically
// installs the selected subtree at destination.
func (materializer Materializer) Materialize(ctx context.Context, resolved ResolvedLab, destination string) (MaterializedSource, error) {
	if err := validateSource(resolved.Source); err != nil {
		return MaterializedSource{}, fmt.Errorf("validate resolved source: %w", err)
	}
	if err := validateRelativePath(resolved.Runtime.File); err != nil {
		return MaterializedSource{}, fmt.Errorf("validate runtime file: %w", err)
	}
	if materializer.backend == nil {
		return MaterializedSource{}, errors.New("Git backend is not configured")
	}

	snapshot, err := materializer.backend.Fetch(ctx, resolved.Source)
	if err != nil {
		return MaterializedSource{}, fmt.Errorf("fetch fixed Git revision: %w", err)
	}
	defer snapshot.Close()

	entries, err := snapshot.Entries(ctx, resolved.Source.Subdirectory)
	if err != nil {
		return MaterializedSource{}, fmt.Errorf("list source subtree: %w", err)
	}
	if len(entries) == 0 {
		return MaterializedSource{}, errors.New("source subdirectory is missing or empty")
	}
	if len(entries) > materializer.maxFiles {
		return MaterializedSource{}, fmt.Errorf("source subtree exceeds %d files", materializer.maxFiles)
	}
	slices.SortFunc(entries, func(left, right treeEntry) int {
		return strings.Compare(left.Path, right.Path)
	})

	destination, err = filepath.Abs(destination)
	if err != nil {
		return MaterializedSource{}, fmt.Errorf("resolve destination: %w", err)
	}
	if _, err := os.Lstat(destination); err == nil {
		return MaterializedSource{}, fmt.Errorf("destination %q already exists", destination)
	} else if !errors.Is(err, os.ErrNotExist) {
		return MaterializedSource{}, fmt.Errorf("inspect destination: %w", err)
	}
	parent := filepath.Dir(destination)
	if err := os.MkdirAll(parent, 0o700); err != nil {
		return MaterializedSource{}, fmt.Errorf("create destination parent: %w", err)
	}
	staging, err := os.MkdirTemp(parent, ".reprokit-source-*")
	if err != nil {
		return MaterializedSource{}, fmt.Errorf("create source staging directory: %w", err)
	}
	defer os.RemoveAll(staging)

	result := MaterializedSource{
		URL:          resolved.Source.URL,
		Revision:     resolved.Source.Revision,
		Subdirectory: resolved.Source.Subdirectory,
		Files:        make([]SourceFile, 0, len(entries)),
	}
	prefix := resolved.Source.Subdirectory + "/"
	var totalBytes int64
	for _, entry := range entries {
		if entry.Type != "blob" || entry.Mode != "100644" && entry.Mode != "100755" {
			return MaterializedSource{}, fmt.Errorf("source entry %q is not a regular file", entry.Path)
		}
		if !strings.HasPrefix(entry.Path, prefix) {
			return MaterializedSource{}, fmt.Errorf("Git returned out-of-scope path %q", entry.Path)
		}
		relative := strings.TrimPrefix(entry.Path, prefix)
		if err := validateRelativePath(relative); err != nil {
			return MaterializedSource{}, fmt.Errorf("unsafe source entry %q: %w", entry.Path, err)
		}
		if path.Clean(path.Join(resolved.Source.Subdirectory, relative)) != entry.Path {
			return MaterializedSource{}, fmt.Errorf("source entry %q escapes the selected subtree", entry.Path)
		}

		size, err := snapshot.BlobSize(ctx, entry.OID)
		if err != nil {
			return MaterializedSource{}, fmt.Errorf("inspect source entry %q: %w", entry.Path, err)
		}
		if size > materializer.maxFileBytes {
			return MaterializedSource{}, fmt.Errorf("source entry %q exceeds %d bytes", entry.Path, materializer.maxFileBytes)
		}
		totalBytes += size
		if totalBytes > materializer.maxTotalBytes {
			return MaterializedSource{}, fmt.Errorf("source subtree exceeds %d bytes", materializer.maxTotalBytes)
		}
		contents, err := snapshot.ReadBlob(ctx, entry.OID)
		if err != nil {
			return MaterializedSource{}, fmt.Errorf("read source entry %q: %w", entry.Path, err)
		}
		if int64(len(contents)) != size {
			return MaterializedSource{}, fmt.Errorf("source entry %q changed size while reading", entry.Path)
		}

		target := filepath.Join(staging, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			return MaterializedSource{}, fmt.Errorf("create directory for %q: %w", relative, err)
		}
		mode := os.FileMode(0o600)
		if entry.Mode == "100755" {
			mode = 0o700
		}
		if err := os.WriteFile(target, contents, mode); err != nil {
			return MaterializedSource{}, fmt.Errorf("write source entry %q: %w", relative, err)
		}

		digest := sha256.Sum256(contents)
		digestText := hex.EncodeToString(digest[:])
		result.Files = append(result.Files, SourceFile{Path: relative, SHA256: digestText})
		if relative == resolved.Runtime.File {
			result.ComposeSHA256 = digestText
			result.RuntimeCompose = slices.Clone(contents)
		}
	}
	if result.ComposeSHA256 == "" {
		return MaterializedSource{}, fmt.Errorf("runtime file %q is not present in the selected subtree", resolved.Runtime.File)
	}
	if err := os.Rename(staging, destination); err != nil {
		return MaterializedSource{}, fmt.Errorf("install materialized source: %w", err)
	}
	return result, nil
}
