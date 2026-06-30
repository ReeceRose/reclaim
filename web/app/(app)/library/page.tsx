'use client';

import { Suspense, useCallback, useEffect, useMemo, useRef, useState, useTransition, type Dispatch, type SetStateAction } from 'react';
import { parseQueryEnum, useQueryParam, useQueryParams } from '@/hooks/use-query-params';
import { useInfiniteQuery, useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useVirtualizer } from '@tanstack/react-virtual';
import { toast } from 'sonner';
import { api, type CandidateState, type Episode, type FileFilters, type LibrarySeriesGroup, type MediaFile } from '@/lib/api';
import { formatBytes, formatCoverage, formatInt } from '@/lib/format';
import { cn } from '@/lib/utils';
import { codecFilterOptions, libraryFilterOptions, resolutionFilterOptions } from '@/lib/filter-options';
import { useRouter } from 'next/navigation';
import { BROWSE_ROUTES } from '@/app/(app)/browse/browse';
import { FilterSelect } from '@/components/filter-select';
import { Button } from '@/components/ui/button';
import { Checkbox } from '@/components/ui/checkbox';
import { Input } from '@/components/ui/input';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Skeleton } from '@/components/ui/skeleton';
import { MediaFlatRow } from '@/components/media/media-flat-row';
import { QueueConfirmDialog } from '@/components/media/queue-confirm-dialog';
import { QueueSelectionBar } from '@/components/media/selection-bar';
import { GroupedSkeleton } from '@/components/media/grouped-skeleton';
import { ShowRow } from '@/components/media/show-row';
import { STATE_OPTIONS, isQueueable } from '@/components/media/candidate-state';
import { PageHeader } from '@/components/ui/page-header';

const PAGE_SIZE = 100;

type SortKey = 'path_asc' | 'size_desc' | 'mtime_desc' | 'codec' | 'resolution';

const SORT_OPTIONS: { value: SortKey; label: string }[] = [
  { value: 'path_asc', label: 'Path' },
  { value: 'size_desc', label: 'Largest file' },
  { value: 'mtime_desc', label: 'Recently modified' },
  { value: 'codec', label: 'Codec' },
  { value: 'resolution', label: 'Resolution' },
];

const STATUS_OPTIONS = [
  { value: 'active', label: 'Active' },
  { value: 'missing', label: 'Missing' },
];

function EpisodeRow(props: { ep: Episode; selected: boolean; onToggle: (id: number) => void; onOpen: (file: MediaFile) => void }) {
  return (
    <div className="pl-[42px]">
      <MediaFlatRow item={props.ep} selected={props.selected} onToggle={props.onToggle} onOpen={props.onOpen} showState gateSelection />
    </div>
  );
}

function SeasonEpisodes({ seriesTitle, season, filters, selectedIds, onToggle, onOpen, onEpisodesLoaded }: {
  seriesTitle: string;
  season: number;
  filters: FileFilters;
  selectedIds: Set<number>;
  onToggle: (id: number) => void;
  onOpen: (file: MediaFile) => void;
  onEpisodesLoaded: (files: MediaFile[]) => void;
}) {
  const { data, fetchNextPage, hasNextPage, isFetchingNextPage, isLoading } = useInfiniteQuery({
    queryKey: ['library', 'grouped', 'episodes', filters, seriesTitle, season],
    queryFn: ({ pageParam }: { pageParam: number }) =>
      api.groupedFileEpisodes({ ...filters, series: seriesTitle, season, limit: PAGE_SIZE, offset: pageParam }),
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
        Loading episodes...
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
            {isFetchingNextPage ? 'Loading more...' : 'Load more episodes'}
          </Button>
        </div>
      )}
    </>
  );
}

