'use client';

import { Label } from '@/components/ui/label';
import { Input } from '@/components/ui/input';
import { Button } from '@/components/ui/button';
import { LabelWithHelp } from './help-tip';
import { TimeSelect } from './time-select';

export function EncodingPanel({
  windowStart,
  windowEnd,
  onWindowStartChange,
  onWindowEndChange,
  probeConcurrency,
  onProbeConcurrencyChange,
  scanIntervalHours,
  onScanIntervalHoursChange,
  scanAnchor,
  onScanAnchorChange,
  onSave,
  isSaving,
}: {
  windowStart: string;
  windowEnd: string;
  onWindowStartChange: (v: string) => void;
  onWindowEndChange: (v: string) => void;
  probeConcurrency: number;
  onProbeConcurrencyChange: (v: number) => void;
  scanIntervalHours: number;
  onScanIntervalHoursChange: (v: number) => void;
  scanAnchor: string;
  onScanAnchorChange: (v: string) => void;
  onSave: () => void;
  isSaving: boolean;
}) {
  return (
    <div className="border border-line rounded-(--radius) p-5" style={{ background: 'var(--surface)' }}>
      <div className="text-[0.72rem] uppercase tracking-[0.11em] text-muted-fg font-bold mb-4">Encoding</div>
      <div className="mb-4">
        <Label className="text-[0.8rem] font-semibold mb-1.5 block">
          Encode window <span className="text-muted-dim font-normal">· when jobs may run</span>
        </Label>
        <div className="flex items-center gap-2 flex-wrap sm:gap-3">
          <TimeSelect value={windowStart} onChange={onWindowStartChange} />
          <span className="text-muted-fg">to</span>
          <TimeSelect value={windowEnd} onChange={onWindowEndChange} />
        </div>
        <p className="text-[0.75rem] text-muted-dim mt-1.5">A running job finishes even if the window closes — only new pulls stop.</p>
      </div>
      <div className="mb-4">
        <LabelWithHelp
          label="Probe concurrency"
          help={
            <>
              How many <span className="font-mono">ffprobe</span> processes run in parallel
              while indexing your library. Higher values scan faster but use more CPU and
              disk I/O. <strong>4</strong> is a safe default; bump it up on fast NAS/SSD
              storage, lower it if scans are saturating a spinning disk.
            </>
          }
        />
        <Input
          type="number"
          min={1}
          max={32}
          value={probeConcurrency}
          onChange={(e) => onProbeConcurrencyChange(Number(e.target.value))}
        />
        <p className="text-[0.75rem] text-muted-dim mt-1.5">Parallel ffprobe cap during scans.</p>
      </div>
      <div className="mb-4">
        <LabelWithHelp
          label="Scan interval"
          help={
            <>
              How often Reclaim re-walks your libraries to pick up new or changed files.
              The <strong>at</strong> time anchors the schedule, so a 24h interval anchored
              to 12:00 AM rescans nightly at midnight. File changes are also caught live via
              a filesystem watcher between scans.
            </>
          }
        />
        <div className="flex items-center gap-3 flex-wrap">
          <div className="flex items-center gap-2">
            <Input
              type="number"
              min={1}
              max={168}
              value={scanIntervalHours}
              onChange={(e) => onScanIntervalHoursChange(Number(e.target.value))}
              className="w-24"
            />
            <span className="text-[0.85rem] text-muted-fg">hours · at</span>
          </div>
          <TimeSelect value={scanAnchor} onChange={onScanAnchorChange} />
        </div>
        <p className="text-[0.75rem] text-muted-dim mt-1.5">Rescans repeat every N hours, aligned to the chosen time.</p>
      </div>
      <Button
        onClick={onSave}
        disabled={isSaving}
        className="rounded-[11px]"
        style={{ background: 'linear-gradient(145deg, var(--brand), var(--brand-2))' }}
      >
        {isSaving ? 'Saving…' : 'Save settings'}
      </Button>
    </div>
  );
}
