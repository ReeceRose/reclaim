"use client";

import {
  keepPreviousData,
  useMutation,
  useQuery,
  useQueryClient,
  useSuspenseQuery,
} from "@tanstack/react-query";
import Link from "next/link";
import { type ReactNode, Suspense } from "react";
import { toast } from "sonner";
import { BROWSE_ROUTES } from "@/app/(app)/browse/browse";
import {
  QUEUE_PAGE_SIZE,
  QUEUE_QUERY_PARAMS,
  QUEUE_TAB,
} from "@/app/(app)/queue/queue";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { EmptyState } from "@/components/ui/empty-state";
import { PageHeader } from "@/components/ui/page-header";
import { Pagination } from "@/components/ui/pagination";
import { Skeleton } from "@/components/ui/skeleton";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import { useClockFormat } from "@/hooks/use-clock-format";
import { useNow } from "@/hooks/use-now";
import { parseQueryEnum, useQueryParams } from "@/hooks/use-query-params";
import {
  api,
  type Job,
  type JobsListResult,
  type Profile,
  type VerificationResult,
} from "@/lib/api";
import { encodeSettingsLabel, targetCodecLabel } from "@/lib/codec";
import {
  baseName,
  dirName,
  formatBytes,
  formatDurationCompact,
  formatInt,
  relativeTime,
  windowInfo,
} from "@/lib/format";
import { cn } from "@/lib/utils";

// jobName renders the originating file name, falling back to the temp output
// path and finally a synthetic label if the media row was deleted.
function jobName(job: Job): string {
  const path = job.source_path ?? job.output_path;
  return path ? baseName(path) : `File #${job.media_file_id}`;
}

function jobSettingsLabel(job: Job): string {
  return encodeSettingsLabel(
    job.encode_codec,
    job.encode_crf ?? 26,
    job.encode_preset ?? "medium",
  );
}

function formatSignedBytes(diffBytes: number): string {
  const sign = diffBytes > 0 ? "+" : diffBytes < 0 ? "-" : "";
  return `${sign}${formatBytes(Math.abs(diffBytes))}`;
}

function formatSignedDuration(diffSeconds: number): string {
  const sign = diffSeconds > 0 ? "+" : diffSeconds < 0 ? "-" : "";
  return `${sign}${formatDurationCompact(Math.abs(diffSeconds))}`;
}

// sharePct renders part-of-whole as a whole-number percent, or an empty string
// when the whole is zero and the ratio would be meaningless.
function sharePct(part: number, whole: number): string {
  if (whole <= 0) return "";
  return `${Math.round((part / whole) * 100)}%`;
}

function estimateTooltip(job: Job, profileName?: string): string | undefined {
  if (job.estimate_source === "learned_profile") return undefined;
  const preset = job.encode_preset ?? "medium";
  const crf = job.encode_crf ?? 26;
  const n = job.estimate_sample_count;
  switch (job.estimate_source) {
    case "learned_preset_crf":
      return `Based on ${n} jobs at ${preset}/CRF ${crf}`;
    case "learned_preset":
      return `Based on ${n} jobs at preset ${preset}`;
    case "learned_global":
      return `Based on ${n} completed ${targetCodecLabel(job.encode_codec)} jobs on this instance`;
    case "seed":
      return profileName
        ? `Conservative estimate for ${preset}/CRF ${crf} — ${profileName} has no encode history yet`
        : `Conservative estimate for ${preset}/CRF ${crf} until enough jobs complete`;
    default:
      return undefined;
  }
}

