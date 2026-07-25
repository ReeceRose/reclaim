import Image from "next/image";
import Link from "next/link";
import { type RankedSeason, tmdbImageURL } from "@/lib/api";
import { formatBytes, formatInt } from "@/lib/format";
import { cn } from "@/lib/utils";
import { EncodeHealthBar } from "./encode-health-bar";

export function SeasonRankCard({
  season,
  rank,
  href,
}: {
  season: RankedSeason;
  rank: number;
  href: string;
}) {
  const letter = season.series_title
    .replace(/^(the |a |an )/i, "")
    .charAt(0)
    .toUpperCase();
  const fullyConverted =
    season.eligible_count === 0 && season.missing_count === 0;
  const allMissing =
    season.file_count > 0 && season.missing_count === season.file_count;
  const imageURL = tmdbImageURL(season.poster_path, "w342");
  const isTop = rank <= 3;

  return (
    <Link
      href={href}
      className="relative bg-surface border border-line rounded-2xl overflow-hidden cursor-pointer hover:border-brand-line transition-colors group block"
    >
      <span
        className={cn(
          "absolute top-2 left-2 z-10 grid place-items-center w-7 h-7 rounded-full text-xs font-bold font-mono tabular-nums backdrop-blur-sm",
          isTop
            ? "bg-brand text-on-brand shadow-lg"
            : "bg-black/55 text-white/90",
        )}
      >
        {rank}
      </span>

      <div
        className="relative h-48 overflow-hidden"
        style={{ background: "var(--surface-2)" }}
      >
        {imageURL ? (
          <>
            <Image
              src={imageURL}
              alt={season.series_title}
              fill
              sizes="(max-width: 640px) 50vw, (max-width: 1024px) 33vw, 240px"
              className="object-cover transition-transform duration-300 group-hover:scale-105"
            />
            <div
              className="absolute inset-0"
              style={{
                background:
                  "linear-gradient(to bottom, transparent 35%, rgba(10,10,10,0.88) 100%)",
              }}
            />
          </>
        ) : (
          <div className="w-full h-full flex items-center justify-center">
            <span
              className="font-black select-none pointer-events-none leading-none opacity-10 text-8xl"
              aria-hidden
            >
              {letter}
            </span>
          </div>
        )}
        <div className="absolute bottom-0 left-0 right-0 px-3 pb-2.5">
          <div
            className={cn(
              "font-bold text-sm leading-snug line-clamp-2",
              imageURL && "text-white drop-shadow",
            )}
          >
            {season.series_title}
          </div>
          <div
            className={cn(
              "text-xs font-medium",
              imageURL ? "text-white/75" : "text-muted-dim",
            )}
          >
            Season {season.season}
          </div>
        </div>
      </div>

      <div className="px-3 pt-2 pb-3 flex flex-col gap-1">
        <div className="text-xs text-muted-dim">
          {formatInt(season.file_count)} files
        </div>
        <div className="flex items-center justify-between gap-2">
          <span className="text-xs text-muted-fg font-mono">
            {formatBytes(season.total_bytes)}
          </span>
          {allMissing ? (
            <span className="text-xs font-medium text-muted-fg">
              All missing
            </span>
          ) : season.missing_count > 0 ? (
            <span className="text-xs font-medium text-muted-fg">
              {formatInt(season.missing_count)} missing
            </span>
          ) : fullyConverted ? (
            <span className="text-xs font-medium text-green">
              All converted
            </span>
          ) : season.predicted_savings_bytes > 0 ? (
            <span className="text-xs font-semibold text-brand font-mono">
              -{formatBytes(season.predicted_savings_bytes)}
            </span>
          ) : null}
        </div>
      </div>

      <EncodeHealthBar
        fileCount={season.file_count}
        eligibleCount={season.eligible_count}
        missingCount={season.missing_count}
      />
    </Link>
  );
}
