import { isQueueable } from "@/components/media/candidate-state";
import { mediaColumns } from "@/components/media/media-columns";
import type { MediaFile } from "@/lib/api";
import { formatBytes } from "@/lib/format";
import type { CandidateSortColumn } from "./constants";

export const CANDIDATES_TABLE_ID = "candidates";

export const CANDIDATE_COLUMNS = mediaColumns<MediaFile, CandidateSortColumn>([
  "file",
  { id: "codec", sort: "codec" },
  "resolution",
  { id: "added", sort: "added" },
  "duration",
  "bitrate",
  "audio",
  "container",
  { id: "size", sort: "size", breakpoint: "sm", width: "w-18 shrink-0" },
  {
    id: "savings",
    sort: "savings",
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