// QueuedStats is the metric line under a queued job: what the file weighs now,
// what the queue-time prediction expects it to weigh after, and how long that
// is expected to take.
function QueuedStats({ job, profileName }: { job: Job; profileName?: string }) {
  const predicted = job.predicted_savings_bytes ?? null;
  const expected =
    predicted != null ? job.original_size_bytes - predicted : null;
  const est = job.estimated_duration_seconds;
  const tip = estimateTooltip(job, profileName);

  const time =
    est && est > 0 ? (
      <span className="inline-flex items-center gap-1.5">
        ~{formatDurationCompact(est)}
        {job.estimate_source === "seed" && (
          <Badge className="text-xs font-bold tracking-widest text-brand bg-brand-soft border-brand-line rounded-md uppercase">
            estimate
          </Badge>
        )}
      </span>
    ) : null;

  return (
    <div className="flex items-center gap-x-2 gap-y-0.5 mt-1 flex-wrap text-xs font-mono text-muted-dim">
      <span className="text-muted-fg">
        {formatBytes(job.original_size_bytes)}
        {expected != null && expected > 0 && (
          <>
            {" → "}
            {formatBytes(expected)}
          </>
        )}
      </span>
      {predicted != null && predicted > 0 && (
        <>
          <span aria-hidden="true">·</span>
          <span className="text-green font-semibold">
            -{formatBytes(predicted)}
            {sharePct(predicted, job.original_size_bytes) &&
              ` (${sharePct(predicted, job.original_size_bytes)})`}
          </span>
        </>
      )}
      {time && (
        <>
          <span aria-hidden="true">·</span>
          {tip ? (
            <Tooltip>
              <TooltipTrigger asChild>
                <span className="cursor-help">{time}</span>
              </TooltipTrigger>
              <TooltipContent side="bottom" className="max-w-xs text-xs">
                {tip}
              </TooltipContent>
            </Tooltip>
          ) : (
            time
          )}
        </>
      )}
      <span aria-hidden="true">·</span>
      <span>{jobSettingsLabel(job)}</span>
    </div>
  );
}

function VerifyChecks({ json }: { json: string | null }) {
  if (!json) return null;
  let vr: VerificationResult;
  try {
    vr = JSON.parse(json) as VerificationResult;
  } catch {
    return <span className="text-xs text-muted-dim">{json}</span>;
  }

  const checks: { label: string; pass: boolean }[] = [];
  if (vr.duration_match !== undefined)
    checks.push({
      label:
        vr.duration_delta_seconds != null
          ? `duration ±${vr.duration_delta_seconds.toFixed(1)}s`
          : "duration",
      pass: vr.duration_match,
    });
  if (vr.playable !== undefined)
    checks.push({ label: "playable", pass: vr.playable });
  if (vr.stream_count_match !== undefined)
    checks.push({ label: "stream count", pass: vr.stream_count_match });
  if (vr.resolution_match !== undefined)
    checks.push({ label: "resolution", pass: vr.resolution_match });
  if (vr.codec_match !== undefined)
    checks.push({ label: "codec", pass: vr.codec_match });

  return (
    <div className="flex gap-2 mt-2 flex-wrap">
      {checks.map(({ label, pass }) => (
        <Badge
          key={label}
          className={`text-xs rounded-md gap-1.5 border-transparent ${pass ? "text-green bg-green-soft" : "text-red bg-red-soft"}`}
        >
          {pass ? "✓" : "✕"} {label}
        </Badge>
      ))}
    </div>
  );
}

function SummaryStrip({
  items,
}: {
  items: { label: string; value: ReactNode; sub?: ReactNode; tone?: string }[];
}) {
  return (
    <div
      className="grid grid-cols-2 gap-x-6 gap-y-4 border border-line rounded-(--radius) px-5 py-4 mb-5 sm:grid-cols-3 lg:grid-cols-5"
      style={{ background: "var(--surface)" }}
    >
      {items.map((item) => (
        <div key={item.label} className="min-w-0">
          <div className="text-xs uppercase tracking-wider font-bold text-muted-fg">
            {item.label}
          </div>
          <div
            className={cn(
              "text-lg font-bold tracking-tight tnum mt-1 truncate",
              item.tone,
            )}
          >
            {item.value}
          </div>
          {item.sub && (
            <div className="text-xs text-muted-dim mt-0.5 truncate">
              {item.sub}
            </div>
          )}
        </div>
      ))}
    </div>
  );
}

