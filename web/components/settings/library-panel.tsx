"use client";

import { useState } from "react";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import type { MissingFilesSummary } from "@/lib/api";
import { formatBytes, formatInt, relativeTime } from "@/lib/format";
import { LabelWithHelp } from "./help-tip";

export const RETENTION_OPTIONS = [
  { value: "0", label: "Never" },
  { value: "168h0m0s", label: "7 days" },
  { value: "336h0m0s", label: "14 days" },
  { value: "720h0m0s", label: "30 days" },
  { value: "1440h0m0s", label: "60 days" },
  { value: "2160h0m0s", label: "90 days" },
];

export const LOOKBACK_OPTIONS = [
  { value: "0", label: "Off" },
  { value: "168h0m0s", label: "7 days" },
  { value: "336h0m0s", label: "14 days" },
  { value: "720h0m0s", label: "30 days" },
  { value: "2160h0m0s", label: "90 days" },
  { value: "8760h0m0s", label: "1 year" },
];

// The backend renders durations via Go's Duration.String(), so "720h0m0s" comes
// back for a value sent as "720h". Anything unrecognised (a hand-set env var
// like MISSING_RETENTION=100h) falls back to the zero option so the select
// stays valid.
function normalizeDuration(
  value: string | undefined,
  options: { value: string }[],
): string {
  if (!value || value === "0" || value === "0s") return "0";
  return options.some((o) => o.value === value) ? value : "0";
}

export function LibraryPanel({
  retention,
  onRetentionChange,
  replaceLookback,
  onReplaceLookbackChange,
  missing,
  onPurge,
  isPurging,
}: {
  retention: string;
  onRetentionChange: (v: string) => void;
  replaceLookback: string;
  onReplaceLookbackChange: (v: string) => void;
  missing: MissingFilesSummary | undefined;
  onPurge: () => void;
  isPurging: boolean;
}) {
  const [confirmOpen, setConfirmOpen] = useState(false);
  const count = missing?.count ?? 0;
  const selected = normalizeDuration(retention, RETENTION_OPTIONS);
  const lookback = normalizeDuration(replaceLookback, LOOKBACK_OPTIONS);

  return (
    <div
      className="border border-line rounded-(--radius) p-5 mt-5"
      style={{ background: "var(--surface)" }}
    >
      <div className="text-xs uppercase tracking-widest text-muted-fg font-bold mb-4">
        Library cleanup
      </div>

      <div className="mb-4">
        <LabelWithHelp
          label="Remove missing files after"
          help={
            <>
              A file that disappears from disk is kept as a{" "}
              <strong>missing</strong> row so its history survives a temporary
              NAS dropout or an unmounted share. Once it has been gone this
              long, Reclaim deletes the row and its job history for good. The
              clock starts when the file was first noticed missing, and resets
              if it comes back. <strong>Never</strong> keeps every row forever.
            </>
          }
        />
        <Select value={selected} onValueChange={onRetentionChange}>
          <SelectTrigger className="w-40">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {RETENTION_OPTIONS.map((o) => (
              <SelectItem key={o.value} value={o.value}>
                {o.label}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        <p className="text-xs text-muted-dim mt-1.5">
          {selected === "0"
            ? "Missing files are kept indefinitely."
            : "Cleanup runs after each library scan."}
        </p>
      </div>

      <div className="mb-4">
        <LabelWithHelp
          label="Match redownloads within"
          help={
            <>
              When a file is indexed that is another copy of something already
              missing — the same episode or movie from a different release —
              Reclaim pairs the two and records the size difference as reclaimed
              storage, so deleting a bloated release and grabbing a leaner one
              counts alongside a re-encode. This is how far back it will look,
              in both directions: the replacement can arrive before or after the
              old file goes away, but the other half has to fall inside the
              window. Longer windows catch slower redownloads but raise the
              chance of crediting an unrelated re-add — if you keep two cuts of
              a title side by side, a long window makes deleting one of them
              look like a replacement of the other. A missing row removed by the
              cleanup above can no longer be matched.
            </>
          }
        />
        <Select value={lookback} onValueChange={onReplaceLookbackChange}>
          <SelectTrigger className="w-40">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {LOOKBACK_OPTIONS.map((o) => (
              <SelectItem key={o.value} value={o.value}>
                {o.label}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        <p className="text-xs text-muted-dim mt-1.5">
          {lookback === "0"
            ? "Replacements are not tracked; a deleted file just goes missing."
            : "Matched pairs show up on Insights and in the activity feed."}
        </p>
      </div>

      <div className="flex items-center justify-between flex-wrap gap-3 pt-4 border-t border-line">
        <div>
          <div className="text-xs font-semibold mb-0.5">
            {count === 0
              ? "No missing files"
              : `${formatInt(count)} missing ${count === 1 ? "file" : "files"}`}
          </div>
          <p className="text-xs text-muted-dim font-mono">
            {count === 0
              ? "Every indexed file was present on the last scan."
              : `${formatBytes(missing?.size_bytes ?? 0)} last known · oldest ${relativeTime(missing?.oldest_since)}`}
          </p>
        </div>
        <Button
          variant="outline"
          onClick={() => setConfirmOpen(true)}
          disabled={count === 0 || isPurging}
          className="rounded-xl"
        >
          {isPurging ? "Removing…" : "Remove missing files now"}
        </Button>
      </div>

      <Dialog open={confirmOpen} onOpenChange={setConfirmOpen}>
        <DialogContent
          className="max-w-sm p-0 overflow-hidden border-line"
          style={{ background: "var(--surface)" }}
          showCloseButton={false}
        >
          <DialogHeader className="px-6 pt-6 pb-4 border-b border-line">
            <DialogTitle className="text-lg font-bold tracking-tight">
              Remove missing files
            </DialogTitle>
          </DialogHeader>
          <div className="px-6 py-5">
            <p className="text-sm text-muted-fg">
              Permanently delete{" "}
              <span className="font-semibold text-text">
                {formatInt(count)} missing {count === 1 ? "file" : "files"}
              </span>{" "}
              and their job history, ignoring the retention period? Files on
              disk are never touched — only Reclaim&rsquo;s records. This cannot
              be undone.
            </p>
            <p className="text-xs text-muted-dim mt-3">
              Files with a queued or running job are kept.
            </p>
          </div>
          <DialogFooter className="px-6 py-4 border-t border-line flex justify-end gap-2">
            <Button
              variant="outline"
              onClick={() => setConfirmOpen(false)}
              className="rounded-xl"
            >
              Cancel
            </Button>
            <Button
              onClick={() => {
                onPurge();
                setConfirmOpen(false);
              }}
              className="rounded-xl bg-red hover:bg-red/90 text-white border-0"
            >
              Delete {formatInt(count)}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
