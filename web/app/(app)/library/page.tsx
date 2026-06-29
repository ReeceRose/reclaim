'use client';

import { Suspense, useCallback, useEffect, useMemo, useRef, useState, useTransition, type Dispatch, type SetStateAction } from 'react';
import { parseQueryEnum, useQueryParam, useQueryParams } from '@/hooks/use-query-params';
import { useInfiniteQuery, useMutation, useQuery, useQueryClient, useSuspenseQuery } from '@tanstack/react-query';
import { useVirtualizer } from '@tanstack/react-virtual';
import { toast } from 'sonner';
import { api, type CandidateState, type Episode, type FileFilters, type LibrarySeriesGroup, type MediaFile, type Profile } from '@/lib/api';
import { baseName, dirName, formatBytes, formatCoverage, formatInt, resolutionLabel } from '@/lib/format';
import { cn } from '@/lib/utils';
import { codecFilterOptions, libraryFilterOptions, resolutionFilterOptions } from '@/lib/filter-options';
import { useFileDetail } from '@/components/file-detail-sheet';
import { FilterSelect } from '@/components/filter-select';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Checkbox } from '@/components/ui/checkbox';
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Skeleton } from '@/components/ui/skeleton';

const PAGE_SIZE = 100;

type SortKey = 'path_asc' | 'size_desc' | 'mtime_desc' | 'codec' | 'resolution';

const SORT_OPTIONS: { value: SortKey; label: string }[] = [
  { value: 'path_asc', label: 'Path' },
  { value: 'size_desc', label: 'Largest file' },
  { value: 'mtime_desc', label: 'Recently modified' },
  { value: 'codec', label: 'Codec' },
  { value: 'resolution', label: 'Resolution' },
];

const STATE_OPTIONS: { value: CandidateState; label: string }[] = [
  { value: 'candidate', label: 'Candidate' },
  { value: 'already_hevc', label: 'Already HEVC' },
  { value: 'probe_failed', label: 'Probe failed' },
  { value: 'unknown_codec', label: 'Unknown codec' },
  { value: 'queued', label: 'Queued' },
  { value: 'completed', label: 'Completed' },
  { value: 'missing', label: 'Missing' },
];

const STATUS_OPTIONS = [
  { value: 'active', label: 'Active' },
  { value: 'missing', label: 'Missing' },
];

const CODEC_COLORS: Record<string, string> = {
  h264: 'text-gold',
  hevc: 'text-green',
  h265: 'text-green',
  mpeg2: 'text-rose',
  mpeg2video: 'text-rose',
  vc1: 'text-violet',
  av1: 'text-sky',
};

const CODEC_BORDER: Record<string, string> = {
  h264: 'border-[rgba(241,194,27,.3)] bg-[rgba(241,194,27,.1)]',
  hevc: 'border-green-soft bg-green-soft',
  h265: 'border-green-soft bg-green-soft',
  mpeg2: 'border-[rgba(255,126,182,.3)] bg-[rgba(255,126,182,.1)]',
  mpeg2video: 'border-[rgba(255,126,182,.3)] bg-[rgba(255,126,182,.1)]',
  vc1: 'border-[rgba(190,149,255,.3)] bg-[rgba(190,149,255,.1)]',
  av1: 'border-[rgba(51,177,255,.32)] bg-[rgba(51,177,255,.1)]',
};

function isQueueable(file: MediaFile): boolean {
  return file.candidate_state === 'candidate';
}

function stateLabel(state: CandidateState): string {
  return STATE_OPTIONS.find((o) => o.value === state)?.label ?? state;
}

function queueBlockReason(file: MediaFile): string {
  switch (file.candidate_state) {
    case 'already_hevc':
      return 'Already HEVC';
    case 'probe_failed':
      return 'Probe failed';
    case 'unknown_codec':
      return 'Unknown codec';
    case 'queued':
      return 'Already queued';
    case 'completed':
      return 'Already completed';
    case 'missing':
      return 'Missing from disk';
    default:
      return '';
  }
}