function QueueSkeleton() {
  return (
    <div className="px-4 py-6 w-full pb-14 sm:px-7 sm:py-7">
      <div
        className="border border-line rounded-(--radius) p-5 mb-5"
        style={{ background: "var(--surface)" }}
      >
        <Skeleton className="h-4 w-24 mb-3" />
        <Skeleton className="h-5 w-64 mb-1" />
        <Skeleton className="h-3 w-40 mb-3" />
        <Skeleton className="h-3 w-full rounded-lg mb-3" />
        <div className="flex justify-between">
          <Skeleton className="h-3 w-8" />
          <Skeleton className="h-3 w-32" />
        </div>
      </div>
      <Skeleton className="h-3 w-20 mb-3" />
      {[0, 1, 2].map((i) => (
        <div
          key={i}
          className="flex items-center gap-3.5 px-4 py-3.5 border border-line rounded-xl bg-surface mb-2.5"
        >
          <Skeleton className="w-7 h-7 rounded-lg shrink-0" />
          <div className="flex-1 min-w-0">
            <Skeleton className="h-4 w-48 mb-1.5" />
            <Skeleton className="h-3 w-24" />
          </div>
          <Skeleton className="h-5 w-14 rounded-3xl" />
          <Skeleton className="h-7 w-16 rounded-xl" />
        </div>
      ))}
    </div>
  );
}

function JobRowSkeleton() {
  return (
    <div className="flex items-center gap-3.5 px-4 py-3.5 border border-line rounded-xl bg-surface mb-2.5">
      <Skeleton className="w-7 h-7 rounded-lg shrink-0" />
      <div className="flex-1 min-w-0">
        <Skeleton className="h-4 w-48 mb-1.5" />
        <Skeleton className="h-3 w-24" />
      </div>
      <Skeleton className="h-5 w-14 rounded-3xl" />
    </div>
  );
}

function TabButton({
  active,
  count,
  onClick,
  children,
}: {
  active: boolean;
  count?: number;
  onClick: () => void;
  children: ReactNode;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={cn(
        "rounded-lg text-xs font-semibold py-2 px-3 transition-colors cursor-pointer inline-flex items-center gap-1.5",
        active ? "bg-brand-soft text-brand" : "text-muted-fg hover:text-text",
      )}
    >
      {children}
      {count != null && count > 0 && (
        <span
          className={cn(
            "tnum text-xs font-bold",
            active ? "text-brand" : "text-muted-dim",
          )}
        >
          {formatInt(count)}
        </span>
      )}
    </button>
  );
}

function RunningCard({
  job,
  livePercent,
  elapsed,
}: {
  job: Job;
  livePercent: number;
  elapsed: number;
}) {
  const remaining =
    livePercent >= 3
      ? Math.round((elapsed * (100 - livePercent)) / livePercent)
      : Math.max((job.estimated_duration_seconds ?? 0) - elapsed, 0);
  const predicted = job.predicted_savings_bytes ?? null;
  const expected =
    predicted != null ? job.original_size_bytes - predicted : null;

  return (
    <div
      className="border border-brand-line rounded-(--radius) p-5 mb-5"
      style={{
        background:
          "radial-gradient(120% 140% at 0% 0%, var(--brand-soft), transparent 50%), var(--surface)",
      }}
    >
      <div className="flex flex-wrap items-center gap-x-3 gap-y-1 mb-1.5">
        <span className="flex items-center gap-1.5 text-xs font-bold tracking-widest uppercase text-brand">
          <span className="w-2 h-2 rounded-full bg-brand animate-pulse" />
          Encoding
        </span>
        <span className="text-muted-fg text-xs">{jobSettingsLabel(job)}</span>
        {job.forced && (
          <Badge
            className="text-xs rounded-3xl border-transparent text-brand bg-brand-soft shrink-0"
            data-tooltip="Started with Run now, so it ignores the encode window"
          >
            forced
          </Badge>
        )}
      </div>
      <Link
        href={BROWSE_ROUTES.FILE(job.media_file_id)}
        className="block group cursor-pointer"
      >
        <div className="font-semibold text-sm group-hover:text-brand transition-colors">
          {jobName(job)}
        </div>
        <div className="text-xs text-muted-dim font-mono mt-0.5 group-hover:text-brand transition-colors">
          {dirName(job.source_path ?? job.output_path ?? "")}
        </div>
      </Link>
      <div className="h-3 bg-surface-2 rounded-lg overflow-hidden my-3 shadow-[inset_0_0_0_1px_var(--line)]">
        <div
          className="h-full rounded-lg transition-[width_.4s]"
          style={{
            width: `${livePercent}%`,
            background: "linear-gradient(90deg, var(--brand), var(--brand-2))",
            boxShadow: "0 0 16px var(--brand-soft)",
          }}
        />
      </div>
      <div className="flex flex-wrap justify-between gap-x-4 gap-y-1 text-xs text-muted-fg">
        <span>
          {livePercent}%
          {elapsed > 0 && <> · {formatDurationCompact(elapsed)} elapsed</>}
          {remaining > 0 && (
            <> · ~{formatDurationCompact(remaining)} remaining</>
          )}
        </span>
        {job.original_size_bytes > 0 && (
          <span className="font-mono">
            {formatBytes(job.original_size_bytes)}
            {expected != null && expected > 0 && (
              <>
                {" → est. "}
                {formatBytes(expected)}
                <span className="text-green font-semibold">
                  {" "}
                  -{formatBytes(predicted ?? 0)}
                </span>
              </>
            )}
          </span>
        )}
      </div>
    </div>
  );
}

