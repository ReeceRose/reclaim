import Image from "next/image";
import Link from "next/link";
import { type LibrarySeriesGroup, tmdbImageURL } from "@/lib/api";
import { formatBytes, formatInt } from "@/lib/format";
import { EncodeHealthBar } from "./encode-health-bar";

export function ShowCard({
  show,
  href,
}: {
  show: LibrarySeriesGroup;
  href: string;
}) {
  const letter = show.title
    .replace(/^(the |a |an )/i, "")
    .charAt(0)
    .toUpperCase();
  const fullyConverted = show.eligible_count === 0 && show.missing_count === 0;
  const allMissing =
    show.file_count > 0 && show.missing_count === show.file_count;
  const imageURL =
    tmdbImageURL(show.backdrop_path, "w780") ??
    tmdbImageURL(show.poster_path, "w342");

  return (
    <Link
      href={href}
      className="relative bg-surface border border-line rounded-2xl overflow-hidden cursor-pointer hover:border-brand-line transition-colors group block"
    >
      <div
        className="relative h-48 overflow-hidden"
        style={{ background: "var(--surface-2)" }}
      >
        {imageURL ? (
          <>
            <Image
              src={imageURL}
              alt={show.title}
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
            <div className="absolute bottom-0 left-0 right-0 px-3 pb-2.5">
              <div className="font-bold text-sm leading-snug line-clamp-2 text-white drop-shadow">
                {show.title}
              </div>
            </div>
          </>
        ) : (
          <>
            <div className="w-full h-full flex items-center justify-center">
              <span
                className="font-black select-none pointer-events-none leading-none opacity-10 text-8xl"
                aria-hidden
              >
                {letter}
              </span>
            </div>
            <div className="absolute bottom-0 left-0 right-0 px-3 pb-2.5">
              <div className="font-bold text-sm leading-snug line-clamp-2">
                {show.title}
              </div>
            </div>
          </>
        )}
      </div>

      <div className="px-3 pt-2 pb-3 flex flex-col gap-1">
        <div className="text-xs text-muted-dim">
          {show.season_count} {show.season_count === 1 ? "season" : "seasons"} ·{" "}
          {formatInt(show.file_count)} files
        </div>
        <div className="flex items-center justify-between gap-2">
          <span className="text-xs text-muted-fg font-mono">
            {formatBytes(show.total_bytes)}
          </span>
          {allMissing ? (
            <span className="text-xs font-medium text-muted-fg">
              All files missing
            </span>
          ) : show.missing_count > 0 ? (
            <span className="text-xs font-medium text-muted-fg">
              {formatInt(show.missing_count)} missing
            </span>
          ) : fullyConverted ? (
            <span className="text-xs font-medium text-green">
              All converted
            </span>
          ) : show.predicted_savings_bytes > 0 ? (
            <span className="text-xs font-semibold text-brand">
              -{formatBytes(show.predicted_savings_bytes)}
            </span>
          ) : null}
        </div>
      </div>

      <EncodeHealthBar
        fileCount={show.file_count}
        eligibleCount={show.eligible_count}
        missingCount={show.missing_count}
      />
    </Link>
  );
}
