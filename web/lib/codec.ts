// Canonical codec → Tailwind class maps shared by every media surface
// (candidate browser, library, browse pages). Keeping a single source here
// avoids the badge palette drifting between screens.

export const CODEC_COLORS: Record<string, string> = {
  h264: "text-gold",
  hevc: "text-green",
  h265: "text-green",
  mpeg2: "text-rose",
  mpeg2video: "text-rose",
  vc1: "text-violet",
  av1: "text-sky",
};

export const CODEC_CSS_COLORS: Record<string, string> = {
  h264: "var(--gold)",
  hevc: "var(--green)",
  h265: "var(--green)",
  mpeg2: "var(--rose)",
  mpeg2video: "var(--rose)",
  vc1: "var(--violet)",
  av1: "var(--sky)",
};

export function codecCSSColor(codec: string): string {
  return CODEC_CSS_COLORS[codec.toLowerCase()] ?? "var(--slate)";
}

export const CODEC_BORDER: Record<string, string> = {
  h264: "border-gold/30 bg-gold/10",
  hevc: "border-green-soft bg-green-soft",
  h265: "border-green-soft bg-green-soft",
  mpeg2: "border-rose/30 bg-rose/10",
  mpeg2video: "border-rose/30 bg-rose/10",
  vc1: "border-violet/30 bg-violet/10",
  av1: "border-sky/32 bg-sky/10",
};

const EFFICIENT_CODECS = new Set(["hevc", "h265", "av1", "vvc", "h266"]);

/**
 * isEfficientCodec mirrors media.IsEfficientCodec on the server: files already
 * in one of these codecs are never re-encode candidates, whatever the target.
 */
export function isEfficientCodec(codec: string | null | undefined): boolean {
  return !!codec && EFFICIENT_CODECS.has(codec.toLowerCase());
}

const TARGET_CODEC_LABELS: Record<string, string> = {
  hevc: "HEVC",
  av1: "AV1",
};

const TARGET_CODEC_ENCODERS: Record<string, string> = {
  hevc: "libx265",
  av1: "libsvtav1",
};

/** targetCodecLabel names a profile's output codec, defaulting to HEVC. */
export function targetCodecLabel(codec: string | null | undefined): string {
  const c = codec || "hevc";
  return TARGET_CODEC_LABELS[c] ?? c.toUpperCase();
}

/** encodeSettingsLabel renders a profile or job's encoder settings on one line. */
export function encodeSettingsLabel(
  codec: string | null | undefined,
  crf: number,
  preset: string,
): string {
  const c = codec || "hevc";
  return `${TARGET_CODEC_ENCODERS[c] ?? c} · CRF ${crf} · preset ${preset}`;
}
