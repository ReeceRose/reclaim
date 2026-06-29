'use client';

import { Suspense } from 'react';
import { useRouter } from 'next/navigation';
import { useQuery } from '@tanstack/react-query';
import { api, type MediaFile } from '@/lib/api';
import {
  baseName,
  dirName,
  formatBytes,
  formatDuration,
  relativeTime,
  resolutionLabel,
} from '@/lib/format';
import { Badge } from '@/components/ui/badge';
import { Skeleton } from '@/components/ui/skeleton';
import { useQueryParams } from '@/hooks/use-query-params';

function DetailRow({ label, value, mono }: { label: string; value: React.ReactNode; mono?: boolean }) {
  if (value == null || value === '' || value === '—') return null;
  return (
    <div className="flex items-baseline justify-between gap-4 py-[9px] border-b border-line-soft last:border-b-0">
      <span className="text-[0.78rem] text-muted-fg shrink-0">{label}</span>
      <span className={`text-[0.82rem] text-right ${mono ? 'font-mono' : ''}`}>{value}</span>
    </div>
  );
}

function DetailSection({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <div className="mb-5 last:mb-0">
      <div className="text-[0.68rem] uppercase tracking-[0.11em] text-muted-dim font-bold mb-2">{title}</div>
      <div className="rounded-[11px] border border-line px-3" style={{ background: 'var(--surface-2)' }}>
        {children}
      </div>
    </div>
  );
}

function candidateStateReason(file: MediaFile): string | null {
  switch (file.candidate_state) {
    case 'already_hevc':
      return 'This file is already HEVC/H.265, so Reclaim does not queue it for another HEVC encode.';
    case 'probe_failed':
      return 'ffprobe could not read this file successfully. Fix the source file or rescan after the probe issue is resolved.';
    case 'unknown_codec':
      return 'Reclaim could not identify a source video codec for this file.';
    case 'queued':
      return 'This file already has an active encode job.';
    case 'completed':
      return 'This file has already completed an encode job.';
    case 'missing':
      return 'This file was seen before, but is currently missing from disk or outside the configured library roots.';
    default:
      return null;
  }
}

