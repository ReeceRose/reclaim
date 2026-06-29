'use client';

import { Suspense, useEffect, useMemo, useRef, useState } from 'react';
import { useRouter } from 'next/navigation';
import { useInfiniteQuery } from '@tanstack/react-query';
import { api, type LibrarySeriesGroup, type MediaFile } from '@/lib/api';
import { baseName, formatBytes, formatInt, resolutionLabel } from '@/lib/format';
import { cn } from '@/lib/utils';
import { parseQueryEnum, useQueryParam } from '@/hooks/use-query-params';
import { Badge } from '@/components/ui/badge';
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
  VIEW_MODE,
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

function tmdbPosterURL(path: string | null | undefined, size: string): string | undefined {
  if (!path) return undefined;
  return `https://image.tmdb.org/t/p/${size}${path}`;
}

// ── Shared UI ─────────────────────────────────────────────────────────────────

export function CodecBadge({ codec }: { codec: string | null }) {
  if (!codec) return <Badge variant="outline" className="font-mono text-xs rounded-md">unknown</Badge>;
  const c = codec.toLowerCase();
  return (
    <Badge className={`font-mono text-xs rounded-md font-semibold ${CODEC_COLORS[c] ?? 'text-slate'} ${CODEC_BORDER[c] ?? 'border-line bg-surface-3'}`}>
      {codec}
    </Badge>
  );
}

