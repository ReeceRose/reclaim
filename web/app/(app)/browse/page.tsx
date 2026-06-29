'use client';

import { Suspense, useEffect, useMemo, useState } from 'react';
import { useRouter } from 'next/navigation';
import { useInfiniteQuery } from '@tanstack/react-query';
import { api, type LibrarySeriesGroup, type MediaFile } from '@/lib/api';
import { baseName, formatBytes, formatInt, resolutionLabel } from '@/lib/format';
import { cn } from '@/lib/utils';
import { parseQueryEnum, useQueryParam } from '@/hooks/use-query-params';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Skeleton } from '@/components/ui/skeleton';
import {
  BROWSE_ROUTES,
  CODEC_BORDER,
  CODEC_COLORS,
  LIBRARY_TYPE,
  MOVIE_SORT,
  PAGE_SIZE,
  QUERY_PARAMS,
  TV_SORT,
} from './browse';

const TV_SORT_OPTIONS = [
  { value: TV_SORT.ALPHA,   label: 'A – Z' },
  { value: TV_SORT.SAVINGS, label: 'Most savings' },
  { value: TV_SORT.SIZE,    label: 'Largest' },
  { value: TV_SORT.FILES,   label: 'Most episodes' },
] as const;

const MOVIE_SORT_OPTIONS = [
  { value: MOVIE_SORT.ALPHA,   label: 'A – Z' },
  { value: MOVIE_SORT.SIZE,    label: 'Largest' },
  { value: MOVIE_SORT.RECENT,  label: 'Recently added' },
] as const;

type TVSortValue = typeof TV_SORT_OPTIONS[number]['value'];
type MovieSortValue = typeof MOVIE_SORT_OPTIONS[number]['value'];

function sortShows(shows: LibrarySeriesGroup[], sort: TVSortValue): LibrarySeriesGroup[] {
  const s = [...shows];
  if (sort === TV_SORT.SAVINGS) return s.sort((a, b) => b.predicted_savings_bytes - a.predicted_savings_bytes);
  if (sort === TV_SORT.SIZE)    return s.sort((a, b) => b.total_bytes - a.total_bytes);
  if (sort === TV_SORT.FILES)   return s.sort((a, b) => b.file_count - a.file_count);
  return s;
}

// ── Shared UI ─────────────────────────────────────────────────────────────────

export function CodecBadge({ codec }: { codec: string | null }) {
  if (!codec) return <Badge variant="outline" className="font-mono text-[0.7rem] rounded-[7px]">unknown</Badge>;
  const c = codec.toLowerCase();
  return (
    <Badge className={`font-mono text-[0.7rem] rounded-[7px] font-semibold ${CODEC_COLORS[c] ?? 'text-slate'} ${CODEC_BORDER[c] ?? 'border-line bg-surface-3'}`}>
      {codec}
    </Badge>
  );
}

export function EncodeHealthBar({ fileCount, eligibleCount, height = 3 }: {
  fileCount: number;
  eligibleCount: number;
  height?: number;
}) {
  const donePct = fileCount > 0 ? Math.round(Math.max(0, fileCount - eligibleCount) / fileCount * 100) : 100;
  return (
    <div className="w-full overflow-hidden rounded-full" style={{ height, background: 'var(--surface-3)' }}>
      <div
        className="h-full transition-[width] duration-500"
        style={{
          width: `${donePct}%`,
          background: donePct === 100 ? 'var(--green)' : 'linear-gradient(90deg, var(--green), var(--brand))',
        }}
      />
    </div>
  );
}

// ── Cards ─────────────────────────────────────────────────────────────────────

