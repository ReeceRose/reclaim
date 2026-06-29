'use client';

import { Suspense, useMemo } from 'react';
import Link from 'next/link';
import { useRouter } from 'next/navigation';
import { useInfiniteQuery, useQuery } from '@tanstack/react-query';
import { api, type Episode, type LibrarySeasonGroup } from '@/lib/api';
import { baseName, formatBytes, formatInt, resolutionLabel } from '@/lib/format';
import { cn } from '@/lib/utils';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { useQueryParams } from '@/hooks/use-query-params';
import { CodecBadge, EncodeHealthBar } from '../page';
import { BROWSE_ROUTES, EPISODES_PER_PAGE, LIBRARY_TYPE } from '../browse';

const STATE_LABELS: Record<string, string> = {
  candidate:     'Candidate',
  already_hevc:  'HEVC',
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
    <Badge className={`text-[0.68rem] rounded-[6px] font-semibold ${cls}`}>
      {STATE_LABELS[state] ?? state}
    </Badge>
  );
}

function EpisodeRow({ ep, onClick }: { ep: Episode; onClick: () => void }) {
  const dimmed = ep.candidate_state === 'already_hevc' || ep.candidate_state === 'completed';
  return (
    <div
      onClick={onClick}
      className={cn(
        'grid items-center gap-3 px-4 py-[10px] border-b border-line-soft last:border-b-0 text-[0.82rem]',
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
      <div className="text-right w-[80px]">
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
      <div className="flex items-center gap-3 px-4 py-3 bg-surface-2 border border-line rounded-t-[12px] border-b-0">
        <h2 className="font-bold text-[0.95rem] flex-1">Season {seasonData.season}</h2>
        <span className="text-[0.75rem] text-muted-dim">{formatInt(seasonData.file_count)} files</span>
        <span className="text-muted-dim">·</span>
        <span className="font-mono text-[0.75rem] text-muted-fg">{formatBytes(seasonData.total_bytes)}</span>
        {seasonData.predicted_savings_bytes > 0 && (
          <>
            <span className="text-muted-dim">·</span>
            <span className="text-[0.78rem] font-semibold text-brand font-mono">
              -{formatBytes(seasonData.predicted_savings_bytes)}
            </span>
          </>
        )}
      </div>

      <div className="bg-surface border border-line rounded-b-[12px] overflow-hidden">
        <div className="flex items-center gap-3 px-4 py-[7px] border-b border-line text-[0.68rem] uppercase tracking-wider text-muted-dim font-bold">
          <span className="flex-1">File</span>
          <span>Codec</span>
          <span className="hidden sm:inline w-[48px] text-right">Res</span>
          <span className="hidden md:inline w-[64px] text-right">Size</span>
          <span className="w-[80px] text-right">Savings</span>
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

function TVShowPageContent() {
  const { get } = useQueryParams();
  const title = get('show') ?? '';

  const { data: showData, isLoading: showLoading } = useQuery({
    queryKey: ['browse', 'show', title],
    queryFn: async () => {
      if (!title) return null;
      const result = await api.groupedFiles({ library_type: LIBRARY_TYPE.TV, search: title, limit: 10 });
      return result.series.find((s) => s.title === title) ?? null;
    },
    enabled: Boolean(title),
  });

  const { data: seasonsData, isLoading: seasonsLoading } = useQuery({
    queryKey: ['browse', 'seasons', title],
    queryFn: () => api.groupedFileSeasons(title),
    enabled: Boolean(title) && Boolean(showData),
  });

  const isLoading = showLoading || (Boolean(showData) && seasonsLoading);

  if (!title) {
    return (
      <div className="flex flex-col items-center justify-center h-full py-32 text-center">
        <div className="text-[0.9rem] font-semibold text-text">No show selected</div>
        <Link href={BROWSE_ROUTES.ROOT} className="text-[0.82rem] text-brand hover:underline mt-3 cursor-pointer">← Back to Browse</Link>
      </div>
    );
  }

  if (isLoading) {
    return (
      <div className="flex flex-col min-w-0">
        <div className="px-4 py-[18px] border-b border-line sm:px-7">
          <Skeleton className="h-3 w-16 mb-4" />
          <Skeleton className="h-8 w-64 mb-3" />
          <Skeleton className="h-3 w-48 mb-5" />
          <Skeleton className="h-[4px] w-full rounded-full" />
        </div>
        <div className="px-4 pt-5 sm:px-7 space-y-4">
          {Array.from({ length: 2 }).map((_, i) => (
            <div key={i} className="bg-surface border border-line rounded-[12px] overflow-hidden">
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
        <div className="text-[0.9rem] font-semibold text-text">Show not found</div>
        <div className="text-[0.8rem] text-muted-dim mt-2 mb-5">&quot;{title}&quot; could not be found in the library.</div>
        <Link href={BROWSE_ROUTES.ROOT} className="text-[0.82rem] text-brand hover:underline cursor-pointer">← Back to Browse</Link>
      </div>
    );
  }

  const donePct = showData.file_count > 0
    ? Math.round(Math.max(0, showData.file_count - showData.eligible_count) / showData.file_count * 100)
    : 100;

  return (
    <div className="flex flex-col min-w-0 h-screen overflow-hidden max-sm:h-full">
      <div
        className="px-4 py-[18px] border-b border-line shrink-0 sm:px-7"
        style={{ background: 'rgba(22,22,22,.82)', backdropFilter: 'blur(10px)' }}
      >
        <Link
          href={BROWSE_ROUTES.ROOT}
          className="inline-flex items-center gap-1 text-[0.75rem] text-muted-dim hover:text-text transition-colors mb-3 cursor-pointer"
        >
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" className="w-3 h-3">
            <path d="M19 12H5M12 5l-7 7 7 7"/>
          </svg>
          Browse
        </Link>

        <div className="flex items-start gap-4">
          <div className="min-w-0 flex-1">
            <div className="flex items-center gap-2 mb-1">
              <Badge className="text-[0.68rem] rounded-[6px] text-sky border-[rgba(51,177,255,.32)] bg-[rgba(51,177,255,.1)]">TV</Badge>
            </div>
            <h1 className="text-[1.5rem] font-bold tracking-tight leading-tight mb-2">{showData.title}</h1>
            <div className="flex items-center gap-[6px] flex-wrap text-[0.78rem] text-muted-fg">
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
          <EncodeHealthBar fileCount={showData.file_count} eligibleCount={showData.eligible_count} height={4} />
          <div className="flex justify-between text-[0.68rem] text-muted-dim mt-[5px]">
            <span>{formatInt(showData.file_count - showData.eligible_count)} converted · {donePct}%</span>
            <span>{formatInt(showData.eligible_count)} remaining</span>
          </div>
        </div>
      </div>

      <div className="flex-1 overflow-auto px-4 pt-5 pb-8 sm:px-7">
        {(seasonsData?.seasons ?? []).map((s) => (
          <SeasonSection key={s.season} seriesTitle={showData.title} seasonData={s} />
        ))}
      </div>
    </div>
  );
}

function PageSkeleton() {
  return (
    <div className="flex flex-col min-w-0">
      <div className="px-4 py-[18px] border-b border-line sm:px-7">
        <Skeleton className="h-3 w-16 mb-4" />
        <Skeleton className="h-8 w-56 mb-3" />
        <Skeleton className="h-3 w-44 mb-5" />
        <Skeleton className="h-[4px] w-full rounded-full" />
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
