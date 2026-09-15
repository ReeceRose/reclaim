package startup

import "testing"

const encodersSample = `Encoders:
 V..... = Video
 A..... = Audio
 S..... = Subtitle
 ------
 V....D libx265              libx265 H.265 / HEVC (codec hevc)
 V..... libsvtav1            SVT-AV1(Scalable Video Technology for AV1) encoder (codec av1)
 A....D aac                  AAC (Advanced Audio Coding)
`

func TestParseEncoders(t *testing.T) {
	got := parseEncoders([]byte(encodersSample))
	for _, name := range []string{"libx265", "libsvtav1", "aac"} {
		if !got[name] {
			t.Errorf("%s missing from %v", name, got)
		}
	}
	if got["="] || got["Video"] {
		t.Errorf("legend lines above the separator must be skipped: %v", got)
	}
}

func TestParseEncoders_withoutSVTAV1(t *testing.T) {
	got := parseEncoders([]byte(" ------\n V....D libx265   libx265 H.265 / HEVC (codec hevc)\n"))
	if !got["libx265"] || got["libsvtav1"] {
		t.Fatalf("got %v", got)
	}
}
