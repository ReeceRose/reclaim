import Link from "next/link";
import type { LibrarySeriesGroup } from "@/lib/api";
import { formatBytes, formatInt } from "@/lib/format";

export function ShowRow({
  show,
  href,
  expanded,
}: {
  show: LibrarySeriesGroup;
  href: string;
  expanded?: boolean;
}) {
  const fullyConverted = show.eligible_count === 0 && show.missing_count === 0;
  const allMissing =
    show.file_count > 0 && show.missing_count === show.file_count;
  const hasChevron = expanded !== undefined;
  return (
    <Link
      href={href}
      className="grid items-center gap-3 px-4 py-2.5 border-b border-line-soft last:border-b-0 cursor-pointer hover:bg-surface-2 transition-colors"
      style={{
        gridTemplateColumns: hasChevron
          ? "auto 1fr auto auto auto auto"
          : "1fr auto auto auto auto",
      }}
    >
      {hasChevron && (
        <span
          className={`w-5 h-5 shrink-0 grid place-items-center text-muted-fg transition-transform ${expanded ? "rotate-90" : ""}`}
        >
          <svg
            aria-hidden="true"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            strokeWidth="2.5"
            className="w-3 h-3"
          >
            <path d="M9 18l6-6-6-6" />
          </svg>
        </span>
      )}
      <div className="min-w-0 truncate font-medium text-sm">{show.title}</div>
      <span className="text-xs text-muted-dim hidden sm:inline whitespace-nowrap">
        {show.season_count} {show.season_count === 1 ? "season" : "seasons"}
      </span>
      <span className="text-xs text-muted-dim hidden md:inline whitespace-nowrap">
        {formatInt(show.file_count)} files
      </span>
      <span className="font-mono text-xs text-muted-fg">
        {formatBytes(show.total_bytes)}
      </span>
      <div className="text-right w-24">
        {allMissing ? (
          <span className="text-xs font-medium text-muted-fg">All missing</span>
        ) : show.missing_count > 0 ? (
          <span className="text-xs font-medium text-muted-fg">
            {formatInt(show.missing_count)} missing
          </span>
        ) : fullyConverted ? (
          <span className="text-xs font-medium text-green">All converted</span>
        ) : show.predicted_savings_bytes > 0 ? (
          <span className="text-xs font-semibold text-brand font-mono">
            -{formatBytes(show.predicted_savings_bytes)}
          </span>
        ) : (
          <span className="text-xs text-muted-dim">—</span>
        )}
      </div>
    </Link>
  );
}
