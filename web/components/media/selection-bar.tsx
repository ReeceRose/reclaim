'use client';

import { Button } from '@/components/ui/button';
import { formatBytes, formatInt } from '@/lib/format';

/**
 * QueueSelectionBar is the sticky action bar shown while files are selected for
 * queuing on the candidate browser and library screens.
 */
export function QueueSelectionBar({
  count,
  totalSavings,
  onClear,
  onQueue,
}: {
  count: number;
  totalSavings: number;
  onClear: () => void;
  onQueue: () => void;
}) {
  return (
    <div
      className="mx-3 mb-3 flex flex-wrap items-center gap-x-4 gap-y-2 rounded-[13px] px-4 py-[13px] border border-brand-line sticky bottom-3 sm:mx-7 sm:px-[18px]"
      style={{ background: 'var(--surface-2)', boxShadow: '0 10px 30px rgba(0,0,0,.35)' }}
    >
      <div className="font-bold">
        <b className="text-brand">{formatInt(count)}</b> selected
      </div>
      <div className="text-muted-fg text-[0.85rem] hidden sm:block">
        ≈ <span className="text-brand font-semibold">{formatBytes(totalSavings)}</span> estimated recoverable
      </div>
      <div className="ml-auto flex gap-2.5 items-center">
        <Button variant="ghost" onClick={onClear} className="rounded-[11px] text-sm">
          Clear
        </Button>
        <Button
          onClick={onQueue}
          className="rounded-[11px] text-sm"
          style={{ background: 'linear-gradient(145deg, var(--brand), var(--brand-2))', boxShadow: '0 4px 14px var(--brand-soft)' }}
        >
          Queue selected →
        </Button>
      </div>
    </div>
  );
}
