package version

// These variables are set at build time via ldflags
var (
	// Version is the semantic version (e.g., "v1.2.3" or "dev")
	Version = "dev"

	// Commit is the git commit hash
	Commit = "unknown"

	// CommitDate is the git commit timestamp (ISO 8601 format)
	// Using commit date instead of build date ensures reproducible builds
	CommitDate = "unknown"
)

// Info returns all version information as a struct
type Info struct {
	Version    string `json:"version"`
	Commit     string `json:"commit"`
	CommitDate string `json:"commit_date"`
}

// Get returns the current version information
func Get() Info {
	return Info{
		Version:    Version,
		Commit:     Commit,
		CommitDate: CommitDate,
	}
}
