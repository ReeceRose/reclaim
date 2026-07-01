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
import type { MediaFile, Profile } from "@/lib/api";
import { baseName, formatBytes, formatInt } from "@/lib/format";

/**
 * QueueConfirmDialog previews a selection before queuing encode jobs.
 *
 * `subtitle` lets each caller explain its own selection semantics, and the
 * candidate browser opts into `showSafetyNote`/`showMoreCount` for its richer
 * summary while the library keeps the compact form.
 */
export function QueueConfirmDialog({
  open,
  onClose,
  selectedFiles,
  profiles,
  onConfirm,
  subtitle,
  showSafetyNote = false,
  showMoreCount = false,
}: {
  open: boolean;
  onClose: () => void;
  selectedFiles: MediaFile[];
  profiles: Profile[];
  onConfirm: (profileId: number | null) => Promise<void>;
  subtitle: string;
  showSafetyNote?: boolean;
  showMoreCount?: boolean;
}) {
  const defaultProfile = profiles.find((p) => p.is_default) ?? profiles[0];
  const [profileId, setProfileId] = useState<number | null>(null);
  const [loading, setLoading] = useState(false);

  const totalSavings = selectedFiles.reduce(
    (s, f) => s + f.predicted_savings_bytes,
    0,
  );
  const preview = selectedFiles.slice(0, 8);
  const more = selectedFiles.length - preview.length;

  async function handleConfirm() {
    setLoading(true);
    try {
      await onConfirm(profileId ?? defaultProfile?.id ?? null);
      onClose();
    } finally {
      setLoading(false);
    }
  }

  return (
    <Dialog open={open} onOpenChange={(o) => !o && onClose()}>
      <DialogContent
        className="max-w-[540px] p-0 overflow-hidden border-line"
        style={{ background: "var(--surface)" }}
      >
        <DialogHeader className="px-6 pt-[22px] pb-4 border-b border-line">
          <DialogTitle className="text-[1.2rem] font-bold tracking-tight">
            Confirm queue
          </DialogTitle>
          <p className="text-[0.85rem] text-muted-fg mt-1">{subtitle}</p>
        </DialogHeader>

        <div className="px-6 py-5 max-h-[300px] overflow-auto">
          <div className="flex gap-6 mb-[18px] flex-wrap">
            <div>
              <div className="text-[0.72rem] uppercase tracking-wider text-muted-fg">
                Files
              </div>
              <div className="text-[1.55rem] font-bold tracking-tight mt-0.5">
                {formatInt(selectedFiles.length)}
              </div>
            </div>
            <div>
              <div className="text-[0.72rem] uppercase tracking-wider text-muted-fg">
                Est. recoverable
              </div>
              <div className="text-[1.55rem] font-bold tracking-tight mt-0.5 text-brand">
                {formatBytes(totalSavings)}
              </div>
            </div>
            <div>
              <div className="text-[0.72rem] uppercase tracking-wider text-muted-fg">
                Profile
              </div>
              {profiles.length > 1 ? (
                <Select
                  value={String(profileId ?? defaultProfile?.id ?? "")}
                  onValueChange={(v) => setProfileId(Number(v))}
                >
                  <SelectTrigger className="mt-0.5 h-8 rounded-lg text-sm">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {profiles.map((p) => (
                      <SelectItem key={p.id} value={String(p.id)}>
                        {p.name}
                        {p.is_default ? " (default)" : ""}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              ) : (
                <div className="text-[1.1rem] font-bold tracking-tight mt-0.5">
                  {defaultProfile?.name ?? "—"}
                </div>
              )}
            </div>
          </div>

          <div className="text-[0.82rem]">
            {preview.map((f) => (
              <div
                key={f.id}
                className="flex justify-between gap-3 py-[7px] border-b border-line-soft last:border-b-0"
              >
                <span className="truncate text-muted-fg">
                  {baseName(f.path)}
                </span>
                <span className="text-brand font-medium shrink-0">
                  -{formatBytes(f.predicted_savings_bytes)}
                </span>
              </div>
            ))}
            {showMoreCount && more > 0 && (
              <div className="text-[0.78rem] text-muted-dim pt-2">
                …and {more} more
              </div>
            )}
          </div>

          {showSafetyNote && (
            <div
              className="flex items-start gap-[9px] text-[0.8rem] text-muted-fg mt-4 rounded-[11px] px-[13px] py-[11px] border"
              style={{
                background: "var(--green-soft)",
                borderColor:
                  "color-mix(in srgb, var(--green) 26%, transparent)",
              }}
            >
              <svg
                aria-hidden="true"
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                strokeWidth="2"
                className="w-4 h-4 text-green shrink-0 mt-px"
              >
                <path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z" />
              </svg>
              <div>
                <b className="text-text font-semibold">Non-destructive.</b> Each
                original is kept until its re-encode passes verification, then
                swapped atomically. Jobs run only inside your encode window.
              </div>
            </div>
          )}
        </div>

        <DialogFooter className="px-6 py-4 border-t border-line flex justify-end gap-2">
          <Button
            variant="outline"
            onClick={onClose}
            className="rounded-[11px]"
          >
            Cancel
          </Button>
          <Button
            onClick={() => void handleConfirm()}
            disabled={loading || selectedFiles.length === 0}
            className="rounded-[11px]"
            style={{
              background:
                "linear-gradient(145deg, var(--brand), var(--brand-2))",
            }}
          >
            Queue {formatInt(selectedFiles.length)} jobs
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
