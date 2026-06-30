import { type LibrarySeriesGroup } from '@/lib/api';
import { formatBytes, formatInt } from '@/lib/format';

export function ShowRow({ show, onClick, expanded }: {
  show: LibrarySeriesGroup;
  onClick: () => void;
  expanded?: boolean;
}) {
  const fullyConverted = show.eligible_count === 0;
  const hasChevron = expanded !== undefined;
  return (
    <div
      onClick={onClick}
      className="grid items-center gap-3 px-4 py-2.5 border-b border-line-soft last:border-b-0 cursor-pointer hover:bg-surface-2 transition-colors"
      style={{ gridTemplateColumns: hasChevron ? 'auto 1fr auto auto auto auto' : '1fr auto auto auto auto' }}
    >
      {hasChevron && (
        <span className={`w-[18px] h-[18px] shrink-0 grid place-items-center text-muted-fg transition-transform ${expanded ? 'rotate-90' : ''}`}>
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" className="w-3 h-3">
            <path d="M9 18l6-6-6-6"/>
          </svg>
        </span>
      )}
      <div className="min-w-0 truncate font-medium text-sm">{show.title}</div>
      <span className="text-xs text-muted-dim hidden sm:inline whitespace-nowrap">
        {show.season_count} {show.season_count === 1 ? 'season' : 'seasons'}
      </span>
      <span className="text-xs text-muted-dim hidden md:inline whitespace-nowrap">
        {formatInt(show.file_count)} files
      </span>
      <span className="font-mono text-xs text-muted-fg">{formatBytes(show.total_bytes)}</span>
      <div className="text-right w-24">
        {fullyConverted
          ? <span className="text-xs font-medium text-green">All converted</span>
          : show.predicted_savings_bytes > 0
            ? <span className="text-xs font-semibold text-brand font-mono">-{formatBytes(show.predicted_savings_bytes)}</span>
            : <span className="text-xs text-muted-dim">—</span>
        }
      </div>
    </div>
  );
}
