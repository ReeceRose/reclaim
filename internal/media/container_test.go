package media

import "testing"

func TestOutputContainer(t *testing.T) {
	cases := []struct {
		path     string
		target   TargetCodec
		ext, mux string
	}{
		{"/tv/a.mkv", TargetHEVC, ".mkv", ""},
		{"/tv/a.MP4", TargetHEVC, ".MP4", ""},
		{"/tv/a.m4v", TargetHEVC, ".m4v", "mp4"},
		{"/tv/a.m4v", TargetAV1, ".m4v", "mp4"},
		{"/tv/a.avi", TargetHEVC, ".mkv", ""},
		{"/tv/a.avi", TargetAV1, ".mkv", ""},
		{"/tv/a.wmv", TargetHEVC, ".mkv", ""},
		{"/tv/a.mpg", TargetHEVC, ".mkv", ""},
		{"/tv/a.webm", TargetHEVC, ".mkv", ""},
		{"/tv/a.webm", TargetAV1, ".webm", ""},
		{"/tv/a.mov", TargetHEVC, ".mov", ""},
		{"/tv/a.mov", TargetAV1, ".mkv", ""},
		{"/tv/a.ts", TargetAV1, ".mkv", ""},
	}
	for _, c := range cases {
		ext, mux := OutputContainer(c.path, c.target)
		if ext != c.ext || mux != c.mux {
			t.Errorf("OutputContainer(%q, %s) = %q, %q; want %q, %q", c.path, c.target, ext, mux, c.ext, c.mux)
		}
	}
}

func TestWithExt(t *testing.T) {
	if got := WithExt("/tv/Joey/S02E12 - Trip.avi", ".mkv"); got != "/tv/Joey/S02E12 - Trip.mkv" {
		t.Errorf("WithExt = %q", got)
	}
}