export function EncodeHealthBar({ fileCount, eligibleCount }: {
  fileCount: number;
  eligibleCount: number;
}) {
  const donePct = fileCount > 0 ? Math.round(Math.max(0, fileCount - eligibleCount) / fileCount * 100) : 100;
  return (
    <div className="w-full h-1 overflow-hidden rounded-full" style={{ background: 'var(--surface-3)' }}>
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
  const imageURL = tmdbPosterURL(show.backdrop_path, 'w780') ?? tmdbPosterURL(show.poster_path, 'w342');

  return (
    <div
      onClick={onClick}
      className="relative bg-surface border border-line rounded-2xl overflow-hidden cursor-pointer hover:border-[var(--brand-line)] transition-[border-color] group"
    >
      <div className="relative h-48 overflow-hidden" style={{ background: 'var(--surface-2)' }}>
        {imageURL ? (
          <>
            <img
              src={imageURL}
              alt={show.title}
              className="w-full h-full object-cover transition-transform duration-300 group-hover:scale-105"
              loading="lazy"
            />
            <div className="absolute inset-0" style={{ background: 'linear-gradient(to bottom, transparent 35%, rgba(10,10,10,0.88) 100%)' }} />
            <div className="absolute bottom-0 left-0 right-0 px-3 pb-2.5">
              <div className="font-bold text-sm leading-snug line-clamp-2 text-white drop-shadow">{show.title}</div>
            </div>
          </>
        ) : (
          <>
            <div className="w-full h-full flex items-center justify-center">
              <span className="font-black select-none pointer-events-none leading-none opacity-10 text-8xl" aria-hidden>
                {letter}
              </span>
            </div>
            <div className="absolute bottom-0 left-0 right-0 px-3 pb-2.5">
              <div className="font-bold text-sm leading-snug line-clamp-2">{show.title}</div>
            </div>
          </>
        )}
      </div>

      <div className="px-3 pt-2 pb-3 flex flex-col gap-1">
        <div className="text-xs text-muted-dim">
          {show.season_count} {show.season_count === 1 ? 'season' : 'seasons'} · {formatInt(show.file_count)} files
        </div>
        <div className="flex items-center justify-between gap-2">
          <span className="text-xs text-muted-fg font-mono">{formatBytes(show.total_bytes)}</span>
          {fullyConverted
            ? <span className="text-xs font-medium text-green">All converted</span>
            : show.predicted_savings_bytes > 0
              ? <span className="text-xs font-semibold text-brand">-{formatBytes(show.predicted_savings_bytes)}</span>
              : null
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
  const imageURL = tmdbPosterURL(file.backdrop_path, 'w780') ?? tmdbPosterURL(file.poster_path, 'w342');

  return (
    <div
      onClick={onClick}
      className="relative bg-surface border border-line rounded-2xl overflow-hidden cursor-pointer hover:border-[var(--brand-line)] transition-[border-color] group"
    >
      <div className="relative h-48 overflow-hidden" style={{ background: 'var(--surface-2)' }}>
        {imageURL ? (
          <>
            <img
              src={imageURL}
              alt={title}
              className="w-full h-full object-cover transition-transform duration-300 group-hover:scale-105"
              loading="lazy"
            />
            <div className="absolute inset-0" style={{ background: 'linear-gradient(to bottom, transparent 35%, rgba(10,10,10,0.88) 100%)' }} />
            <div className="absolute bottom-0 left-0 right-0 px-3 pb-2.5">
              <div className="font-bold text-sm leading-snug line-clamp-2 text-white drop-shadow">{title}</div>
            </div>
          </>
        ) : (
          <>
            <div className="w-full h-full flex items-center justify-center gap-1.5 flex-wrap px-3">
              <CodecBadge codec={file.video_codec} />
              {file.width && file.height && (
                <span className="text-xs text-muted-dim">{resolutionLabel(file.width, file.height)}</span>
              )}
            </div>
            <div className="absolute bottom-0 left-0 right-0 px-3 pb-2.5">
              <div className="font-bold text-sm leading-snug line-clamp-2">{title}</div>
            </div>
          </>
        )}
      </div>

      <div className="px-3 pt-2 pb-3 flex flex-col gap-1">
        <div className="flex items-center gap-1.5 flex-wrap">
          <CodecBadge codec={file.video_codec} />
          {file.width && file.height && (
            <span className="text-xs text-muted-dim">{resolutionLabel(file.width, file.height)}</span>
          )}
        </div>
        <div className="flex items-center justify-between gap-2">
          <span className="text-xs text-muted-fg font-mono">{formatBytes(file.size_bytes)}</span>
          {isCandidate && file.predicted_savings_bytes > 0
            ? <span className="text-xs font-semibold text-brand">-{formatBytes(file.predicted_savings_bytes)}</span>
            : isConverted
              ? <span className="text-xs font-medium text-green">Converted</span>
              : null
          }
        </div>
      </div>
      <div className="h-1 w-full" style={{ background: isConverted ? 'var(--green)' : isCandidate ? 'var(--brand)' : 'var(--surface-3)' }} />
    </div>
  );
}

function CardSkeleton() {
  return (
    <div className="bg-surface border border-line rounded-2xl overflow-hidden">
      <Skeleton className="h-48 w-full rounded-none" />
      <div className="px-3 pt-2 pb-3 flex flex-col gap-1">
        <Skeleton className="h-3 w-1/2" />
        <div className="flex items-center justify-between">
          <Skeleton className="h-3 w-1/3" />
          <Skeleton className="h-3 w-1/4" />
        </div>
      </div>
      <div className="h-1" style={{ background: 'var(--surface-3)' }} />
    </div>
  );
}

// ── List rows ─────────────────────────────────────────────────────────────────

function ShowRow({ show, onClick }: { show: LibrarySeriesGroup; onClick: () => void }) {
  const fullyConverted = show.eligible_count === 0;
  return (
    <div
      onClick={onClick}
      className="grid items-center gap-3 px-4 py-2.5 border-b border-line-soft last:border-b-0 cursor-pointer hover:bg-surface-2 transition-colors"
      style={{ gridTemplateColumns: '1fr auto auto auto auto' }}
    >
      <div className="min-w-0 truncate font-medium text-sm">{show.title}</div>
      <span className="text-xs text-muted-dim hidden sm:inline whitespace-nowrap">
        {show.season_count} {show.season_count === 1 ? 'season' : 'seasons'}
      </span>
      <span className="text-xs text-muted-dim hidden md:inline whitespace-nowrap">
        {formatInt(show.file_count)} files
      </span>
      <span className="font-mono text-xs text-muted-fg">{formatBytes(show.total_bytes)}</span>
      <div className="text-right w-24">
        {fullyConverted
          ? <span className="text-xs font-medium text-green">All converted</span>
          : show.predicted_savings_bytes > 0
            ? <span className="text-xs font-semibold text-brand font-mono">-{formatBytes(show.predicted_savings_bytes)}</span>
            : <span className="text-xs text-muted-dim">—</span>
        }
      </div>
    </div>
  );
}

function MovieRow({ file, onClick }: { file: MediaFile; onClick: () => void }) {
  const title = baseName(file.path).replace(/\.[^/.]+$/, '');
  const isConverted = file.candidate_state === 'already_hevc' || file.candidate_state === 'completed';
  const isCandidate = file.candidate_state === 'candidate';
  return (
    <div
      onClick={onClick}
      className="grid items-center gap-3 px-4 py-2.5 border-b border-line-soft last:border-b-0 cursor-pointer hover:bg-surface-2 transition-colors"
      style={{ gridTemplateColumns: '1fr auto auto auto auto' }}
    >
      <div className="min-w-0 truncate font-medium text-sm">{title}</div>
      <CodecBadge codec={file.video_codec} />
      <span className="text-xs text-muted-dim hidden sm:inline">
        {file.width && file.height ? resolutionLabel(file.width, file.height) : '—'}
      </span>
      <span className="font-mono text-xs text-muted-fg hidden md:inline">{formatBytes(file.size_bytes)}</span>
      <div className="text-right w-24">
        {isCandidate && file.predicted_savings_bytes > 0
          ? <span className="text-xs font-semibold text-brand font-mono">-{formatBytes(file.predicted_savings_bytes)}</span>
          : isConverted
            ? <span className="text-xs font-medium text-green">Converted</span>
            : <span className="text-xs text-muted-dim">—</span>
        }
      </div>
    </div>
  );
}

function ListSkeleton() {
  return (
    <div className="bg-surface border border-line rounded-xl overflow-hidden">
      <div className="px-4 py-2 border-b border-line bg-surface-2">
        <div className="flex gap-3">
          <Skeleton className="h-3 flex-1" />
          <Skeleton className="h-3 w-16" />
          <Skeleton className="h-3 w-16" />
          <Skeleton className="h-3 w-14" />
          <Skeleton className="h-3 w-16" />
        </div>
      </div>
      {Array.from({ length: 8 }).map((_, i) => (
        <div key={i} className="flex items-center gap-3 px-4 py-2.5 border-b border-line-soft last:border-b-0">
          <Skeleton className="h-3 flex-1" />
          <Skeleton className="h-3 w-16" />
          <Skeleton className="h-3 w-12" />
          <Skeleton className="h-3 w-14" />
          <Skeleton className="h-3 w-16" />
        </div>
      ))}
    </div>
  );
}

// ── Tab content ───────────────────────────────────────────────────────────────

function TVContent({ search, sort, view }: { search: string; sort: TVSortValue; view: string }) {
  const router = useRouter();
  const sentinelRef = useRef<HTMLDivElement>(null);

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

  useEffect(() => {
    const el = sentinelRef.current;
    if (!el) return;
    const observer = new IntersectionObserver(
      ([entry]) => { if (entry.isIntersecting && hasNextPage && !isFetchingNextPage) void fetchNextPage(); },
      { rootMargin: '200px' },
    );
    observer.observe(el);
    return () => observer.disconnect();
  }, [hasNextPage, isFetchingNextPage, fetchNextPage]);

  const rawShows = useMemo(() => data?.pages.flatMap((p) => p.series) ?? [], [data]);
  const shows = useMemo(() => sortShows(rawShows, sort), [rawShows, sort]);
  const totalCount = data?.pages[0]?.total_count;

  const isList = view === VIEW_MODE.LIST;

  return (
    <>
      <div className="text-xs text-muted-dim mb-4">
        {isLoading && shows.length === 0
          ? <Skeleton className="h-3 w-20" />
          : `${formatInt(totalCount ?? shows.length)} shows`
        }
      </div>

      {isLoading && shows.length === 0 ? (
        isList ? <ListSkeleton /> : (
          <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-4 gap-3">
            {Array.from({ length: 12 }).map((_, i) => <CardSkeleton key={i} />)}
          </div>
        )
      ) : shows.length === 0 ? (
        <div className="text-center py-24 text-muted-dim text-sm">
          {search ? 'No shows match your search.' : 'No TV shows found. Run a scan to index your library.'}
        </div>
      ) : isList ? (
        <div className="bg-surface border border-line rounded-xl overflow-hidden">
          <div className="grid items-center gap-3 px-4 py-2 border-b border-line text-xs uppercase tracking-wider text-muted-dim font-bold" style={{ gridTemplateColumns: '1fr auto auto auto auto' }}>
            <span>Title</span>
            <span className="hidden sm:inline">Seasons</span>
            <span className="hidden md:inline">Files</span>
            <span>Size</span>
            <span className="w-24 text-right">Savings</span>
          </div>
          {shows.map((s) => (
            <ShowRow key={s.title} show={s} onClick={() => router.push(BROWSE_ROUTES.TV_SHOW(s.title, view))} />
          ))}
        </div>
      ) : (
        <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-4 gap-3">
          {shows.map((s) => (
            <ShowCard key={s.title} show={s} onClick={() => router.push(BROWSE_ROUTES.TV_SHOW(s.title, view))} />
          ))}
        </div>
      )}

      <div ref={sentinelRef} className="h-px" />
      {isFetchingNextPage && (
        isList ? null : (
          <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-4 gap-3 mt-3">
            {Array.from({ length: 4 }).map((_, i) => <CardSkeleton key={i} />)}
          </div>
        )
      )}
    </>
  );
}

function MoviesContent({ search, sort, view }: { search: string; sort: MovieSortValue; view: string }) {
  const router = useRouter();
  const sentinelRef = useRef<HTMLDivElement>(null);

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

  useEffect(() => {
    const el = sentinelRef.current;
    if (!el) return;
    const observer = new IntersectionObserver(
      ([entry]) => { if (entry.isIntersecting && hasNextPage && !isFetchingNextPage) void fetchNextPage(); },
      { rootMargin: '200px' },
    );
    observer.observe(el);
    return () => observer.disconnect();
  }, [hasNextPage, isFetchingNextPage, fetchNextPage]);

  const movies = useMemo(() => data?.pages.flatMap((p) => p.items) ?? [], [data]);
  const totalCount = data?.pages[0]?.total_count;
  const isList = view === VIEW_MODE.LIST;

  return (
    <>
      <div className="text-xs text-muted-dim mb-4">
        {isLoading && movies.length === 0
          ? <Skeleton className="h-3 w-20" />
          : `${formatInt(totalCount ?? movies.length)} movies`
        }
      </div>

      {isLoading && movies.length === 0 ? (
        isList ? <ListSkeleton /> : (
          <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-4 gap-3">
            {Array.from({ length: 12 }).map((_, i) => <CardSkeleton key={i} />)}
          </div>
        )
      ) : movies.length === 0 ? (
        <div className="text-center py-24 text-muted-dim text-sm">
          {search ? 'No movies match your search.' : 'No movies found. Run a scan to index your library.'}
        </div>
      ) : isList ? (
        <div className="bg-surface border border-line rounded-xl overflow-hidden">
          <div className="grid items-center gap-3 px-4 py-2 border-b border-line text-xs uppercase tracking-wider text-muted-dim font-bold" style={{ gridTemplateColumns: '1fr auto auto auto auto' }}>
            <span>Title</span>
            <span>Codec</span>
            <span className="hidden sm:inline">Res</span>
            <span className="hidden md:inline">Size</span>
            <span className="w-24 text-right">Savings</span>
          </div>
          {movies.map((f) => (
            <MovieRow key={f.id} file={f} onClick={() => router.push(BROWSE_ROUTES.MOVIE(f.id))} />
          ))}
        </div>
      ) : (
        <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-4 gap-3">
          {movies.map((f) => (
            <MovieCard key={f.id} file={f} onClick={() => router.push(BROWSE_ROUTES.MOVIE(f.id))} />
          ))}
        </div>
      )}

      <div ref={sentinelRef} className="h-px" />
      {isFetchingNextPage && (
        isList ? null : (
          <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-4 gap-3 mt-3">
            {Array.from({ length: 4 }).map((_, i) => <CardSkeleton key={i} />)}
          </div>
        )
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

  const [viewRaw, setView] = useQueryParam(QUERY_PARAMS.VIEW, VIEW_MODE.GRID);
  const view = parseQueryEnum(viewRaw, [VIEW_MODE.GRID, VIEW_MODE.LIST] as const, VIEW_MODE.GRID);

  const [search, setSearch] = useState('');
  const [debouncedSearch, setDebouncedSearch] = useState('');

  useEffect(() => {
    const t = setTimeout(() => setDebouncedSearch(search), 300);
    return () => clearTimeout(t);
  }, [search]);

  const isTV = tab === LIBRARY_TYPE.TV;
  const isList = view === VIEW_MODE.LIST;
  const sortOptions = isTV ? TV_SORT_OPTIONS : MOVIE_SORT_OPTIONS;
  const sortValue = isTV ? tvSort : movieSort;
  const setSort = isTV ? setTVSort : setMovieSort;

  return (
    <div className="flex flex-col min-w-0 h-screen overflow-hidden max-sm:h-full">
      <div
        className="flex flex-col gap-2 px-4 py-3.5 border-b border-line shrink-0 sm:flex-row sm:items-center sm:gap-4 sm:px-7 sm:py-4"
        style={{ background: 'rgba(22,22,22,.82)', backdropFilter: 'blur(10px)' }}
      >
        <div className="min-w-0">
          <div className="text-title font-bold tracking-tight">Browse</div>
          <div className="text-sm text-muted-fg mt-px">Your media library at a glance</div>
        </div>
      </div>

      <div className="border-b border-line-soft shrink-0" style={{ background: 'var(--bg)' }}>
        <div className="flex items-center gap-3 px-4 py-3 sm:px-7 flex-wrap">
          <div className="inline-flex bg-surface border border-line rounded-xl p-1 gap-0.5 shrink-0">
            <button
              onClick={() => { setTabRaw(LIBRARY_TYPE.TV); setSearch(''); }}
              className={cn(
                'rounded-lg text-xs font-semibold py-2 px-3 transition-colors cursor-pointer',
                isTV ? 'bg-brand-soft text-brand' : 'text-muted-fg hover:text-text',
              )}
            >
              TV Shows
            </button>
            <button
              onClick={() => { setTabRaw(LIBRARY_TYPE.MOVIES); setSearch(''); }}
              className={cn(
                'rounded-lg text-xs font-semibold py-2 px-3 transition-colors cursor-pointer',
                !isTV ? 'bg-brand-soft text-brand' : 'text-muted-fg hover:text-text',
              )}
            >
              Movies
            </button>
          </div>

          <Select value={sortValue} onValueChange={setSort}>
            <SelectTrigger className="rounded-xl bg-surface text-sm h-auto py-2 gap-1 w-40">
              <span className="text-xs text-muted-dim shrink-0">Sort</span>
              <span className="text-muted-dim mx-px">·</span>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {sortOptions.map((o) => <SelectItem key={o.value} value={o.value}>{o.label}</SelectItem>)}
            </SelectContent>
          </Select>

          <div className="flex-1 relative max-w-xs">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" className="w-3.5 h-3.5 absolute left-3 top-1/2 -translate-y-1/2 text-muted-dim pointer-events-none">
              <circle cx="11" cy="11" r="8"/><line x1="21" y1="21" x2="16.65" y2="16.65"/>
            </svg>
            <Input
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              placeholder={isTV ? 'Search TV shows…' : 'Search movies…'}
              className="rounded-xl pl-8 text-sm"
            />
          </div>

          <div className="inline-flex bg-surface border border-line rounded-xl p-1 gap-0.5 shrink-0">
            <button
              onClick={() => setView(VIEW_MODE.GRID)}
              className={cn(
                'rounded-lg p-2 transition-colors cursor-pointer',
                !isList ? 'bg-brand-soft text-brand' : 'text-muted-fg hover:text-text',
              )}
              aria-label="Grid view"
            >
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" className="w-3.5 h-3.5">
                <rect x="3" y="3" width="7" height="7"/><rect x="14" y="3" width="7" height="7"/>
                <rect x="3" y="14" width="7" height="7"/><rect x="14" y="14" width="7" height="7"/>
              </svg>
            </button>
            <button
              onClick={() => setView(VIEW_MODE.LIST)}
              className={cn(
                'rounded-lg p-2 transition-colors cursor-pointer',
                isList ? 'bg-brand-soft text-brand' : 'text-muted-fg hover:text-text',
              )}
              aria-label="List view"
            >
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" className="w-3.5 h-3.5">
                <line x1="8" y1="6" x2="21" y2="6"/><line x1="8" y1="12" x2="21" y2="12"/>
                <line x1="8" y1="18" x2="21" y2="18"/>
                <circle cx="3" cy="6" r="1" fill="currentColor" stroke="none"/>
                <circle cx="3" cy="12" r="1" fill="currentColor" stroke="none"/>
                <circle cx="3" cy="18" r="1" fill="currentColor" stroke="none"/>
              </svg>
            </button>
          </div>
        </div>
      </div>

      <div className="flex-1 overflow-auto px-4 pt-4 pb-6 sm:px-7">
        {isTV
          ? <TVContent search={debouncedSearch} sort={tvSort} view={view} />
          : <MoviesContent search={debouncedSearch} sort={movieSort} view={view} />
        }
      </div>
    </div>
  );
}

function BrowseSkeleton() {
  return (
    <div className="flex flex-col min-w-0 h-screen overflow-hidden max-sm:h-full">
      <div className="px-4 py-3.5 border-b border-line sm:px-7 sm:py-4">
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