function CodecBadge({ codec }: { codec: string | null }) {
  if (!codec) return <Badge variant="outline" className="font-mono text-[0.7rem] rounded-[7px]">unknown</Badge>;
  const c = codec.toLowerCase();
  return (
    <Badge className={`font-mono text-[0.7rem] rounded-[7px] font-semibold ${CODEC_COLORS[c] ?? 'text-slate'} ${CODEC_BORDER[c] ?? 'border-line bg-surface-3'}`}>
      {codec}
    </Badge>
  );
}

function StateBadge({ state }: { state: CandidateState }) {
  const cls =
    state === 'candidate'
      ? 'text-brand border-brand-line bg-brand-soft'
      : state === 'already_hevc'
        ? 'text-green border-green-soft bg-green-soft'
        : state === 'probe_failed'
          ? 'text-red border-[rgba(255,120,120,.28)] bg-[rgba(255,120,120,.09)]'
          : 'text-muted-fg border-line bg-surface-3';
  return <Badge className={`text-[0.7rem] rounded-[7px] font-semibold ${cls}`}>{stateLabel(state)}</Badge>;
}

function ConfirmDialog({
  open,
  onClose,
  selectedFiles,
  profiles,
  onConfirm,
}: {
  open: boolean;
  onClose: () => void;
  selectedFiles: MediaFile[];
  profiles: Profile[];
  onConfirm: (profileId: number | null) => Promise<void>;
}) {
  const defaultProfile = profiles.find((p) => p.is_default) ?? profiles[0];
  const [profileId, setProfileId] = useState<number | null>(null);
  const [loading, setLoading] = useState(false);
  const totalSavings = selectedFiles.reduce((s, f) => s + f.predicted_savings_bytes, 0);

  async function handleConfirm() {
    setLoading(true);
    try {
      await onConfirm(profileId ?? defaultProfile?.id ?? null);
      onClose();
    } finally {
      setLoading(false);
    }
  }

  return (
    <Dialog open={open} onOpenChange={(o) => !o && onClose()}>
      <DialogContent className="max-w-[540px] p-0 overflow-hidden border-line" style={{ background: 'var(--surface)' }}>
        <DialogHeader className="px-6 pt-[22px] pb-4 border-b border-line">
          <DialogTitle className="text-[1.2rem] font-bold tracking-tight">Confirm queue</DialogTitle>
          <p className="text-[0.85rem] text-muted-fg mt-1">Only candidate-eligible files will be queued.</p>
        </DialogHeader>
        <div className="px-6 py-5 max-h-[300px] overflow-auto">
          <div className="flex gap-6 mb-[18px] flex-wrap">
            <div>
              <div className="text-[0.72rem] uppercase tracking-wider text-muted-fg">Files</div>
              <div className="text-[1.55rem] font-bold tracking-tight mt-0.5">{formatInt(selectedFiles.length)}</div>
            </div>
            <div>
              <div className="text-[0.72rem] uppercase tracking-wider text-muted-fg">Est. recoverable</div>
              <div className="text-[1.55rem] font-bold tracking-tight mt-0.5 text-brand">{formatBytes(totalSavings)}</div>
            </div>
            <div>
              <div className="text-[0.72rem] uppercase tracking-wider text-muted-fg">Profile</div>
              {profiles.length > 1 ? (
                <Select value={String(profileId ?? defaultProfile?.id ?? '')} onValueChange={(v) => setProfileId(Number(v))}>
                  <SelectTrigger className="mt-0.5 h-8 rounded-lg text-sm">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {profiles.map((p) => (
                      <SelectItem key={p.id} value={String(p.id)}>{p.name}{p.is_default ? ' (default)' : ''}</SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              ) : (
                <div className="text-[1.1rem] font-bold tracking-tight mt-0.5">{defaultProfile?.name ?? '-'}</div>
              )}
            </div>
          </div>
          {selectedFiles.slice(0, 8).map((f) => (
            <div key={f.id} className="flex justify-between gap-3 py-[7px] border-b border-line-soft last:border-b-0 text-[0.82rem]">
              <span className="truncate text-muted-fg">{baseName(f.path)}</span>
              <span className="text-brand font-medium shrink-0">-{formatBytes(f.predicted_savings_bytes)}</span>
            </div>
          ))}
        </div>
        <DialogFooter className="px-6 py-4 border-t border-line flex justify-end gap-2">
          <Button variant="outline" onClick={onClose} className="rounded-[11px]">Cancel</Button>
          <Button onClick={() => void handleConfirm()} disabled={loading || selectedFiles.length === 0} className="rounded-[11px]" style={{ background: 'linear-gradient(145deg, var(--brand), var(--brand-2))' }}>
            Queue {formatInt(selectedFiles.length)} jobs
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

function FlatRow({ item, selected, onToggle, onOpen }: {
  item: MediaFile;
  selected: boolean;
  onToggle: (id: number) => void;
  onOpen: (file: MediaFile) => void;
}) {
  const queueable = isQueueable(item);
  return (
    <div
      className={cn(
        'flex items-center gap-0 border-b border-line-soft hover:bg-surface-2 cursor-pointer transition-colors',
        selected && 'bg-brand-soft',
        item.status === 'missing' && 'opacity-70',
      )}
      style={{ height: 52 }}
      onClick={() => onOpen(item)}
    >
      <div className="w-[52px] flex justify-center shrink-0" title={queueable ? 'Queue candidate' : queueBlockReason(item)}>
        <Checkbox
          checked={selected}
          disabled={!queueable}
          onCheckedChange={() => queueable && onToggle(item.id)}
          onClick={(e) => e.stopPropagation()}
          className="size-[17px] rounded-[5px]"
        />
      </div>
      <div className="flex-1 min-w-0 pr-3">
        <div className={cn('font-semibold text-[0.88rem] truncate', item.status === 'missing' && 'line-through text-muted-fg')}>{baseName(item.path)}</div>
        <div className="text-[0.74rem] text-muted-dim truncate font-mono">{dirName(item.path)}</div>
      </div>
      <div className="w-[64px] sm:w-[80px] shrink-0"><CodecBadge codec={item.video_codec} /></div>
      <div className="hidden sm:block w-[60px] shrink-0 text-[0.82rem] text-muted-fg">{resolutionLabel(item.width, item.height)}</div>
      <div className="hidden lg:block w-[118px] shrink-0"><StateBadge state={item.candidate_state} /></div>
      <div className="hidden sm:block w-[90px] shrink-0 text-right text-[0.82rem] text-muted-fg pr-2 font-mono">{formatBytes(item.size_bytes)}</div>
      <div className="w-[84px] sm:w-[110px] shrink-0 text-right text-[0.84rem] sm:text-[0.88rem] pr-3 sm:pr-4 font-mono">
        {queueable ? <span className="text-brand font-semibold">{formatBytes(item.predicted_savings_bytes)}</span> : <span className="text-muted-dim">-</span>}
      </div>
    </div>
  );
}

function EpisodeRow(props: { ep: Episode; selected: boolean; onToggle: (id: number) => void; onOpen: (file: MediaFile) => void }) {
  return (
    <div className="pl-[42px]">
      <FlatRow item={props.ep} selected={props.selected} onToggle={props.onToggle} onOpen={props.onOpen} />
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
  const { data, isLoading } = useQuery({
    queryKey: ['library', 'grouped', 'episodes', filters, seriesTitle, season],
    queryFn: () => api.groupedFileEpisodes({ ...filters, series: seriesTitle, season }),
  });

  useEffect(() => {
    if (data?.episodes) onEpisodesLoaded(data.episodes);
  }, [data?.episodes, onEpisodesLoaded]);

  if (isLoading) {
    return (
      <div className="px-4 py-3 pl-[42px] text-sm text-muted-dim border-b border-line-soft">
        Loading episodes...
      </div>
    );
  }

  return (
    <>
      {(data?.episodes ?? []).map((ep) => (
        <EpisodeRow key={ep.id} ep={ep} selected={selectedIds.has(ep.id)} onToggle={onToggle} onOpen={onOpen} />
      ))}
    </>
  );
}

function GroupedSkeleton() {
  return (
    <div className="bg-surface border border-line rounded-(--radius) overflow-hidden">
      {[0, 1, 2, 3].map((i) => (
        <div key={i} className="flex items-center gap-[11px] px-4 py-[13px] border-b border-line-soft">
          <Skeleton className="w-[18px] h-[18px] shrink-0 rounded" />
          <div className="flex-1 min-w-0">
            <Skeleton className="h-4 w-48 mb-1.5" />
            <Skeleton className="h-3 w-64" />
          </div>
          <Skeleton className="h-4 w-20" />
        </div>
      ))}
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
  const { data } = useSuspenseQuery({
    queryKey: ['library', 'grouped', filters],
    queryFn: () => api.groupedFiles(filters),
  });
  const movieFilters = { ...filters, library_type: 'movies' as const };
  const { data: movieData, fetchNextPage, hasNextPage, isFetchingNextPage } = useInfiniteQuery({
    queryKey: ['library', 'grouped', 'movies', movieFilters],
    queryFn: ({ pageParam }: { pageParam: Record<string, number | undefined> }) =>
      api.files({ ...movieFilters, sort: 'path_asc', limit: PAGE_SIZE, ...pageParam }),
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
  const [expandedSeasons, setExpandedSeasons] = useState<Set<string>>(new Set());

  function toggleSet(key: string, setter: Dispatch<SetStateAction<Set<string>>>) {
    setter((prev) => {
      const next = new Set(prev);
      if (next.has(key)) { next.delete(key); } else { next.add(key); }
      return next;
    });
  }

  return (
    <div className="bg-surface border border-line rounded-(--radius) overflow-hidden">
      {data.series.map((s: LibrarySeriesGroup) => {
        const expanded = expandedSeries.has(s.title);
        return (
          <div key={s.title}>
            <div className="flex items-center gap-[11px] px-4 py-[13px] border-b border-line-soft hover:bg-surface-2 cursor-pointer transition-colors" onClick={() => toggleSet(s.title, setExpandedSeries)}>
              <span className={`w-[18px] h-[18px] shrink-0 grid place-items-center text-muted-fg transition-transform ${expanded ? 'rotate-90' : ''}`}>
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" className="w-3 h-3"><path d="M9 18l6-6-6-6"/></svg>
              </span>
              <div className="flex-1 min-w-0">
                <div className="font-bold text-[0.92rem] flex items-center gap-2">
                  {s.title}
                  <Badge className="text-[0.7rem] rounded text-sky border-[rgba(51,177,255,.32)] bg-[rgba(51,177,255,.1)]">TV</Badge>
                </div>
                <div className="text-[0.76rem] text-muted-fg mt-0.5">
                  {s.season_count} seasons · {formatCoverage(s.file_count, s.eligible_count)} · {formatBytes(s.total_bytes)}
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
                  return (
                    <div key={se.season}>
                      <div className="flex items-center gap-[10px] px-4 py-[9px] pl-[18px] text-[0.76rem] font-semibold text-muted-fg bg-surface-2 border-b border-line-soft cursor-pointer" onClick={() => toggleSet(seasonKey, setExpandedSeasons)}>
                        <span className={`w-[18px] h-[18px] shrink-0 grid place-items-center transition-transform ${seasonExpanded ? 'rotate-90' : ''}`}>
                          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" className="w-3 h-3"><path d="M9 18l6-6-6-6"/></svg>
                        </span>
                        Season {se.season}
                        <span className="text-muted-dim">{formatCoverage(se.file_count, se.eligible_count)}</span>
                        <span className="ml-auto text-brand font-semibold">{formatBytes(se.predicted_savings_bytes)}</span>
                      </div>
                      {seasonExpanded && <SeasonEpisodes seriesTitle={s.title} season={se.season} filters={filters} selectedIds={selectedIds} onToggle={onToggle} onOpen={onOpen} onEpisodesLoaded={onEpisodesLoaded} />}
                    </div>
                  );
                })}
              </div>
            )}
          </div>
        );
      })}
      {filters.library_type !== 'tv' && movies.length > 0 && (
        <>
          <div className="text-[0.7rem] uppercase tracking-widest text-muted-dim font-bold px-4 pt-[15px] pb-[9px] border-b border-line-soft">Movies</div>
          {movies.map((f) => <FlatRow key={f.id} item={f} selected={selectedIds.has(f.id)} onToggle={onToggle} onOpen={onOpen} />)}
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
  return (
    <Suspense fallback={<GroupedSkeleton />}>
      <GroupedContent {...props} />
    </Suspense>
  );
}

function LibraryPage() {
  const qc = useQueryClient();
  const { openFile } = useFileDetail();
  const [isPending, startTransition] = useTransition();
  const { get, set: setQuery } = useQueryParams();
  const [search, setSearch] = useState(() => get('q') ?? '');
  const [debouncedSearch, setDebouncedSearch] = useState(() => get('q') ?? '');
  const [sortRaw, setSortRaw] = useQueryParam('sort', 'path_asc');
  const sort = parseQueryEnum(sortRaw, SORT_OPTIONS.map((o) => o.value), 'path_asc');
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
    resolution_band: effectiveResolution || undefined,
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
      <div className="flex flex-col gap-2 px-4 py-[14px] border-b border-line shrink-0 sm:flex-row sm:items-center sm:gap-4 sm:px-7 sm:py-[18px]" style={{ background: 'rgba(22,22,22,.82)', backdropFilter: 'blur(10px)' }}>
        <div className="min-w-0">
          <div className="text-title font-bold tracking-tight">Library</div>
          <div className="text-[0.82rem] text-muted-fg mt-px">
            {data === undefined
              ? <Skeleton className="h-3 w-52 mt-1" />
              : totalCount != null
                ? `${formatInt(totalCount)} scanned files`
                : hasNextPage
                  ? `${formatInt(allItems.length)}+ scanned files`
                  : `${formatInt(allItems.length)} scanned files`}
          </div>
        </div>
      </div>

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
                        <FlatRow item={allItems[vRow.index]} selected={selectedIds.has(allItems[vRow.index].id)} onToggle={toggleId} onOpen={(file) => openFile(file.id, file)} />
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
            <GroupedView selectedIds={selectedIds} onToggle={toggleId} onOpen={(file) => openFile(file.id, file)} filters={filters} onEpisodesLoaded={registerLoadedFiles} />
          </div>
        )}
      </div>

      {selectedFiles.length > 0 && (
        <div className="mx-3 mb-3 flex flex-wrap items-center gap-x-4 gap-y-2 rounded-[13px] px-4 py-[13px] border border-brand-line sticky bottom-3 sm:mx-7 sm:px-[18px]" style={{ background: 'var(--surface-2)', boxShadow: '0 10px 30px rgba(0,0,0,.35)' }}>
          <div className="font-bold"><b className="text-brand">{formatInt(selectedFiles.length)}</b> selected</div>
          <div className="text-muted-fg text-[0.85rem] hidden sm:block">~ <span className="text-brand font-semibold">{formatBytes(totalSavings)}</span> estimated recoverable</div>
          <div className="ml-auto flex gap-2.5 items-center">
            <Button variant="ghost" onClick={clearSel} className="rounded-[11px] text-sm">Clear</Button>
            <Button onClick={() => setConfirmOpen(true)} className="rounded-[11px] text-sm" style={{ background: 'linear-gradient(145deg, var(--brand), var(--brand-2))', boxShadow: '0 4px 14px var(--brand-soft)' }}>Queue selected -&gt;</Button>
          </div>
        </div>
      )}

      <ConfirmDialog
        open={confirmOpen}
        onClose={() => setConfirmOpen(false)}
        selectedFiles={selectedFiles}
        profiles={profiles}
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
