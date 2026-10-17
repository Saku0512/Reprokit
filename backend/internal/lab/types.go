package lab

// Source identifies an immutable directory in a Git repository.
type Source struct {
	Type         string `yaml:"type"`
	URL          string `yaml:"url"`
	Revision     string `yaml:"revision"`
	Subdirectory string `yaml:"subdirectory"`
}

// Runtime identifies the Compose file that belongs to a resolved Lab.
type Runtime struct {
	Type     string `yaml:"type"`
	File     string `yaml:"file"`
	Platform string `yaml:"platform"`
}

// RegistryReference records where the Lab manifest was resolved from.
type RegistryReference struct {
	Revision     string
	ManifestPath string
}

// ManifestReference records the exact manifest bytes used for resolution.
type ManifestReference struct {
	SHA256 string
	Bytes  []byte
}

// ResolvedLab is the validated, immutable input to source materialization.
type ResolvedLab struct {
	ID       string
	Name     string
	CVEs     []string
	Source   Source
	Runtime  Runtime
	Registry RegistryReference
	Manifest ManifestReference
}

// SourceFile records a file materialized from the selected Git subtree.
type SourceFile struct {
	Path   string
	SHA256 string
}

// MaterializedSource contains the hashes needed for the source section of a
// Reprokit lockfile.
type MaterializedSource struct {
	URL            string
	Revision       string
	Subdirectory   string
	ComposeSHA256  string
	RuntimeCompose []byte
	Files          []SourceFile
}
