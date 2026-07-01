import type { CompatibilityAction, CompatibilitySeverity } from "@/lib/api";

export const COMPATIBILITY_PAGE_SIZE = 100;

export type CompatibilitySortKey =
  | "risk_desc"
  | "size_desc"
  | "mtime_desc"
  | "library_type"
  | "codec";

export const COMPATIBILITY_SORT_OPTIONS: {
  value: CompatibilitySortKey;
  label: string;
}[] = [
  { value: "risk_desc", label: "Predicted risk" },
  { value: "size_desc", label: "Largest file" },
  { value: "mtime_desc", label: "Newest file" },
  { value: "codec", label: "Source codec" },
];

// Reason codes are generated dynamically by internal/compatibility (see
// docs/COMPATIBILITY PLAN.md §6), so this is a display-label lookup for the
// common ones, not an exhaustive enum — unrecognized codes fall back to a
// humanized version of the raw code.
const REASON_LABELS: Record<string, string> = {
  container_mkv: "MKV container",
  container_mp4: "MP4 container",
  hevc_10bit: "10-bit HEVC",
  hevc_12bit: "12-bit HEVC",
  subtitle_pgs: "PGS subtitles",
  audio_channels_exceeded: "Too many audio channels",
  audio_dts: "DTS audio",
  "audio_dts-hd": "DTS-HD MA audio",
  audio_dtsx: "DTS:X audio",
  audio_truehd: "Dolby TrueHD audio",
};

export function reasonLabel(code: string): string {
  if (REASON_LABELS[code]) return REASON_LABELS[code];
  const [prefix, ...rest] = code.split("_");
  const suffix = rest.join(" ").replace(/-/g, " ");
  const prefixLabel =
    prefix === "video"
      ? "Video codec"
      : prefix === "audio"
        ? "Audio"
        : prefix === "container"
          ? "Container"
          : prefix === "subtitle"
            ? "Subtitle"
            : prefix;
  return suffix ? `${prefixLabel}: ${suffix}` : prefixLabel;
}

export const SEVERITY_LABELS: Record<CompatibilitySeverity, string> = {
  hard: "Hard",
  advisory: "Advisory",
};

export const ACTION_LABELS: Record<CompatibilityAction, string> = {
  none: "None needed",
  reencode_hevc: "Re-encode to HEVC",
  remux: "Remux container",
  audio_transcode: "Transcode audio",
  manual: "Manual review",
};

/** riskBand buckets a 0-100 risk score into a 3-tier color band for the UI. */
export function riskBand(score: number): "low" | "medium" | "high" {
  if (score >= 60) return "high";
  if (score >= 30) return "medium";
  return "low";
}

export const RISK_BAND_CLASSES: Record<"low" | "medium" | "high", string> = {
  low: "text-green border-green-soft bg-green-soft",
  medium: "text-gold border-[rgba(241,194,27,.32)] bg-[rgba(241,194,27,.1)]",
  high: "text-red border-[rgba(255,120,120,.28)] bg-[rgba(255,120,120,.09)]",
};
