'use client';

import { Suspense, useCallback, useEffect, useMemo, useRef, useState, useTransition } from 'react';
import { parseQueryEnum, useQueryParam, useQueryParams } from '@/hooks/use-query-params';
import { useInfiniteQuery, useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useVirtualizer } from '@tanstack/react-virtual';
import { toast } from 'sonner';
import { api, type CandidateState, type FileFilters, type MediaFile } from '@/lib/api';
import { formatInt } from '@/lib/format';
import { cn } from '@/lib/utils';
import { codecFilterOptions, libraryFilterOptions, resolutionFilterOptions } from '@/lib/filter-options';
import { useRouter } from 'next/navigation';
import { BROWSE_ROUTES } from '@/app/(app)/browse/browse';
import { FilterSelect } from '@/components/filter-select';
import { Checkbox } from '@/components/ui/checkbox';
import { Input } from '@/components/ui/input';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Skeleton } from '@/components/ui/skeleton';
import { MediaFlatRow } from '@/components/media/media-flat-row';
import { QueueConfirmDialog } from '@/components/media/queue-confirm-dialog';
import { QueueSelectionBar } from '@/components/media/selection-bar';
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
