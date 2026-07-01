'use client';

import { Suspense, useState } from 'react';
import Image from 'next/image';
import { useRouter } from 'next/navigation';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import { api, tmdbImageURL, type MediaFile, type MetadataSearchResult } from '@/lib/api';
import {
  baseName,
  formatBytes,
  formatDuration,
  relativeTime,
  resolutionLabel,
} from '@/lib/format';
import { cn } from '@/lib/utils';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Skeleton } from '@/components/ui/skeleton';
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from '@/components/ui/dialog';
import { useQueryParams } from '@/hooks/use-query-params';
import { DetailRow, DetailSection } from '@/components/ui/detail';
import { StateBadge } from '@/components/media/candidate-state';
import { QueueConfirmDialog } from '@/components/media/queue-confirm-dialog';
import { BROWSE_ROUTES } from '@/app/(app)/browse/browse';

function goBack(router: ReturnType<typeof useRouter>) {
  if (window.history.length > 1) {
    router.back();
  } else {
    router.push(BROWSE_ROUTES.ROOT());
  }
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

function EditPosterDialog({
  open,
  onOpenChange,
  title,
  mediaType,
  currentPosterPath,
  currentBackdropPath,
  onSaved,
}: {
  open: boolean;
  onOpenChange: (v: boolean) => void;
  title: string;
  mediaType: 'tv' | 'movie';
  currentPosterPath?: string | null;
  currentBackdropPath?: string | null;
  onSaved: () => void;
}) {
  const [query, setQuery] = useState(title);
  const [results, setResults] = useState<MetadataSearchResult[]>([]);
  const [selected, setSelected] = useState<string | null>(null);
  const [searching, setSearching] = useState(false);
  const [saving, setSaving] = useState(false);
  const [customURL, setCustomURL] = useState('');
  const [useCustom, setUseCustom] = useState(false);

  async function handleSearch() {
    if (!query.trim()) return;
    setSearching(true);
    try {
      const res = await api.searchMetadata(query, mediaType);
      setResults(res.results);
      setSelected(null);
    } finally {
      setSearching(false);
    }
  }

  async function handleSave() {
    const posterURL = useCustom
      ? (customURL.trim() || null)
      : selected
        ? selected.replace('/w185/', '/w500/')
        : null;
    if (!posterURL) return;
    setSaving(true);
    try {
      await api.overrideMetadata(title, mediaType, posterURL, currentBackdropPath ?? null);
      onSaved();
      onOpenChange(false);
    } finally {
      setSaving(false);
    }
  }

  const currentPosterURL = tmdbImageURL(currentPosterPath, 'w185');

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-xl">
        <DialogHeader>
          <DialogTitle>Edit Poster</DialogTitle>
        </DialogHeader>

        <div className="space-y-4">
          {currentPosterURL && !useCustom && selected === null && (
            <div className="flex items-center gap-3 pb-3 border-b border-line">
              <Image src={currentPosterURL} alt="current poster" width={48} height={72} className="w-12 h-auto rounded-md shrink-0" />
              <span className="text-sm text-muted-fg">Current poster</span>
            </div>
          )}

          <div className="flex gap-2">
            <Input
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              onKeyDown={(e) => { if (e.key === 'Enter') void handleSearch(); }}
              placeholder="Search TMDB…"
              className="flex-1"
            />
            <Button variant="outline" onClick={() => void handleSearch()} disabled={searching}>
              {searching ? 'Searching…' : 'Search'}
            </Button>
          </div>

          {results.length > 0 && !useCustom && (
            <div className="grid grid-cols-4 gap-2 max-h-72 overflow-y-auto">
              {results.map((r) => (
                <button
                  key={r.tmdb_id}
                  onClick={() => setSelected(r.poster_url)}
                  className={cn(
                    'rounded-md overflow-hidden border-2 transition-colors cursor-pointer',
                    selected === r.poster_url ? 'border-brand' : 'border-transparent hover:border-line',
                  )}
                >
                  {r.poster_url ? (
                    <div className="relative w-full aspect-[2/3]">
                      <Image src={r.poster_url} alt={r.title} fill sizes="120px" className="object-cover" />
                    </div>
                  ) : (
                    <div className="w-full aspect-[2/3] bg-surface-3 flex items-center justify-center text-xs text-muted-dim p-1 text-center">{r.title}</div>
                  )}
                  <div className="text-xs text-muted-dim px-1 py-0.5 truncate">{r.title} {r.year ? `(${r.year})` : ''}</div>
                </button>
              ))}
            </div>
          )}

          <div>
            <button
              className="text-xs text-muted-fg hover:text-text underline cursor-pointer"
              onClick={() => { setUseCustom(!useCustom); setSelected(null); }}
            >
              {useCustom ? 'Search TMDB instead' : 'Use a custom URL instead'}
            </button>
          </div>

          {useCustom && (
            <Input
              value={customURL}
              onChange={(e) => setCustomURL(e.target.value)}
              placeholder="https://… (full poster image URL)"
            />
          )}
        </div>

        <DialogFooter>
          <Button variant="ghost" onClick={() => onOpenChange(false)}>Cancel</Button>
          <Button
            onClick={() => void handleSave()}
            disabled={saving || (!selected && !(useCustom && customURL.trim()))}
          >
            {saving ? 'Saving…' : 'Save'}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

function FilePageContent() {
  const router = useRouter();
  const queryClient = useQueryClient();
  const { get } = useQueryParams();
  const idRaw = get('id');
  const id = idRaw ? Number(idRaw) : null;

  const [refreshing, setRefreshing] = useState(false);
  const [editOpen, setEditOpen] = useState(false);
  const [queueOpen, setQueueOpen] = useState(false);

  const { data: file, isLoading, isError } = useQuery({
    queryKey: ['file', id],
    queryFn: () => api.file(id!),
    enabled: id !== null && !Number.isNaN(id),
  });

  const { data: profilesData } = useQuery({
    queryKey: ['profiles'],
    queryFn: api.profiles,
    staleTime: 60_000,
  });
  const profiles = profilesData?.items ?? [];

  const { data: queuedJobsData } = useQuery({
    queryKey: ['jobs', 'queued'],
    queryFn: () => api.jobs({ status: 'queued', limit: 200 }),
    enabled: file?.candidate_state === 'queued',
  });
  const activeJob = queuedJobsData?.items.find((j) => j.media_file_id === id);

  const queueMutation = useMutation({
    mutationFn: (profileId: number | null) => api.createJobs([id!], profileId ?? undefined),
    onSuccess: () => {
      toast.success('Job queued');
      void queryClient.invalidateQueries({ queryKey: ['jobs'] });
      void queryClient.invalidateQueries({ queryKey: ['candidates'] });
      void queryClient.invalidateQueries({ queryKey: ['file', id] });
    },
    onError: () => toast.error('Failed to queue job'),
  });

  const forceMutation = useMutation({
    mutationFn: (jobId: number) => api.forceJob(jobId),
    onSuccess: () => {
      toast.success('Job will run immediately');
      void queryClient.invalidateQueries({ queryKey: ['jobs'] });
    },
    onError: () => toast.error('Force failed'),
  });

  if (!id || Number.isNaN(id)) {
    return (
      <div className="flex flex-col items-center justify-center h-full py-32 text-center">
        <div className="text-sm font-semibold text-text">No file selected</div>
        <button onClick={() => goBack(router)} className="text-sm text-brand hover:underline mt-3 cursor-pointer">
          ← Back
        </button>
      </div>
    );
  }

  if (isLoading) return <FileSkeleton />;

  if (isError || !file) {
    return (
      <div className="flex flex-col items-center justify-center h-full py-32 text-center">
        <div className="text-sm font-semibold text-text">File not found</div>
        <div className="text-xs text-muted-dim mt-2 mb-5">This file could not be found in the library.</div>
        <button onClick={() => goBack(router)} className="text-sm text-brand hover:underline cursor-pointer">
          ← Back
        </button>
      </div>
    );
  }

  const displayTitle = baseName(file.path).replace(/\.[^/.]+$/, '');
  const posterURL = tmdbImageURL(file.poster_path, 'w342');
  const backdropURL = tmdbImageURL(file.backdrop_path, 'w1280');
  const genres = file.genres?.length ? file.genres : null;
  const isTV = file.library_type === 'tv';
  const mediaType: 'tv' | 'movie' = isTV ? 'tv' : 'movie';
  const metadataKey = isTV ? (file.path.split('/').at(-3) ?? displayTitle) : displayTitle;
  const notCandidateReason = candidateStateReason(file);

  const resolution =
    file.width && file.height
      ? `${file.width}×${file.height} (${resolutionLabel(file.width, file.height)})`
      : resolutionLabel(file.width, file.height);

  const audioChannels =
    file.audio_channels != null
      ? file.audio_channels === 1
        ? 'Mono'
        : file.audio_channels === 2
          ? 'Stereo'
          : `${file.audio_channels} ch`
      : null;

  async function handleRefresh() {
    setRefreshing(true);
    try {
      await api.refreshMetadata(metadataKey, mediaType);
      await queryClient.invalidateQueries({ queryKey: ['file', id] });
    } finally {
      setRefreshing(false);
    }
  }

  return (
    <div className="flex flex-col min-w-0 h-screen overflow-hidden max-sm:h-full">
      <div className="relative px-4 py-4 border-b border-line shrink-0 sm:px-7 overflow-hidden">
        {backdropURL && (
          <div
            className="absolute inset-0"
            style={{
              backgroundImage: `url(${backdropURL})`,
              backgroundSize: 'cover',
              backgroundPosition: 'center 25%',
              filter: 'blur(3px) brightness(0.28)',
              transform: 'scale(1.06)',
            }}
          />
        )}
        <div
          className="absolute inset-0"
          style={{ background: backdropURL ? 'rgba(10,10,10,.55)' : 'rgba(22,22,22,.82)', backdropFilter: 'blur(10px)' }}
        />

        <div className="relative">
          <div className="flex items-center justify-between mb-3">
            <button
              onClick={() => goBack(router)}
              className="inline-flex items-center gap-1 text-xs text-muted-dim hover:text-text transition-colors cursor-pointer"
            >
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" className="w-3 h-3">
                <path d="M19 12H5M12 5l-7 7 7 7" />
              </svg>
              Back
            </button>

            <div className="flex items-center gap-2">
              {file.candidate_state === 'candidate' && (
                <Button
                  size="sm"
                  onClick={() => setQueueOpen(true)}
                  className="h-7 text-xs gap-1.5"
                  style={{ background: 'linear-gradient(145deg, var(--brand), var(--brand-2))', boxShadow: '0 4px 14px var(--brand-soft)' }}
                >
                  Queue
                </Button>
              )}
              {file.candidate_state === 'queued' && activeJob && !activeJob.forced && (
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => forceMutation.mutate(activeJob.id)}
                  disabled={forceMutation.isPending}
                  className="h-7 text-xs"
                  title="Run now, bypassing the encode window"
                >
                  {forceMutation.isPending ? 'Forcing…' : 'Run now'}
                </Button>
              )}
              <Button
                variant="ghost"
                size="sm"
                onClick={() => void handleRefresh()}
                disabled={refreshing}
                className="h-7 text-xs text-muted-fg hover:text-text gap-1.5"
              >
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" className={cn('w-3.5 h-3.5', refreshing && 'animate-spin')}>
                  <path d="M1 4v6h6M23 20v-6h-6"/>
                  <path d="M20.49 9A9 9 0 0 0 5.64 5.64L1 10m22 4l-4.64 4.36A9 9 0 0 1 3.51 15"/>
                </svg>
                {refreshing ? 'Refreshing…' : 'Refresh'}
              </Button>
              <Button
                variant="ghost"
                size="sm"
                onClick={() => setEditOpen(true)}
                className="h-7 text-xs text-muted-fg hover:text-text gap-1.5"
              >
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" className="w-3.5 h-3.5">
                  <path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7"/>
                  <path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z"/>
                </svg>
                Edit Poster
              </Button>
            </div>
          </div>

          <div className="flex items-start gap-4">
            {posterURL && (
              <Image
                src={posterURL}
                alt={displayTitle}
                width={64}
                height={96}
                className="w-16 h-auto rounded-lg shrink-0 shadow-lg hidden sm:block"
              />
            )}

            <div className="min-w-0 flex-1">
              <div className="flex items-center gap-2 mb-1 flex-wrap">
                <Badge
                  className={`text-xs rounded-md font-semibold ${
                    isTV
                      ? 'text-sky border-[rgba(51,177,255,.32)] bg-[rgba(51,177,255,.1)]'
                      : 'text-violet border-[rgba(190,149,255,.3)] bg-[rgba(190,149,255,.1)]'
                  }`}
                >
                  {isTV ? 'TV' : 'Movie'}
                </Badge>
                {file.candidate_state !== 'candidate' && <StateBadge state={file.candidate_state} />}
                {file.release_year != null && (
                  <span className="text-xs text-muted-dim">{file.release_year}</span>
                )}
                {file.vote_average != null && file.vote_average > 0 && (
                  <span className="text-xs text-muted-dim">★ {file.vote_average.toFixed(1)}</span>
                )}
                {file.runtime_mins != null && (
                  <span className="text-xs text-muted-dim">{file.runtime_mins} min</span>
                )}
              </div>

              <h1 className="text-2xl font-bold tracking-tight leading-tight mb-1">{displayTitle}</h1>

              {file.tagline && (
                <p className="text-sm text-muted-fg italic mb-1">{file.tagline}</p>
              )}

              {genres && (
                <div className="flex items-center gap-1 flex-wrap mb-1">
                  {genres.map((g, i) => (
                    <span key={g} className="text-xs text-muted-dim">
                      {g}{i < genres.length - 1 ? ' ·' : ''}
                    </span>
                  ))}
                </div>
              )}

              {file.overview && (
                <p className="text-xs text-muted-fg leading-relaxed line-clamp-2 mb-2">{file.overview}</p>
              )}

              <div className="flex items-center gap-1.5 flex-wrap text-xs text-muted-fg">
                <span className="font-mono">{formatBytes(file.size_bytes)}</span>
                {file.candidate_state === 'candidate' && file.predicted_savings_bytes > 0 && (
                  <>
                    <span className="text-muted-dim">·</span>
                    <span className="text-brand font-semibold">-{formatBytes(file.predicted_savings_bytes)} recoverable</span>
                  </>
                )}
              </div>
            </div>
          </div>
        </div>
      </div>

      <div className="flex-1 overflow-auto px-4 pt-5 pb-8 sm:px-7">
        <div>
          {notCandidateReason && (
            <div className="mb-5 rounded-xl border border-line px-3 py-3" style={{ background: 'var(--surface-2)' }}>
              <div className="text-xs uppercase tracking-wider text-muted-fg">Why not a candidate?</div>
              <div className="text-sm text-muted-fg mt-1 leading-relaxed">{notCandidateReason}</div>
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
              <div className="py-2 border-b border-line-soft last:border-b-0">
                <div className="text-xs text-muted-fg mb-1">Probe error</div>
                <div className="text-xs text-red font-mono break-all">{file.probe_error}</div>
              </div>
            )}
          </DetailSection>

          <div className="mt-4 rounded-xl border border-line px-3 py-3" style={{ background: 'var(--bg)' }}>
            <div className="text-xs uppercase tracking-wider text-muted-dim mb-1">Path</div>
            <div className="text-xs font-mono text-muted-fg break-all leading-relaxed">{file.path}</div>
          </div>
        </div>
      </div>

      <EditPosterDialog
        open={editOpen}
        onOpenChange={setEditOpen}
        title={metadataKey}
        mediaType={mediaType}
        currentPosterPath={file.poster_path}
        currentBackdropPath={file.backdrop_path}
        onSaved={() => void queryClient.invalidateQueries({ queryKey: ['file', id] })}
      />

      <QueueConfirmDialog
        open={queueOpen}
        onClose={() => setQueueOpen(false)}
        selectedFiles={[file]}
        profiles={profiles}
        subtitle="Review the file. Nothing runs until you confirm."
        showSafetyNote
        onConfirm={async (profileId) => {
          await queueMutation.mutateAsync(profileId);
        }}
      />
    </div>
  );
}

function FileSkeleton() {
  return (
    <div className="flex flex-col min-w-0">
      <div className="px-4 py-4 border-b border-line sm:px-7">
        <Skeleton className="h-3 w-16 mb-4" />
        <Skeleton className="h-8 w-64 mb-3" />
        <Skeleton className="h-3 w-48 mb-5" />
      </div>
      <div className="px-4 pt-5 pb-8 sm:px-7">
        <div className="max-w-2xl">
          <Skeleton className="h-20 w-full rounded-xl mb-5" />
          <Skeleton className="h-14 w-full rounded-xl mb-5" />
          <Skeleton className="h-20 w-full rounded-xl" />
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
