package lab

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

const maxGitMetadataBytes = 8 << 20

type treeEntry struct {
	Mode string
	Type string
	OID  string
	Path string
}

type gitSnapshot interface {
	Entries(context.Context, string) ([]treeEntry, error)
	BlobSize(context.Context, string) (int64, error)
	ReadBlob(context.Context, string) ([]byte, error)
	Close() error
}

type gitBackend interface {
	Fetch(context.Context, Source) (gitSnapshot, error)
}

type commandRunner interface {
	Run(context.Context, string, int64, []string, ...string) ([]byte, error)
}

type execCommandRunner struct{}

func (execCommandRunner) Run(ctx context.Context, directory string, outputLimit int64, environment []string, arguments ...string) ([]byte, error) {
	command := exec.CommandContext(ctx, "git", arguments...)
	command.Dir = directory
	command.Env = append(append([]string{}, environment...),
		"GIT_TERMINAL_PROMPT=0",
		"GIT_CONFIG_NOSYSTEM=1",
		"GIT_CONFIG_GLOBAL=/dev/null",
	)

	var standardOutput limitedBuffer
	standardOutput.limit = outputLimit
	var standardError limitedBuffer
	standardError.limit = 64 << 10
	command.Stdout = &standardOutput
	command.Stderr = &standardError
	if err := command.Run(); err != nil {
		return nil, fmt.Errorf("git %s: %w: %s", arguments[0], err, strings.TrimSpace(standardError.String()))
	}
	return standardOutput.Bytes(), nil
}

type limitedBuffer struct {
	bytes.Buffer
	limit int64
}

func (buffer *limitedBuffer) Write(contents []byte) (int, error) {
	remaining := buffer.limit - int64(buffer.Len())
	if remaining <= 0 {
		return 0, errors.New("command output exceeds limit")
	}
	if int64(len(contents)) > remaining {
		written, _ := buffer.Buffer.Write(contents[:remaining])
		return written, errors.New("command output exceeds limit")
	}
	return buffer.Buffer.Write(contents)
}

type localGitBackend struct {
	repositoryPath string
	runner         commandRunner
	environment    []string
}

type localGitSnapshot struct{ *execGitSnapshot }

func (snapshot *localGitSnapshot) Close() error { return nil }

// Fetch はローカルのGitオブジェクトから固定コミットだけを読み、外部へ接続しない。
func (backend localGitBackend) Fetch(ctx context.Context, source Source) (gitSnapshot, error) {
	if err := validateSource(source); err != nil {
		return nil, err
	}
	if source.URL != "https://github.com/Saku0512/Reprokit.git" {
		return nil, errors.New("only the Reprokit repository is available locally")
	}
	resolved, err := backend.runner.Run(ctx, backend.repositoryPath, 256, backend.environment,
		"rev-parse", "--verify", source.Revision+"^{commit}")
	if err != nil {
		return nil, fmt.Errorf("read fixed local source revision: %w", err)
	}
	if strings.TrimSpace(string(resolved)) != source.Revision {
		return nil, errors.New("local source revision does not match the requested commit")
	}
	return &localGitSnapshot{&execGitSnapshot{repositoryPath: backend.repositoryPath, revision: source.Revision, runner: backend.runner, environment: backend.environment}}, nil
}

type execGitSnapshot struct {
	repositoryPath string
	revision       string
	runner         commandRunner
	environment    []string
}

func (snapshot *execGitSnapshot) Entries(ctx context.Context, subdirectory string) ([]treeEntry, error) {
	output, err := snapshot.runner.Run(ctx, snapshot.repositoryPath, maxGitMetadataBytes, snapshot.environment,
		"ls-tree", "-r", "-z", "--full-tree", snapshot.revision, "--", subdirectory,
	)
	if err != nil {
		return nil, err
	}
	return parseTreeEntries(output)
}

func (snapshot *execGitSnapshot) BlobSize(ctx context.Context, oid string) (int64, error) {
	output, err := snapshot.runner.Run(ctx, snapshot.repositoryPath, 128, snapshot.environment, "cat-file", "-s", oid)
	if err != nil {
		return 0, err
	}
	size, err := strconv.ParseInt(strings.TrimSpace(string(output)), 10, 64)
	if err != nil || size < 0 {
		return 0, fmt.Errorf("invalid Git object size %q", strings.TrimSpace(string(output)))
	}
	return size, nil
}

func (snapshot *execGitSnapshot) ReadBlob(ctx context.Context, oid string) ([]byte, error) {
	size, err := snapshot.BlobSize(ctx, oid)
	if err != nil {
		return nil, err
	}
	return snapshot.runner.Run(ctx, snapshot.repositoryPath, size+1, snapshot.environment, "cat-file", "blob", oid)
}

func (snapshot *execGitSnapshot) Close() error { return nil }

func parseTreeEntries(contents []byte) ([]treeEntry, error) {
	if len(contents) == 0 {
		return nil, nil
	}
	records := bytes.Split(contents, []byte{0})
	if len(records[len(records)-1]) != 0 {
		return nil, errors.New("Git tree output is not NUL terminated")
	}
	records = records[:len(records)-1]
	entries := make([]treeEntry, 0, len(records))
	for _, record := range records {
		header, pathBytes, found := bytes.Cut(record, []byte{'\t'})
		if !found {
			return nil, errors.New("invalid Git tree entry")
		}
		fields := strings.Fields(string(header))
		if len(fields) != 3 {
			return nil, errors.New("invalid Git tree entry header")
		}
		entries = append(entries, treeEntry{
			Mode: fields[0],
			Type: fields[1],
			OID:  fields[2],
			Path: string(pathBytes),
		})
	}
	return entries, nil
}
