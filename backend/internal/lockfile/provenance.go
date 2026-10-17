package lockfile

import (
	"errors"

	"github.com/jphacks/sd_2601/internal/lab"
)

// Provenance is the immutable Registry, Manifest, and source portion embedded
// in a generated reprokit.lock. Runtime and image resolution append their own
// sections before the complete lockfile is written.
type Provenance struct {
	Registry Registry `yaml:"registry"`
	Manifest Manifest `yaml:"manifest"`
	Source   Source   `yaml:"source"`
}

type Registry struct {
	Revision     string `yaml:"revision"`
	ManifestPath string `yaml:"manifestPath"`
}

type Manifest struct {
	SHA256 string `yaml:"sha256"`
}

type Source struct {
	URL           string       `yaml:"url"`
	Revision      string       `yaml:"revision"`
	Subdirectory  string       `yaml:"subdirectory"`
	ComposeSHA256 string       `yaml:"composeSha256"`
	Files         []SourceFile `yaml:"files"`
}

type SourceFile struct {
	Path   string `yaml:"path"`
	SHA256 string `yaml:"sha256"`
}

// FromLab converts a resolved and materialized Lab into lockfile provenance.
func FromLab(resolved lab.ResolvedLab, materialized lab.MaterializedSource) (Provenance, error) {
	if materialized.URL != resolved.Source.URL ||
		materialized.Revision != resolved.Source.Revision ||
		materialized.Subdirectory != resolved.Source.Subdirectory {
		return Provenance{}, errors.New("materialized source does not match the resolved Lab")
	}
	files := make([]SourceFile, len(materialized.Files))
	for index, file := range materialized.Files {
		files[index] = SourceFile{Path: file.Path, SHA256: file.SHA256}
	}
	return Provenance{
		Registry: Registry{
			Revision:     resolved.Registry.Revision,
			ManifestPath: resolved.Registry.ManifestPath,
		},
		Manifest: Manifest{SHA256: resolved.Manifest.SHA256},
		Source: Source{
			URL:           materialized.URL,
			Revision:      materialized.Revision,
			Subdirectory:  materialized.Subdirectory,
			ComposeSHA256: materialized.ComposeSHA256,
			Files:         files,
		},
	}, nil
}
