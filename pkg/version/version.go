package version

import (
	"fmt"
	"strconv"
	"strings"
)

const (
	DefaultVersion             = "0.0.0"
	DefaultMaxMinorSkew uint64 = 2
	HeaderName                 = "X-Harbor-Satellite-Version"
)

var (
	Version   = DefaultVersion
	GitCommit = "unknown"
)

type Info struct {
	Version   string `json:"version"`
	GitCommit string `json:"git_commit,omitempty"`
}

type SemVer struct {
	Major uint64
	Minor uint64
	Patch uint64
}

func BuildInfo() Info {
	return Info{
		Version:   Current(),
		GitCommit: GitCommit,
	}
}

func Current() string {
	return Normalize(Version)
}

func Normalize(v string) string {
	v = strings.TrimSpace(v)
	return strings.TrimPrefix(v, "v")
}

func Parse(v string) (SemVer, error) {
	normalized := Normalize(v)
	if normalized == "" {
		return SemVer{}, fmt.Errorf("version is required")
	}

	core, err := stripSuffixes(normalized)
	if err != nil {
		return SemVer{}, err
	}

	parts := strings.Split(core, ".")
	if len(parts) != 3 {
		return SemVer{}, fmt.Errorf("invalid semver version %q: expected MAJOR.MINOR.PATCH", v)
	}

	major, err := parsePart(parts[0], "major", v)
	if err != nil {
		return SemVer{}, err
	}
	minor, err := parsePart(parts[1], "minor", v)
	if err != nil {
		return SemVer{}, err
	}
	patch, err := parsePart(parts[2], "patch", v)
	if err != nil {
		return SemVer{}, err
	}

	return SemVer{Major: major, Minor: minor, Patch: patch}, nil
}

func Compatible(groundControlVersion, satelliteVersion string, maxMinorSkew uint64) error {
	gc, err := Parse(groundControlVersion)
	if err != nil {
		return fmt.Errorf("ground control version invalid: %w", err)
	}

	sat, err := Parse(satelliteVersion)
	if err != nil {
		return fmt.Errorf("satellite version invalid: %w", err)
	}

	if gc.Major != sat.Major {
		return fmt.Errorf("incompatible satellite version %s with ground control version %s: major versions must match", satelliteVersion, groundControlVersion)
	}

	if minorDistance(gc.Minor, sat.Minor) > maxMinorSkew {
		return fmt.Errorf("incompatible satellite version %s with ground control version %s: minor versions differ by more than %d release(s)", satelliteVersion, groundControlVersion, maxMinorSkew)
	}

	return nil
}

func minorDistance(a, b uint64) uint64 {
	if a > b {
		return a - b
	}

	return b - a
}

// stripSuffixes validates and removes the prerelease (-) and build (+) suffixes
// from a SemVer string, returning the bare MAJOR.MINOR.PATCH core.
func stripSuffixes(v string) (string, error) {
	if idx := strings.IndexByte(v, '+'); idx >= 0 {
		if err := validSuffixIdentifiers(v[idx+1:], false); err != nil {
			return "", fmt.Errorf("invalid build metadata: %w", err)
		}
		v = v[:idx]
	}

	if idx := strings.IndexByte(v, '-'); idx >= 0 {
		if err := validSuffixIdentifiers(v[idx+1:], true); err != nil {
			return "", fmt.Errorf("invalid prerelease: %w", err)
		}
		v = v[:idx]
	}

	return v, nil
}

// validSuffixIdentifiers checks that a dot-separated prerelease or build suffix
// is non-empty and each identifier consists of ASCII alphanumerics and hyphens.
// Numeric prerelease identifiers must not contain leading zeroes.
func validSuffixIdentifiers(s string, checkLeadingZeros bool) error {
	if s == "" {
		return fmt.Errorf("suffix must not be empty")
	}

	for _, part := range strings.Split(s, ".") {
		if part == "" {
			return fmt.Errorf("identifier must not be empty")
		}
		if !validIdentifier(part) {
			return fmt.Errorf("identifier %q must consist of ASCII alphanumerics and hyphens", part)
		}
		if checkLeadingZeros && isNumeric(part) && len(part) > 1 && part[0] == '0' {
			return fmt.Errorf("numeric identifier %q must not contain leading zeroes", part)
		}
	}

	return nil
}

func validIdentifier(s string) bool {
	hasAlphaNumeric := false
	for _, r := range s {
		if !isIdentifierRune(r) {
			return false
		}
		if r != '-' {
			hasAlphaNumeric = true
		}
	}

	return hasAlphaNumeric
}

func isIdentifierRune(r rune) bool {
	return r >= '0' && r <= '9' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r == '-'
}

func isNumeric(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}

	return true
}

func parsePart(part, name, original string) (uint64, error) {
	if part == "" {
		return 0, fmt.Errorf("invalid semver version %q: empty %s version", original, name)
	}
	if len(part) > 1 && strings.HasPrefix(part, "0") {
		return 0, fmt.Errorf("invalid semver version %q: %s version must not contain leading zeroes", original, name)
	}
	for _, r := range part {
		if r < '0' || r > '9' {
			return 0, fmt.Errorf("invalid semver version %q: %s version must be numeric", original, name)
		}
	}

	value, err := strconv.ParseUint(part, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid semver version %q: parse %s version: %w", original, name, err)
	}

	return value, nil
}