function QueuedList({
  page,
  setPage,
  profileByID,
  onForce,
  onCancel,
  forcePending,
  cancelPending,
}: {
  page: number;
  setPage: (page: number) => void;
  profileByID: Map<number, Profile>;
  onForce: (id: number) => void;
  onCancel: (id: number) => void;
  forcePending: boolean;
  cancelPending: boolean;
}) {
  const { data, isPlaceholderData } = useQuery<JobsListResult>({
    queryKey: ["jobs", "queued", page],
    queryFn: () =>
      api.jobs({
        status: "queued",
        order: "queue",
        limit: QUEUE_PAGE_SIZE,
        offset: (page - 1) * QUEUE_PAGE_SIZE,
      }),
    placeholderData: keepPreviousData,
  });

  if (!data) {
    return (
      <>
        <JobRowSkeleton />
        <JobRowSkeleton />
        <JobRowSkeleton />
      </>
    );
  }

  const jobs = data.items ?? [];
  const total = data.total_count ?? jobs.length;

  if (jobs.length === 0) {
    return (
      <EmptyState
        icon={
          <svg
            aria-hidden="true"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            strokeWidth="1.5"
            className="w-5 h-5"
          >
            <polygon points="5 3 19 12 5 21 5 3" />
          </svg>
        }
        title="Queue is empty"
        description={
          <>
            Jobs run inside your encode window.{" "}
            <Link href="/candidates" className="text-brand hover:underline">
              Browse candidates
            </Link>{" "}
            to select files.
          </>
        }
      />
    );
  }

  return (
    <div className={cn(isPlaceholderData && "opacity-60 transition-opacity")}>
      {jobs.map((job) => (
        <div
          key={job.id}
          className="flex flex-wrap items-center gap-x-3 gap-y-2.5 px-4 py-3.5 border border-line rounded-xl bg-surface mb-2.5"
        >
          <div className="w-7 h-7 rounded-lg bg-surface-3 text-muted-fg grid place-items-center font-bold text-sm shrink-0 tnum">
            {job.queue_position}
          </div>
          <Link
            href={BROWSE_ROUTES.FILE(job.media_file_id)}
            className="block flex-1 min-w-0 hover:opacity-80 transition-opacity cursor-pointer"
          >
            <div className="font-semibold text-sm truncate">{jobName(job)}</div>
            <QueuedStats
              job={job}
              profileName={profileByID.get(job.profile_id)?.name}
            />
          </Link>
          {job.forced ? (
            <Badge className="text-xs rounded-3xl border-transparent text-brand bg-brand-soft shrink-0">
              forced
            </Badge>
          ) : (
            <Badge
              variant="secondary"
              className="text-xs rounded-3xl text-muted-fg shrink-0"
            >
              queued
            </Badge>
          )}
          <div className="flex gap-2 basis-full justify-end sm:basis-auto sm:ml-0">
            {!job.forced && (
              <Button
                size="sm"
                variant="outline"
                onClick={() => onForce(job.id)}
                disabled={forcePending}
                className="rounded-xl text-xs"
                data-tooltip="Run now, bypassing the encode window"
              >
                Run now
              </Button>
            )}
            <Button
              size="sm"
              variant="outline"
              onClick={() => onCancel(job.id)}
              disabled={cancelPending}
              className="rounded-xl text-red border-red/30 hover:bg-red-soft hover:text-red"
            >
              Cancel
            </Button>
          </div>
        </div>
      ))}
      <Pagination
        page={page}
        pageCount={Math.max(1, Math.ceil(total / QUEUE_PAGE_SIZE))}
        onPageChange={setPage}
        totalItems={total}
        itemLabel="queued"
      />
    </div>
  );
}

