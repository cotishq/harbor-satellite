package version_test

import (
	"testing"
	v "github.com/container-registry/harbor-satellite/internal/version"
)

func TestDefaults(t *testing.T) {
	if v.Version == "" {
		t.Fatal("Version must not be empty")
	}
	if v.GitCommit == "" {
		t.Fatal("GitCommit must not be empty")
	}
}
