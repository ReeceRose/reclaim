'use client';

import { useInfiniteQuery, useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { useVirtualizer } from '@tanstack/react-virtual';
import { api, type MediaFile, type CandidateFilters, type Episode, type SeriesGroup } from '@/lib/api';
import { formatBytes, formatInt, formatCoverage } from '@/lib/format';
import { cn } from '@/lib/utils';
import { Skeleton } from '@/components/ui/skeleton';
import { toast } from 'sonner';
import { useRef, useState, useEffect, useMemo, useCallback, useTransition, Suspense } from 'react';
import { useRouter } from 'next/navigation';
import { parseQueryEnum, useQueryParam, useQueryParams } from '@/hooks/use-query-params';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Checkbox } from '@/components/ui/checkbox';
import { Input } from '@/components/ui/input';
import { FilterSelect } from '@/components/filter-select';
import { BROWSE_ROUTES } from '@/app/(app)/browse/browse';
import { codecFilterOptions, libraryFilterOptions, resolutionFilterOptions } from '@/lib/filter-options';
import { MediaFlatRow } from '@/components/media/media-flat-row';
import { QueueConfirmDialog } from '@/components/media/queue-confirm-dialog';
import { QueueSelectionBar } from '@/components/media/selection-bar';
import { GroupedSkeleton } from '@/components/media/grouped-skeleton';
import { PageHeader } from '@/components/ui/page-header';
import { EmptyState } from '@/components/ui/empty-state';

const PAGE_SIZE = 100;

type SortKey = 'savings_desc' | 'size_desc' | 'mtime_desc' | 'mtime_asc' | 'codec';

const SORT_OPTIONS: { value: SortKey; label: string }[] = [
  { value: 'savings_desc', label: 'Predicted savings' },
  { value: 'size_desc', label: 'Largest file' },
  { value: 'mtime_desc', label: 'Newest file' },
  { value: 'mtime_asc', label: 'Oldest file' },
  { value: 'codec', label: 'Source codec' },
];

function EpisodeRow(props: { ep: Episode; selected: boolean; onToggle: (id: number) => void; onOpen: (file: MediaFile) => void }) {
  return (
    <div className="pl-[42px]">
      <MediaFlatRow item={props.ep} selected={props.selected} onToggle={props.onToggle} onOpen={props.onOpen} />
    </div>
  );
}

function SeasonEpisodes({
  seriesTitle,
  season,
  filters,
  selectedIds,
  onToggle,
  onOpen,
  onEpisodesLoaded,
}: {
  seriesTitle: string;
  season: number;
  filters: CandidateFilters;
  selectedIds: Set<number>;
  onToggle: (id: number) => void;
  onOpen: (file: MediaFile) => void;
  onEpisodesLoaded: (files: MediaFile[]) => void;
}) {
  const { data, fetchNextPage, hasNextPage, isFetchingNextPage, isLoading } = useInfiniteQuery({
    queryKey: ['candidates', 'grouped', 'episodes', filters, seriesTitle, season],
    queryFn: ({ pageParam }: { pageParam: number }) =>
      api.groupedSeasonEpisodes({ ...filters, series: seriesTitle, season, limit: PAGE_SIZE, offset: pageParam }),
    initialPageParam: 0,
    getNextPageParam: (lastPage, allPages) => {
      const loaded = allPages.flatMap((p) => p.episodes).length;
      if (lastPage.total_count != null) return loaded < lastPage.total_count ? loaded : undefined;
      return lastPage.episodes.length === PAGE_SIZE ? loaded : undefined;
    },
  });

  const episodes = useMemo(() => data?.pages.flatMap((p) => p.episodes) ?? [], [data]);

  useEffect(() => {
    if (episodes.length > 0) onEpisodesLoaded(episodes);
  }, [episodes, onEpisodesLoaded]);

  if (isLoading) {
    return (
      <div className="px-4 py-3 pl-[42px] text-sm text-muted-dim border-b border-line-soft">
        Loading episodes…
      </div>
    );
  }

  return (
    <>
      {episodes.map((ep) => (
        <EpisodeRow key={ep.id} ep={ep} selected={selectedIds.has(ep.id)} onToggle={onToggle} onOpen={onOpen} />
      ))}
      {(hasNextPage || isFetchingNextPage) && (
        <div className="px-4 py-2 pl-[42px] border-b border-line-soft">
          <Button variant="ghost" size="sm" disabled={isFetchingNextPage} onClick={() => void fetchNextPage()} className="text-xs text-muted-fg">
            {isFetchingNextPage ? 'Loading more…' : 'Load more episodes'}
          </Button>
        </div>
      )}
    </>
  );
}