function ShowCard({ show, onClick }: { show: LibrarySeriesGroup; onClick: () => void }) {
  const letter = show.title.replace(/^(the |a |an )/i, '').charAt(0).toUpperCase();
  const fullyConverted = show.eligible_count === 0;

  return (
    <div
      onClick={onClick}
      className="relative bg-surface border border-line rounded-[14px] overflow-hidden cursor-pointer hover:border-[var(--brand-line)] transition-[border-color] group"
    >
      <div
        className="absolute right-1 bottom-0 font-black select-none pointer-events-none leading-none opacity-[0.045]"
        style={{ fontSize: '5.5rem' }}
        aria-hidden
      >
        {letter}
      </div>

      <div className="relative px-[14px] pt-[14px] pb-[13px] flex flex-col gap-[6px] min-h-[130px]">
        <div className="font-bold text-[0.92rem] leading-snug line-clamp-2 pr-5">{show.title}</div>
        <div className="text-[0.72rem] text-muted-dim">
          {show.season_count} {show.season_count === 1 ? 'season' : 'seasons'} · {formatInt(show.file_count)} files
        </div>
        <div className="text-[0.72rem] text-muted-fg font-mono">{formatBytes(show.total_bytes)}</div>
        <div className="mt-auto pt-1">
          {fullyConverted
            ? <span className="text-[0.72rem] font-medium text-green">All converted</span>
            : <span className="text-[0.78rem] font-semibold text-brand">-{formatBytes(show.predicted_savings_bytes)} recoverable</span>
          }
        </div>
      </div>

      <EncodeHealthBar fileCount={show.file_count} eligibleCount={show.eligible_count} />
    </div>
  );
}

function MovieCard({ file, onClick }: { file: MediaFile; onClick: () => void }) {
  const title = baseName(file.path).replace(/\.[^/.]+$/, '');
  const isConverted = file.candidate_state === 'already_hevc' || file.candidate_state === 'completed';
  const isCandidate = file.candidate_state === 'candidate';

  return (
    <div
      onClick={onClick}
      className="relative bg-surface border border-line rounded-[14px] overflow-hidden cursor-pointer hover:border-[var(--brand-line)] transition-[border-color]"
    >
      <div className="px-[14px] pt-[14px] pb-[13px] flex flex-col gap-[6px] min-h-[110px]">
        <div className="font-bold text-[0.88rem] leading-snug line-clamp-2">{title}</div>
        <div className="flex items-center gap-[6px] flex-wrap">
          <CodecBadge codec={file.video_codec} />
          {file.width && file.height && (
            <span className="text-[0.7rem] text-muted-dim">{resolutionLabel(file.width, file.height)}</span>
          )}
        </div>
        <div className="text-[0.72rem] text-muted-fg font-mono">{formatBytes(file.size_bytes)}</div>
        <div className="mt-auto pt-1">
          {isCandidate && file.predicted_savings_bytes > 0
            ? <span className="text-[0.78rem] font-semibold text-brand">-{formatBytes(file.predicted_savings_bytes)} recoverable</span>
            : isConverted
              ? <span className="text-[0.72rem] font-medium text-green">Converted</span>
              : null
          }
        </div>
      </div>
      <div
        className="h-[3px] w-full"
        style={{ background: isConverted ? 'var(--green)' : isCandidate ? 'var(--brand)' : 'var(--surface-3)' }}
      />
    </div>
  );
}

function CardSkeleton() {
  return (
    <div className="bg-surface border border-line rounded-[14px] overflow-hidden">
      <div className="px-[14px] pt-[14px] pb-[13px] flex flex-col gap-[6px] min-h-[130px]">
        <Skeleton className="h-4 w-3/4" />
        <Skeleton className="h-3 w-1/2 mt-0.5" />
        <Skeleton className="h-3 w-1/3" />
        <div className="mt-auto pt-1"><Skeleton className="h-3 w-2/3" /></div>
      </div>
      <div className="h-[3px]" style={{ background: 'var(--surface-3)' }} />
    </div>
  );
}

// ── Tab content ───────────────────────────────────────────────────────────────

