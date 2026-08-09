package version

import (
	"testing"

	"github.com/stretchr/testify/require"
)

type compatibleCase struct {
	name             string
	groundControl    string
	satellite        string
	maxMinorSkew     uint64
	wantErrSubstring string
}

var compatibleCases = []compatibleCase{
	{
		name:          "same version",
		groundControl: "1.2.3",
		satellite:     "1.2.4",
		maxMinorSkew:  2,
	},
	{
		name:          "tag with v prefix",
		groundControl: "v1.4.0",
		satellite:     "1.2.0",
		maxMinorSkew:  2,
	},
	{
		name:             "major mismatch",
		groundControl:    "2.1.0",
		satellite:        "1.1.0",
		maxMinorSkew:     2,
		wantErrSubstring: "major versions must match",
	},
	{
		name:             "minor skew too large",
		groundControl:    "1.5.0",
		satellite:        "1.2.0",
		maxMinorSkew:     2,
		wantErrSubstring: "minor versions differ by more than 2 release(s)",
	},
	{
		name:             "missing satellite version",
		groundControl:    "1.0.0",
		satellite:        "",
		maxMinorSkew:     2,
		wantErrSubstring: "version is required",
	},
	{
		name:             "invalid satellite version",
		groundControl:    "1.0.0",
		satellite:        "1.0",
		maxMinorSkew:     2,
		wantErrSubstring: "expected MAJOR.MINOR.PATCH",
	},
}

func TestCompatible(t *testing.T) {
	for _, tt := range compatibleCases {
		t.Run(tt.name, func(t *testing.T) {
			err := Compatible(tt.groundControl, tt.satellite, tt.maxMinorSkew)
			if tt.wantErrSubstring == "" {
				require.NoError(t, err)
				return
			}

			require.Error(t, err)
			require.Contains(t, err.Error(), tt.wantErrSubstring)
		})
	}
}

func TestParse(t *testing.T) {
	tests := []struct {
		version string
		wantErr bool
	}{
		{version: "1.2.3", wantErr: false},
		{version: "v1.2.3", wantErr: false},
		{version: "1.2.3-alpha.1", wantErr: false},
		{version: "1.2.3-rc.1", wantErr: false},
		{version: "1.2.3+build.5", wantErr: false},
		{version: "1.2.3-alpha.1+build.5", wantErr: false},
		{version: "1.2.3-alpha-1", wantErr: false},
		{version: "1.2.3-4.5.6", wantErr: false},
		{version: "", wantErr: true},
		{version: "1.2", wantErr: true},
		{version: "1.2.3.4", wantErr: true},
		{version: "01.2.3", wantErr: true},
		{version: "1.2.3-", wantErr: true},
		{version: "1.2.3+", wantErr: true},
		{version: "1.2.3--", wantErr: true},
		{version: "1.2.3-alpha..1", wantErr: true},
		{version: "1.2.3-alpha.", wantErr: true},
		{version: "1.2.3-alpha_1", wantErr: true},
		{version: "1.2.3+bad build", wantErr: true},
		{version: "1.2.3+//", wantErr: true},
		{version: "1.2.3-01", wantErr: true},
		{version: "1.2.3-0", wantErr: false},
	}

	for _, tt := range tests {
		t.Run(tt.version, func(t *testing.T) {
			_, err := Parse(tt.version)
			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
		})
	}
}
