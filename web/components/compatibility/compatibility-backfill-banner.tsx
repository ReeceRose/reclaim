"use client";

import { useQuery } from "@tanstack/react-query";
import { api, type BackfillTask, type ScanProgress } from "@/lib/api";
import { formatInt } from "@/lib/format";

const COMPATIBILITY_PROBE_KEY = "compatibility_probe";

function compatibilityTask(tasks: BackfillTask[] | undefined) {
  return tasks?.find((t) => t.key === COMPATIBILITY_PROBE_KEY);
}

/**
 * Surfaces backfill progress for compatibility evaluation. The backend
 * coordinator auto-starts a full rescan when needed; this banner only
 * reports status and offers a manual retry if the user dismissed a failure.
 */
export function CompatibilityBackfillBanner() {
  const { data: backfill } = useQuery({
    queryKey: ["backfill"],
    queryFn: api.backfill,
    refetchInterval: (query) => {
      const task = compatibilityTask(query.state.data?.tasks);
      return task?.needed || task?.running ? 5_000 : false;
    },
  });
  const { data: isScanning } = useQuery<boolean>({
    queryKey: ["scanning"],
    queryFn: () => false,
    initialData: false,
    staleTime: Infinity,
    gcTime: Infinity,
  });
  const { data: scanProgress } = useQuery<ScanProgress | null>({
    queryKey: ["scan_progress"],
    queryFn: () => null,
    initialData: null,
    staleTime: Infinity,
    gcTime: Infinity,
  });

  const task = compatibilityTask(backfill?.tasks);
  const runningFullScan =
    !!task?.running ||
    (isScanning &&
      (scanProgress == null ||
        (scanProgress.kind === "full" && scanProgress.trigger === "backfill")));

  if (runningFullScan) {
    const detail = scanProgress
      ? [
          `${formatInt(scanProgress.files_processed)} processed`,
          scanProgress.files_seen > scanProgress.files_processed
            ? `${formatInt(scanProgress.files_seen)} found`
            : null,
          scanProgress.errors > 0
            ? `${formatInt(scanProgress.errors)} errors`
            : null,
        ]
          .filter(Boolean)
          .join(" · ")
      : null;
    return (
      <div
        className="flex items-center gap-2 px-4 py-[9px] text-[0.75rem] font-medium text-brand border-b border-brand-line sm:px-7"
        style={{ background: "var(--brand-soft)" }}
      >
        <svg
          aria-hidden="true"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          strokeWidth="2.5"
          className="w-3.5 h-3.5 shrink-0 animate-spin"
          style={{ animationDuration: "1.1s" }}
        >
          <path d="M21 12a9 9 0 11-6.219-8.56" strokeLinecap="round" />
        </svg>
        <div className="min-w-0">
          <div>Backfilling compatibility data…</div>
          {detail && (
            <div className="truncate text-[0.66rem] font-normal opacity-80">
              {detail}
            </div>
          )}
        </div>
      </div>
    );
  }

  if (!task?.needed) return null;

  return (
    <div
      className="px-4 py-[10px] text-[0.78rem] sm:px-7"
      style={{
        background: "var(--surface-2)",
        borderBottom: "1px solid var(--line)",
      }}
    >
      <div className="text-muted-fg">
        <span className="font-medium text-text">
          Compatibility data is being prepared.
        </span>{" "}
        A full library rescan will start automatically — this may take several
        minutes on large libraries.
      </div>
    </div>
  );
}
