import {
  isQueueable,
  queueBlockReason,
  StateBadge,
  stateLabel,
} from "@/components/media/candidate-state";
import { CodecBadge } from "@/components/media/codec-badge";
import { OversizeBadge } from "@/components/media/oversize-badge";
import type { MediaFile } from "@/lib/api";
import {
  baseName,
  dirName,
  formatAudio,
  formatBitrate,
  formatBytes,
  formatContainer,
  formatDuration,
  formatDurationCompact,
  formatFileDate,
  formatFileDateTime,
  formatInt,
  formatPct,
  resolutionLabel,
} from "@/lib/format";
import type { ColumnDef } from "@/lib/table-columns";
import { cn } from "@/lib/utils";

export type MediaColumnId =
  | "file"
  | "name"
  | "title"
  | "folder"
  | "codec"
  | "resolution"
  | "added"
  | "state"
  | "duration"
  | "bitrate"
  | "audio"
  | "container"
  | "size"
  | "savings";

type BaseColumn = Omit<ColumnDef<MediaFile, never>, "sort">;

const BASE: Record<MediaColumnId, BaseColumn> = {
  file: {
    id: "file",
    label: "File",
    width: "flex-1 min-w-0",
    locked: true,
    cell: (item) => {
      const missing = item.status === "missing";
      return (
        <>
          <div className="flex items-center gap-2 min-w-0">
            <div
              className={cn(
                "font-semibold text-sm truncate",
                missing && "line-through text-muted-fg",
              )}
            >
              {baseName(item.path)}
            </div>
            <OversizeBadge file={item} />
          </div>
          <div className="text-xs text-muted-dim truncate font-mono">
            {dirName(item.path)}
          </div>
        </>
      );
    },
    title: (item) => item.path,
    description: "File name, with its folder underneath",
  },
  name: {
    id: "name",
    label: "File",
    width: "flex-1 min-w-0",
    locked: true,
    cellClassName: "truncate font-medium text-sm",
    cell: (item) => baseName(item.path),
    title: (item) => item.path,
    description: "File name",
  },
  title: {
    id: "title",
    label: "Title",
    width: "flex-1 min-w-0",
    locked: true,
    cellClassName: "truncate font-medium text-sm",
    cell: (item) => baseName(item.path).replace(/\.[^/.]+$/, ""),
    title: (item) => item.path,
    description: "Title, taken from the file name",
  },
  folder: {
    id: "folder",
    label: "Folder",
    width: "w-40 shrink-0",
    breakpoint: "xl",
    defaultVisible: false,
    cellClassName: "truncate font-mono text-xs text-muted-dim",
    cell: (item) => dirName(item.path) || "—",
    title: (item) => dirName(item.path) || undefined,
    description: "Folder holding the file",
  },
  codec: {
    id: "codec",
    label: "Codec",
    width: "w-18 shrink-0",
    cell: (item) => <CodecBadge codec={item.video_codec} />,
    title: (item) =>
      item.video_codec
        ? item.video_codec_profile
          ? `${item.video_codec} · ${item.video_codec_profile} profile`
          : item.video_codec
        : "Codec unknown — the file has not been probed successfully",
    description: "Source video codec",
  },
  resolution: {
    id: "resolution",
    label: "Res",
    width: "w-12 shrink-0",
    breakpoint: "sm",
    align: "right",
    cellClassName: "text-sm text-muted-fg",
    cell: (item) => resolutionLabel(item.width, item.height),
    title: (item) =>
      item.width && item.height
        ? `${formatInt(item.width)} × ${formatInt(item.height)}`
        : "Resolution unknown",
    description: "Video resolution",
  },
  added: {
    id: "added",
    label: "Added",
    width: "w-16 shrink-0",
    breakpoint: "lg",
    align: "right",
    cellClassName: "font-mono text-xs text-muted-fg",
    cell: (item) => formatFileDate(item.mtime),
    title: (item) => formatFileDateTime(item.mtime),
    description: "File modification time on disk",
  },
  state: {
    id: "state",
    label: "State",
    width: "w-24 shrink-0",
    breakpoint: "lg",
    cell: (item) => <StateBadge state={item.candidate_state} compact />,
    title: (item) => stateLabel(item.candidate_state),
    description: "Whether the file is eligible to be queued",
  },
  duration: {
    id: "duration",
    label: "Length",
    width: "w-14 shrink-0",
    breakpoint: "lg",
    align: "right",
    defaultVisible: false,
    cellClassName: "font-mono text-xs text-muted-fg",
    cell: (item) => formatDurationCompact(item.duration_seconds),
    title: (item) => formatDuration(item.duration_seconds),
    description: "Runtime reported by ffprobe",
  },
  bitrate: {
    id: "bitrate",
    label: "Bitrate",
    width: "w-20 shrink-0",
    breakpoint: "xl",
    align: "right",
    defaultVisible: false,
    cellClassName: "font-mono text-xs text-muted-fg",
    cell: (item) => formatBitrate(item.bitrate_kbps),
    title: (item) =>
      item.bitrate_kbps
        ? `${formatInt(Math.round(item.bitrate_kbps))} kbps overall`
        : "Bitrate unknown",
    description: "Overall bitrate across all streams",
  },
  audio: {
    id: "audio",
    label: "Audio",
    width: "w-20 shrink-0",
    breakpoint: "xl",
    defaultVisible: false,
    cellClassName: "truncate text-xs text-muted-fg",
    cell: (item) => formatAudio(item.audio_codec, item.audio_channels),
    title: (item) =>
      item.audio_codec
        ? `${item.audio_codec}${item.audio_channels ? ` · ${item.audio_channels} channels` : ""}`
        : "No audio stream detected",
    description: "Primary audio stream",
  },
  container: {
    id: "container",
    label: "Container",
    width: "w-20 shrink-0",
    breakpoint: "xl",
    defaultVisible: false,
    cellClassName: "truncate font-mono text-xs text-muted-fg",
    cell: (item) => formatContainer(item.path, item.container_format),
    title: (item) =>
      item.container_format
        ? `ffprobe format: ${item.container_format}`
        : undefined,
    description: "Container format",
  },
  size: {
    id: "size",
    label: "Size",
    width: "w-18 shrink-0",
    breakpoint: "md",
    align: "right",
    cellClassName: "font-mono text-xs text-muted-fg",
    cell: (item) => formatBytes(item.size_bytes),
    title: (item) => `${formatInt(item.size_bytes)} bytes`,
    description: "File size on disk",
  },
  savings: {
    id: "savings",
    label: "Savings",
    width: "w-20 shrink-0",
    align: "right",
    cell: (item) => formatBytes(item.predicted_savings_bytes),
    title: (item) => {
      if (!isQueueable(item)) return queueBlockReason(item);
      if (item.predicted_savings_bytes <= 0) {
        return "No savings predicted for this file";
      }
      return `${formatInt(item.predicted_savings_bytes)} bytes predicted · ${formatPct(item.predicted_savings_bytes, item.size_bytes)} of the file`;
    },
    description: "Bytes a HEVC re-encode is predicted to reclaim",
  },
};

type MediaColumnSpec<T extends MediaFile, S extends string> =
  | MediaColumnId
  | ({ id: MediaColumnId } & Partial<ColumnDef<T, S>>);

export function mediaColumns<T extends MediaFile, S extends string = never>(
  specs: readonly MediaColumnSpec<T, S>[],
): ColumnDef<T, S>[] {
  return specs.map((spec) => {
    if (typeof spec === "string") {
      return { ...BASE[spec] } as ColumnDef<T, S>;
    }
    return { ...BASE[spec.id], ...spec } as ColumnDef<T, S>;
  });
}
