package media

import (
	"path/filepath"
	"strings"
)

// RemuxExt is the container an encode moves to when its source's cannot carry
// the target codec. Matroska holds both targets and every audio and subtitle
// codec the scanned containers do.
const RemuxExt = ".mkv"

// RemuxFormatName is ffprobe's format_name for RemuxExt, stamped on a row whose
// encode moved it there.
const RemuxFormatName = "matroska,webm"

// targetContainers lists, per target, the source extensions whose container can
// carry that codec, and the ffmpeg muxer to force where the extension alone
// would pick the wrong one. Anything else is remuxed to RemuxExt: ffmpeg's AVI
// muxer writes HEVC with no fourcc, so it probes back as rawvideo; WMV, MPEG-PS,
// and WebM (for HEVC) refuse the stream outright; and MOV refuses AV1.
var targetContainers = map[TargetCodec]map[string]string{
	TargetHEVC: {".mkv": "", ".mp4": "", ".m4v": "mp4", ".mov": "", ".ts": "", ".m2ts": ""},
	TargetAV1:  {".mkv": "", ".mp4": "", ".m4v": "mp4", ".webm": ""},
}

// OutputContainer returns the extension an encode of path to target should
// carry and the ffmpeg muxer to force ("" to let ffmpeg infer it from the
// extension). ".m4v" needs forcing because ffmpeg reads it as a raw MPEG-4
// elementary stream, not the MP4 file it is in practice.
func OutputContainer(path string, target TargetCodec) (ext, muxer string) {
	ext = filepath.Ext(path)
	if muxer, ok := targetContainers[target][strings.ToLower(ext)]; ok {
		return ext, muxer
	}
	return RemuxExt, ""
}

// WithExt returns path with its extension replaced by ext.
func WithExt(path, ext string) string {
	return strings.TrimSuffix(path, filepath.Ext(path)) + ext
}
