"use client";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { useClockFormat } from "@/hooks/use-clock-format";
import { useNow } from "@/hooks/use-now";
import type { ClockFormat, Settings } from "@/lib/api";
import { formatClock, formatZoneClock, windowInfo } from "@/lib/format";
import { LabelWithHelp } from "./help-tip";
import { TimeSelect } from "./time-select";
import { TimezoneSelect } from "./timezone-select";

export function EncodingPanel({
  timezone,
  savedTimezone,
  onTimezoneChange,
  serverTime,
  windowState,
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
  onClockFormatChange,
  onSave,
  isSaving,
}: {
  timezone: string;
  savedTimezone: string;
  onTimezoneChange: (v: string) => void;
  serverTime: string;
  windowState: Pick<
    Settings,
    | "encode_window_start"
    | "encode_window_end"
    | "window_open"
    | "window_changes_at"
  >;
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
  onClockFormatChange: (v: ClockFormat) => void;
  onSave: () => void;
  isSaving: boolean;
}) {
  const now = useNow();
  const clockFormat = useClockFormat();
  const win = windowInfo(windowState, now, clockFormat);
  const zoneChanged = timezone !== savedTimezone;
  const clock = zoneChanged
    ? formatZoneClock(now, timezone, clockFormat)
    : formatClock(serverTime, clockFormat);
  const windowChanged =
    windowStart !== windowState.encode_window_start ||
    windowEnd !== windowState.encode_window_end;

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
          {clock
            ? `${zoneChanged ? "Would read" : "Server clock"}: ${clock} in ${timezone.replace(/_/g, " ")}.`
            : `Server clock unavailable in ${timezone.replace(/_/g, " ")}.`}
          {zoneChanged && " Save to switch the server to this zone."}
        </p>
      </div>
      <div className="mb-4">
        <LabelWithHelp
          label="Clock format"
          help={
            <>
              How times are displayed across Reclaim — the encode window, the
              scan schedule, and the window badge in the sidebar. It is stored
              on the server and shared by everyone using this instance, it saves
              as soon as you pick it, and it never changes when jobs actually
              run.
            </>
          }
        />
        <Select
          value={clockFormat}
          onValueChange={(v) => onClockFormatChange(v as ClockFormat)}
        >
          <SelectTrigger className="w-full rounded-xl text-sm sm:w-72">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="12h">12-hour · 10 PM</SelectItem>
            <SelectItem value="24h">24-hour · 22:00</SelectItem>
          </SelectContent>
        </Select>
        <p className="text-xs text-muted-dim mt-1.5">
          Applies everywhere in Reclaim, and saves immediately.
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
          <TimeSelect
            value={windowStart}
            onChange={onWindowStartChange}
            format={clockFormat}
          />
          <span className="text-muted-fg">to</span>
          <TimeSelect
            value={windowEnd}
            onChange={onWindowEndChange}
            format={clockFormat}
          />
        </div>
        <div className="flex items-center gap-2 flex-wrap mt-2.5">
          <Badge
            variant="outline"
            className="gap-2 text-xs font-semibold px-2.5 py-1 rounded-xl border-line bg-surface-2"
          >
            <span
              className={`w-1.5 h-1.5 rounded-full shrink-0 ${win.open ? "bg-green" : "bg-muted-dim"}`}
              style={
                win.open
                  ? { boxShadow: "0 0 0 3px var(--green-soft)" }
                  : undefined
              }
            />
            {win.open ? "Open" : "Closed"} · {win.detail}
          </Badge>
          <span className="text-xs text-muted-dim">
            {windowChanged
              ? `Running window ${win.label} ${savedTimezone.replace(/_/g, " ")} — save to apply the change.`
              : `${win.label} ${savedTimezone.replace(/_/g, " ")}.`}
          </span>
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
              so a 24h interval anchored to midnight rescans nightly at
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
          <TimeSelect
            value={scanAnchor}
            onChange={onScanAnchorChange}
            format={clockFormat}
          />
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
