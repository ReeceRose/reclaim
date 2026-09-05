import { CodecBadge } from "@/components/media/codec-badge";
import { mediaColumns } from "@/components/media/media-columns";
import type { MediaFile } from "@/lib/api";
import { formatBytes } from "@/lib/format";

export const MOVIES_TABLE_ID = "browse-movies";

export const MOVIE_COLUMNS = mediaColumns<MediaFile>([
  "title",
  "folder",
  {
    id: "codec",
    cell: (item) => <CodecBadge codec={item.video_codec} showUnknown />,
  },
  "resolution",
  "added",
  "duration",
  "bitrate",
  "audio",
  "container",
  "size",
  {
    id: "savings",
    cell: (item) => {
      if (
        item.candidate_state === "candidate" &&
        item.predicted_savings_bytes > 0
      ) {
        return (
          <span className="text-xs font-semibold text-brand font-mono">
            -{formatBytes(item.predicted_savings_bytes)}
          </span>
        );
      }
      if (
        item.candidate_state === "already_hevc" ||
        item.candidate_state === "completed"
      ) {
        return (
          <span className="text-xs font-medium text-green">Converted</span>
        );
      }
      return <span className="text-xs text-muted-dim">—</span>;
    },
  },
]);
