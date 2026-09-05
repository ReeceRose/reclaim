"use client";

import Link from "next/link";
import { ColumnCells } from "@/components/table/column-cells";
import type { Episode } from "@/lib/api";
import type { ColumnDef } from "@/lib/table-columns";
import { cn } from "@/lib/utils";

export const EPISODE_ROW_CLASS = "flex items-center gap-2 px-4";

export function TvEpisodeRow<S extends string>({
  ep,
  columns,
  href,
}: {
  ep: Episode;
  columns: readonly ColumnDef<Episode, S>[];
  href: string;
}) {
  const dimmed =
    ep.candidate_state === "already_hevc" || ep.candidate_state === "completed";
  return (
    <Link
      href={href}
      className={cn(
        EPISODE_ROW_CLASS,
        "py-2.5 border-b border-line-soft last:border-b-0 text-sm",
        "cursor-pointer hover:bg-surface-2 transition-colors",
        dimmed && "opacity-60",
      )}
    >
      <ColumnCells columns={columns} item={ep} />
    </Link>
  );
}
