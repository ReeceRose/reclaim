'use client';

import { Suspense } from 'react';
import Link from 'next/link';
import { useQuery } from '@tanstack/react-query';
import { api } from '@/lib/api';
import { baseName, dirName, formatBytes, resolutionLabel } from '@/lib/format';
import { Badge } from '@/components/ui/badge';
import { Skeleton } from '@/components/ui/skeleton';
import { useQueryParams } from '@/hooks/use-query-params';
import { CodecBadge } from '../page';
import { BROWSE_ROUTES, LIBRARY_TYPE, QUERY_PARAMS } from '../browse';

const STATE_LABELS: Record<string, string> = {
  candidate:     'Candidate',
  already_hevc:  'Already HEVC',
  probe_failed:  'Probe failed',
  unknown_codec: 'Unknown codec',
  queued:        'Queued',
  completed:     'Completed',
  missing:       'Missing',
};

function StateBadge({ state }: { state: string }) {
  const isGood   = state === 'already_hevc' || state === 'completed';
  const isBrand  = state === 'candidate';
  const isQueued = state === 'queued';
  const cls = isGood
    ? 'text-green border-green-soft bg-green-soft'
    : isBrand
      ? 'text-brand border-brand-line bg-brand-soft'
      : isQueued
        ? 'text-sky border-[rgba(51,177,255,.32)] bg-[rgba(51,177,255,.1)]'
        : 'text-muted-fg border-line bg-surface-3';
  return (
    <Badge className={`text-[0.7rem] rounded-[6px] font-semibold ${cls}`}>
      {STATE_LABELS[state] ?? state}
    </Badge>
  );
}

function DetailRow({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="flex items-start gap-4 py-[11px] border-b border-line-soft last:border-b-0">
      <span className="text-[0.72rem] uppercase tracking-wider text-muted-dim font-bold w-[100px] shrink-0 pt-px">{label}</span>
      <div className="text-[0.88rem] text-text min-w-0">{children}</div>
    </div>
  );
}

function MoviePageContent() {
  const { get } = useQueryParams();
  const idRaw = get('id');
  const id = idRaw ? Number(idRaw) : null;

  const { data: file, isLoading } = useQuery({
    queryKey: ['file', id],
    queryFn: () => api.file(id!),
    enabled: id !== null && !Number.isNaN(id),
  });

  const backHref = `${BROWSE_ROUTES.ROOT}?${QUERY_PARAMS.TAB}=${LIBRARY_TYPE.MOVIES}`;

  if (!id || Number.isNaN(id)) {
    return (
      <div className="flex flex-col items-center justify-center h-full py-32 text-center">
        <div className="text-[0.9rem] font-semibold text-text">No movie selected</div>
        <Link href={backHref} className="text-[0.82rem] text-brand hover:underline mt-3 cursor-pointer">← Back to Browse</Link>
      </div>
    );
  }

  if (isLoading) {
    return (
      <div className="flex flex-col min-w-0">
        <div className="px-4 py-[18px] border-b border-line sm:px-7">
          <Skeleton className="h-3 w-16 mb-4" />
          <Skeleton className="h-8 w-72 mb-3" />
          <Skeleton className="h-3 w-44" />
        </div>
        <div className="px-4 pt-6 sm:px-7">
          <div className="max-w-[680px] bg-surface border border-line rounded-[14px] px-5 divide-y">
            {Array.from({ length: 5 }).map((_, i) => (
              <div key={i} className="flex items-center gap-4 py-[11px]">
                <Skeleton className="h-3 w-[90px] shrink-0" />
                <Skeleton className="h-4 w-48" />
              </div>
            ))}
          </div>
        </div>
      </div>
    );
  }

  if (!file) {
    return (
      <div className="flex flex-col items-center justify-center h-full py-32 text-center">
        <div className="text-[0.9rem] font-semibold text-text">File not found</div>
        <div className="text-[0.8rem] text-muted-dim mt-2 mb-5">This file could not be found in the library.</div>
        <Link href={backHref} className="text-[0.82rem] text-brand hover:underline cursor-pointer">← Back to Browse</Link>
      </div>
    );
  }

  const title = baseName(file.path).replace(/\.[^/.]+$/, '');
  const isCandidate = file.candidate_state === 'candidate';

  return (
    <div className="flex flex-col min-w-0 h-screen overflow-hidden max-sm:h-full">
      <div
        className="px-4 py-[18px] border-b border-line shrink-0 sm:px-7"
        style={{ background: 'rgba(22,22,22,.82)', backdropFilter: 'blur(10px)' }}
      >
        <Link
          href={backHref}
          className="inline-flex items-center gap-1 text-[0.75rem] text-muted-dim hover:text-text transition-colors mb-3 cursor-pointer"
        >
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" className="w-3 h-3">
            <path d="M19 12H5M12 5l-7 7 7 7"/>
          </svg>
          Browse
        </Link>

        <div className="flex items-center gap-2 mb-2">
          <Badge className="text-[0.68rem] rounded-[6px] text-violet border-[rgba(190,149,255,.3)] bg-[rgba(190,149,255,.1)]">Movie</Badge>
          <StateBadge state={file.candidate_state} />
        </div>

        <h1 className="text-[1.5rem] font-bold tracking-tight leading-tight">{title}</h1>

        {isCandidate && file.predicted_savings_bytes > 0 && (
          <div className="mt-2 text-[0.88rem] text-brand font-semibold">
            -{formatBytes(file.predicted_savings_bytes)} estimated savings
          </div>
        )}
      </div>

      <div className="flex-1 overflow-auto px-4 pt-5 pb-8 sm:px-7">
        <div className="max-w-[680px]">
          <div className="bg-surface border border-line rounded-[14px] px-5">
            <DetailRow label="Codec">
              <CodecBadge codec={file.video_codec} />
            </DetailRow>

            {file.width && file.height && (
              <DetailRow label="Resolution">
                <span className="font-mono">{file.width}×{file.height}</span>
                <span className="text-muted-dim ml-2">({resolutionLabel(file.width, file.height)})</span>
              </DetailRow>
            )}

            <DetailRow label="Size">
              <span className="font-mono">{formatBytes(file.size_bytes)}</span>
            </DetailRow>

            {isCandidate && file.predicted_savings_bytes > 0 && (
              <DetailRow label="Est. savings">
                <span className="font-semibold text-brand font-mono">-{formatBytes(file.predicted_savings_bytes)}</span>
              </DetailRow>
            )}

            <DetailRow label="Directory">
              <span className="font-mono text-[0.82rem] text-muted-fg break-all">{dirName(file.path)}</span>
            </DetailRow>

            <DetailRow label="File">
              <span className="font-mono text-[0.82rem] break-all">{baseName(file.path)}</span>
            </DetailRow>
          </div>
        </div>
      </div>
    </div>
  );
}

function PageSkeleton() {
  return (
    <div className="flex flex-col min-w-0">
      <div className="px-4 py-[18px] border-b border-line sm:px-7">
        <Skeleton className="h-3 w-16 mb-4" />
        <Skeleton className="h-8 w-72 mb-3" />
        <Skeleton className="h-3 w-44" />
      </div>
    </div>
  );
}

export default function Page() {
  return (
    <Suspense fallback={<PageSkeleton />}>
      <MoviePageContent />
    </Suspense>
  );
}
