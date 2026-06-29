'use client';

import { Checkbox } from '@/components/ui/checkbox';
import { baseName, dirName, formatBytes, resolutionLabel } from '@/lib/format';
import { cn } from '@/lib/utils';
import type { MediaFile } from '@/lib/api';
import { CodecBadge } from './codec-badge';
import { StateBadge, isQueueable, queueBlockReason } from './candidate-state';

/**
 * MediaFlatRow is the single 52px file row shared by the candidate browser and
 * the full library.
 *
 * - `showState` adds the candidate-state column and the "unknown" codec pill
 *   (the library shows both; the candidate browser does not).
 * - `gateSelection` restricts selection to queueable files and renders the
 *   predicted-savings cell as "-" for everything else (library behaviour). The
 *   candidate browser only ever lists queueable files, so it leaves this off.
 */
export function MediaFlatRow({
  item,
  selected,
  onToggle,
  onOpen,
  showState = false,
  gateSelection = false,
}: {
  item: MediaFile;
  selected: boolean;
  onToggle: (id: number) => void;
  onOpen: (file: MediaFile) => void;
  showState?: boolean;
  gateSelection?: boolean;
}) {
  const queueable = !gateSelection || isQueueable(item);
  const missing = item.status === 'missing';
  return (
    <div
      className={cn(
        'flex items-center gap-0 border-b border-line-soft hover:bg-surface-2 cursor-pointer transition-colors',
        selected && 'bg-brand-soft',
        missing && 'opacity-70',
      )}
      style={{ height: 52 }}
      onClick={() => onOpen(item)}
    >
      <div
        className="w-[52px] flex justify-center shrink-0"
        title={gateSelection ? (queueable ? 'Queue candidate' : queueBlockReason(item)) : undefined}
      >
        <Checkbox
          checked={selected}
          disabled={!queueable}
          onCheckedChange={() => queueable && onToggle(item.id)}
          onClick={(e) => e.stopPropagation()}
          className="size-[17px] rounded-[5px]"
        />
      </div>
      <div className="flex-1 min-w-0 pr-3">
        <div className={cn('font-semibold text-[0.88rem] truncate', missing && 'line-through text-muted-fg')}>
          {baseName(item.path)}
        </div>
        <div className="text-[0.74rem] text-muted-dim truncate font-mono">{dirName(item.path)}</div>
      </div>
      <div className="w-[64px] sm:w-[80px] shrink-0">
        <CodecBadge codec={item.video_codec} showUnknown={showState} />
      </div>
      <div className="hidden sm:block w-[60px] shrink-0 text-[0.82rem] text-muted-fg">
        {resolutionLabel(item.width, item.height)}
      </div>
      {showState && (
        <div className="hidden lg:block w-[118px] shrink-0">
          <StateBadge state={item.candidate_state} />
        </div>
      )}
      <div className="hidden sm:block w-[90px] shrink-0 text-right text-[0.82rem] text-muted-fg pr-2 font-mono">
        {formatBytes(item.size_bytes)}
      </div>
      <div className="w-[84px] sm:w-[110px] shrink-0 text-right text-[0.84rem] sm:text-[0.88rem] pr-3 sm:pr-4 font-mono">
        {queueable ? (
          <span className="text-brand font-semibold">{formatBytes(item.predicted_savings_bytes)}</span>
        ) : (
          <span className="text-muted-dim">-</span>
        )}
      </div>
    </div>
  );
}