function HistoryList({
  page,
  setPage,
  onDelete,
  deletePending,
}: {
  page: number;
  setPage: (page: number) => void;
  onDelete: (id: number) => void;
  deletePending: boolean;
}) {
  const { data, isPlaceholderData } = useQuery<JobsListResult>({
    queryKey: ["jobs", "history", page],
    queryFn: () =>
      api.jobs({
        status: "completed,failed",
        order: "recent",
        limit: QUEUE_PAGE_SIZE,
        offset: (page - 1) * QUEUE_PAGE_SIZE,
      }),
    placeholderData: keepPreviousData,
  });

  if (!data) {
    return (
      <>
        <JobRowSkeleton />
        <JobRowSkeleton />
        <JobRowSkeleton />
      </>
    );
  }

  const jobs = data.items ?? [];
  const total = data.total_count ?? jobs.length;
  const summary = data.history;

  if (jobs.length === 0) {
    return (
      <EmptyState
        icon={
          <svg
            aria-hidden="true"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            strokeWidth="1.5"
            className="w-5 h-5"
          >
            <circle cx="12" cy="12" r="9" />
            <polyline points="12 7 12 12 15 14" />
          </svg>
        }
        title="No history yet"
        description="Completed and failed encodes land here once the first job finishes."
      />
    );
  }

  return (
    <>
      {summary && (
        <SummaryStrip
          items={[
            {
              label: "Completed",
              value: formatInt(summary.completed_count),
            },
            {
              label: "Failed",
              value: formatInt(summary.failed_count),
              tone: summary.failed_count > 0 ? "text-red" : undefined,
            },
            {
              label: "Encoded",
              value: formatBytes(summary.original_size_bytes),
              sub: `now ${formatBytes(summary.output_size_bytes)} on disk`,
            },
            {
              label: "Reclaimed",
              value: formatSignedBytes(-summary.bytes_saved),
              tone: summary.bytes_saved > 0 ? "text-green" : undefined,
              sub:
                sharePct(summary.bytes_saved, summary.original_size_bytes) &&
                `${sharePct(summary.bytes_saved, summary.original_size_bytes)} of source`,
            },
            {
              label: "Encode time",
              value: formatDurationCompact(summary.encode_seconds),
            },
          ]}
        />
      )}
      <div className={cn(isPlaceholderData && "opacity-60 transition-opacity")}>
        {jobs.map((job) => {
          const saved =
            job.original_size_bytes -
            (job.output_size_bytes ?? job.original_size_bytes);
          const failed = job.status === "failed";
          const predictedSavings = job.predicted_savings_bytes;
          const savingsDiff =
            !failed && predictedSavings != null
              ? saved - predictedSavings
              : null;
          const durationDiff =
            !failed &&
            job.encode_duration_seconds != null &&
            job.estimated_duration_seconds != null
              ? job.encode_duration_seconds - job.estimated_duration_seconds
              : null;
          return (
            <div
              key={job.id}
              className="group flex flex-wrap items-start gap-3.5 px-4 py-3.5 border rounded-xl bg-surface mb-2.5"
              style={{
                borderColor: failed
                  ? "color-mix(in srgb, var(--red) 35%, transparent)"
                  : "var(--line)",
              }}
            >
              <Link
                href={BROWSE_ROUTES.FILE(job.media_file_id)}
                className="block flex-1 min-w-3/5 hover:opacity-80 transition-opacity cursor-pointer"
              >
                <div className="font-semibold text-sm">{jobName(job)}</div>
                <div className="flex items-baseline gap-2 mt-0.5 flex-wrap">
                  <span className="text-xs text-muted-dim font-mono">
                    {failed
                      ? (job.error_message ?? "failed")
                      : `${formatBytes(job.original_size_bytes)} → ${formatBytes(job.output_size_bytes ?? 0)}`}
                  </span>
                  {!failed && saved > 0 && (
                    <span className="text-xs text-green font-semibold">
                      -{formatBytes(saved)}
                      {sharePct(saved, job.original_size_bytes) &&
                        ` (${sharePct(saved, job.original_size_bytes)})`}
                    </span>
                  )}
                  {!failed && predictedSavings != null && (
                    <span className="text-xs text-muted-dim font-mono">
                      predicted -{formatBytes(predictedSavings)}
                      {savingsDiff != null && Math.abs(savingsDiff) > 0 && (
                        <>
                          {" · "}
                          <span
                            className={
                              savingsDiff >= 0 ? "text-green" : "text-red"
                            }
                          >
                            {formatSignedBytes(savingsDiff)}
                          </span>
                        </>
                      )}
                    </span>
                  )}
                </div>
                <VerifyChecks json={job.verification_result} />
                {!failed && job.encode_duration_seconds != null && (
                  <div className="text-xs text-muted-dim mt-1">
                    took {formatDurationCompact(job.encode_duration_seconds)}
                    {job.estimated_duration_seconds != null && (
                      <>
                        {" "}
                        (predicted ~
                        {formatDurationCompact(job.estimated_duration_seconds)}
                        {durationDiff != null && Math.abs(durationDiff) > 0 && (
                          <>
                            {" · "}
                            <span
                              className={
                                durationDiff <= 0 ? "text-green" : "text-red"
                              }
                            >
                              {formatSignedDuration(durationDiff)}
                            </span>
                          </>
                        )}
                        )
                      </>
                    )}
                    {job.encode_preset && (
                      <>
                        {" "}
                        · {job.encode_preset}
                        {job.encode_crf != null && <> · CRF {job.encode_crf}</>}
                      </>
                    )}
                  </div>
                )}
                {failed && job.output_path && (
                  <div
                    className="text-xs text-red mt-2 rounded-lg px-3 py-2 border"
                    style={{
                      background: "var(--red-soft)",
                      borderColor:
                        "color-mix(in srgb, var(--red) 28%, transparent)",
                    }}
                  >
                    Temp output kept for inspection:
                    <br />
                    <span className="font-mono text-xs">{job.output_path}</span>
                  </div>
                )}
              </Link>
              <div className="ml-auto flex items-center gap-2 shrink-0">
                {job.completed_at && (
                  <span className="text-xs text-muted-dim">
                    {relativeTime(job.completed_at)}
                  </span>
                )}
                <Badge
                  className={`text-xs rounded-3xl border-transparent ${failed ? "text-red bg-red-soft" : "text-green bg-green-soft"}`}
                >
                  {failed ? "failed" : "completed"}
                </Badge>
                <Button
                  type="button"
                  variant="ghost"
                  size="icon-sm"
                  onClick={() => onDelete(job.id)}
                  disabled={deletePending}
                  aria-label="Remove from history"
                  className="text-muted-dim opacity-0 max-sm:opacity-100 transition-opacity hover:bg-surface-2 hover:text-text group-hover:opacity-100 focus:opacity-100 disabled:opacity-40"
                >
                  <svg
                    aria-hidden="true"
                    viewBox="0 0 24 24"
                    fill="none"
                    stroke="currentColor"
                    strokeWidth="2"
                    className="w-3.5 h-3.5"
                  >
                    <line x1="18" y1="6" x2="6" y2="18" />
                    <line x1="6" y1="6" x2="18" y2="18" />
                  </svg>
                </Button>
              </div>
            </div>
          );
        })}
        <Pagination
          page={page}
          pageCount={Math.max(1, Math.ceil(total / QUEUE_PAGE_SIZE))}
          onPageChange={setPage}
          totalItems={total}
          itemLabel="jobs"
        />
      </div>
    </>
  );
}

