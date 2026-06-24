'use client';

import { useQuery, useSuspenseQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { api, type Job, type VerificationResult } from '@/lib/api';
import { formatBytes, baseName, dirName, windowInfo } from '@/lib/format';
import { Skeleton } from '@/components/ui/skeleton';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { toast } from 'sonner';
import { Suspense } from 'react';

// jobName renders the originating file name, falling back to the temp output
// path and finally a synthetic label if the media row was deleted.
function jobName(job: Job): string {
  const path = job.source_path ?? job.output_path;
  return path ? baseName(path) : `File #${job.media_file_id}`;
}

function VerifyChecks({ json }: { json: string | null }) {
  if (!json) return null;
  let vr: VerificationResult;
  try {
    vr = JSON.parse(json) as VerificationResult;
  } catch {
    return <span className="text-[0.76rem] text-muted-dim">{json}</span>;
  }

  const checks: { label: string; pass: boolean }[] = [];
  if (vr.duration_match !== undefined) checks.push({ label: vr.duration_delta_seconds != null ? `duration ±${vr.duration_delta_seconds.toFixed(1)}s` : 'duration', pass: vr.duration_match });
  if (vr.playable !== undefined) checks.push({ label: 'playable', pass: vr.playable });
  if (vr.stream_count_match !== undefined) checks.push({ label: 'stream count', pass: vr.stream_count_match });
  if (vr.resolution_match !== undefined) checks.push({ label: 'resolution', pass: vr.resolution_match });

  return (
    <div className="flex gap-[7px] mt-[7px] flex-wrap">
      {checks.map(({ label, pass }) => (
        <Badge
          key={label}
          className={`text-[0.71rem] rounded-[6px] gap-[5px] border-transparent ${pass ? 'text-green bg-green-soft' : 'text-red bg-red-soft'}`}
        >
          {pass ? '✓' : '✕'} {label}
        </Badge>
      ))}
    </div>
  );
}

function QueueSkeleton() {
  return (
    <div className="px-7 py-[26px] max-w-[1180px] w-full pb-14">
      <div className="border border-line rounded-[var(--radius)] p-5 mb-[18px]" style={{ background: 'var(--surface)' }}>
        <Skeleton className="h-4 w-24 mb-3" />
        <Skeleton className="h-5 w-64 mb-1" />
        <Skeleton className="h-3 w-40 mb-3" />
        <Skeleton className="h-[11px] w-full rounded-[7px] mb-3" />
        <div className="flex justify-between">
          <Skeleton className="h-3 w-8" />
          <Skeleton className="h-3 w-32" />
        </div>
      </div>
      <Skeleton className="h-3 w-20 mb-3" />
      {[0, 1, 2].map((i) => (
        <div key={i} className="flex items-center gap-3.5 px-4 py-[13px] border border-line rounded-[12px] bg-surface mb-2.5">
          <Skeleton className="w-7 h-7 rounded-[9px] flex-shrink-0" />
          <div className="flex-1 min-w-0">
            <Skeleton className="h-4 w-48 mb-1.5" />
            <Skeleton className="h-3 w-24" />
          </div>
          <Skeleton className="h-5 w-14 rounded-[20px]" />
          <Skeleton className="h-7 w-16 rounded-[11px]" />
        </div>
      ))}
    </div>
  );
}

function QueueContent() {
  const qc = useQueryClient();

  const { data: jobsAll } = useSuspenseQuery({
    queryKey: ['jobs'],
    queryFn: () => api.jobs(),
    refetchInterval: 3000,
  });

  const { data: progressMap = {} } = useQuery<Record<number, number>>({
    queryKey: ['job_progress'],
    queryFn: () => ({}),
    staleTime: Infinity,
    gcTime: Infinity,
  });

  const { data: settingsData } = useSuspenseQuery({ queryKey: ['settings'], queryFn: api.settings });

  const cancelMutation = useMutation({
    mutationFn: (id: number) => api.cancelJob(id),
    onSuccess: () => {
      toast.success('Job cancelled');
      qc.invalidateQueries({ queryKey: ['jobs'] });
    },
    onError: () => toast.error('Cancel failed'),
  });

  const forceMutation = useMutation({
    mutationFn: (id: number) => api.forceJob(id),
    onSuccess: () => {
      toast.success('Job will run immediately');
      qc.invalidateQueries({ queryKey: ['jobs'] });
    },
    onError: () => toast.error('Force failed'),
  });

  const all = jobsAll.items ?? [];
  const running = all.filter((j) => j.status === 'running');
  const queued = all.filter((j) => j.status === 'queued').sort((a, b) => a.queue_position - b.queue_position);
  const history = all.filter((j) => j.status === 'completed' || j.status === 'failed').sort((a, b) => (b.completed_at ?? 0) - (a.completed_at ?? 0));

  const runningJob = running[0] as Job | undefined;
  const livePercent = runningJob ? (progressMap[runningJob.id] ?? runningJob.progress_percent) : 0;
  const win = windowInfo(settingsData.encode_window_start, settingsData.encode_window_end);

  return (
    <>
      <div
        className="flex items-center gap-4 px-7 py-[18px] border-b border-line"
        style={{ background: 'rgba(22,22,22,.82)', backdropFilter: 'blur(10px)' }}
      >
        <div>
          <div className="text-[1.38rem] font-bold tracking-tight">Queue &amp; history</div>
          <div className="text-[0.82rem] text-muted-fg mt-px">
            {running.length > 0 ? `${running.length} running · ` : ''}{queued.length} queued
            {` · window ${win.label}`}
          </div>
        </div>
        <div className="ml-auto">
          <Badge variant="outline" className="gap-2 text-[0.82rem] font-semibold px-[13px] py-[7px] rounded-[10px] border-line bg-surface">
            <span className={`w-[7px] h-[7px] rounded-full flex-shrink-0 ${win.open ? 'bg-green' : 'bg-muted-dim'}`}
              style={win.open ? { boxShadow: '0 0 0 3px var(--green-soft)' } : undefined}
            />
            Window {win.open ? 'open' : 'closed'} · {win.detail}
          </Badge>
        </div>
      </div>

      <div className="px-7 py-[26px] max-w-[1180px] w-full pb-14">
        {runningJob && (
          <div
            className="border border-brand-line rounded-[var(--radius)] p-5 mb-[18px]"
            style={{ background: 'radial-gradient(120% 140% at 0% 0%, var(--brand-soft), transparent 50%), var(--surface)' }}
          >
            <div className="flex items-center gap-3 mb-1.5">
              <span className="flex items-center gap-1.5 text-[0.68rem] font-bold tracking-widest uppercase text-brand">
                <span className="w-[7px] h-[7px] rounded-full bg-brand animate-pulse" />
                Encoding
              </span>
              <span className="text-muted-fg text-[0.8rem]">libx265 · CRF 26 · preset medium</span>
            </div>
            <div className="font-semibold text-[0.88rem]">{jobName(runningJob)}</div>
            <div className="text-[0.74rem] text-muted-dim font-mono mt-0.5">{dirName(runningJob.source_path ?? runningJob.output_path ?? '')}</div>
            <div className="h-[11px] bg-surface-2 rounded-[7px] overflow-hidden my-3 shadow-[inset_0_0_0_1px_var(--line)]">
              <div
                className="h-full rounded-[7px] transition-[width_.4s]"
                style={{ width: `${livePercent}%`, background: 'linear-gradient(90deg, var(--brand), var(--brand-2))', boxShadow: '0 0 16px var(--brand-soft)' }}
              />
            </div>
            <div className="flex justify-between text-[0.8rem] text-muted-fg">
              <span>{livePercent}%</span>
              {runningJob.original_size_bytes > 0 && (
                <span className="font-mono">{formatBytes(runningJob.original_size_bytes)} → est. {formatBytes(runningJob.original_size_bytes - (runningJob.output_size_bytes ?? runningJob.original_size_bytes * 0.5))}</span>
              )}
            </div>
          </div>
        )}

        {queued.length > 0 && (
          <>
            <div className="text-[0.72rem] uppercase tracking-[0.11em] text-muted-fg font-bold mb-3">
              Queued · {queued.length}
            </div>
            {queued.map((job) => (
              <div key={job.id} className="flex items-center gap-3.5 px-4 py-[13px] border border-line rounded-[12px] bg-surface mb-2.5">
                <div className="w-7 h-7 rounded-[9px] bg-surface-3 text-muted-fg grid place-items-center font-bold text-[0.82rem] flex-shrink-0">
                  {job.queue_position}
                </div>
                <div className="flex-1 min-w-0">
                  <div className="font-semibold text-[0.88rem] truncate">{jobName(job)}</div>
                  <div className="text-[0.74rem] text-muted-dim font-mono mt-0.5 truncate">{formatBytes(job.original_size_bytes)}</div>
                </div>
                {job.forced
                  ? <Badge className="text-[0.72rem] rounded-[20px] border-transparent text-brand bg-brand-soft">forced</Badge>
                  : <Badge variant="secondary" className="text-[0.72rem] rounded-[20px] text-muted-fg">queued</Badge>
                }
                {!job.forced && (
                  <Button
                    size="sm"
                    variant="outline"
                    onClick={() => forceMutation.mutate(job.id)}
                    disabled={forceMutation.isPending}
                    className="rounded-[11px] text-[0.78rem]"
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
                  className="rounded-[11px] text-red border-red/30 hover:bg-red-soft hover:text-red"
                >
                  Cancel
                </Button>
              </div>
            ))}
          </>
        )}

        {history.length > 0 && (
          <>
            <div className="text-[0.72rem] uppercase tracking-[0.11em] text-muted-fg font-bold mb-3 mt-6">
              History
            </div>
            {history.map((job) => {
              const saved = job.original_size_bytes - (job.output_size_bytes ?? job.original_size_bytes);
              const failed = job.status === 'failed';
              return (
                <div
                  key={job.id}
                  className="flex flex-wrap items-start gap-3.5 px-4 py-[13px] border rounded-[12px] bg-surface mb-2.5"
                  style={{ borderColor: failed ? 'color-mix(in srgb, var(--red) 35%, transparent)' : 'var(--line)' }}
                >
                  <div className="flex-1 min-w-[60%]">
                    <div className="font-semibold text-[0.88rem]">{jobName(job)}</div>
                    <div className="text-[0.74rem] text-muted-dim font-mono mt-0.5">
                      {failed
                        ? 'verification failed — original untouched'
                        : `${formatBytes(job.original_size_bytes)} → ${formatBytes(job.output_size_bytes ?? 0)} · saved ${formatBytes(saved)}`}
                    </div>
                    <VerifyChecks json={job.verification_result} />
                    {failed && job.output_path && (
                      <div
                        className="text-[0.76rem] text-red mt-2 rounded-[9px] px-[11px] py-2 border"
                        style={{ background: 'var(--red-soft)', borderColor: 'color-mix(in srgb, var(--red) 28%, transparent)' }}
                      >
                        Temp output kept for inspection:<br />
                        <span className="font-mono text-[0.72rem]">{job.output_path}</span>
                      </div>
                    )}
                  </div>
                  <Badge className={`ml-auto text-[0.72rem] rounded-[20px] border-transparent ${failed ? 'text-red bg-red-soft' : 'text-green bg-green-soft'}`}>
                    {failed ? 'failed' : 'completed'}
                  </Badge>
                </div>
              );
            })}
          </>
        )}

        {all.length === 0 && (
          <div className="text-center text-muted-dim text-sm py-20">
            No jobs yet. Select candidates and queue them to get started.
          </div>
        )}
      </div>
    </>
  );
}

export default function Page() {
  return (
    <div className="flex flex-col min-w-0">
      <Suspense fallback={
        <>
          <div
            className="flex items-center gap-4 px-7 py-[18px] border-b border-line"
            style={{ background: 'rgba(22,22,22,.82)', backdropFilter: 'blur(10px)' }}
          >
            <div>
              <div className="text-[1.38rem] font-bold tracking-tight">Queue &amp; history</div>
              <Skeleton className="h-3 w-40 mt-1.5" />
            </div>
          </div>
          <QueueSkeleton />
        </>
      }>
        <QueueContent />
      </Suspense>
    </div>
  );
}