function GroupedContent({
  selectedIds,
  onToggle,
  onToggleSeries,
  onOpen,
  filters,
  onEpisodesLoaded,
}: {
  selectedIds: Set<number>;
  onToggle: (id: number) => void;
  onToggleSeries: (ids: number[]) => void;
  onOpen: (file: MediaFile) => void;
  filters: CandidateFilters;
  onEpisodesLoaded: (files: MediaFile[]) => void;
}) {
  const { data, fetchNextPage: fetchNextSeries, hasNextPage: hasMoreSeries, isFetchingNextPage: isFetchingMoreSeries, isLoading } = useInfiniteQuery({
    queryKey: ['candidates', 'grouped', filters],
    queryFn: ({ pageParam }: { pageParam: number }) =>
      api.groupedCandidates({ ...filters, limit: PAGE_SIZE, offset: pageParam }),
    initialPageParam: 0,
    getNextPageParam: (lastPage, allPages) => {
      const loaded = allPages.flatMap((p) => p.series).length;
      if (lastPage.total_count != null) return loaded < lastPage.total_count ? loaded : undefined;
      return lastPage.series.length === PAGE_SIZE ? loaded : undefined;
    },
  });
  const groupedSeries = useMemo(() => data?.pages.flatMap((p) => p.series) ?? [], [data]);

  const movieFilters = { ...filters, library_type: 'movies' as const };
  const {
    data: movieData,
    fetchNextPage: fetchNextMovies,
    hasNextPage: hasMoreMovies,
    isFetchingNextPage: isFetchingMovies,
  } = useInfiniteQuery({
    queryKey: ['candidates', 'grouped', 'movies', movieFilters],
    queryFn: ({ pageParam }: { pageParam: Record<string, number | undefined> }) =>
      api.candidates({ ...movieFilters, sort: filters.sort, limit: PAGE_SIZE, ...pageParam }),
    initialPageParam: {} as Record<string, number | undefined>,
    getNextPageParam: (lastPage, allPages) => {
      if (lastPage.next_cursor) {
        return { after_savings: lastPage.next_cursor.after_savings, after_id: lastPage.next_cursor.after_id };
      }
      if (lastPage.items.length === PAGE_SIZE) {
        return { offset: allPages.flatMap((p) => p.items).length };
      }
      return undefined;
    },
    enabled: filters.library_type !== 'tv',
  });

  const movies = useMemo(
    () => movieData?.pages.flatMap((p) => p.items) ?? [],
    [movieData],
  );

  useEffect(() => {
    if (movies.length > 0) onEpisodesLoaded(movies);
  }, [movies, onEpisodesLoaded]);

  const [expandedSeries, setExpandedSeries] = useState<Set<string>>(new Set());
  const [expandedSeasons, setExpandedSeasons] = useState<Set<string>>(new Set());

  function toggleSeriesExpand(title: string) {
    setExpandedSeries((prev) => {
      const next = new Set(prev);
      if (next.has(title)) { next.delete(title); } else { next.add(title); }
      return next;
    });
  }

  function toggleSeasonExpand(key: string) {
    setExpandedSeasons((prev) => {
      const next = new Set(prev);
      if (next.has(key)) { next.delete(key); } else { next.add(key); }
      return next;
    });
  }

  function seriesEpisodeIds(s: SeriesGroup): number[] {
    return s.seasons.flatMap((se) => se.episode_ids);
  }

  function seriesSelState(s: SeriesGroup): 'none' | 'partial' | 'all' {
    const ids = seriesEpisodeIds(s);
    const selCount = ids.filter((id) => selectedIds.has(id)).length;
    if (selCount === 0) return 'none';
    if (selCount === ids.length) return 'all';
    return 'partial';
  }

  return (
    <div className="bg-surface border border-line rounded-(--radius) overflow-hidden">
      {isLoading && groupedSeries.length === 0 ? <GroupedSkeleton withCheckbox /> : groupedSeries.map((s) => {
        const expanded = expandedSeries.has(s.title);
        const selState = seriesSelState(s);
        const allIds = seriesEpisodeIds(s);
        return (
          <div key={s.title}>
            <div
              className="flex items-center gap-[11px] px-4 py-[13px] border-b border-line-soft hover:bg-surface-2 cursor-pointer transition-colors"
              onClick={() => toggleSeriesExpand(s.title)}
            >
              <Checkbox
                checked={selState === 'all' ? true : selState === 'partial' ? 'indeterminate' : false}
                onCheckedChange={() => onToggleSeries(allIds)}
                onClick={(e) => e.stopPropagation()}
                className="size-[17px] rounded-[5px] shrink-0"
              />
              <span className={`w-[18px] h-[18px] shrink-0 grid place-items-center text-muted-fg transition-transform ${expanded ? 'rotate-90' : ''}`}>
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" className="w-3 h-3"><path d="M9 18l6-6-6-6"/></svg>
              </span>
              <div className="flex-1 min-w-0">
                <div className="font-bold text-[0.92rem] flex items-center gap-2">
                  {s.title}
                  <Badge className="text-[0.7rem] rounded text-sky border-[rgba(51,177,255,.32)] bg-[rgba(51,177,255,.1)]">TV</Badge>
                </div>
                <div className="text-[0.76rem] text-muted-fg mt-0.5">
                  {s.season_count} seasons · {formatCoverage(s.file_count, s.candidate_count)} · {formatBytes(s.total_bytes)}
                </div>
              </div>
              <div className="text-right shrink-0">
                <div className="text-[0.78rem] text-muted-fg">{formatBytes(s.total_bytes)}</div>
                <div className="text-[0.92rem] font-semibold text-brand">{formatBytes(s.predicted_savings_bytes)}</div>
              </div>
            </div>

            {expanded && (
              <div style={{ background: 'var(--bg)' }}>
                {s.seasons.map((se) => {
                  const seasonKey = `${s.title}-${se.season}`;
                  const seasonExpanded = expandedSeasons.has(seasonKey);
                  const seasonSelCount = se.episode_ids.filter((id) => selectedIds.has(id)).length;
                  return (
                    <div key={se.season}>
                      <div
                        className="flex items-center gap-[10px] px-4 py-[9px] pl-[18px] text-[0.76rem] font-semibold text-muted-fg bg-surface-2 border-b border-line-soft cursor-pointer"
                        onClick={() => toggleSeasonExpand(seasonKey)}
                      >
                        <span className={`w-[18px] h-[18px] shrink-0 grid place-items-center transition-transform ${seasonExpanded ? 'rotate-90' : ''}`}>
                          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" className="w-3 h-3"><path d="M9 18l6-6-6-6"/></svg>
                        </span>
                        Season {se.season}
                        <span className="text-muted-dim">{formatCoverage(se.file_count, se.candidate_count)}</span>
                        {seasonSelCount > 0 && <span className="text-brand">({seasonSelCount} sel)</span>}
                        <span className="ml-auto text-brand font-semibold">{formatBytes(se.predicted_savings_bytes)}</span>
                      </div>
                      {seasonExpanded && (
                        <SeasonEpisodes
                          seriesTitle={s.title}
                          season={se.season}
                          filters={filters}
                          selectedIds={selectedIds}
                          onToggle={onToggle}
                          onOpen={onOpen}
                          onEpisodesLoaded={onEpisodesLoaded}
                        />
                      )}
                    </div>
                  );
                })}
              </div>
            )}
          </div>
        );
      })}
      {(hasMoreSeries || isFetchingMoreSeries) && (
        <div className="px-4 py-3 text-center border-t border-line-soft">
          <Button variant="ghost" size="sm" disabled={isFetchingMoreSeries} onClick={() => void fetchNextSeries()} className="text-sm text-muted-fg">
            {isFetchingMoreSeries ? 'Loading more…' : 'Load more series'}
          </Button>
        </div>
      )}
      {filters.library_type !== 'tv' && movies.length > 0 && (
        <>
          <div className="text-[0.7rem] uppercase tracking-widest text-muted-dim font-bold px-4 pt-[15px] pb-[9px] border-b border-line-soft">
            Movies
          </div>
          {movies.map((f) => (
            <MediaFlatRow key={f.id} item={f} selected={selectedIds.has(f.id)} onToggle={onToggle} onOpen={onOpen} />
          ))}
          {(hasMoreMovies || isFetchingMovies) && (
            <div className="px-4 py-3 text-center border-t border-line-soft">
              <Button
                variant="ghost"
                size="sm"
                disabled={isFetchingMovies}
                onClick={() => void fetchNextMovies()}
                className="text-sm text-muted-fg"
              >
                {isFetchingMovies ? 'Loading more…' : 'Load more movies'}
              </Button>
            </div>
          )}
        </>
      )}
    </div>
  );
}