function SeriesSeasonList({ seriesTitle, filters, selectedIds, onToggle, onOpen, onEpisodesLoaded }: {
  seriesTitle: string;
  filters: FileFilters;
  selectedIds: Set<number>;
  onToggle: (id: number) => void;
  onOpen: (file: MediaFile) => void;
  onEpisodesLoaded: (files: MediaFile[]) => void;
}) {
  const [expandedSeasons, setExpandedSeasons] = useState<Set<string>>(new Set());
  const { data, isLoading } = useQuery({
    queryKey: ['library', 'grouped', 'seasons', seriesTitle],
    queryFn: () => api.groupedFileSeasons(seriesTitle),
  });

  function toggleSeason(key: string) {
    setExpandedSeasons((prev) => {
      const next = new Set(prev);
      if (next.has(key)) { next.delete(key); } else { next.add(key); }
      return next;
    });
  }

  if (isLoading) {
    return (
      <div style={{ background: 'var(--bg)' }}>
        <div className="px-4 py-3 pl-[18px] text-[0.76rem] text-muted-dim border-b border-line-soft">Loading seasons...</div>
      </div>
    );
  }

  const seasons = data?.seasons ?? [];
  return (
    <div style={{ background: 'var(--bg)' }}>
      {seasons.map((se) => {
        const seasonKey = `${seriesTitle}-${se.season}`;
        const seasonExpanded = expandedSeasons.has(seasonKey);
        return (
          <div key={se.season}>
            <div className="flex items-center gap-[10px] px-4 py-[9px] pl-[18px] text-[0.76rem] font-semibold text-muted-fg bg-surface-2 border-b border-line-soft cursor-pointer" onClick={() => toggleSeason(seasonKey)}>
              <span className={`w-[18px] h-[18px] shrink-0 grid place-items-center transition-transform ${seasonExpanded ? 'rotate-90' : ''}`}>
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" className="w-3 h-3"><path d="M9 18l6-6-6-6"/></svg>
              </span>
              Season {se.season}
              <span className="text-muted-dim">{formatCoverage(se.file_count, se.eligible_count)}</span>
              <span className="ml-auto text-brand font-semibold">{formatBytes(se.predicted_savings_bytes)}</span>
            </div>
            {seasonExpanded && <SeasonEpisodes seriesTitle={seriesTitle} season={se.season} filters={filters} selectedIds={selectedIds} onToggle={onToggle} onOpen={onOpen} onEpisodesLoaded={onEpisodesLoaded} />}
          </div>
        );
      })}
    </div>
  );
}

