// Human-readable formatting helpers shared across screens.

import type { ClockFormat, Settings } from "@/lib/api";

const TB = 1024 ** 4;
const GB = 1024 ** 3;
const MB = 1024 ** 2;
const KB = 1024;

/** formatBytes renders a byte count as TB/GB/MB/KB with sensible precision. */
export function formatBytes(bytes: number, digits = 1): string {
  if (bytes == null || bytes <= 0) return "0 B";
  if (bytes >= TB) return `${(bytes / TB).toFixed(digits)} TB`;
  if (bytes >= GB) return `${(bytes / GB).toFixed(digits)} GB`;
  if (bytes >= MB) return `${(bytes / MB).toFixed(digits)} MB`;
  if (bytes >= KB) return `${(bytes / KB).toFixed(0)} KB`;
  return `${bytes} B`;
}

/** formatInt adds thousands separators. */
export function formatInt(n: number): string {
  return n.toLocaleString("en-US");
}

/** formatPct renders part/total as a percentage string. */
export function formatPct(part: number, total: number, digits = 0): string {
  if (total <= 0) return "0%";
  return `${((part / total) * 100).toFixed(digits)}%`;
}

/** formatCoverage renders files, candidates, and the candidate share of the season. */
export function formatCoverage(files: number, candidates: number): string {
  return `${formatInt(files)} files · ${formatInt(candidates)} candidates · ${formatPct(candidates, files)}`;
}

/** formatVersion renders the build version + short commit, e.g. "v1.4.2 · a3f9c1d" or "dev · a3f9c1d". */
export function formatVersion(version: string, commit: string): string {
  const short = (commit ?? "").slice(0, 7);
  const label = version === "dev" ? "dev" : `v${version}`;
  return short && short !== "unknown" ? `${label} · ${short}` : label;
}

/** relativeTime renders a unix-seconds timestamp as "18 min ago". */
export function relativeTime(unixSeconds: number | null | undefined): string {
  if (!unixSeconds) return "never";
  const diff = Date.now() / 1000 - unixSeconds;
  if (diff < 60) return "just now";
  if (diff < 3600) return `${Math.floor(diff / 60)} min ago`;
  if (diff < 86400) return `${Math.floor(diff / 3600)}h ago`;
  return `${Math.floor(diff / 86400)}d ago`;
}

export function resolutionLabel(
  width: number | null | undefined,
  height: number | null | undefined,
): string {
  const w = width ?? 0;
  const h = height ?? 0;
  if (!w && !h) return "—";
  if (w >= 3840 || h >= 2160) return "4K";
  if (w >= 1920 || h >= 1080) return "1080p";
  if (w >= 1280 || h >= 720) return "720p";
  return "SD";
}

/** resolutionBucketLabel formats a stats bucket key for display. */
export function resolutionBucketLabel(bucket: string): string {
  const labels: Record<string, string> = {
    uhd8k: "8K",
    uhd: "4K/UHD",
    qhd: "1440p",
    fhd: "1080p",
    hd: "720p",
    sd: "SD",
    unknown: "Unknown",
  };
  if (labels[bucket]) return labels[bucket];
  if (!bucket || bucket === "unknown") return "Unknown";
  const h = Number(bucket);
  if (!Number.isFinite(h) || h <= 0) return bucket;
  return `${h}p`;
}

/**
 * formatClock renders an "HH:MM" wall time on the chosen clock. On the 12-hour
 * clock minutes are dropped when zero, so the common whole-hour window stays
 * short; the 24-hour clock always keeps them.
 */
export function formatClock(hhmm: string, format: ClockFormat): string {
  const [rawH, rawM] = hhmm.split(":");
  const h24 = Number(rawH);
  if (!Number.isFinite(h24)) return hhmm;
  const m = Number(rawM);
  const mins = Number.isFinite(m) ? m : 0;
  if (format === "24h") {
    return `${String(h24).padStart(2, "0")}:${String(mins).padStart(2, "0")}`;
  }
  const h12 = h24 % 12 || 12;
  const suffix = h24 >= 12 ? "PM" : "AM";
  return mins > 0
    ? `${h12}:${String(mins).padStart(2, "0")} ${suffix}`
    : `${h12} ${suffix}`;
}