function GroupedView(props: {
  selectedIds: Set<number>;
  onToggle: (id: number) => void;
  onToggleSeries: (ids: number[]) => void;
  onOpen: (file: MediaFile) => void;
  filters: CandidateFilters;
  onEpisodesLoaded: (files: MediaFile[]) => void;
}) {
  return <GroupedContent {...props} />;
}

function CandidatesPage() {
  const qc = useQueryClient();
  const router = useRouter();
  const [isPending, startTransition] = useTransition();
  const { get, set: setQuery } = useQueryParams();
  const [search, setSearch] = useState(() => get('q') ?? '');
  const [debouncedSearch, setDebouncedSearch] = useState(() => get('q') ?? '');
  const [sortRaw, setSortRaw] = useQueryParam('sort', 'savings_desc');
  const sort = parseQueryEnum(sortRaw, SORT_OPTIONS.map((o) => o.value), 'savings_desc');
  const [codec, setCodec] = useQueryParam('codec');
  const [resolution, setResolution] = useQueryParam('res');
  const [library, setLibrary] = useQueryParam('library');
  const [viewRaw, setViewRaw] = useQueryParam('view', 'flat');
  const view = parseQueryEnum(viewRaw, ['flat', 'grouped'] as const, 'flat');
  const [selectedIds, setSelectedIds] = useState<Set<number>>(new Set());
  const [confirmOpen, setConfirmOpen] = useState(false);
  const fileMapRef = useRef<Map<number, MediaFile>>(new Map());
  const parentRef = useRef<HTMLDivElement>(null);

  const { data: stats } = useQuery({
    queryKey: ['stats'],
    queryFn: api.stats,
    staleTime: 30_000,
  });
  const codecOptions = useMemo(() => codecFilterOptions(stats, { excludeHEVC: true, excludeUnknown: true }), [stats]);
  const resolutionOptions = useMemo(() => resolutionFilterOptions(stats, { excludeUnknown: true }), [stats]);
  const libraryOptions = useMemo(() => libraryFilterOptions(stats, { excludeUnknown: true }), [stats]);

  // Clamp to options that actually exist so a stale selection (e.g. after a
  // rescan drops a codec) reads as "All" without a setState-in-effect.
  const effectiveCodec = codec && codecOptions.some((o) => o.value === codec) ? codec : '';
  const effectiveResolution = resolution && resolutionOptions.some((o) => o.value === resolution) ? resolution : '';
  const effectiveLibrary = library && libraryOptions.some((o) => o.value === library) ? library : '';

  const qFromUrl = get('q') ?? '';
  useEffect(() => {
    setSearch(qFromUrl);
    setDebouncedSearch(qFromUrl);
  }, [qFromUrl]);

  useEffect(() => {
    const t = setTimeout(() => {
      startTransition(() => {
        setDebouncedSearch(search);
        setQuery({ q: search || null });
      });
    }, 300);
    return () => clearTimeout(t);
  }, [search, setQuery]);

  const filters: CandidateFilters = {
    sort,
    video_codec: effectiveCodec || undefined,
    height: effectiveResolution || undefined,
    library_type: effectiveLibrary || undefined,
    search: debouncedSearch || undefined,
  };

  const { data, fetchNextPage, hasNextPage, isFetchingNextPage } = useInfiniteQuery({
    queryKey: ['candidates', filters],
    queryFn: ({ pageParam }: { pageParam: Record<string, number | undefined> }) =>
      api.candidates({ ...filters, limit: PAGE_SIZE, ...pageParam }),
    initialPageParam: {} as Record<string, number | undefined>,
    getNextPageParam: (lastPage, allPages) => {
      if (lastPage.next_cursor) {
        return { after_savings: lastPage.next_cursor.after_savings, after_id: lastPage.next_cursor.after_id };
      }
      if (lastPage.items.length === PAGE_SIZE) {
        return { offset: allPages.flatMap((p) => p.items).length };
      }
      return undefined;
    },
    placeholderData: (prev) => prev,
  });

  const allItems = useMemo(
    () => data?.pages.flatMap((p) => p.items) ?? [],
    [data],
  );

  useEffect(() => {
    allItems.forEach((item) => fileMapRef.current.set(item.id, item));
  }, [allItems]);

  // eslint-disable-next-line react-hooks/incompatible-library -- TanStack Virtual returns non-memoizable functions; React Compiler intentionally skips this hook.
  const virtualizer = useVirtualizer({
    count: hasNextPage ? allItems.length + 1 : allItems.length,
    getScrollElement: () => parentRef.current,
    estimateSize: () => 52,
    overscan: 15,
  });

  const virtualItems = virtualizer.getVirtualItems();
  useEffect(() => {
    const last = virtualItems[virtualItems.length - 1];
    if (!last) return;
    if (last.index >= allItems.length - 1 && hasNextPage && !isFetchingNextPage) {
      void fetchNextPage();
    }
  }, [virtualItems, allItems.length, hasNextPage, isFetchingNextPage, fetchNextPage]);

  const { data: profilesData } = useQuery({
    queryKey: ['profiles'],
    queryFn: api.profiles,
    staleTime: 60_000,
  });
  const profiles = profilesData?.items ?? [];

  function toggleId(id: number) {
    setSelectedIds((prev) => {
      const next = new Set(prev);
      if (next.has(id)) { next.delete(id); } else { next.add(id); }
      return next;
    });
  }

  function toggleSeries(ids: number[]) {
    setSelectedIds((prev) => {
      const next = new Set(prev);
      const allSelected = ids.every((id) => next.has(id));
      ids.forEach((id) => { if (allSelected) { next.delete(id); } else { next.add(id); } });
      return next;
    });
  }

  function toggleAll() {
    if (selectedIds.size === allItems.length && allItems.length > 0) {
      setSelectedIds(new Set());
    } else {
      setSelectedIds(new Set(allItems.map((i) => i.id)));
    }
  }

  const clearSel = useCallback(() => setSelectedIds(new Set()), []);

  const registerLoadedFiles = useCallback((files: MediaFile[]) => {
    files.forEach((item) => fileMapRef.current.set(item.id, item));
  }, []);

  const selectedFiles = useMemo(
    () => [...selectedIds].map((id) => fileMapRef.current.get(id)).filter(Boolean) as MediaFile[],
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [selectedIds, allItems],
  );

  const totalSavings = selectedFiles.reduce((s, f) => s + f.predicted_savings_bytes, 0);

  const queueMutation = useMutation({
    mutationFn: ({ ids, profileId }: { ids: number[]; profileId: number | null }) =>
      api.createJobs(ids, profileId ?? undefined),
    onSuccess: (result) => {
      toast.success(`${result.queued.length} jobs queued`);
      clearSel();
      qc.invalidateQueries({ queryKey: ['jobs'] });
      qc.invalidateQueries({ queryKey: ['candidates'] });
    },
    onError: () => toast.error('Failed to queue jobs'),
  });

  const allSelected = allItems.length > 0 && allItems.every((i) => selectedIds.has(i.id));
  const totalCount = data?.pages[0]?.total_count;

  return (
    <div className="flex flex-col min-w-0 h-screen overflow-hidden max-sm:h-full">
      <PageHeader
        title="Candidate browser"
        subtitle={
          data === undefined
            ? <Skeleton className="h-3 w-52 mt-1" />
            : totalCount != null
              ? `${formatInt(totalCount)} files · ranked by predicted savings`
              : hasNextPage
                ? `${formatInt(allItems.length)}+ files · ranked by predicted savings`
                : `${formatInt(allItems.length)} files · ranked by predicted savings`
        }
      >
        {profiles[0] && (
          <Badge variant="outline" className="sm:ml-auto self-start text-[0.82rem] font-semibold px-[13px] py-[7px] rounded-[10px] border-line bg-surface gap-1.5">
            <span className="font-mono text-[0.8rem]">Profile</span>
            {profiles.find((p) => p.is_default)?.name ?? profiles[0].name}
          </Badge>
        )}
      </PageHeader>

      <div className="border-b border-line-soft shrink-0" style={{ background: 'var(--bg)' }}>
        <div className="flex items-center gap-2 px-4 py-3 sm:px-7">
          <div className="flex-1 relative">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" className="w-[14px] h-[14px] absolute left-[11px] top-1/2 -translate-y-1/2 text-muted-dim pointer-events-none">
              <circle cx="11" cy="11" r="8"/><line x1="21" y1="21" x2="16.65" y2="16.65"/>
            </svg>
            <Input
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              placeholder="Search by title or path…"
              className="rounded-[11px] pl-[34px] text-sm"
            />
          </div>
          <div className="inline-flex bg-surface border border-line rounded-[11px] p-[3px] gap-[2px] shrink-0">
            <Button
              variant="ghost"
              size="sm"
              onClick={() => setViewRaw('flat')}
              className={cn('rounded-[8px] text-xs font-semibold h-auto py-[7px] px-[13px]', view === 'flat' ? 'bg-brand-soft text-brand hover:bg-brand-soft hover:text-brand' : 'text-muted-fg')}
            >
              Flat
            </Button>
            <Button
              variant="ghost"
              size="sm"
              onClick={() => setViewRaw('grouped')}
              className={cn('rounded-[8px] text-xs font-semibold h-auto py-[7px] px-[13px]', view === 'grouped' ? 'bg-brand-soft text-brand hover:bg-brand-soft hover:text-brand' : 'text-muted-fg')}
            >
              By series
            </Button>
          </div>
        </div>
        <div className="flex items-center gap-2 px-4 pb-3 flex-wrap sm:px-7">
          <Select value={sort} onValueChange={(v) => startTransition(() => setSortRaw(v))}>
            <SelectTrigger className="rounded-[11px] bg-surface text-sm h-auto py-[7px] gap-1 min-w-[185px]">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" className="w-[13px] h-[13px] text-muted-dim shrink-0">
                <path d="M3 8h18M6 12h12M10 16h4"/>
              </svg>
              <span className="text-xs text-muted-dim shrink-0">Sort</span>
              <span className="text-muted-dim mx-px">·</span>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {SORT_OPTIONS.map((o) => <SelectItem key={o.value} value={o.value}>{o.label}</SelectItem>)}
            </SelectContent>
          </Select>
          <FilterSelect label="Codec" value={effectiveCodec} options={codecOptions} onChange={(v) => startTransition(() => setCodec(v))} className="min-w-[120px]" />
          <FilterSelect label="Res" value={effectiveResolution} options={resolutionOptions} onChange={(v) => startTransition(() => setResolution(v))} className="min-w-[100px]" />
          <FilterSelect label="Library" value={effectiveLibrary} options={libraryOptions} onChange={(v) => startTransition(() => setLibrary(v))} className="min-w-[130px]" />
        </div>
      </div>

      <div className={cn('flex-1 overflow-hidden relative px-3 pt-3 pb-3 transition-opacity duration-150 sm:px-7', isPending && 'opacity-50')}>
        {view === 'flat' ? (
          <div className="bg-surface border border-line rounded-(--radius) overflow-hidden flex flex-col h-full">
            <div className="flex items-center text-[0.7rem] uppercase tracking-wider text-muted-fg font-bold bg-surface-2 border-b border-line shrink-0">
              <div className="w-[52px] flex justify-center py-3">
                <Checkbox
                  checked={allSelected}
                  onCheckedChange={toggleAll}
                  className="size-[17px] rounded-[5px] cursor-pointer"
                />
              </div>
              <div className="flex-1 py-3 pr-3">File</div>
              <div className="w-[64px] sm:w-[80px] py-3">Codec</div>
              <div className="hidden sm:block w-[60px] py-3">Res</div>
              <div className="hidden sm:block w-[90px] py-3 text-right pr-2">Size</div>
              <div className="w-[84px] sm:w-[110px] py-3 text-right pr-3 sm:pr-4 text-brand">Est. savings ↓</div>
            </div>

            {data === undefined ? (
              <div className="flex-1 overflow-auto">
                {Array.from({ length: 10 }).map((_, i) => (
                  <div key={i} className="flex items-center gap-0 border-b border-line-soft px-0" style={{ height: 52 }}>
                    <div className="w-[52px] flex justify-center shrink-0">
                      <Skeleton className="w-[17px] h-[17px] rounded-[5px]" />
                    </div>
                    <div className="flex-1 min-w-0 pr-3">
                      <Skeleton className="h-4 w-48 mb-1.5" />
                      <Skeleton className="h-3 w-64" />
                    </div>
                    <Skeleton className="w-[64px] sm:w-[80px] h-5 rounded-[7px] shrink-0" />
                    <Skeleton className="hidden sm:block w-[60px] h-3 shrink-0 mx-1" />
                    <Skeleton className="hidden sm:block w-[90px] h-3 shrink-0 mr-2" />
                    <Skeleton className="w-[84px] sm:w-[110px] h-4 shrink-0 mr-3 sm:mr-4" />
                  </div>
                ))}
              </div>
            ) : data !== undefined && allItems.length === 0 ? (
              <EmptyState
                className="flex-1"
                icon={
                  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" className="w-5 h-5">
                    <circle cx="11" cy="11" r="8"/><line x1="21" y1="21" x2="16.65" y2="16.65"/>
                  </svg>
                }
                title="No candidates match"
                description="Try adjusting your filters or trigger a scan to index new files."
              />
            ) : (
            <div ref={parentRef} className="flex-1 overflow-auto">
              <div style={{ height: virtualizer.getTotalSize(), position: 'relative' }}>
                {virtualItems.map((vRow) => (
                  <div
                    key={vRow.key}
                    style={{ position: 'absolute', top: vRow.start, height: vRow.size, width: '100%' }}
                  >
                    {vRow.index < allItems.length ? (
                      <MediaFlatRow
                        item={allItems[vRow.index]}
                        selected={selectedIds.has(allItems[vRow.index].id)}
                        onToggle={toggleId}
                        onOpen={(file) => router.push(BROWSE_ROUTES.FILE(file.id))}
                      />
                    ) : (
                      <div className="flex items-center justify-center h-full text-muted-dim text-sm">
                        {isFetchingNextPage ? 'Loading more…' : hasNextPage ? 'Scroll to load more' : 'End of list'}
                      </div>
                    )}
                  </div>
                ))}
              </div>
            </div>
            )}
          </div>
        ) : (
          <div className="h-full overflow-auto">
            <GroupedView
              selectedIds={selectedIds}
              onToggle={toggleId}
              onToggleSeries={toggleSeries}
              onOpen={(file) => router.push(BROWSE_ROUTES.FILE(file.id))}
              filters={filters}
              onEpisodesLoaded={registerLoadedFiles}
            />
          </div>
        )}
      </div>

      {selectedIds.size > 0 && (
        <QueueSelectionBar
          count={selectedIds.size}
          totalSavings={totalSavings}
          onClear={clearSel}
          onQueue={() => setConfirmOpen(true)}
        />
      )}

      <QueueConfirmDialog
        open={confirmOpen}
        onClose={() => setConfirmOpen(false)}
        selectedFiles={selectedFiles}
        profiles={profiles}
        subtitle="Review the selection. Nothing runs until you confirm."
        showSafetyNote
        showMoreCount
        onConfirm={async (profileId) => {
          await queueMutation.mutateAsync({ ids: [...selectedIds], profileId });
        }}
      />
    </div>
  );
}

function CandidatesSkeleton() {
  return (
    <div className="flex flex-col min-w-0 h-screen overflow-hidden max-sm:h-full">
      <div className="px-4 py-[14px] border-b border-line sm:px-7 sm:py-[18px]">
        <Skeleton className="h-7 w-48 mb-2" />
        <Skeleton className="h-3 w-52" />
      </div>
    </div>
  );
}

export default function Page() {
  return (
    <Suspense fallback={<CandidatesSkeleton />}>
      <CandidatesPage />
    </Suspense>
  );
}