function QueueContent() {
  const now = useNow();
  const clockFormat = useClockFormat();
  const qc = useQueryClient();

  // Tab and page are written through one `set` call: useQueryParams reads the
  // live URL from a ref that only refreshes after a render, so two successive
  // single-key writes in the same handler would drop the first.
  const { get, set } = useQueryParams();
  const tab = parseQueryEnum(
    get(QUEUE_QUERY_PARAMS.TAB),
    [QUEUE_TAB.QUEUED, QUEUE_TAB.HISTORY] as const,
    QUEUE_TAB.QUEUED,
  );
  const page = Math.max(
    1,
    Number.parseInt(get(QUEUE_QUERY_PARAMS.PAGE) ?? "1", 10) || 1,
  );
  const setPage = (next: number) =>
    set({ [QUEUE_QUERY_PARAMS.PAGE]: next <= 1 ? null : String(next) });
  const selectTab = (next: string) =>
    set({
      [QUEUE_QUERY_PARAMS.TAB]: next === QUEUE_TAB.QUEUED ? null : next,
      [QUEUE_QUERY_PARAMS.PAGE]: null,
    });

  const { data: runningData } = useSuspenseQuery({
    queryKey: ["jobs", "running"],
    queryFn: () => api.jobs({ status: "running" }),
  });

  // Same key as the shell's sidebar badge, so the queue totals ride along on a
  // request the app already makes on every page and both stay in step.
  const { data: queueSummary } = useQuery({
    queryKey: ["jobs", "queued-count"],
    queryFn: () => api.jobs({ status: "queued", limit: 1 }),
  });

  const { data: progressMap = {} } = useQuery<Record<number, number>>({
    queryKey: ["job_progress"],
    queryFn: () => ({}),
    staleTime: Infinity,
    gcTime: Infinity,
  });

  const { data: settingsData } = useSuspenseQuery({
    queryKey: ["settings"],
    queryFn: api.settings,
    refetchInterval: 60_000,
  });

  const { data: profilesData } = useSuspenseQuery({
    queryKey: ["profiles"],
    queryFn: api.profiles,
  });
  const profileByID = new Map<number, Profile>(
    (profilesData.items ?? []).map((p) => [p.id, p]),
  );

  const cancelMutation = useMutation({
    mutationFn: (id: number) => api.cancelJob(id),
    onSuccess: () => {
      toast.success("Job cancelled");
      qc.invalidateQueries({ queryKey: ["jobs"] });
    },
    onError: () => toast.error("Cancel failed"),
  });

  const forceMutation = useMutation({
    mutationFn: (id: number) => api.forceJob(id),
    onSuccess: () => {
      toast.success("Queued to run outside the encode window");
      qc.invalidateQueries({ queryKey: ["jobs"] });
    },
    onError: () => toast.error("Force failed"),
  });

  const deleteMutation = useMutation({
    mutationFn: (id: number) => api.deleteJob(id),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["jobs"] });
    },
    onError: () => toast.error("Delete failed"),
  });

  const running = runningData.items ?? [];
  const runningJob = running[0] as Job | undefined;
  const livePercent = runningJob
    ? (progressMap[runningJob.id] ?? runningJob.progress_percent)
    : 0;
  const runningElapsed = runningJob?.started_at
    ? Math.max(Math.floor(now.getTime() / 1000) - runningJob.started_at, 0)
    : 0;

  const queuedCount = queueSummary?.total_count ?? 0;
  const queuedBytes = queueSummary?.queue_total_original_bytes ?? 0;
  const queuedSavings = queueSummary?.queue_total_predicted_savings_bytes ?? 0;
  const queuedSeconds = queueSummary?.queue_total_estimated_seconds ?? 0;
  const win = windowInfo(settingsData, now, clockFormat);

  const isQueued = tab === QUEUE_TAB.QUEUED;

  return (
    <>
      <PageHeader
        title="Queue & history"
        subtitle={`${running.length > 0 ? `${running.length} running · ` : ""}${formatInt(queuedCount)} queued · window ${win.label} ${settingsData.timezone}`}
      >
        <div className="sm:ml-auto">
          <Badge
            variant="outline"
            className="gap-2 text-sm font-semibold px-3.5 py-2 rounded-xl border-line bg-surface"
          >
            <span
              className={`w-2 h-2 rounded-full shrink-0 ${win.open ? "bg-green" : "bg-muted-dim"}`}
              style={
                win.open
                  ? { boxShadow: "0 0 0 3px var(--green-soft)" }
                  : undefined
              }
            />
            Window {win.open ? "open" : "closed"} · {win.detail}
          </Badge>
        </div>
      </PageHeader>

      <div className="px-4 py-6 w-full pb-14 sm:px-7 sm:py-7">
        {runningJob && (
          <RunningCard
            job={runningJob}
            livePercent={livePercent}
            elapsed={runningElapsed}
          />
        )}

        <div className="inline-flex bg-surface border border-line rounded-xl p-1 gap-0.5 mb-5">
          <TabButton
            active={isQueued}
            count={queuedCount}
            onClick={() => selectTab(QUEUE_TAB.QUEUED)}
          >
            Queued
          </TabButton>
          <TabButton
            active={!isQueued}
            onClick={() => selectTab(QUEUE_TAB.HISTORY)}
          >
            History
          </TabButton>
        </div>

        {isQueued ? (
          <>
            {queuedCount > 0 && (
              <SummaryStrip
                items={[
                  { label: "In queue", value: formatInt(queuedCount) },
                  { label: "Current size", value: formatBytes(queuedBytes) },
                  {
                    label: "Estimated after",
                    value: formatBytes(
                      Math.max(queuedBytes - queuedSavings, 0),
                    ),
                  },
                  {
                    label: "Estimated savings",
                    value: `-${formatBytes(queuedSavings)}`,
                    tone: queuedSavings > 0 ? "text-green" : undefined,
                    sub:
                      sharePct(queuedSavings, queuedBytes) &&
                      `${sharePct(queuedSavings, queuedBytes)} of queued bytes`,
                  },
                  {
                    label: "Estimated time",
                    value: formatDurationCompact(queuedSeconds),
                    sub:
                      running.length > 0
                        ? "includes the running job"
                        : undefined,
                  },
                ]}
              />
            )}
            <QueuedList
              page={page}
              setPage={setPage}
              profileByID={profileByID}
              onForce={(id) => forceMutation.mutate(id)}
              onCancel={(id) => cancelMutation.mutate(id)}
              forcePending={forceMutation.isPending}
              cancelPending={cancelMutation.isPending}
            />
          </>
        ) : (
          <HistoryList
            page={page}
            setPage={setPage}
            onDelete={(id) => deleteMutation.mutate(id)}
            deletePending={deleteMutation.isPending}
          />
        )}
      </div>
    </>
  );
}

export default function Page() {
  return (
    <div className="flex flex-col min-w-0">
      <Suspense
        fallback={
          <>
            <PageHeader
              title="Queue & history"
              subtitle={<Skeleton className="h-3 w-40 mt-1.5" />}
            />
            <QueueSkeleton />
          </>
        }
      >
        <QueueContent />
      </Suspense>
    </div>
  );
}