function TVContent({ search, sort }: { search: string; sort: TVSortValue }) {
  const router = useRouter();

  const { data, fetchNextPage, hasNextPage, isFetchingNextPage, isLoading } = useInfiniteQuery({
    queryKey: ['browse', LIBRARY_TYPE.TV, search],
    queryFn: ({ pageParam }: { pageParam: number }) =>
      api.groupedFiles({ library_type: LIBRARY_TYPE.TV, search: search || undefined, limit: PAGE_SIZE, offset: pageParam }),
    initialPageParam: 0,
    getNextPageParam: (lastPage, allPages) => {
      const loaded = allPages.flatMap((p) => p.series).length;
      if (lastPage.total_count != null) return loaded < lastPage.total_count ? loaded : undefined;
      return lastPage.series.length === PAGE_SIZE ? loaded : undefined;
    },
    placeholderData: (prev) => prev,
  });

  const rawShows = useMemo(() => data?.pages.flatMap((p) => p.series) ?? [], [data]);
  const shows = useMemo(() => sortShows(rawShows, sort), [rawShows, sort]);
  const totalCount = data?.pages[0]?.total_count;

  return (
    <>
      <div className="text-[0.8rem] text-muted-dim mb-4">
        {isLoading && shows.length === 0
          ? <Skeleton className="h-3 w-20" />
          : `${formatInt(totalCount ?? shows.length)} shows`
        }
      </div>

      {isLoading && shows.length === 0 ? (
        <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-4 gap-3">
          {Array.from({ length: 12 }).map((_, i) => <CardSkeleton key={i} />)}
        </div>
      ) : shows.length === 0 ? (
        <div className="text-center py-24 text-muted-dim text-[0.88rem]">
          {search ? 'No shows match your search.' : 'No TV shows found. Run a scan to index your library.'}
        </div>
      ) : (
        <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-4 gap-3">
          {shows.map((s) => (
            <ShowCard key={s.title} show={s} onClick={() => router.push(BROWSE_ROUTES.TV_SHOW(s.title))} />
          ))}
        </div>
      )}

      {hasNextPage && (
        <div className="mt-5 text-center">
          <Button variant="outline" disabled={isFetchingNextPage} onClick={() => void fetchNextPage()} className="rounded-[11px]">
            {isFetchingNextPage ? 'Loading…' : 'Load more'}
          </Button>
        </div>
      )}
    </>
  );
}

function MoviesContent({ search, sort }: { search: string; sort: MovieSortValue }) {
  const router = useRouter();

  const { data, fetchNextPage, hasNextPage, isFetchingNextPage, isLoading } = useInfiniteQuery({
    queryKey: ['browse', LIBRARY_TYPE.MOVIES, search, sort],
    queryFn: ({ pageParam }: { pageParam: Record<string, number | undefined> }) =>
      api.files({ library_type: LIBRARY_TYPE.MOVIES, sort, search: search || undefined, limit: PAGE_SIZE, ...pageParam }),
    initialPageParam: {} as Record<string, number | undefined>,
    getNextPageParam: (lastPage, allPages) => lastPage.items.length === PAGE_SIZE
      ? { offset: allPages.flatMap((p) => p.items).length }
      : undefined,
    placeholderData: (prev) => prev,
  });

  const movies = useMemo(() => data?.pages.flatMap((p) => p.items) ?? [], [data]);
  const totalCount = data?.pages[0]?.total_count;

  return (
    <>
      <div className="text-[0.8rem] text-muted-dim mb-4">
        {isLoading && movies.length === 0
          ? <Skeleton className="h-3 w-20" />
          : `${formatInt(totalCount ?? movies.length)} movies`
        }
      </div>

      {isLoading && movies.length === 0 ? (
        <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-4 gap-3">
          {Array.from({ length: 12 }).map((_, i) => <CardSkeleton key={i} />)}
        </div>
      ) : movies.length === 0 ? (
        <div className="text-center py-24 text-muted-dim text-[0.88rem]">
          {search ? 'No movies match your search.' : 'No movies found. Run a scan to index your library.'}
        </div>
      ) : (
        <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-4 gap-3">
          {movies.map((f) => (
            <MovieCard key={f.id} file={f} onClick={() => router.push(BROWSE_ROUTES.MOVIE(f.id))} />
          ))}
        </div>
      )}

      {hasNextPage && (
        <div className="mt-5 text-center">
          <Button variant="outline" disabled={isFetchingNextPage} onClick={() => void fetchNextPage()} className="rounded-[11px]">
            {isFetchingNextPage ? 'Loading…' : 'Load more'}
          </Button>
        </div>
      )}
    </>
  );
}

// ── Page ──────────────────────────────────────────────────────────────────────

