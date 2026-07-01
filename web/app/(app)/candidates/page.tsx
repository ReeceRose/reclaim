"use client";

import {
  useInfiniteQuery,
  useMutation,
  useQuery,
  useQueryClient,
} from "@tanstack/react-query";
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
import { CandidatesFilterBar } from "@/components/candidates/candidates-filter-bar";
import { CandidatesFlatList } from "@/components/candidates/candidates-flat-list";
import { CandidatesPageSkeleton } from "@/components/candidates/candidates-page-skeleton";
import {
  CANDIDATE_SORT_OPTIONS,
  CANDIDATES_PAGE_SIZE,
} from "@/components/candidates/constants";
import { GroupedContent } from "@/components/candidates/grouped-content";
import { QueueConfirmDialog } from "@/components/media/queue-confirm-dialog";
import { QueueSelectionBar } from "@/components/media/selection-bar";
import { Badge } from "@/components/ui/badge";
import { PageHeader } from "@/components/ui/page-header";
import { Skeleton } from "@/components/ui/skeleton";
import { useIdSelection } from "@/hooks/use-id-selection";
import {
  parseQueryEnum,
  useQueryParam,
  useQueryParams,
} from "@/hooks/use-query-params";
import { api, type CandidateFilters, type MediaFile } from "@/lib/api";
import {
  codecFilterOptions,
  libraryFilterOptions,
  resolutionFilterOptions,
} from "@/lib/filter-options";
import { formatInt } from "@/lib/format";
import { cn } from "@/lib/utils";

