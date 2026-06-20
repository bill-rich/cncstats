package statsfile

import "testing"

func TestCheckVersion(t *testing.T) {
	cases := []struct {
		name     string
		version  int
		expected VersionStatus
	}{
		{"missing", 0, VersionMissing},
		{"negative", -1, VersionTooOld},
		{"min_supported", MinStatsVersion, VersionOK},
		{"current", CurrentStatsVersion, VersionOK},
		{"newer", CurrentStatsVersion + 1, VersionNewer},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			g := &GameStats{Version: tc.version}
			if got := g.CheckVersion(); got != tc.expected {
				t.Errorf("CheckVersion() with version %d = %v, want %v",
					tc.version, got, tc.expected)
			}
		})
	}
}

func TestVersionStatusString(t *testing.T) {
	cases := map[VersionStatus]string{
		VersionOK:      "ok",
		VersionMissing: "missing",
		VersionTooOld:  "too-old",
		VersionNewer:   "newer-than-server",
	}
	for status, want := range cases {
		if got := status.String(); got != want {
			t.Errorf("%d.String() = %q, want %q", int(status), got, want)
		}
	}
}
