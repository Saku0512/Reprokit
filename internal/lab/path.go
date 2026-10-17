package lab

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const (
	maxIndexBytes    = 4 << 20
	maxManifestBytes = 1 << 20
)

func validateRelativePath(value string) error {
	if value == "" {
		return errors.New("path is empty")
	}
	if len(value) > 1024 {
		return errors.New("path is longer than 1024 bytes")
	}
	if strings.HasPrefix(value, "/") || strings.Contains(value, "\\") {
		return errors.New("path must be a slash-separated relative path")
	}

	for _, segment := range strings.Split(value, "/") {
		if segment == "" || segment == "." || segment == ".." {
			return errors.New("path contains an empty, current, or parent segment")
		}
		for _, character := range segment {
			if !isPathCharacter(character) {
				return fmt.Errorf("path contains unsupported character %q", character)
			}
		}
	}
	return nil
}

func isPathCharacter(character rune) bool {
	return character >= 'a' && character <= 'z' ||
		character >= 'A' && character <= 'Z' ||
		character >= '0' && character <= '9' ||
		strings.ContainsRune("._-", character)
}

func readFileWithin(root, relative string, limit int64) ([]byte, error) {
	if err := validateRelativePath(relative); err != nil {
		return nil, fmt.Errorf("unsafe relative path %q: %w", relative, err)
	}

	rootInfo, err := os.Lstat(root)
	if err != nil {
		return nil, fmt.Errorf("inspect root: %w", err)
	}
	if !rootInfo.IsDir() || rootInfo.Mode()&os.ModeSymlink != 0 {
		return nil, errors.New("root must be a real directory")
	}

	current := root
	segments := strings.Split(relative, "/")
	for index, segment := range segments {
		current = filepath.Join(current, segment)
		info, inspectErr := os.Lstat(current)
		if inspectErr != nil {
			return nil, fmt.Errorf("inspect %q: %w", relative, inspectErr)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("path %q contains a symbolic link", relative)
		}
		if index < len(segments)-1 && !info.IsDir() {
			return nil, fmt.Errorf("path component in %q is not a directory", relative)
		}
	}

	file, err := os.Open(current)
	if err != nil {
		return nil, fmt.Errorf("open %q: %w", relative, err)
	}
	defer file.Close()

	contents, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil {
		return nil, fmt.Errorf("read %q: %w", relative, err)
	}
	if int64(len(contents)) > limit {
		return nil, fmt.Errorf("file %q exceeds %d bytes", relative, limit)
	}
	return contents, nil
}