function CandidatesPage() {
  const qc = useQueryClient();
  const [isPending, startTransition] = useTransition();
  const { get, set: setQuery } = useQueryParams();
  const [search, setSearch] = useState(() => get("q") ?? "");
  const [debouncedSearch, setDebouncedSearch] = useState(() => get("q") ?? "");
  const [sortRaw, setSortRaw] = useQueryParam("sort", "savings_desc");
  const sort = parseQueryEnum(
    sortRaw,
    CANDIDATE_SORT_OPTIONS.map((o) => o.value),
    "savings_desc",
  );
  const [codec, setCodec] = useQueryParam("codec");
  const [resolution, setResolution] = useQueryParam("res");
  const [library, setLibrary] = useQueryParam("library");
  const [viewRaw, setViewRaw] = useQueryParam("view", "flat");
  const view = parseQueryEnum(viewRaw, ["flat", "grouped"] as const, "flat");
  const {
    selectedIds,
    setSelectedIds,
    toggle: toggleId,
    clear: clearSel,
    toggleAll: selectAllToggle,
  } = useIdSelection();
  const [confirmOpen, setConfirmOpen] = useState(false);
  const fileMapRef = useRef<Map<number, MediaFile>>(new Map());
  const parentRef = useRef<HTMLDivElement>(null);

  const { data: stats } = useQuery({
    queryKey: ["stats"],
    queryFn: api.stats,
    staleTime: 30_000,
  });
  const codecOptions = useMemo(
    () =>
      codecFilterOptions(stats, { excludeHEVC: true, excludeUnknown: true }),
    [stats],
  );
  const resolutionOptions = useMemo(
    () => resolutionFilterOptions(stats, { excludeUnknown: true }),
    [stats],
  );
  const libraryOptions = useMemo(
    () => libraryFilterOptions(stats, { excludeUnknown: true }),
    [stats],
  );

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
    // Sync local search input when the URL changes (back/forward navigation)
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

  const {
    data,
    fetchNextPage,
    hasNextPage,
    isFetchingNextPage,
    isError,
    error,
    refetch,
  } = useInfiniteQuery({
    queryKey: ["candidates", filters],
    queryFn: ({
      pageParam,
    }: {
      pageParam: Record<string, number | undefined>;
    }) =>
      api.candidates({ ...filters, limit: CANDIDATES_PAGE_SIZE, ...pageParam }),
    initialPageParam: {} as Record<string, number | undefined>,
    getNextPageParam: (lastPage, allPages) => {
      if (lastPage.next_cursor) {
        return {
          after_savings: lastPage.next_cursor.after_savings,
          after_id: lastPage.next_cursor.after_id,
        };
      }
      if (lastPage.items.length === CANDIDATES_PAGE_SIZE) {
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
  const orderedIds = useMemo(() => allItems.map((i) => i.id), [allItems]);

  useEffect(() => {
    allItems.forEach((item) => {
      fileMapRef.current.set(item.id, item);
    });
  }, [allItems]);

  const { data: profilesData } = useQuery({
    queryKey: ["profiles"],
    queryFn: api.profiles,
    staleTime: 60_000,
  });
  const profiles = profilesData?.items ?? [];

  function toggleSeries(ids: number[]) {
    setSelectedIds((prev) => {
      const next = new Set(prev);
      const allSelected = ids.every((id) => next.has(id));
      ids.forEach((id) => {
        if (allSelected) {
          next.delete(id);
        } else {
          next.add(id);
        }
      });
      return next;
    });
  }

  function toggleAll() {
    selectAllToggle(allItems.map((i) => i.id));
  }

  const registerLoadedFiles = useCallback((files: MediaFile[]) => {
    files.forEach((item) => {
      fileMapRef.current.set(item.id, item);
    });
  }, []);

  // biome-ignore lint/correctness/useExhaustiveDependencies: allItems must stay listed so this recomputes once fileMapRef is repopulated after new pages load
  const selectedFiles = useMemo(
    // fileMapRef is a side cache for grouped-view selections, read directly rather than via a reactive value
    () =>
      [...selectedIds]
        .map((id) => fileMapRef.current.get(id))
        .filter(Boolean) as MediaFile[],
    [selectedIds, allItems],
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
    },
    onError: () => toast.error("Failed to queue jobs"),
  });

  const allSelected =
    allItems.length > 0 && allItems.every((i) => selectedIds.has(i.id));
  const totalCount = data?.pages[0]?.total_count;

  const handleLoadMore = useCallback(() => {
    void fetchNextPage();
  }, [fetchNextPage]);

  return (
    <div className="flex flex-col min-w-0 h-screen overflow-hidden max-sm:h-full">
      <PageHeader
        title="Candidate browser"
        subtitle={
          data === undefined ? (
            <Skeleton className="h-3 w-52 mt-1" />
          ) : totalCount != null ? (
            `${formatInt(totalCount)} files · ranked by predicted savings`
          ) : hasNextPage ? (
            `${formatInt(allItems.length)}+ files · ranked by predicted savings`
          ) : (
            `${formatInt(allItems.length)} files · ranked by predicted savings`
          )
        }
      >
        {profiles[0] && (
          <Badge
            variant="outline"
            className="sm:ml-auto self-start text-[0.82rem] font-semibold px-[13px] py-[7px] rounded-[10px] border-line bg-surface gap-1.5"
          >
            <span className="font-mono text-[0.8rem]">Profile</span>
            {profiles.find((p) => p.is_default)?.name ?? profiles[0].name}
          </Badge>
        )}
      </PageHeader>

      <CandidatesFilterBar
        search={search}
        onSearchChange={setSearch}
        view={view}
        onViewChange={setViewRaw}
        sort={sort}
        onSortChange={(v) => startTransition(() => setSortRaw(v))}
        codec={effectiveCodec}
        codecOptions={codecOptions}
        onCodecChange={(v) => startTransition(() => setCodec(v))}
        resolution={effectiveResolution}
        resolutionOptions={resolutionOptions}
        onResolutionChange={(v) => startTransition(() => setResolution(v))}
        library={effectiveLibrary}
        libraryOptions={libraryOptions}
        onLibraryChange={(v) => startTransition(() => setLibrary(v))}
        isPending={isPending}
      />

      <div
        className={cn(
          "flex-1 overflow-hidden relative px-3 pt-3 pb-3 transition-opacity duration-150 sm:px-7",
          isPending && "opacity-50",
        )}
      >
        {view === "flat" ? (
          <CandidatesFlatList
            parentRef={parentRef}
            allItems={allItems}
            orderedIds={orderedIds}
            selectedIds={selectedIds}
            onToggle={toggleId}
            allSelected={allSelected}
            onToggleAll={toggleAll}
            showError={isError && !data}
            error={error}
            onRetry={() => void refetch()}
            isInitialLoading={data === undefined}
            hasNextPage={hasNextPage}
            isFetchingNextPage={isFetchingNextPage}
            onLoadMore={handleLoadMore}
          />
        ) : (
          <div className="h-full overflow-auto">
            <GroupedContent
              selectedIds={selectedIds}
              onToggle={toggleId}
              onToggleSeries={toggleSeries}
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

export default function Page() {
  return (
    <Suspense fallback={<CandidatesPageSkeleton />}>
      <CandidatesPage />
    </Suspense>
  );
}