function GroupedContent({ selectedIds, onToggle, onOpen, filters, onEpisodesLoaded }: {
  selectedIds: Set<number>;
  onToggle: (id: number) => void;
  onOpen: (file: MediaFile) => void;
  filters: FileFilters;
  onEpisodesLoaded: (files: MediaFile[]) => void;
}) {
  const { data, fetchNextPage: fetchNextSeries, hasNextPage: hasMoreSeries, isFetchingNextPage: isFetchingMoreSeries, isLoading } = useInfiniteQuery({
    queryKey: ['library', 'grouped', filters],
    queryFn: ({ pageParam }: { pageParam: number }) =>
      api.groupedFiles({ ...filters, limit: PAGE_SIZE, offset: pageParam }),
    initialPageParam: 0,
    getNextPageParam: (lastPage, allPages) => {
      const loaded = allPages.flatMap((p) => p.series).length;
      if (lastPage.total_count != null) return loaded < lastPage.total_count ? loaded : undefined;
      return lastPage.series.length === PAGE_SIZE ? loaded : undefined;
    },
  });
  const series = useMemo(() => data?.pages.flatMap((p) => p.series) ?? [], [data]);
  const movieFilters = { ...filters, library_type: 'movies' as const };
  const { data: movieData, fetchNextPage, hasNextPage, isFetchingNextPage } = useInfiniteQuery({
    queryKey: ['library', 'grouped', 'movies', movieFilters],
    queryFn: ({ pageParam }: { pageParam: Record<string, number | undefined> }) =>
      api.files({ ...movieFilters, sort: filters.sort, limit: PAGE_SIZE, ...pageParam }),
    initialPageParam: {} as Record<string, number | undefined>,
    getNextPageParam: (lastPage, allPages) => lastPage.items.length === PAGE_SIZE
      ? { offset: allPages.flatMap((p) => p.items).length }
      : undefined,
    enabled: filters.library_type !== 'tv',
  });

  const movies = useMemo(() => movieData?.pages.flatMap((p) => p.items) ?? [], [movieData]);
  useEffect(() => {
    if (movies.length > 0) onEpisodesLoaded(movies);
  }, [movies, onEpisodesLoaded]);

  const [expandedSeries, setExpandedSeries] = useState<Set<string>>(new Set());
  function toggleSet(key: string, setter: Dispatch<SetStateAction<Set<string>>>) {
    setter((prev) => {
      const next = new Set(prev);
      if (next.has(key)) { next.delete(key); } else { next.add(key); }
      return next;
    });
  }

  return (
    <div className="bg-surface border border-line rounded-(--radius) overflow-hidden">
      {isLoading && series.length === 0 ? <GroupedSkeleton /> : series.map((s: LibrarySeriesGroup) => {
        const expanded = expandedSeries.has(s.title);
        return (
          <div key={s.title}>
            <ShowRow show={s} expanded={expanded} onClick={() => toggleSet(s.title, setExpandedSeries)} />
            {expanded && (
              <SeriesSeasonList seriesTitle={s.title} filters={filters} selectedIds={selectedIds} onToggle={onToggle} onOpen={onOpen} onEpisodesLoaded={onEpisodesLoaded} />
            )}
          </div>
        );
      })}
      {(hasMoreSeries || isFetchingMoreSeries) && (
        <div className="px-4 py-3 text-center border-t border-line-soft">
          <Button variant="ghost" size="sm" disabled={isFetchingMoreSeries} onClick={() => void fetchNextSeries()} className="text-sm text-muted-fg">
            {isFetchingMoreSeries ? 'Loading more...' : 'Load more series'}
          </Button>
        </div>
      )}
      {filters.library_type !== 'tv' && movies.length > 0 && (
        <>
          <div className="text-[0.7rem] uppercase tracking-widest text-muted-dim font-bold px-4 pt-[15px] pb-[9px] border-b border-line-soft">Movies</div>
          {movies.map((f) => <MediaFlatRow key={f.id} item={f} selected={selectedIds.has(f.id)} onToggle={onToggle} onOpen={onOpen} showState gateSelection />)}
          {(hasNextPage || isFetchingNextPage) && (
            <div className="px-4 py-3 text-center border-t border-line-soft">
              <Button variant="ghost" size="sm" disabled={isFetchingNextPage} onClick={() => void fetchNextPage()} className="text-sm text-muted-fg">
                {isFetchingNextPage ? 'Loading more...' : 'Load more movies'}
              </Button>
            </div>
          )}
        </>
      )}
    </div>
  );
}

function GroupedView(props: Parameters<typeof GroupedContent>[0]) {
  return <GroupedContent {...props} />;
}