/** formatZoneClock renders an instant as a wall time in the given IANA zone. */
export function formatZoneClock(
  at: Date,
  zone: string,
  format: ClockFormat,
): string | null {
  try {
    const parts = new Intl.DateTimeFormat("en-US", {
      hour: "numeric",
      minute: "2-digit",
      hourCycle: format === "24h" ? "h23" : "h12",
      timeZone: zone,
    }).formatToParts(at);
    const get = (t: string) => parts.find((p) => p.type === t)?.value ?? "";
    const h = get("hour");
    const m = get("minute");
    if (!h || !m) return null;
    if (format === "24h") return `${h.padStart(2, "0")}:${m}`;
    const suffix = get("dayPeriod").toUpperCase();
    if (!suffix) return null;
    return m === "00" ? `${h} ${suffix}` : `${h}:${m} ${suffix}`;
  } catch {
    return null;
  }
}

/**
 * windowInfo labels the encode window from the state the server reported. Open/
 * closed and the transition instant are decided server-side, in the configured
 * timezone, because that is the clock the worker gates on — computing them from
 * the browser clock makes the badge disagree with reality whenever the viewer
 * is in another timezone than the server.
 */
export function windowInfo(
  settings: Pick<
    Settings,
    | "encode_window_start"
    | "encode_window_end"
    | "window_open"
    | "window_changes_at"
  >,
  now: Date,
  format: ClockFormat,
): { open: boolean; label: string; detail: string } {
  const {
    encode_window_start: start,
    encode_window_end: end,
    window_open: open,
    window_changes_at: changesAt,
  } = settings;
  const label = `${formatClock(start, format)} – ${formatClock(end, format)}`;

  // A window with equal bounds is always open and never flips.
  if (changesAt == null) {
    return { open, label, detail: open ? "always open" : "closed" };
  }

  const diff = Math.max(
    0,
    Math.round((changesAt * 1000 - now.getTime()) / 60000),
  );
  const h = Math.floor(diff / 60);
  const m = diff % 60;
  const timeStr = h > 0 ? `${h}h ${m}m` : `${m}m`;

  return {
    open,
    label,
    detail: open ? `closes in ${timeStr}` : `opens in ${timeStr}`,
  };
}

/** baseName extracts the file name from an absolute path. */
export function baseName(path: string): string {
  const i = path.lastIndexOf("/");
  return i >= 0 ? path.slice(i + 1) : path;
}

/** dirName extracts the directory portion of an absolute path. */
export function dirName(path: string): string {
  const i = path.lastIndexOf("/");
  return i >= 0 ? path.slice(0, i) : "";
}

/** formatDurationCompact renders seconds without trailing zero seconds. */
export function formatDurationCompact(
  seconds: number | null | undefined,
): string {
  if (seconds == null || seconds <= 0) return "—";
  const h = Math.floor(seconds / 3600);
  const m = Math.floor((seconds % 3600) / 60);
  const s = Math.floor(seconds % 60);
  if (h > 0 && m > 0) return `${h}h ${m}m`;
  if (h > 0) return `${h}h`;
  if (m > 0) return `${m}m`;
  return `${s}s`;
}

/** formatDuration renders seconds as h/m/s. */
export function formatDuration(seconds: number | null | undefined): string {
  if (seconds == null || seconds <= 0) return "—";
  const h = Math.floor(seconds / 3600);
  const m = Math.floor((seconds % 3600) / 60);
  const s = Math.floor(seconds % 60);
  if (h > 0) return `${h}h ${m}m`;
  if (m > 0) return `${m}m ${s}s`;
  return `${s}s`;
}

/** formatDayLabel renders a YYYY-MM-DD ledger day as a short axis label. */
export function formatDayLabel(day: string): string {
  const [y, m, d] = day.split("-").map(Number);
  if (!y || !m || !d) return day;
  return new Date(Date.UTC(y, m - 1, d)).toLocaleDateString("en-US", {
    month: "short",
    day: "numeric",
    timeZone: "UTC",
  });
}

/** formatDayFull renders a YYYY-MM-DD ledger day with its year. */
export function formatDayFull(day: string): string {
  const [y, m, d] = day.split("-").map(Number);
  if (!y || !m || !d) return day;
  return new Date(Date.UTC(y, m - 1, d)).toLocaleDateString("en-US", {
    month: "short",
    day: "numeric",
    year: "numeric",
    timeZone: "UTC",
  });
}
