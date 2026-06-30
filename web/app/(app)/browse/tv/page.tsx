'use client';

import { Suspense, useMemo, useState } from 'react';
import Image from 'next/image';
import Link from 'next/link';
import { useRouter } from 'next/navigation';
import { useInfiniteQuery, useQuery, useQueryClient } from '@tanstack/react-query';
import { api, tmdbImageURL, type Episode, type LibrarySeasonGroup, type MetadataSearchResult } from '@/lib/api';
import { baseName, formatBytes, formatInt, resolutionLabel } from '@/lib/format';
import { cn } from '@/lib/utils';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Skeleton } from '@/components/ui/skeleton';
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from '@/components/ui/dialog';
import { useQueryParams } from '@/hooks/use-query-params';
import { CodecBadge } from '@/components/media/codec-badge';
import { EncodeHealthBar } from '@/components/media/encode-health-bar';
import { StateBadge } from '@/components/media/candidate-state';
import { BROWSE_ROUTES, EPISODES_PER_PAGE, LIBRARY_TYPE } from '../browse';

function EpisodeRow({ ep, onClick }: { ep: Episode; onClick: () => void }) {
  const dimmed = ep.candidate_state === 'already_hevc' || ep.candidate_state === 'completed';
  return (
    <div
      onClick={onClick}
      className={cn(
        'grid items-center gap-3 px-4 py-2.5 border-b border-line-soft last:border-b-0 text-sm',
        'grid-cols-[1fr_auto_auto_auto_auto]',
        'cursor-pointer hover:bg-surface-2 transition-colors',
        dimmed && 'opacity-60',
      )}
    >
      <div className="min-w-0 truncate font-medium">{baseName(ep.path)}</div>
      <CodecBadge codec={ep.video_codec} />
      <span className="text-muted-dim hidden sm:inline">
        {ep.width && ep.height ? resolutionLabel(ep.width, ep.height) : '—'}
      </span>
      <span className="text-muted-fg font-mono hidden md:inline">{formatBytes(ep.size_bytes)}</span>
      <div className="text-right w-20">
        {ep.candidate_state === 'candidate' && ep.predicted_savings_bytes > 0
          ? <span className="text-brand font-semibold font-mono">-{formatBytes(ep.predicted_savings_bytes)}</span>
          : <StateBadge state={ep.candidate_state} />
        }
      </div>
    </div>
  );
}

function SeasonSection({ seriesTitle, seasonData }: {
  seriesTitle: string;
  seasonData: LibrarySeasonGroup;
}) {
  const router = useRouter();
  const { data, fetchNextPage, hasNextPage, isFetchingNextPage, isLoading } = useInfiniteQuery({
    queryKey: ['browse', 'episodes', seriesTitle, seasonData.season],
    queryFn: ({ pageParam }: { pageParam: number }) =>
      api.groupedFileEpisodes({
        series: seriesTitle,
        season: seasonData.season,
        limit: EPISODES_PER_PAGE,
        offset: pageParam,
      }),
    initialPageParam: 0,
    getNextPageParam: (_lastPage, allPages) => {
      const loaded = allPages.flatMap((p) => p.episodes).length;
      return loaded < seasonData.file_count ? loaded : undefined;
    },
  });

  const episodes = useMemo(() => data?.pages.flatMap((p) => p.episodes) ?? [], [data]);

  return (
    <section className="mb-5">
      <div className="flex items-center gap-3 px-4 py-3 bg-surface-2 border border-line rounded-t-xl border-b-0">
        <h2 className="font-bold text-sm flex-1">Season {seasonData.season}</h2>
        <span className="text-xs text-muted-dim">{formatInt(seasonData.file_count)} files</span>
        <span className="text-muted-dim">·</span>
        <span className="font-mono text-xs text-muted-fg">{formatBytes(seasonData.total_bytes)}</span>
        {seasonData.predicted_savings_bytes > 0 && (
          <>
            <span className="text-muted-dim">·</span>
            <span className="text-xs font-semibold text-brand font-mono">
              -{formatBytes(seasonData.predicted_savings_bytes)}
            </span>
          </>
        )}
      </div>

      <div className="bg-surface border border-line rounded-b-xl overflow-hidden">
        <div className="flex items-center gap-3 px-4 py-2 border-b border-line text-xs uppercase tracking-wider text-muted-dim font-bold">
          <span className="flex-1">File</span>
          <span>Codec</span>
          <span className="hidden sm:inline w-12 text-right">Res</span>
          <span className="hidden md:inline w-16 text-right">Size</span>
          <span className="w-20 text-right">Savings</span>
        </div>

        {isLoading ? (
          <div className="px-4 py-3 space-y-2">
            {Array.from({ length: 4 }).map((_, i) => <Skeleton key={i} className="h-4 w-full" />)}
          </div>
        ) : (
          <>
            {episodes.map((ep) => <EpisodeRow key={ep.id} ep={ep} onClick={() => router.push(BROWSE_ROUTES.FILE(ep.id))} />)}
            {hasNextPage && (
              <div className="px-4 py-3 border-t border-line-soft">
                <Button variant="ghost" size="sm" disabled={isFetchingNextPage} onClick={() => void fetchNextPage()} className="text-xs text-muted-fg">
                  {isFetchingNextPage ? 'Loading…' : 'Load more episodes'}
                </Button>
              </div>
            )}
          </>
        )}
      </div>
    </section>
  );
}

