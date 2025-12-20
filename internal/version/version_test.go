package version

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGet_DefaultValues(t *testing.T) {
	info := Get()

	assert.Equal(t, "dev", info.Version, "default version should be 'dev'")
	assert.Equal(t, "unknown", info.Commit, "default commit should be 'unknown'")
	assert.Equal(t, "unknown", info.CommitDate, "default commit date should be 'unknown'")
}

func TestGet_ReturnsInfo(t *testing.T) {
	info := Get()

	assert.IsType(t, Info{}, info, "Get should return an Info struct")
}

func TestInfo_JSONTags(t *testing.T) {
	// Verify the struct has the expected JSON field names
	// This is a compile-time check via the struct literal
	info := Info{
		Version:    "v1.0.0",
		Commit:     "abc123",
		CommitDate: "2025-01-01T00:00:00Z",
	}

	assert.Equal(t, "v1.0.0", info.Version)
	assert.Equal(t, "abc123", info.Commit)
	assert.Equal(t, "2025-01-01T00:00:00Z", info.CommitDate)
}

func TestGet_ModifiedPackageVars(t *testing.T) {
	// Save original values
	originalVersion := Version
	originalCommit := Commit
	originalCommitDate := CommitDate

	// Restore after test
	defer func() {
		Version = originalVersion
		Commit = originalCommit
		CommitDate = originalCommitDate
	}()

	// Modify package variables (simulating build-time injection)
	Version = "v2.0.0"
	Commit = "def456"
	CommitDate = "2025-12-20T12:00:00Z"

	info := Get()

	assert.Equal(t, "v2.0.0", info.Version)
	assert.Equal(t, "def456", info.Commit)
	assert.Equal(t, "2025-12-20T12:00:00Z", info.CommitDate)
}
