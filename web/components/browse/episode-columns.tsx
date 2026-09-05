import { StateBadge } from "@/components/media/candidate-state";
import { mediaColumns } from "@/components/media/media-columns";
import type { Episode } from "@/lib/api";
import { formatBytes } from "@/lib/format";

export const EPISODES_TABLE_ID = "browse-episodes";

export const EPISODE_COLUMNS = mediaColumns<Episode>([
  "name",
  "folder",
  "codec",
  "resolution",
  "added",
  "duration",
  "bitrate",
  "audio",
  "container",
  "size",
  {
    id: "savings",
    width: "w-20 shrink-0",
    cell: (ep) =>
      ep.candidate_state === "candidate" && ep.predicted_savings_bytes > 0 ? (
        <span className="text-brand font-semibold font-mono text-xs">
          -{formatBytes(ep.predicted_savings_bytes)}
        </span>
      ) : (
        <StateBadge state={ep.candidate_state} />
      ),
  },
]);
