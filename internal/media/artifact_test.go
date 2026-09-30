package media

import "testing"

func TestIsReclaimArtifact(t *testing.T) {
	cases := map[string]bool{
		"/tv/Joey/Season 2/S02E12.avi":                       false,
		"/tv/Joey/Season 2/S02E12.avi.reclaim-tmp.avi":       true,
		"/movies/Dune (2021)/Dune (2021).mkv.reclaim-backup": true,
		"/movies/reclaim-tmp/Dune (2021).mkv":                false,
	}
	for path, want := range cases {
		if got := IsReclaimArtifact(path); got != want {
			t.Errorf("IsReclaimArtifact(%q) = %v, want %v", path, got, want)
		}
	}
}
