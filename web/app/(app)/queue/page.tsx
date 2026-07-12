"use client";

import {
  useInfiniteQuery,
  useMutation,
  useQuery,
  useQueryClient,
  useSuspenseQuery,
} from "@tanstack/react-query";
import Link from "next/link";
import { Suspense, useEffect, useRef } from "react";
import { toast } from "sonner";
import { BROWSE_ROUTES } from "@/app/(app)/browse/browse";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { EmptyState } from "@/components/ui/empty-state";
import { PageHeader } from "@/components/ui/page-header";
import { Skeleton } from "@/components/ui/skeleton";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import { useNow } from "@/hooks/use-now";
import {
  api,
  type Job,
  type Profile,
  type VerificationResult,
} from "@/lib/api";
import {
  baseName,
  dirName,
  formatBytes,
  formatDurationCompact,
  relativeTime,
  windowInfo,
} from "@/lib/format";

const QUEUED_PAGE_SIZE = 25;
const HISTORY_PAGE_SIZE = 25;

// jobName renders the originating file name, falling back to the temp output
// path and finally a synthetic label if the media row was deleted.
function jobName(job: Job): string {
  const path = job.source_path ?? job.output_path;
  return path ? baseName(path) : `File #${job.media_file_id}`;
}

function encodeSettingsLabel(job: Job): string {
  const preset = job.encode_preset ?? "medium";
  const crf = job.encode_crf ?? 26;
  return `libx265 · CRF ${crf} · preset ${preset}`;
}

function formatSignedBytes(diffBytes: number): string {
  const sign = diffBytes > 0 ? "+" : diffBytes < 0 ? "-" : "";
  return `${sign}${formatBytes(Math.abs(diffBytes))}`;
}

function formatSignedDuration(diffSeconds: number): string {
  const sign = diffSeconds > 0 ? "+" : diffSeconds < 0 ? "-" : "";
  return `${sign}${formatDurationCompact(Math.abs(diffSeconds))}`;
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
      return `Based on ${n} completed jobs on this instance`;
    case "seed":
      return profileName
        ? `Conservative estimate for ${preset}/CRF ${crf} — ${profileName} has no encode history yet`
        : `Conservative estimate for ${preset}/CRF ${crf} until enough jobs complete`;
    default:
      return undefined;
  }
}

function EstimateLine({
  job,
  profileName,
}: {
  job: Job;
  profileName?: string;
}) {
  const est = job.estimated_duration_seconds;
  if (!est || est <= 0) return null;
  const tip = estimateTooltip(job, profileName);
  const line = (
    <span className="text-xs text-muted-dim font-mono mt-0.5 truncate">
      {formatBytes(job.original_size_bytes)} · ~{formatDurationCompact(est)}{" "}
      estimated
      {job.estimate_source === "seed" && (
        <Badge className="ml-1.5 text-xs font-bold tracking-widest text-brand bg-brand-soft border-brand-line rounded-md uppercase align-middle">
          estimate
        </Badge>
      )}
    </span>
  );
  if (!tip) return line;
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <span className="block cursor-help">{line}</span>
      </TooltipTrigger>
      <TooltipContent side="bottom" className="max-w-xs text-xs">
        {tip}
      </TooltipContent>
    </Tooltip>
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

// LoadMoreSentinel sits at the bottom of a paginated section; an
// IntersectionObserver on it triggers fetchNextPage as it scrolls into view.
function LoadMoreSentinel({
  onVisible,
  loading,
}: {
  onVisible: () => void;
  loading: boolean;
}) {
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const el = ref.current;
    if (!el) return;
    const observer = new IntersectionObserver(
      ([entry]) => {
        if (entry.isIntersecting) onVisible();
      },
      { rootMargin: "200px" },
    );
    observer.observe(el);
    return () => observer.disconnect();
  }, [onVisible]);

  return (
    <div ref={ref} className="py-2">
      {loading && <JobRowSkeleton />}
    </div>
  );
}