function BrowsePage() {
  const [tabRaw, setTabRaw] = useQueryParam(QUERY_PARAMS.TAB, LIBRARY_TYPE.TV);
  const tab = parseQueryEnum(tabRaw, [LIBRARY_TYPE.TV, LIBRARY_TYPE.MOVIES] as const, LIBRARY_TYPE.TV);

  const [tvSortRaw, setTVSort] = useQueryParam(QUERY_PARAMS.TV_SORT, TV_SORT.ALPHA);
  const tvSort = parseQueryEnum(tvSortRaw, TV_SORT_OPTIONS.map((o) => o.value), TV_SORT.ALPHA);

  const [movieSortRaw, setMovieSort] = useQueryParam(QUERY_PARAMS.MOVIE_SORT, MOVIE_SORT.ALPHA);
  const movieSort = parseQueryEnum(movieSortRaw, MOVIE_SORT_OPTIONS.map((o) => o.value), MOVIE_SORT.ALPHA);

  const [search, setSearch] = useState('');
  const [debouncedSearch, setDebouncedSearch] = useState('');

  useEffect(() => {
    const t = setTimeout(() => setDebouncedSearch(search), 300);
    return () => clearTimeout(t);
  }, [search]);

  const isTV = tab === LIBRARY_TYPE.TV;
  const sortOptions = isTV ? TV_SORT_OPTIONS : MOVIE_SORT_OPTIONS;
  const sortValue = isTV ? tvSort : movieSort;
  const setSort = isTV ? setTVSort : setMovieSort;

  return (
    <div className="flex flex-col min-w-0 h-screen overflow-hidden max-sm:h-full">
      <div
        className="flex flex-col gap-2 px-4 py-[14px] border-b border-line shrink-0 sm:flex-row sm:items-center sm:gap-4 sm:px-7 sm:py-[18px]"
        style={{ background: 'rgba(22,22,22,.82)', backdropFilter: 'blur(10px)' }}
      >
        <div className="min-w-0">
          <div className="text-title font-bold tracking-tight">Browse</div>
          <div className="text-[0.82rem] text-muted-fg mt-px">Your media library at a glance</div>
        </div>
      </div>

      <div className="border-b border-line-soft shrink-0" style={{ background: 'var(--bg)' }}>
        <div className="flex items-center gap-3 px-4 py-3 sm:px-7 flex-wrap">
          <div className="inline-flex bg-surface border border-line rounded-[11px] p-[3px] gap-[2px] shrink-0">
            <button
              onClick={() => { setTabRaw(LIBRARY_TYPE.TV); setSearch(''); }}
              className={cn(
                'rounded-[8px] text-xs font-semibold py-[7px] px-[13px] transition-colors cursor-pointer',
                isTV ? 'bg-brand-soft text-brand' : 'text-muted-fg hover:text-text',
              )}
            >
              TV Shows
            </button>
            <button
              onClick={() => { setTabRaw(LIBRARY_TYPE.MOVIES); setSearch(''); }}
              className={cn(
                'rounded-[8px] text-xs font-semibold py-[7px] px-[13px] transition-colors cursor-pointer',
                !isTV ? 'bg-brand-soft text-brand' : 'text-muted-fg hover:text-text',
              )}
            >
              Movies
            </button>
          </div>

          <Select value={sortValue} onValueChange={setSort}>
            <SelectTrigger className="rounded-[11px] bg-surface text-sm h-auto py-[7px] gap-1 w-[160px]">
              <span className="text-xs text-muted-dim shrink-0">Sort</span>
              <span className="text-muted-dim mx-px">·</span>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {sortOptions.map((o) => <SelectItem key={o.value} value={o.value}>{o.label}</SelectItem>)}
            </SelectContent>
          </Select>

          <div className="flex-1 relative max-w-[300px]">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" className="w-[14px] h-[14px] absolute left-[11px] top-1/2 -translate-y-1/2 text-muted-dim pointer-events-none">
              <circle cx="11" cy="11" r="8"/><line x1="21" y1="21" x2="16.65" y2="16.65"/>
            </svg>
            <Input
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              placeholder={isTV ? 'Search TV shows…' : 'Search movies…'}
              className="rounded-[11px] pl-[34px] text-sm"
            />
          </div>
        </div>
      </div>

      <div className="flex-1 overflow-auto px-4 pt-4 pb-6 sm:px-7">
        {isTV
          ? <TVContent search={debouncedSearch} sort={tvSort} />
          : <MoviesContent search={debouncedSearch} sort={movieSort} />
        }
      </div>
    </div>
  );
}

function BrowseSkeleton() {
  return (
    <div className="flex flex-col min-w-0 h-screen overflow-hidden max-sm:h-full">
      <div className="px-4 py-[14px] border-b border-line sm:px-7 sm:py-[18px]">
        <Skeleton className="h-7 w-28 mb-2" />
        <Skeleton className="h-3 w-48" />
      </div>
    </div>
  );
}

export default function Page() {
  return (
    <Suspense fallback={<BrowseSkeleton />}>
      <BrowsePage />
    </Suspense>
  );
}
