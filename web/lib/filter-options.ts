import type { Stats } from '@/lib/api';

export type FilterOption = { value: string; label: string };

const CODEC_LABELS: Record<string, string> = {
  h264: 'H.264',
  hevc: 'HEVC',
  h265: 'HEVC',
  mpeg2: 'MPEG-2',
  mpeg2video: 'MPEG-2',
  vc1: 'VC-1',
  av1: 'AV1',
  vp9: 'VP9',
  unknown: 'Unknown',
};

const RESOLUTION_LABELS: Record<string, string> = {
  uhd: '4K',
  hd: 'HD',
  sd: 'SD',
  unknown: 'Unknown',
};

const LIBRARY_LABELS: Record<string, string> = {
  movies: 'Movies',
  tv: 'TV',
};

const RESOLUTION_ORDER = ['uhd', 'hd', 'sd', 'unknown'];
const LIBRARY_ORDER = ['movies', 'tv'];

function codecLabel(codec: string): string {
  return CODEC_LABELS[codec.toLowerCase()] ?? codec;
}

function resolutionBandLabel(band: string): string {
  return RESOLUTION_LABELS[band] ?? band;
}

function libraryLabel(libraryType: string): string {
  return LIBRARY_LABELS[libraryType] ?? libraryType;
}

function sortByOrder<T extends { value: string }>(items: T[], order: string[]): T[] {
  const rank = new Map(order.map((value, index) => [value, index]));
  return [...items].sort(
    (a, b) => (rank.get(a.value) ?? Number.MAX_SAFE_INTEGER) - (rank.get(b.value) ?? Number.MAX_SAFE_INTEGER),
  );
}

export function codecFilterOptions(
  stats: Stats | undefined,
  opts?: { excludeHEVC?: boolean; excludeUnknown?: boolean },
): FilterOption[] {
  if (!stats) return [];
  return stats.by_codec
    .filter((c) => {
      const codec = c.codec.toLowerCase();
      if (opts?.excludeHEVC && (codec === 'hevc' || codec === 'h265')) return false;
      if (opts?.excludeUnknown && codec === 'unknown') return false;
      return true;
    })
    .map((c) => ({ value: c.codec, label: codecLabel(c.codec) }));
}

export function resolutionFilterOptions(stats: Stats | undefined): FilterOption[] {
  if (!stats) return [];
  return sortByOrder(
    stats.by_resolution.map((r) => ({ value: r.band, label: resolutionBandLabel(r.band) })),
    RESOLUTION_ORDER,
  );
}

export function libraryFilterOptions(stats: Stats | undefined): FilterOption[] {
  if (!stats) return [];
  return sortByOrder(
    stats.by_library.map((l) => ({ value: l.library_type, label: libraryLabel(l.library_type) })),
    LIBRARY_ORDER,
  );
}