function EditPosterDialog({
  open,
  onOpenChange,
  showTitle,
  currentPosterPath,
  currentBackdropPath,
  onSaved,
}: {
  open: boolean;
  onOpenChange: (v: boolean) => void;
  showTitle: string;
  currentPosterPath?: string | null;
  currentBackdropPath?: string | null;
  onSaved: () => void;
}) {
  const [query, setQuery] = useState(showTitle);
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
      const res = await api.searchMetadata(query, 'tv');
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
      await api.overrideMetadata(showTitle, 'tv', posterURL, currentBackdropPath ?? null);
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

function TVShowPageContent() {
  const { get } = useQueryParams();
  const title = get('show') ?? '';
  const view = get('view') ?? undefined;
  const queryClient = useQueryClient();

  const [editOpen, setEditOpen] = useState(false);
  const [refreshing, setRefreshing] = useState(false);

  const { data: showData, isLoading: showLoading } = useQuery({
    queryKey: ['browse', 'show', title],
    queryFn: async () => {
      if (!title) return null;
      const result = await api.groupedFiles({ library_type: LIBRARY_TYPE.TV, search: title, limit: 10 });
      return result.series.find((s) => s.title === title) ?? null;
    },
    enabled: Boolean(title),
  });

  const { data: metadata } = useQuery({
    queryKey: ['metadata', title],
    queryFn: () => api.getMetadata(title),
    enabled: Boolean(title),
  });

  const { data: seasonsData, isLoading: seasonsLoading } = useQuery({
    queryKey: ['browse', 'seasons', title],
    queryFn: () => api.groupedFileSeasons(title),
    enabled: Boolean(title) && Boolean(showData),
  });

  const isLoading = showLoading || (Boolean(showData) && seasonsLoading);

  const posterPath = showData?.poster_path ?? metadata?.poster_path;
  const backdropPath = showData?.backdrop_path ?? metadata?.backdrop_path;
  const posterURL = tmdbImageURL(posterPath, 'w342');
  const backdropURL = tmdbImageURL(backdropPath, 'w1280');

  async function handleRefresh() {
    setRefreshing(true);
    try {
      await api.refreshMetadata(title, 'tv');
      await queryClient.invalidateQueries({ queryKey: ['browse', 'show', title] });
      await queryClient.invalidateQueries({ queryKey: ['metadata', title] });
    } finally {
      setRefreshing(false);
    }
  }

  function handleSaved() {
    void queryClient.invalidateQueries({ queryKey: ['browse', 'show', title] });
    void queryClient.invalidateQueries({ queryKey: ['metadata', title] });
  }

  if (!title) {
    return (
      <div className="flex flex-col items-center justify-center h-full py-32 text-center">
        <div className="text-sm font-semibold text-text">No show selected</div>
        <Link href={BROWSE_ROUTES.ROOT(view)} className="text-sm text-brand hover:underline mt-3 cursor-pointer">← Back to Browse</Link>
      </div>
    );
  }

  if (isLoading) {
    return (
      <div className="flex flex-col min-w-0">
        <div className="px-4 py-4 border-b border-line sm:px-7">
          <Skeleton className="h-3 w-16 mb-4" />
          <Skeleton className="h-8 w-64 mb-3" />
          <Skeleton className="h-3 w-48 mb-5" />
          <Skeleton className="h-1 w-full rounded-full" />
        </div>
        <div className="px-4 pt-5 sm:px-7 space-y-4">
          {Array.from({ length: 2 }).map((_, i) => (
            <div key={i} className="bg-surface border border-line rounded-xl overflow-hidden">
              <div className="px-4 py-3 bg-surface-2 border-b border-line"><Skeleton className="h-4 w-24" /></div>
              <div className="px-4 py-3 space-y-3">{Array.from({ length: 4 }).map((_, j) => <Skeleton key={j} className="h-4 w-full" />)}</div>
            </div>
          ))}
        </div>
      </div>
    );
  }

  if (!showData) {
    return (
      <div className="flex flex-col items-center justify-center h-full py-32 text-center">
        <div className="text-sm font-semibold text-text">Show not found</div>
        <div className="text-xs text-muted-dim mt-2 mb-5">&quot;{title}&quot; could not be found in the library.</div>
        <Link href={BROWSE_ROUTES.ROOT(view)} className="text-sm text-brand hover:underline cursor-pointer">← Back to Browse</Link>
      </div>
    );
  }

  const donePct = showData.file_count > 0
    ? Math.round(Math.max(0, showData.file_count - showData.eligible_count) / showData.file_count * 100)
    : 100;

  const genres = metadata?.genres?.length ? metadata.genres : null;

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
            <Link
              href={BROWSE_ROUTES.ROOT(view)}
              className="inline-flex items-center gap-1 text-xs text-muted-dim hover:text-text transition-colors cursor-pointer"
            >
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" className="w-3 h-3">
                <path d="M19 12H5M12 5l-7 7 7 7"/>
              </svg>
              Browse
            </Link>

            <div className="flex items-center gap-2">
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
                alt={showData.title}
                width={64}
                height={96}
                className="w-16 h-auto rounded-lg shrink-0 shadow-lg hidden sm:block"
              />
            )}

            <div className="min-w-0 flex-1">
              <div className="flex items-center gap-2 mb-1 flex-wrap">
                <Badge className="text-xs rounded-md text-sky border-[rgba(51,177,255,.32)] bg-[rgba(51,177,255,.1)]">TV</Badge>
                {metadata?.release_year && (
                  <span className="text-xs text-muted-dim">{metadata.release_year}</span>
                )}
                {metadata?.vote_average && metadata.vote_average > 0 && (
                  <span className="text-xs text-muted-dim">★ {metadata.vote_average.toFixed(1)}</span>
                )}
                {metadata?.network && (
                  <span className="text-xs text-muted-dim">{metadata.network}</span>
                )}
              </div>

              <h1 className="text-2xl font-bold tracking-tight leading-tight mb-1">{showData.title}</h1>

              {metadata?.tagline && (
                <p className="text-sm text-muted-fg italic mb-1">{metadata.tagline}</p>
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

              {metadata?.overview && (
                <p className="text-xs text-muted-fg leading-relaxed line-clamp-2 mb-2">{metadata.overview}</p>
              )}

              <div className="flex items-center gap-1.5 flex-wrap text-xs text-muted-fg">
                <span>{showData.season_count} {showData.season_count === 1 ? 'season' : 'seasons'}</span>
                <span className="text-muted-dim">·</span>
                <span>{formatInt(showData.file_count)} episodes</span>
                <span className="text-muted-dim">·</span>
                <span className="font-mono">{formatBytes(showData.total_bytes)}</span>
                {showData.predicted_savings_bytes > 0 && (
                  <>
                    <span className="text-muted-dim">·</span>
                    <span className="text-brand font-semibold">-{formatBytes(showData.predicted_savings_bytes)} recoverable</span>
                  </>
                )}
              </div>
            </div>
          </div>

          <div className="mt-4">
            <EncodeHealthBar fileCount={showData.file_count} eligibleCount={showData.eligible_count} />
            <div className="flex justify-between text-xs text-muted-dim mt-1">
              <span>{formatInt(showData.file_count - showData.eligible_count)} converted · {donePct}%</span>
              <span>{formatInt(showData.eligible_count)} remaining</span>
            </div>
          </div>
        </div>
      </div>

      <div className="flex-1 overflow-auto px-4 pt-5 pb-8 sm:px-7">
        {(seasonsData?.seasons ?? []).map((s) => (
          <SeasonSection key={s.season} seriesTitle={showData.title} seasonData={s} />
        ))}
      </div>

      <EditPosterDialog
        open={editOpen}
        onOpenChange={setEditOpen}
        showTitle={title}
        currentPosterPath={posterPath}
        currentBackdropPath={backdropPath}
        onSaved={handleSaved}
      />
    </div>
  );
}

function PageSkeleton() {
  return (
    <div className="flex flex-col min-w-0">
      <div className="px-4 py-4 border-b border-line sm:px-7">
        <Skeleton className="h-3 w-16 mb-4" />
        <Skeleton className="h-8 w-56 mb-3" />
        <Skeleton className="h-3 w-44 mb-5" />
        <Skeleton className="h-1 w-full rounded-full" />
      </div>
    </div>
  );
}

export default function Page() {
  return (
    <Suspense fallback={<PageSkeleton />}>
      <TVShowPageContent />
    </Suspense>
  );
}
