import { isQueueable } from "@/components/media/candidate-state";
import { CodecBadge } from "@/components/media/codec-badge";
import { mediaColumns } from "@/components/media/media-columns";
import type { MediaFile } from "@/lib/api";
import { formatBytes } from "@/lib/format";
import type { LibrarySortColumn } from "./constants";

export const LIBRARY_TABLE_ID = "library";

export const LIBRARY_COLUMNS = mediaColumns<MediaFile, LibrarySortColumn>([
  { id: "file", sort: "file" },
  {
    id: "codec",
    sort: "codec",
    width: "w-20 shrink-0",
    cell: (item) => <CodecBadge codec={item.video_codec} showUnknown />,
  },
  { id: "resolution", sort: "res" },
  { id: "added", sort: "added" },
  "state",
  "duration",
  "bitrate",
  "audio",
  "container",
  { id: "size", sort: "size", breakpoint: "sm", width: "w-18 shrink-0" },
  {
    id: "savings",
    width: "w-20 sm:w-24 shrink-0",
    cellClassName: "font-mono text-sm",
    cell: (item) =>
      isQueueable(item) ? (
        <span className="text-brand font-semibold">
          {formatBytes(item.predicted_savings_bytes)}
        </span>
      ) : (
        <span className="text-muted-dim">-</span>
      ),
  },
]);
