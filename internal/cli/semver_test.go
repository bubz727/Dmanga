package cli

import (
	"testing"
)

func TestSemVerComparison(t *testing.T) {
	tests := []struct {
		name           string
		current        string
		remote         string
		expectedCmp    int  // CompareVersions(current, remote)
		expectedUpdate bool // IsUpdateAvailable(current, remote)
	}{
		{
			name:           "1. Standard patch upgrade",
			current:        "v1.0.0",
			remote:         "v1.0.1",
			expectedCmp:    -1,
			expectedUpdate: true,
		},
		{
			name:           "2. Crucial test: v1.9.0 vs v1.10.0 (lexicographical string fail check)",
			current:        "v1.9.0",
			remote:         "v1.10.0",
			expectedCmp:    -1,
			expectedUpdate: true,
		},
		{
			name:           "3. Same version should have no update",
			current:        "v1.2.0",
			remote:         "v1.2.0",
			expectedCmp:    0,
			expectedUpdate: false,
		},
		{
			name:           "4. Current is newer than remote (e.g. local beta)",
			current:        "v2.0.0",
			remote:         "v1.9.0",
			expectedCmp:    1,
			expectedUpdate: false,
		},
		{
			name:           "5. Prerelease identifier: rc.2 vs rc.10",
			current:        "v1.0.0-rc.2",
			remote:         "v1.0.0-rc.10",
			expectedCmp:    -1,
			expectedUpdate: true,
		},
		{
			name:           "6. Stable release is newer than prerelease",
			current:        "v1.0.0-rc.1",
			remote:         "v1.0.0",
			expectedCmp:    -1,
			expectedUpdate: true,
		},
		{
			name:           "7. Alpha is older than Beta",
			current:        "v1.0.0-alpha",
			remote:         "v1.0.0-beta",
			expectedCmp:    -1,
			expectedUpdate: true,
		},
		{
			name:           "8. Without leading 'v' prefix normalization",
			current:        "1.0.0",
			remote:         "v1.0.1",
			expectedCmp:    -1,
			expectedUpdate: true,
		},
		{
			name:           "9. Dev build should not trigger standard update",
			current:        "dev",
			remote:         "v1.0.0",
			expectedCmp:    0,
			expectedUpdate: false,
		},
		{
			name:           "10. Invalid version strings handled safely",
			current:        "invalid-tag",
			remote:         "v1.0.0",
			expectedCmp:    0,
			expectedUpdate: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotCmp := CompareVersions(tt.current, tt.remote)
			if gotCmp != tt.expectedCmp {
				t.Errorf("CompareVersions(%q, %q) = %d, expected %d", tt.current, tt.remote, gotCmp, tt.expectedCmp)
			}

			gotUpdate := IsUpdateAvailable(tt.current, tt.remote)
			if gotUpdate != tt.expectedUpdate {
				t.Errorf("IsUpdateAvailable(%q, %q) = %v, expected %v", tt.current, tt.remote, gotUpdate, tt.expectedUpdate)
			}
		})
	}
}

func TestNormalizeVersion(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"1.0.0", "v1.0.0"},
		{"v1.0.0", "v1.0.0"},
		{"V2.1.0", "V2.1.0"},
		{"   1.5.0  ", "v1.5.0"},
		{"", ""},
	}

	for _, tt := range tests {
		got := NormalizeVersion(tt.input)
		if got != tt.expected {
			t.Errorf("NormalizeVersion(%q) = %q, expected %q", tt.input, got, tt.expected)
		}
	}
}
