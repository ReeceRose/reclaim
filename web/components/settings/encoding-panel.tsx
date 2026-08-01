"use client";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { formatClock12 } from "@/lib/format";
import { LabelWithHelp } from "./help-tip";
import { TimeSelect } from "./time-select";
import { TimezoneSelect } from "./timezone-select";

export function EncodingPanel({
  timezone,
  onTimezoneChange,
  serverTime,
  windowStart,
  windowEnd,
  onWindowStartChange,
  onWindowEndChange,
  probeConcurrency,
  onProbeConcurrencyChange,
  oversizeThreshold,
  onOversizeThresholdChange,
  scanIntervalHours,
  onScanIntervalHoursChange,
  scanAnchor,
  onScanAnchorChange,
  onSave,
  isSaving,
}: {
  timezone: string;
  onTimezoneChange: (v: string) => void;
  serverTime: string;
  windowStart: string;
  windowEnd: string;
  onWindowStartChange: (v: string) => void;
  onWindowEndChange: (v: string) => void;
  probeConcurrency: number;
  onProbeConcurrencyChange: (v: number) => void;
  oversizeThreshold: number;
  onOversizeThresholdChange: (v: number) => void;
  scanIntervalHours: number;
  onScanIntervalHoursChange: (v: number) => void;
  scanAnchor: string;
  onScanAnchorChange: (v: string) => void;
  onSave: () => void;
  isSaving: boolean;
}) {
  return (
    <div
      className="border border-line rounded-(--radius) p-5"
      style={{ background: "var(--surface)" }}
    >
      <div className="text-xs uppercase tracking-widest text-muted-fg font-bold mb-4">
        Encoding
      </div>
      <div className="mb-4">
        <LabelWithHelp
          label="Timezone"
          help={
            <>
              The clock the encode window and scan schedule are read in. The
              server itself runs on <span className="font-mono">UTC</span>; this
              setting is what turns that into your wall time, so it does not
              depend on the container&apos;s{" "}
              <span className="font-mono">TZ</span> being set correctly.
            </>
          }
        />
        <TimezoneSelect value={timezone} onChange={onTimezoneChange} />
        <p className="text-xs text-muted-dim mt-1.5">
          Server clock: {formatClock12(serverTime)} in{" "}
          {timezone.replace(/_/g, " ")}.
        </p>
      </div>
      <div className="mb-4">
        <Label className="text-xs font-semibold mb-1.5 block">
          Encode window{" "}
          <span className="text-muted-dim font-normal">
            · when jobs may run
          </span>
        </Label>
        <div className="flex items-center gap-2 flex-wrap sm:gap-3">
          <TimeSelect value={windowStart} onChange={onWindowStartChange} />
          <span className="text-muted-fg">to</span>
          <TimeSelect value={windowEnd} onChange={onWindowEndChange} />
        </div>
        <p className="text-xs text-muted-dim mt-1.5">
          A running job finishes even if the window closes — only new pulls
          stop.
        </p>
      </div>
      <div className="mb-4">
        <LabelWithHelp
          label="Probe concurrency"
          help={
            <>
              How many <span className="font-mono">ffprobe</span> processes run
              in parallel while indexing your library. Higher values scan faster
              but use more CPU and disk I/O. <strong>4</strong> is a safe
              default; bump it up on fast NAS/SSD storage, lower it if scans are
              saturating a spinning disk.
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
        <p className="text-xs text-muted-dim mt-1.5">
          Parallel ffprobe cap during scans.
        </p>
      </div>
      <div className="mb-4">
        <LabelWithHelp
          label="Oversized flag threshold"
          help={
            <>
              Flags a file as <strong>oversized</strong> in the Library when its
              bitrate is at least this many times what a well-encoded file of
              the same codec and resolution would use. It is codec-aware, so it
              catches bloated files in <em>any</em> codec — including HEVC that
              the HEVC-savings ranking skips. <strong>2</strong> means "twice
              the expected bitrate". Lower it to flag more files, raise it to
              flag only the worst offenders.
            </>
          }
        />
        <Input
          type="number"
          min={1.1}
          max={20}
          step={0.1}
          value={oversizeThreshold}
          onChange={(e) => onOversizeThresholdChange(Number(e.target.value))}
        />
        <p className="text-xs text-muted-dim mt-1.5">
          Bitrate multiple over expected before a file is flagged oversized.
        </p>
      </div>
      <div className="mb-4">
        <LabelWithHelp
          label="Scan interval"
          help={
            <>
              How often Reclaim re-walks your libraries to pick up new or
              changed files. The <strong>at</strong> time anchors the schedule,
              so a 24h interval anchored to 12:00 AM rescans nightly at
              midnight. File changes are also caught live via a filesystem
              watcher between scans.
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
              onChange={(e) =>
                onScanIntervalHoursChange(Number(e.target.value))
              }
              className="w-24"
            />
            <span className="text-sm text-muted-fg">hours · at</span>
          </div>
          <TimeSelect value={scanAnchor} onChange={onScanAnchorChange} />
        </div>
        <p className="text-xs text-muted-dim mt-1.5">
          Rescans repeat every N hours, aligned to the chosen time.
        </p>
      </div>
      <Button
        onClick={onSave}
        disabled={isSaving}
        className="rounded-xl"
        style={{
          background: "linear-gradient(145deg, var(--brand), var(--brand-2))",
        }}
      >
        {isSaving ? "Saving…" : "Save settings"}
      </Button>
    </div>
  );
}
