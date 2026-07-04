"use client";

import {
  useInfiniteQuery,
  useMutation,
  useQuery,
  useQueryClient,
} from "@tanstack/react-query";
import { useVirtualizer } from "@tanstack/react-virtual";
import {
  Suspense,
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
  useTransition,
} from "react";
import { toast } from "sonner";
import { BROWSE_ROUTES } from "@/app/(app)/browse/browse";
import { FilterSelect } from "@/components/filter-select";
import {
  LIBRARY_SORT_OPTIONS,
  type LibrarySortColumn,
  type LibrarySortKey,
  librarySortArrow,
  librarySortColumn,
  toggleLibrarySort,
} from "@/components/library/constants";
import { isQueueable, STATE_OPTIONS } from "@/components/media/candidate-state";
import { MediaFlatRow } from "@/components/media/media-flat-row";
import { QueueConfirmDialog } from "@/components/media/queue-confirm-dialog";
import { QueueSelectionBar } from "@/components/media/selection-bar";
import { SortHeaderCell } from "@/components/media/sort-header-cell";
import { Checkbox } from "@/components/ui/checkbox";
import { Input } from "@/components/ui/input";
import { PageHeader } from "@/components/ui/page-header";
import { QueryErrorState } from "@/components/ui/query-error-state";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Skeleton } from "@/components/ui/skeleton";
import { useIdSelection } from "@/hooks/use-id-selection";
import {
  parseQueryEnum,
  useQueryParam,
  useQueryParams,
} from "@/hooks/use-query-params";
import {
  api,
  type CandidateState,
  type FileFilters,
  type MediaFile,
} from "@/lib/api";
import {
  codecFilterOptions,
  libraryFilterOptions,
  resolutionFilterOptions,
} from "@/lib/filter-options";
import { formatInt } from "@/lib/format";
import { cn } from "@/lib/utils";

const PAGE_SIZE = 100;

function LibrarySortHeader({
  column,
  sort,
  className,
  align = "left",
  onSortChange,
  children,
}: {
  column: LibrarySortColumn;
  sort: LibrarySortKey;
  className?: string;
  align?: "left" | "right";
  onSortChange: (sort: LibrarySortKey) => void;
  children: React.ReactNode;
}) {
  const active = librarySortColumn(sort) === column;
  return (
    <SortHeaderCell
      active={active}
      arrow={active ? librarySortArrow(sort) : null}
      onClick={() => onSortChange(toggleLibrarySort(sort, column))}
      className={className}
      align={align}
    >
      {children}
    </SortHeaderCell>
  );
}

const STATUS_OPTIONS = [
  { value: "active", label: "Active" },
  { value: "missing", label: "Missing" },
];

