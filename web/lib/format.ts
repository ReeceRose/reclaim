// Human-readable formatting helpers shared across screens.

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

/** relativeTime renders a unix-seconds timestamp as "18 min ago". */
export function relativeTime(unixSeconds: number | null | undefined): string {
  if (!unixSeconds) return "never";
  const diff = Date.now() / 1000 - unixSeconds;
  if (diff < 60) return "just now";
  if (diff < 3600) return `${Math.floor(diff / 60)} min ago`;
  if (diff < 86400) return `${Math.floor(diff / 3600)}h ago`;
  return `${Math.floor(diff / 86400)}d ago`;
}

/** resolutionLabel maps a height to a band label (2160p / 1080p / 720p / SD). */
export function resolutionLabel(
  _width: number | null | undefined,
  height: number | null | undefined,
): string {
  if (!height) return "—";
  if (height >= 2000) return "2160p";
  if (height >= 1000) return "1080p";
  if (height >= 700) return "720p";
  return "SD";
}

/** windowInfo computes open/closed state and a countdown label for an encode window. */
export function windowInfo(
  start: string,
  end: string,
): { open: boolean; label: string; detail: string } {
  const [sh, sm] = start.split(':').map(Number);
  const [eh, em] = end.split(':').map(Number);
  const now = new Date();
  const nowMins = now.getHours() * 60 + now.getMinutes();
  const startMins = sh * 60 + (sm ?? 0);
  const endMins = eh * 60 + (em ?? 0);

  const open =
    startMins <= endMins
      ? nowMins >= startMins && nowMins < endMins
      : nowMins >= startMins || nowMins < endMins;

  const targetMins = open ? endMins : startMins;
  let diff = targetMins - nowMins;
  if (diff < 0) diff += 1440;
  const h = Math.floor(diff / 60);
  const m = diff % 60;
  const timeStr = h > 0 ? `${h}h ${m}m` : `${m}m`;

  return {
    open,
    label: `${start}–${end}`,
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