function LibraryPage() {
  const qc = useQueryClient();
  const router = useRouter();
  const [isPending, startTransition] = useTransition();
  const { get, set: setQuery } = useQueryParams();
  const [search, setSearch] = useState(() => get('q') ?? '');
  const [debouncedSearch, setDebouncedSearch] = useState(() => get('q') ?? '');
  const [sortRaw, setSortRaw] = useQueryParam('sort', 'mtime_desc');
  const sort = parseQueryEnum(sortRaw, SORT_OPTIONS.map((o) => o.value), 'mtime_desc');
  const [codec, setCodec] = useQueryParam('codec');
  const [resolution, setResolution] = useQueryParam('res');
  const [library, setLibrary] = useQueryParam('library');
  const [status, setStatus] = useQueryParam('status');
  const [candidateState, setCandidateState] = useQueryParam('state');
  const [viewRaw, setViewRaw] = useQueryParam('view', 'flat');
  const view = parseQueryEnum(viewRaw, ['flat', 'grouped'] as const, 'flat');
  const [selectedIds, setSelectedIds] = useState<Set<number>>(new Set());
  const [confirmOpen, setConfirmOpen] = useState(false);
  const fileMapRef = useRef<Map<number, MediaFile>>(new Map());
  const parentRef = useRef<HTMLDivElement>(null);

  const { data: stats } = useQuery({ queryKey: ['stats'], queryFn: api.stats, staleTime: 30_000 });
  const codecOptions = useMemo(() => codecFilterOptions(stats), [stats]);
  const resolutionOptions = useMemo(() => resolutionFilterOptions(stats), [stats]);
  const libraryOptions = useMemo(() => libraryFilterOptions(stats), [stats]);

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

  const filters: FileFilters = {
    sort,
    video_codec: effectiveCodec || undefined,
    height: effectiveResolution || undefined,
    library_type: effectiveLibrary || undefined,
    status: status || undefined,
    candidate_state: (candidateState as CandidateState) || undefined,
    search: debouncedSearch || undefined,
  };

  const { data, fetchNextPage, hasNextPage, isFetchingNextPage } = useInfiniteQuery({
    queryKey: ['library', filters],
    queryFn: ({ pageParam }: { pageParam: Record<string, number | undefined> }) =>
      api.files({ ...filters, limit: PAGE_SIZE, ...pageParam }),
    initialPageParam: {} as Record<string, number | undefined>,
    getNextPageParam: (lastPage, allPages) => lastPage.items.length === PAGE_SIZE
      ? { offset: allPages.flatMap((p) => p.items).length }
      : undefined,
    placeholderData: (prev) => prev,
  });

  const allItems = useMemo(() => data?.pages.flatMap((p) => p.items) ?? [], [data]);
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

  const { data: profilesData } = useQuery({ queryKey: ['profiles'], queryFn: api.profiles, staleTime: 60_000 });
  const profiles = profilesData?.items ?? [];

  function toggleId(id: number) {
    const file = fileMapRef.current.get(id);
    if (file && !isQueueable(file)) return;
    setSelectedIds((prev) => {
      const next = new Set(prev);
      if (next.has(id)) { next.delete(id); } else { next.add(id); }
      return next;
    });
  }

  function toggleAll() {
    const eligible = allItems.filter(isQueueable).map((i) => i.id);
    if (eligible.length > 0 && eligible.every((id) => selectedIds.has(id))) {
      setSelectedIds(new Set());
    } else {
      setSelectedIds(new Set(eligible));
    }
  }

  const clearSel = useCallback(() => setSelectedIds(new Set()), []);
  const registerLoadedFiles = useCallback((files: MediaFile[]) => {
    files.forEach((item) => fileMapRef.current.set(item.id, item));
  }, []);
  const selectedFiles = useMemo(
    () => [...selectedIds].map((id) => fileMapRef.current.get(id)).filter((f): f is MediaFile => Boolean(f && isQueueable(f))),
    [selectedIds],
  );
  const totalSavings = selectedFiles.reduce((s, f) => s + f.predicted_savings_bytes, 0);

  const queueMutation = useMutation({
    mutationFn: ({ ids, profileId }: { ids: number[]; profileId: number | null }) => api.createJobs(ids, profileId ?? undefined),
    onSuccess: (result) => {
      toast.success(`${result.queued.length} jobs queued`);
      clearSel();
      qc.invalidateQueries({ queryKey: ['jobs'] });
      qc.invalidateQueries({ queryKey: ['candidates'] });
      qc.invalidateQueries({ queryKey: ['library'] });
    },
    onError: () => toast.error('Failed to queue jobs'),
  });

  const eligibleLoaded = allItems.filter(isQueueable);
  const allSelected = eligibleLoaded.length > 0 && eligibleLoaded.every((i) => selectedIds.has(i.id));
  const totalCount = data?.pages[0]?.total_count;

  return (
    <div className="flex flex-col min-w-0 h-screen overflow-hidden max-sm:h-full">
      <PageHeader
        title="Library"
        subtitle={
          data === undefined
            ? <Skeleton className="h-3 w-52 mt-1" />
            : totalCount != null
              ? `${formatInt(totalCount)} scanned files`
              : hasNextPage
                ? `${formatInt(allItems.length)}+ scanned files`
                : `${formatInt(allItems.length)} scanned files`
        }
      />

      <div className="border-b border-line-soft shrink-0" style={{ background: 'var(--bg)' }}>
        <div className="flex items-center gap-2 px-4 py-3 sm:px-7">
          <div className="flex-1 relative">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" className="w-[14px] h-[14px] absolute left-[11px] top-1/2 -translate-y-1/2 text-muted-dim pointer-events-none">
              <circle cx="11" cy="11" r="8"/><line x1="21" y1="21" x2="16.65" y2="16.65"/>
            </svg>
            <Input value={search} onChange={(e) => setSearch(e.target.value)} placeholder="Search by title or path..." className="rounded-[11px] pl-[34px] text-sm" />
          </div>
          <div className="inline-flex bg-surface border border-line rounded-[11px] p-[3px] gap-[2px] shrink-0">
            <Button variant="ghost" size="sm" onClick={() => setViewRaw('flat')} className={cn('rounded-[8px] text-xs font-semibold h-auto py-[7px] px-[13px]', view === 'flat' ? 'bg-brand-soft text-brand hover:bg-brand-soft hover:text-brand' : 'text-muted-fg')}>Flat</Button>
            <Button variant="ghost" size="sm" onClick={() => setViewRaw('grouped')} className={cn('rounded-[8px] text-xs font-semibold h-auto py-[7px] px-[13px]', view === 'grouped' ? 'bg-brand-soft text-brand hover:bg-brand-soft hover:text-brand' : 'text-muted-fg')}>By series</Button>
          </div>
        </div>
        <div className="flex items-center gap-2 px-4 pb-3 flex-wrap sm:px-7">
          <Select value={sort} onValueChange={(v) => startTransition(() => setSortRaw(v))}>
            <SelectTrigger className="rounded-[11px] bg-surface text-sm h-auto py-[7px] gap-1 min-w-[155px]">
              <span className="text-xs text-muted-dim shrink-0">Sort</span>
              <span className="text-muted-dim mx-px">·</span>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>{SORT_OPTIONS.map((o) => <SelectItem key={o.value} value={o.value}>{o.label}</SelectItem>)}</SelectContent>
          </Select>
          <FilterSelect label="Codec" value={effectiveCodec} options={codecOptions} onChange={(v) => startTransition(() => setCodec(v))} className="min-w-[120px]" />
          <FilterSelect label="Res" value={effectiveResolution} options={resolutionOptions} onChange={(v) => startTransition(() => setResolution(v))} className="min-w-[100px]" />
          <FilterSelect label="Library" value={effectiveLibrary} options={libraryOptions} onChange={(v) => startTransition(() => setLibrary(v))} className="min-w-[130px]" />
          <FilterSelect label="Status" value={status} options={STATUS_OPTIONS} onChange={(v) => startTransition(() => setStatus(v))} className="min-w-[120px]" />
          <FilterSelect label="State" value={candidateState} options={STATE_OPTIONS} onChange={(v) => startTransition(() => setCandidateState(v))} className="min-w-[150px]" />
        </div>
      </div>

      <div className={cn('flex-1 overflow-hidden relative px-3 pt-3 pb-3 transition-opacity duration-150 sm:px-7', isPending && 'opacity-50')}>
        {view === 'flat' ? (
          <div className="bg-surface border border-line rounded-(--radius) overflow-hidden flex flex-col h-full">
            <div className="flex items-center text-[0.7rem] uppercase tracking-wider text-muted-fg font-bold bg-surface-2 border-b border-line shrink-0">
              <div className="w-[52px] flex justify-center py-3"><Checkbox checked={allSelected} onCheckedChange={toggleAll} className="size-[17px] rounded-[5px] cursor-pointer" /></div>
              <div className="flex-1 py-3 pr-3">File</div>
              <div className="w-[64px] sm:w-[80px] py-3">Codec</div>
              <div className="hidden sm:block w-[60px] py-3">Res</div>
              <div className="hidden lg:block w-[118px] py-3">State</div>
              <div className="hidden sm:block w-[90px] py-3 text-right pr-2">Size</div>
              <div className="w-[84px] sm:w-[110px] py-3 text-right pr-3 sm:pr-4 text-brand">Est. savings</div>
            </div>
            {data === undefined ? (
              <div className="flex-1 overflow-auto">
                {Array.from({ length: 10 }).map((_, i) => <div key={i} className="flex items-center border-b border-line-soft" style={{ height: 52 }}><Skeleton className="h-4 w-64 ml-14" /></div>)}
              </div>
            ) : allItems.length === 0 ? (
              <div className="flex-1 flex flex-col items-center justify-center gap-3 py-20 text-center">
                <div className="text-[0.9rem] font-semibold text-text">No files match</div>
                <div className="text-[0.78rem] text-muted-dim mt-1 max-w-[260px]">Try adjusting your filters or trigger a scan to index new files.</div>
              </div>
            ) : (
              <div ref={parentRef} className="flex-1 overflow-auto">
                <div style={{ height: virtualizer.getTotalSize(), position: 'relative' }}>
                  {virtualItems.map((vRow) => (
                    <div key={vRow.key} style={{ position: 'absolute', top: vRow.start, height: vRow.size, width: '100%' }}>
                      {vRow.index < allItems.length ? (
                        <MediaFlatRow item={allItems[vRow.index]} selected={selectedIds.has(allItems[vRow.index].id)} onToggle={toggleId} onOpen={(file) => router.push(BROWSE_ROUTES.FILE(file.id))} showState gateSelection />
                      ) : (
                        <div className="flex items-center justify-center h-full text-muted-dim text-sm">{isFetchingNextPage ? 'Loading more...' : hasNextPage ? 'Scroll to load more' : 'End of list'}</div>
                      )}
                    </div>
                  ))}
                </div>
              </div>
            )}
          </div>
        ) : (
          <div className="h-full overflow-auto">
            <GroupedView selectedIds={selectedIds} onToggle={toggleId} onOpen={(file) => router.push(BROWSE_ROUTES.FILE(file.id))} filters={filters} onEpisodesLoaded={registerLoadedFiles} />
          </div>
        )}
      </div>

      {selectedFiles.length > 0 && (
        <QueueSelectionBar
          count={selectedFiles.length}
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
        subtitle="Only candidate-eligible files will be queued."
        onConfirm={async (profileId) => {
          await queueMutation.mutateAsync({ ids: selectedFiles.map((f) => f.id), profileId });
        }}
      />
    </div>
  );
}

function LibrarySkeleton() {
  return (
    <div className="flex flex-col min-w-0 h-screen overflow-hidden max-sm:h-full">
      <div className="px-4 py-[14px] border-b border-line sm:px-7 sm:py-[18px]">
        <Skeleton className="h-7 w-32 mb-2" />
        <Skeleton className="h-3 w-44" />
      </div>
    </div>
  );
}

export default function Page() {
  return (
    <Suspense fallback={<LibrarySkeleton />}>
      <LibraryPage />
    </Suspense>
  );
}
