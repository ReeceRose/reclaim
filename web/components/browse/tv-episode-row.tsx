"use client";

import Link from "next/link";
import { StateBadge } from "@/components/media/candidate-state";
import { CodecBadge } from "@/components/media/codec-badge";
import type { Episode } from "@/lib/api";
import { baseName, formatBytes, resolutionLabel } from "@/lib/format";
import { cn } from "@/lib/utils";

export function TvEpisodeRow({ ep, href }: { ep: Episode; href: string }) {
  const dimmed =
    ep.candidate_state === "already_hevc" || ep.candidate_state === "completed";
  return (
    <Link
      href={href}
      className={cn(
        "flex items-center gap-3 px-4 py-2.5 border-b border-line-soft last:border-b-0 text-sm",
        "cursor-pointer hover:bg-surface-2 transition-colors",
        dimmed && "opacity-60",
      )}
    >
      <div className="min-w-0 truncate font-medium flex-1">
        {baseName(ep.path)}
      </div>
      <div className="w-16 shrink-0">
        <CodecBadge codec={ep.video_codec} />
      </div>
      <span className="text-muted-dim hidden sm:inline w-12 shrink-0 text-right">
        {ep.width && ep.height ? resolutionLabel(ep.width, ep.height) : "—"}
      </span>
      <span className="text-muted-fg font-mono hidden md:inline w-16 shrink-0 text-right">
        {formatBytes(ep.size_bytes)}
      </span>
      <div className="text-right w-20 shrink-0">
        {ep.candidate_state === "candidate" &&
        ep.predicted_savings_bytes > 0 ? (
          <span className="text-brand font-semibold font-mono">
            -{formatBytes(ep.predicted_savings_bytes)}
          </span>
        ) : (
          <StateBadge state={ep.candidate_state} />
        )}
      </div>
    </Link>
  );
}