function LibraryPage() {
  const qc = useQueryClient();
  const [isPending, startTransition] = useTransition();
  const { get, set: setQuery } = useQueryParams();
  const [search, setSearch] = useState(() => get("q") ?? "");
  const [debouncedSearch, setDebouncedSearch] = useState(() => get("q") ?? "");
  const [sortRaw, setSortRaw] = useQueryParam("sort", "mtime_desc");
  const sort = parseQueryEnum(
    sortRaw,
    LIBRARY_SORT_OPTIONS.map((o) => o.value),
    "mtime_desc",
  );
  const [codec, setCodec] = useQueryParam("codec");
  const [resolution, setResolution] = useQueryParam("res");
  const [library, setLibrary] = useQueryParam("library");
  const [status, setStatus] = useQueryParam("status");
  const [candidateState, setCandidateState] = useQueryParam("state");
  const [confirmOpen, setConfirmOpen] = useState(false);
  const fileMapRef = useRef<Map<number, MediaFile>>(new Map());
  const isSelectable = useCallback((id: number) => {
    const file = fileMapRef.current.get(id);
    return file ? isQueueable(file) : false;
  }, []);
  const {
    selectedIds,
    toggle: toggleId,
    clear: clearSel,
    toggleAll: selectAllToggle,
  } = useIdSelection({ isSelectable });
  const parentRef = useRef<HTMLDivElement>(null);

  const { data: stats } = useQuery({
    queryKey: ["stats"],
    queryFn: api.stats,
    staleTime: 30_000,
  });
  const codecOptions = useMemo(() => codecFilterOptions(stats), [stats]);
  const resolutionOptions = useMemo(
    () => resolutionFilterOptions(stats),
    [stats],
  );
  const libraryOptions = useMemo(() => libraryFilterOptions(stats), [stats]);

  const effectiveCodec =
    codec && codecOptions.some((o) => o.value === codec) ? codec : "";
  const effectiveResolution =
    resolution && resolutionOptions.some((o) => o.value === resolution)
      ? resolution
      : "";
  const effectiveLibrary =
    library && libraryOptions.some((o) => o.value === library) ? library : "";

  const qFromUrl = get("q") ?? "";
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

  const {
    data,
    fetchNextPage,
    hasNextPage,
    isFetchingNextPage,
    isError,
    error,
    refetch,
  } = useInfiniteQuery({
    queryKey: ["library", filters],
    queryFn: ({
      pageParam,
    }: {
      pageParam: Record<string, number | undefined>;
    }) => api.files({ ...filters, limit: PAGE_SIZE, ...pageParam }),
    initialPageParam: {} as Record<string, number | undefined>,
    getNextPageParam: (lastPage, allPages) =>
      lastPage.items.length === PAGE_SIZE
        ? { offset: allPages.flatMap((p) => p.items).length }
        : undefined,
    placeholderData: (prev) => prev,
  });

  const allItems = useMemo(
    () => data?.pages.flatMap((p) => p.items) ?? [],
    [data],
  );
  const orderedIds = useMemo(() => allItems.map((i) => i.id), [allItems]);
  useEffect(() => {
    allItems.forEach((item) => {
      fileMapRef.current.set(item.id, item);
    });
  }, [allItems]);

  // TanStack Virtual returns non-memoizable functions; React Compiler intentionally skips this hook.
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
    if (
      last.index >= allItems.length - 1 &&
      hasNextPage &&
      !isFetchingNextPage
    ) {
      void fetchNextPage();
    }
  }, [
    virtualItems,
    allItems.length,
    hasNextPage,
    isFetchingNextPage,
    fetchNextPage,
  ]);

  const { data: profilesData } = useQuery({
    queryKey: ["profiles"],
    queryFn: api.profiles,
    staleTime: 60_000,
  });
  const profiles = profilesData?.items ?? [];

  function toggleAll() {
    selectAllToggle(allItems.filter(isQueueable).map((i) => i.id));
  }

  const selectedFiles = useMemo(
    () =>
      [...selectedIds]
        .map((id) => fileMapRef.current.get(id))
        .filter((f): f is MediaFile => Boolean(f && isQueueable(f))),
    [selectedIds],
  );
  const totalSavings = selectedFiles.reduce(
    (s, f) => s + f.predicted_savings_bytes,
    0,
  );

  const queueMutation = useMutation({
    mutationFn: ({
      ids,
      profileId,
    }: {
      ids: number[];
      profileId: number | null;
    }) => api.createJobs(ids, profileId ?? undefined),
    onSuccess: (result) => {
      toast.success(`${result.queued.length} jobs queued`);
      clearSel();
      qc.invalidateQueries({ queryKey: ["jobs"] });
      qc.invalidateQueries({ queryKey: ["candidates"] });
      qc.invalidateQueries({ queryKey: ["library"] });
    },
    onError: () => toast.error("Failed to queue jobs"),
  });

  const eligibleLoaded = allItems.filter(isQueueable);
  const allSelected =
    eligibleLoaded.length > 0 &&
    eligibleLoaded.every((i) => selectedIds.has(i.id));
  const totalCount = data?.pages[0]?.total_count;

  return (
    <div className="flex flex-col min-w-0 h-screen overflow-hidden max-sm:h-full">
      <PageHeader
        title="Library"
        subtitle={
          data === undefined ? (
            <Skeleton className="h-3 w-52 mt-1" />
          ) : totalCount != null ? (
            `${formatInt(totalCount)} scanned files`
          ) : hasNextPage ? (
            `${formatInt(allItems.length)}+ scanned files`
          ) : (
            `${formatInt(allItems.length)} scanned files`
          )
        }
      />

      <div
        className="border-b border-line-soft shrink-0"
        style={{ background: "var(--bg)" }}
      >
        <div className="flex items-center gap-2 px-4 py-3 sm:px-7">
          <div className="flex-1 relative">
            <svg
              aria-hidden="true"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              strokeWidth="2"
              className="w-3.5 h-3.5 absolute left-3 top-1/2 -translate-y-1/2 text-muted-dim pointer-events-none"
            >
              <circle cx="11" cy="11" r="8" />
              <line x1="21" y1="21" x2="16.65" y2="16.65" />
            </svg>
            <Input
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              placeholder="Search by title or path..."
              className="rounded-xl pl-9 text-sm"
            />
          </div>
        </div>
        <div className="flex items-center gap-2 px-4 pb-3 flex-wrap sm:px-7">
          <Select
            value={sort}
            onValueChange={(v) => startTransition(() => setSortRaw(v))}
          >
            <SelectTrigger className="rounded-xl bg-surface text-sm h-auto py-2 gap-1 min-w-40">
              <span className="text-xs text-muted-dim shrink-0">Sort</span>
              <span className="text-muted-dim mx-px">·</span>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {LIBRARY_SORT_OPTIONS.map((o) => (
                <SelectItem key={o.value} value={o.value}>
                  {o.label}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
          <FilterSelect
            label="Codec"
            value={effectiveCodec}
            options={codecOptions}
            onChange={(v) => startTransition(() => setCodec(v))}
            className="min-w-32"
          />
          <FilterSelect
            label="Res"
            value={effectiveResolution}
            options={resolutionOptions}
            onChange={(v) => startTransition(() => setResolution(v))}
            className="min-w-24"
          />
          <FilterSelect
            label="Library"
            value={effectiveLibrary}
            options={libraryOptions}
            onChange={(v) => startTransition(() => setLibrary(v))}
            className="min-w-32"
          />
          <FilterSelect
            label="Status"
            value={status}
            options={STATUS_OPTIONS}
            onChange={(v) => startTransition(() => setStatus(v))}
            className="min-w-32"
          />
          <FilterSelect
            label="State"
            value={candidateState}
            options={STATE_OPTIONS}
            onChange={(v) => startTransition(() => setCandidateState(v))}
            className="min-w-36"
          />
        </div>
      </div>

      <div
        className={cn(
          "flex-1 overflow-hidden relative px-3 pt-3 pb-3 transition-opacity duration-150 sm:px-7",
          isPending && "opacity-50",
        )}
      >
        <div className="bg-surface border border-line rounded-(--radius) overflow-hidden flex flex-col h-full">
          <div className="flex items-center text-xs uppercase tracking-wider text-muted-fg font-bold bg-surface-2 border-b border-line shrink-0">
            <div className="w-14 flex justify-center py-3">
              <Checkbox
                checked={allSelected}
                onCheckedChange={toggleAll}
                className="size-4 rounded-md cursor-pointer"
              />
            </div>
            <LibrarySortHeader
              column="file"
              sort={sort}
              onSortChange={(v) => startTransition(() => setSortRaw(v))}
              className="flex-1 py-3 pr-3 min-w-0"
            >
              File
            </LibrarySortHeader>
            <LibrarySortHeader
              column="codec"
              sort={sort}
              onSortChange={(v) => startTransition(() => setSortRaw(v))}
              className="w-16 sm:w-20 py-3 shrink-0"
            >
              Codec
            </LibrarySortHeader>
            <LibrarySortHeader
              column="res"
              sort={sort}
              onSortChange={(v) => startTransition(() => setSortRaw(v))}
              className="hidden sm:flex w-16 py-3 shrink-0"
            >
              Res
            </LibrarySortHeader>
            <div className="hidden lg:block w-28 py-3 shrink-0">State</div>
            <LibrarySortHeader
              column="size"
              sort={sort}
              align="right"
              onSortChange={(v) => startTransition(() => setSortRaw(v))}
              className="hidden sm:flex w-24 py-3 pr-2 shrink-0"
            >
              Size
            </LibrarySortHeader>
            <div className="w-20 sm:w-28 py-3 text-right pr-3 sm:pr-4 shrink-0">
              Est. savings
            </div>
          </div>
          {isError && !data ? (
            <QueryErrorState
              error={error}
              onRetry={() => void refetch()}
              title="Failed to load library"
            />
          ) : data === undefined ? (
            <div className="flex-1 overflow-auto">
              {Array.from({ length: 10 }).map((_, i) => (
                <div
                  // biome-ignore lint/suspicious/noArrayIndexKey: reason: static, fixed-length skeleton placeholder list with no stable identity
                  key={i}
                  className="flex items-center border-b border-line-soft"
                  style={{ height: 52 }}
                >
                  <Skeleton className="h-4 w-64 ml-14" />
                </div>
              ))}
            </div>
          ) : allItems.length === 0 ? (
            <div className="flex-1 flex flex-col items-center justify-center gap-3 py-20 text-center">
              <div className="text-sm font-semibold text-text">
                No files match
              </div>
              <div className="text-xs text-muted-dim mt-1 max-w-3xs">
                Try adjusting your filters or trigger a scan to index new files.
              </div>
            </div>
          ) : (
            <div ref={parentRef} className="flex-1 overflow-auto">
              <div
                style={{
                  height: virtualizer.getTotalSize(),
                  position: "relative",
                }}
              >
                {virtualItems.map((vRow) => (
                  <div
                    key={vRow.key}
                    style={{
                      position: "absolute",
                      top: vRow.start,
                      height: vRow.size,
                      width: "100%",
                    }}
                  >
                    {vRow.index < allItems.length ? (
                      <MediaFlatRow
                        item={allItems[vRow.index]}
                        index={vRow.index}
                        orderedIds={orderedIds}
                        selected={selectedIds.has(allItems[vRow.index].id)}
                        onToggle={toggleId}
                        href={BROWSE_ROUTES.FILE(allItems[vRow.index].id)}
                        showState
                        gateSelection
                      />
                    ) : (
                      <div className="flex items-center justify-center h-full text-muted-dim text-sm">
                        {isFetchingNextPage
                          ? "Loading more..."
                          : hasNextPage
                            ? "Scroll to load more"
                            : "End of list"}
                      </div>
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
          await queueMutation.mutateAsync({
            ids: selectedFiles.map((f) => f.id),
            profileId,
          });
        }}
      />
    </div>
  );
}

function LibrarySkeleton() {
  return (
    <div className="flex flex-col min-w-0 h-screen overflow-hidden max-sm:h-full">
      <div className="px-4 py-3.5 border-b border-line sm:px-7 sm:py-5">
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