function QueueContent() {
  useNow();
  const qc = useQueryClient();

  const { data: runningData } = useSuspenseQuery({
    queryKey: ["jobs", "running"],
    queryFn: () => api.jobs({ status: "running" }),
  });

  const {
    data: queuedData,
    fetchNextPage: fetchNextQueued,
    hasNextPage: hasNextQueued,
    isFetchingNextPage: isFetchingNextQueued,
  } = useInfiniteQuery({
    queryKey: ["jobs", "queued"],
    queryFn: ({ pageParam }: { pageParam: number }) =>
      api.jobs({
        status: "queued",
        order: "queue",
        limit: QUEUED_PAGE_SIZE,
        offset: pageParam,
      }),
    initialPageParam: 0,
    getNextPageParam: (lastPage, allPages) => {
      const loaded = allPages.flatMap((p) => p.items).length;
      if (lastPage.total_count != null)
        return loaded < lastPage.total_count ? loaded : undefined;
      return lastPage.items.length === QUEUED_PAGE_SIZE ? loaded : undefined;
    },
    placeholderData: (prev) => prev,
  });

  const {
    data: historyData,
    fetchNextPage: fetchNextHistory,
    hasNextPage: hasNextHistory,
    isFetchingNextPage: isFetchingNextHistory,
  } = useInfiniteQuery({
    queryKey: ["jobs", "history"],
    queryFn: ({ pageParam }: { pageParam: number }) =>
      api.jobs({
        status: "completed,failed",
        order: "recent",
        limit: HISTORY_PAGE_SIZE,
        offset: pageParam,
      }),
    initialPageParam: 0,
    getNextPageParam: (lastPage, allPages) => {
      const loaded = allPages.flatMap((p) => p.items).length;
      if (lastPage.total_count != null)
        return loaded < lastPage.total_count ? loaded : undefined;
      return lastPage.items.length === HISTORY_PAGE_SIZE ? loaded : undefined;
    },
    placeholderData: (prev) => prev,
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
  const queuedLoaded = (queuedData?.pages.flatMap((p) => p.items) ?? []).sort(
    (a, b) => a.queue_position - b.queue_position,
  );
  const historyLoaded = (historyData?.pages.flatMap((p) => p.items) ?? []).sort(
    (a, b) => (b.completed_at ?? 0) - (a.completed_at ?? 0),
  );
  const queuedTotalCount =
    queuedData?.pages[0]?.total_count ?? queuedLoaded.length;
  const historyTotalCount =
    historyData?.pages[0]?.total_count ?? historyLoaded.length;
  const queueTotalEstimatedSeconds =
    queuedData?.pages[0]?.queue_total_estimated_seconds;
  const loadingInitial = queuedData === undefined || historyData === undefined;

  const runningJob = running[0] as Job | undefined;
  const livePercent = runningJob
    ? (progressMap[runningJob.id] ?? runningJob.progress_percent)
    : 0;
  const runningElapsed = runningJob?.started_at
    ? Math.floor(Date.now() / 1000) - runningJob.started_at
    : 0;
  const runningRemaining =
    runningJob && runningElapsed >= 0
      ? livePercent >= 3
        ? Math.round((runningElapsed * (100 - livePercent)) / livePercent)
        : Math.max(
            (runningJob.estimated_duration_seconds ?? 0) - runningElapsed,
            0,
          )
      : 0;
  const win = windowInfo(
    settingsData.encode_window_start,
    settingsData.encode_window_end,
  );

  return (
    <>
      <PageHeader
        title="Queue & history"
        subtitle={`${running.length > 0 ? `${running.length} running · ` : ""}${queuedTotalCount} queued · window ${win.label}`}
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
              <span className="text-muted-fg text-xs">
                {encodeSettingsLabel(runningJob)}
              </span>
            </div>
            <Link
              href={BROWSE_ROUTES.FILE(runningJob.media_file_id)}
              className="block group cursor-pointer"
            >
              <div className="font-semibold text-sm group-hover:text-brand transition-colors">
                {jobName(runningJob)}
              </div>
              <div className="text-xs text-muted-dim font-mono mt-0.5 group-hover:text-brand transition-colors">
                {dirName(
                  runningJob.source_path ?? runningJob.output_path ?? "",
                )}
              </div>
            </Link>
            <div className="h-3 bg-surface-2 rounded-lg overflow-hidden my-3 shadow-[inset_0_0_0_1px_var(--line)]">
              <div
                className="h-full rounded-lg transition-[width_.4s]"
                style={{
                  width: `${livePercent}%`,
                  background:
                    "linear-gradient(90deg, var(--brand), var(--brand-2))",
                  boxShadow: "0 0 16px var(--brand-soft)",
                }}
              />
            </div>
            <div className="flex justify-between text-xs text-muted-fg">
              <span>
                {livePercent}%
                {runningRemaining > 0 && (
                  <> · ~{formatDurationCompact(runningRemaining)} remaining</>
                )}
              </span>
              {runningJob.original_size_bytes > 0 && (
                <span className="font-mono">
                  {formatBytes(runningJob.original_size_bytes)} → est.{" "}
                  {formatBytes(
                    runningJob.original_size_bytes -
                      (runningJob.output_size_bytes ??
                        runningJob.original_size_bytes * 0.5),
                  )}
                </span>
              )}
            </div>
          </div>
        )}

        {(queuedData === undefined || queuedLoaded.length > 0) && (
          <>
            <div className="text-xs uppercase tracking-widest text-muted-fg font-bold mb-3">
              Queued
              {queuedData !== undefined && <> · {queuedTotalCount}</>}
              {queueTotalEstimatedSeconds != null &&
                queueTotalEstimatedSeconds > 0 && (
                  <>
                    {" "}
                    · ~{formatDurationCompact(queueTotalEstimatedSeconds)} total
                  </>
                )}
            </div>
            {queuedData === undefined ? (
              <>
                <JobRowSkeleton />
                <JobRowSkeleton />
              </>
            ) : (
              <>
                {queuedLoaded.map((job) => (
                  <div
                    key={job.id}
                    className="flex flex-wrap items-center gap-x-3 gap-y-2.5 px-4 py-3.5 border border-line rounded-xl bg-surface mb-2.5"
                  >
                    <div className="w-7 h-7 rounded-lg bg-surface-3 text-muted-fg grid place-items-center font-bold text-sm shrink-0">
                      {job.queue_position}
                    </div>
                    <Link
                      href={BROWSE_ROUTES.FILE(job.media_file_id)}
                      className="block flex-1 min-w-0 hover:opacity-80 transition-opacity cursor-pointer"
                    >
                      <div className="font-semibold text-sm truncate">
                        {jobName(job)}
                      </div>
                      <EstimateLine
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
                          onClick={() => forceMutation.mutate(job.id)}
                          disabled={forceMutation.isPending}
                          className="rounded-xl text-xs"
                          title="Run now, bypassing the encode window"
                        >
                          Run now
                        </Button>
                      )}
                      <Button
                        size="sm"
                        variant="outline"
                        onClick={() => cancelMutation.mutate(job.id)}
                        disabled={cancelMutation.isPending}
                        className="rounded-xl text-red border-red/30 hover:bg-red-soft hover:text-red"
                      >
                        Cancel
                      </Button>
                    </div>
                  </div>
                ))}
                {hasNextQueued && (
                  <LoadMoreSentinel
                    onVisible={() => void fetchNextQueued()}
                    loading={isFetchingNextQueued}
                  />
                )}
              </>
            )}
          </>
        )}

        {(historyData === undefined || historyLoaded.length > 0) && (
          <>
            <div className="text-xs uppercase tracking-widest text-muted-fg font-bold mb-3 mt-6">
              History
              {historyData !== undefined && <> · {historyTotalCount}</>}
            </div>
            {historyData === undefined ? (
              <>
                <JobRowSkeleton />
                <JobRowSkeleton />
                <JobRowSkeleton />
              </>
            ) : (
              <>
                {historyLoaded.map((job) => {
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
                      ? job.encode_duration_seconds -
                        job.estimated_duration_seconds
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
                        <div className="font-semibold text-sm">
                          {jobName(job)}
                        </div>
                        <div className="flex items-baseline gap-2 mt-0.5 flex-wrap">
                          <span className="text-xs text-muted-dim font-mono">
                            {failed
                              ? (job.error_message ?? "failed")
                              : `${formatBytes(job.original_size_bytes)} → ${formatBytes(job.output_size_bytes ?? 0)}`}
                          </span>
                          {!failed && saved > 0 && (
                            <span className="text-xs text-green font-semibold">
                              -{formatBytes(saved)}
                            </span>
                          )}
                          {!failed && predictedSavings != null && (
                            <span className="text-xs text-muted-dim font-mono">
                              predicted -{formatBytes(predictedSavings)}
                              {savingsDiff != null &&
                                Math.abs(savingsDiff) > 0 && (
                                  <>
                                    {" · "}
                                    <span
                                      className={
                                        savingsDiff >= 0
                                          ? "text-green"
                                          : "text-red"
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
                            took{" "}
                            {formatDurationCompact(job.encode_duration_seconds)}
                            {job.estimated_duration_seconds != null && (
                              <>
                                {" "}
                                (predicted ~
                                {formatDurationCompact(
                                  job.estimated_duration_seconds,
                                )}
                                {durationDiff != null &&
                                  Math.abs(durationDiff) > 0 && (
                                    <>
                                      {" · "}
                                      <span
                                        className={
                                          durationDiff <= 0
                                            ? "text-green"
                                            : "text-red"
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
                                {job.encode_crf != null && (
                                  <> · CRF {job.encode_crf}</>
                                )}
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
                            <span className="font-mono text-xs">
                              {job.output_path}
                            </span>
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
                          onClick={() => deleteMutation.mutate(job.id)}
                          disabled={deleteMutation.isPending}
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
                {hasNextHistory && (
                  <LoadMoreSentinel
                    onVisible={() => void fetchNextHistory()}
                    loading={isFetchingNextHistory}
                  />
                )}
              </>
            )}
          </>
        )}

        {!loadingInitial &&
          running.length === 0 &&
          queuedLoaded.length === 0 &&
          historyLoaded.length === 0 && (
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
                  <Link
                    href="/candidates"
                    className="text-brand hover:underline"
                  >
                    Browse candidates
                  </Link>{" "}
                  to select files.
                </>
              }
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