function FilePageContent() {
  const router = useRouter();
  const { get } = useQueryParams();
  const idRaw = get('id');
  const id = idRaw ? Number(idRaw) : null;

  const { data: file, isLoading, isError } = useQuery({
    queryKey: ['file', id],
    queryFn: () => api.file(id!),
    enabled: id !== null && !Number.isNaN(id),
  });

  if (!id || Number.isNaN(id)) {
    return (
      <div className="flex flex-col items-center justify-center h-full py-32 text-center">
        <div className="text-[0.9rem] font-semibold text-text">No file selected</div>
        <button onClick={() => router.back()} className="text-[0.82rem] text-brand hover:underline mt-3 cursor-pointer">
          ← Back
        </button>
      </div>
    );
  }

  if (isLoading) return <FileSkeleton />;

  if (isError || !file) {
    return (
      <div className="flex flex-col items-center justify-center h-full py-32 text-center">
        <div className="text-[0.9rem] font-semibold text-text">File not found</div>
        <div className="text-[0.8rem] text-muted-dim mt-2 mb-5">This file could not be found in the library.</div>
        <button onClick={() => router.back()} className="text-[0.82rem] text-brand hover:underline cursor-pointer">
          ← Back
        </button>
      </div>
    );
  }

  const resolution =
    file.width && file.height
      ? `${file.width}×${file.height} (${resolutionLabel(file.width, file.height)})`
      : resolutionLabel(file.width, file.height);
  const notCandidateReason = candidateStateReason(file);
  const audioChannels =
    file.audio_channels != null
      ? file.audio_channels === 1
        ? 'Mono'
        : file.audio_channels === 2
          ? 'Stereo'
          : `${file.audio_channels} ch`
      : null;

  return (
    <div className="flex flex-col min-w-0 h-screen overflow-hidden max-sm:h-full">
      <div
        className="px-4 py-[18px] border-b border-line shrink-0 sm:px-7"
        style={{ background: 'rgba(22,22,22,.82)', backdropFilter: 'blur(10px)' }}
      >
        <button
          onClick={() => router.back()}
          className="inline-flex items-center gap-1 text-[0.75rem] text-muted-dim hover:text-text transition-colors mb-3 cursor-pointer"
        >
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" className="w-3 h-3">
            <path d="M19 12H5M12 5l-7 7 7 7" />
          </svg>
          Back
        </button>

        <h1 className="text-[1.05rem] font-bold tracking-tight mb-0.5">{baseName(file.path)}</h1>
        <p className="text-[0.76rem] text-muted-dim font-mono truncate">{dirName(file.path)}</p>
      </div>

      <div className="flex-1 overflow-auto px-4 pt-5 pb-8 sm:px-7">
        <div className="max-w-[680px]">
          <div className="flex flex-wrap gap-2 mb-5">
            <Badge
              className={`text-[0.7rem] rounded-[7px] font-semibold ${
                file.library_type === 'tv'
                  ? 'text-sky border-[rgba(51,177,255,.32)] bg-[rgba(51,177,255,.1)]'
                  : 'text-violet border-[rgba(190,149,255,.3)] bg-[rgba(190,149,255,.1)]'
              }`}
            >
              {file.library_type === 'tv' ? 'TV' : 'Movie'}
            </Badge>
            {file.is_already_hevc && (
              <Badge className="text-[0.7rem] rounded-[7px] text-green border-green-soft bg-green-soft">HEVC</Badge>
            )}
            {file.status !== 'active' && (
              <Badge variant="secondary" className="text-[0.7rem] rounded-[7px]">{file.status}</Badge>
            )}
          </div>

          <div className="grid grid-cols-2 gap-3 mb-5">
            <div className="rounded-[11px] border border-line px-3 py-3" style={{ background: 'var(--surface-2)' }}>
              <div className="text-[0.7rem] uppercase tracking-wider text-muted-fg">Size</div>
              <div className="text-[1.1rem] font-bold font-mono mt-0.5">{formatBytes(file.size_bytes)}</div>
            </div>
            <div className="rounded-[11px] border border-line px-3 py-3" style={{ background: 'var(--surface-2)' }}>
              <div className="text-[0.7rem] uppercase tracking-wider text-muted-fg">Est. savings</div>
              <div className="text-[1.1rem] font-bold font-mono mt-0.5 text-brand">
                {formatBytes(file.predicted_savings_bytes)}
              </div>
            </div>
          </div>

          {notCandidateReason && (
            <div className="mb-5 rounded-[11px] border border-line px-3 py-3" style={{ background: 'var(--surface-2)' }}>
              <div className="text-[0.7rem] uppercase tracking-wider text-muted-fg">Why not a candidate?</div>
              <div className="text-[0.82rem] text-muted-fg mt-1 leading-relaxed">{notCandidateReason}</div>
            </div>
          )}

          <DetailSection title="Video">
            <DetailRow label="Codec" value={file.video_codec} mono />
            <DetailRow label="Profile" value={file.video_codec_profile} mono />
            <DetailRow label="Resolution" value={resolution} />
            <DetailRow
              label="Bitrate"
              value={file.bitrate_kbps != null ? `${file.bitrate_kbps.toLocaleString()} kbps` : null}
              mono
            />
            <DetailRow label="Duration" value={formatDuration(file.duration_seconds)} />
          </DetailSection>

          <DetailSection title="Audio">
            <DetailRow label="Codec" value={file.audio_codec} mono />
            <DetailRow label="Channels" value={audioChannels} />
          </DetailSection>

          <DetailSection title="Container">
            <DetailRow label="Format" value={file.container_format} mono />
          </DetailSection>

          <DetailSection title="File">
            <DetailRow label="Modified" value={relativeTime(file.mtime)} />
            <DetailRow label="Last probed" value={relativeTime(file.last_probed_at)} />
            {file.probe_error && (
              <div className="py-[9px] border-b border-line-soft last:border-b-0">
                <div className="text-[0.78rem] text-muted-fg mb-1">Probe error</div>
                <div className="text-[0.78rem] text-red font-mono break-all">{file.probe_error}</div>
              </div>
            )}
          </DetailSection>

          <div className="mt-4 rounded-[11px] border border-line px-3 py-3" style={{ background: 'var(--bg)' }}>
            <div className="text-[0.7rem] uppercase tracking-wider text-muted-dim mb-1">Path</div>
            <div className="text-[0.74rem] font-mono text-muted-fg break-all leading-relaxed">{file.path}</div>
          </div>
        </div>
      </div>
    </div>
  );
}

function FileSkeleton() {
  return (
    <div className="flex flex-col min-w-0">
      <div className="px-4 py-[18px] border-b border-line sm:px-7">
        <Skeleton className="h-3 w-16 mb-4" />
        <Skeleton className="h-5 w-72 mb-1.5" />
        <Skeleton className="h-3 w-44" />
      </div>
      <div className="px-4 pt-5 pb-8 sm:px-7">
        <div className="max-w-[680px]">
          <div className="flex gap-2 mb-5">
            <Skeleton className="h-5 w-14 rounded-[7px]" />
            <Skeleton className="h-5 w-20 rounded-[7px]" />
          </div>
          <div className="grid grid-cols-2 gap-3 mb-5">
            <Skeleton className="h-[68px] rounded-[11px]" />
            <Skeleton className="h-[68px] rounded-[11px]" />
          </div>
          <Skeleton className="h-[130px] w-full rounded-[11px] mb-5" />
          <Skeleton className="h-[88px] w-full rounded-[11px] mb-5" />
          <Skeleton className="h-[52px] w-full rounded-[11px] mb-5" />
          <Skeleton className="h-[88px] w-full rounded-[11px]" />
        </div>
      </div>
    </div>
  );
}

export default function Page() {
  return (
    <Suspense fallback={<FileSkeleton />}>
      <FilePageContent />
    </Suspense>
  );
}
