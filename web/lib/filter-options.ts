import type { Stats } from "@/lib/api";
import { resolutionBucketLabel } from "@/lib/format";

export type FilterOption = { value: string; label: string };

const CODEC_LABELS: Record<string, string> = {
  h264: "H.264",
  hevc: "HEVC",
  h265: "HEVC",
  mpeg2: "MPEG-2",
  mpeg2video: "MPEG-2",
  vc1: "VC-1",
  av1: "AV1",
  vp9: "VP9",
  unknown: "Unknown",
};

const LIBRARY_LABELS: Record<string, string> = {
  movies: "Movies",
  tv: "TV",
};

const LIBRARY_ORDER = ["movies", "tv"];
const RESOLUTION_ORDER = ["uhd8k", "uhd", "qhd", "fhd", "hd", "sd", "unknown"];

function codecLabel(codec: string): string {
  return CODEC_LABELS[codec.toLowerCase()] ?? codec;
}

function libraryLabel(libraryType: string): string {
  return LIBRARY_LABELS[libraryType] ?? libraryType;
}

function sortByNumericDesc(items: FilterOption[]): FilterOption[] {
  const groupedRank = new Map(
    RESOLUTION_ORDER.map((value, index) => [value, index]),
  );
  return [...items].sort((a, b) => {
    const ar = groupedRank.get(a.value);
    const br = groupedRank.get(b.value);
    if (ar !== undefined && br !== undefined) return ar - br;
    if (ar !== undefined) return -1;
    if (br !== undefined) return 1;

    const ah = Number(a.value);
    const bh = Number(b.value);
    const aNum = Number.isFinite(ah) && ah > 0;
    const bNum = Number.isFinite(bh) && bh > 0;
    if (aNum && bNum) return bh - ah;
    if (aNum) return -1;
    if (bNum) return 1;
    return a.label.localeCompare(b.label);
  });
}

export function codecFilterOptions(
  stats: Stats | undefined,
  opts?: { excludeHEVC?: boolean; excludeUnknown?: boolean },
): FilterOption[] {
  if (!stats) return [];
  return stats.by_codec
    .filter((c) => {
      const codec = c.codec.toLowerCase();
      if (opts?.excludeHEVC && (codec === "hevc" || codec === "h265"))
        return false;
      if (opts?.excludeUnknown && codec === "unknown") return false;
      return true;
    })
    .map((c) => ({ value: c.codec, label: codecLabel(c.codec) }));
}

export function resolutionFilterOptions(
  stats: Stats | undefined,
  opts?: { excludeUnknown?: boolean },
): FilterOption[] {
  if (!stats) return [];
  return sortByNumericDesc(
    stats.by_resolution
      .filter((r) => !opts?.excludeUnknown || r.band !== "unknown")
      .map((r) => ({ value: r.band, label: resolutionBucketLabel(r.band) })),
  );
}

export function libraryFilterOptions(
  stats: Stats | undefined,
  opts?: { excludeUnknown?: boolean },
): FilterOption[] {
  if (!stats) return [];
  const rank = new Map(LIBRARY_ORDER.map((value, index) => [value, index]));
  return [
    ...stats.by_library
      .filter((l) => !opts?.excludeUnknown || l.library_type !== "unknown")
      .map((l) => ({
        value: l.library_type,
        label: libraryLabel(l.library_type),
      })),
  ].sort(
    (a, b) =>
      (rank.get(a.value) ?? Number.MAX_SAFE_INTEGER) -
      (rank.get(b.value) ?? Number.MAX_SAFE_INTEGER),
  );
}
