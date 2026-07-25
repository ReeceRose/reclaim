import Image from "next/image";
import Link from "next/link";
import { type RankedSeason, tmdbImageURL } from "@/lib/api";
import { formatBytes, formatInt } from "@/lib/format";
import { cn } from "@/lib/utils";

export function SeasonRankRow({
  season,
  rank,
  href,
}: {
  season: RankedSeason;
  rank: number;
  href: string;
}) {
  const fullyConverted =
    season.eligible_count === 0 && season.missing_count === 0;
  const allMissing =
    season.file_count > 0 && season.missing_count === season.file_count;
  const imageURL = tmdbImageURL(season.poster_path, "w92");
  const isTop = rank <= 3;

  return (
    <Link
      href={href}
      className="grid items-center gap-3 px-4 py-2 border-b border-line-soft last:border-b-0 cursor-pointer hover:bg-surface-2 transition-colors"
      style={{ gridTemplateColumns: "auto auto 1fr auto auto auto" }}
    >
      <span
        className={cn(
          "grid place-items-center w-6 h-6 shrink-0 rounded-md text-xs font-bold font-mono tabular-nums",
          isTop ? "bg-brand-soft text-brand" : "text-muted-dim",
        )}
      >
        {rank}
      </span>
      <div className="relative w-8 h-11 shrink-0 rounded overflow-hidden bg-surface-2">
        {imageURL ? (
          <Image
            src={imageURL}
            alt=""
            fill
            sizes="32px"
            className="object-cover"
          />
        ) : null}
      </div>
      <div className="min-w-0">
        <div className="truncate font-medium text-sm">
          {season.series_title}
        </div>
        <div className="text-xs text-muted-dim">Season {season.season}</div>
      </div>
      <span className="text-xs text-muted-dim hidden md:inline whitespace-nowrap">
        {formatInt(season.file_count)} files
      </span>
      <span className="font-mono text-xs text-muted-fg whitespace-nowrap">
        {formatBytes(season.total_bytes)}
      </span>
      <div className="text-right w-24">
        {allMissing ? (
          <span className="text-xs font-medium text-muted-fg">All missing</span>
        ) : season.missing_count > 0 ? (
          <span className="text-xs font-medium text-muted-fg">
            {formatInt(season.missing_count)} missing
          </span>
        ) : fullyConverted ? (
          <span className="text-xs font-medium text-green">All converted</span>
        ) : season.predicted_savings_bytes > 0 ? (
          <span className="text-xs font-semibold text-brand font-mono">
            -{formatBytes(season.predicted_savings_bytes)}
          </span>
        ) : (
          <span className="text-xs text-muted-dim">—</span>
        )}
      </div>
    </Link>
  );
}
