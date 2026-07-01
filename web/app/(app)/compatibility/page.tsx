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
  useRef,
  useState,
  useTransition,
} from "react";
import { toast } from "sonner";
import { CompatibilityBackfillBanner } from "@/components/compatibility/compatibility-backfill-banner";
import { CompatibilityFilterBar } from "@/components/compatibility/compatibility-filter-bar";
import { CompatibilityFlatList } from "@/components/compatibility/compatibility-flat-list";
import { CompatibilityPageSkeleton } from "@/components/compatibility/compatibility-page-skeleton";
import {
  COMPATIBILITY_PAGE_SIZE,
  type CompatibilitySortKey,
  isCompatibilityQueueable,
  reasonLabel,
} from "@/components/compatibility/constants";
import { QueueConfirmDialog } from "@/components/media/queue-confirm-dialog";
import { QueueSelectionBar } from "@/components/media/selection-bar";
import { PageHeader } from "@/components/ui/page-header";
import { Skeleton } from "@/components/ui/skeleton";
import { useIdSelection } from "@/hooks/use-id-selection";
import {
  parseQueryEnum,
  useQueryParam,
  useQueryParams,
} from "@/hooks/use-query-params";
import { api, type CompatibilityItem, type MediaFile } from "@/lib/api";
import {
  codecFilterOptions,
  libraryFilterOptions,
  resolutionFilterOptions,
} from "@/lib/filter-options";
import { formatInt } from "@/lib/format";
import { cn } from "@/lib/utils";

const DEFAULT_PROFILE_FALLBACK = "apple_tv_4k";

function CompatibilityPage() {
  const qc = useQueryClient();
  const [isPending, startTransition] = useTransition();
  const { get, set: setQuery } = useQueryParams();
  const [search, setSearch] = useState(() => get("q") ?? "");
  const [debouncedSearch, setDebouncedSearch] = useState(() => get("q") ?? "");
  const [sortRaw, setSortRaw] = useQueryParam("sort", "risk_desc");
  const sort = parseQueryEnum(
    sortRaw,
    ["risk_desc", "size_desc", "mtime_desc", "library_type", "codec"],
    "risk_desc",
  ) as CompatibilitySortKey;
  const [profileParam, setProfileParam] = useQueryParam("profile");
  const [reason, setReason] = useQueryParam("reason");
  const [codec, setCodec] = useQueryParam("codec");
  const [resolution, setResolution] = useQueryParam("res");
  const [library, setLibrary] = useQueryParam("library");
  const [confirmOpen, setConfirmOpen] = useState(false);
  const itemMapRef = useRef<Map<number, CompatibilityItem>>(new Map());
  const isSelectable = useCallback((id: number) => {
    const item = itemMapRef.current.get(id);
    return item
      ? isCompatibilityQueueable(item.compatibility.recommended_action)
      : false;
  }, []);
  const {
    selectedIds,
    toggle: toggleId,
    clear: clearSel,
    toggleAll: selectAllToggle,
  } = useIdSelection({ isSelectable });
  const parentRef = useRef<HTMLDivElement>(null);

  const { data: settings } = useQuery({
    queryKey: ["settings"],
    queryFn: api.settings,
  });
  const profile =
    profileParam ||
    settings?.default_client_profile ||
    DEFAULT_PROFILE_FALLBACK;

  const { data: profilesData } = useQuery({
    queryKey: ["compatibility-profiles"],
    queryFn: api.compatibilityProfiles,
    staleTime: 5 * 60_000,
  });
  const profiles = profilesData?.profiles ?? [];

  const { data: stats } = useQuery({
    queryKey: ["stats"],
    queryFn: api.stats,
    staleTime: 30_000,
  });
  const codecOptions = codecFilterOptions(stats, { excludeUnknown: true });
  const resolutionOptions = resolutionFilterOptions(stats, {
    excludeUnknown: true,
  });
  const libraryOptions = libraryFilterOptions(stats, { excludeUnknown: true });

  const { data: compatibilityStats } = useQuery({
    queryKey: ["compatibility-stats", profile],
    queryFn: () => api.compatibilityStats(profile),
    enabled: !!profile,
    staleTime: 30_000,
  });
  const reasonOptions = (compatibilityStats?.by_reason ?? []).map((r) => ({
    value: r.code,
    label: reasonLabel(r.code),
  }));

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

  const filters = {
    client_profile: profile,
    sort,
    reason: reason || undefined,
    video_codec: codec || undefined,
    height: resolution || undefined,
    library_type: library || undefined,
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
    queryKey: ["compatibility", filters],
    queryFn: ({
      pageParam,
    }: {
      pageParam: Record<string, number | undefined>;
    }) =>
      api.compatibility({
        ...filters,
        limit: COMPATIBILITY_PAGE_SIZE,
        ...pageParam,
      }),
    initialPageParam: {} as Record<string, number | undefined>,
    getNextPageParam: (lastPage, allPages) => {
      if (lastPage.next_cursor) {
        return {
          after_risk: lastPage.next_cursor.after_risk,
          after_id: lastPage.next_cursor.after_id,
        };
      }
      if (lastPage.items.length === COMPATIBILITY_PAGE_SIZE) {
        return { offset: allPages.flatMap((p) => p.items).length };
      }
      return undefined;
    },
    enabled: !!profile,
    placeholderData: (prev) => prev,
  });

  const allItems = data?.pages.flatMap((p) => p.items) ?? [];
  const orderedIds = allItems.map((i) => i.id);
  const totalCount = data?.pages[0]?.total_count;

  useEffect(() => {
    allItems.forEach((item) => {
      itemMapRef.current.set(item.id, item);
    });
  }, [allItems]);

  // biome-ignore lint/correctness/useExhaustiveDependencies: reason: intentionally scoped to `profile` only — recommended_action (and thus queueability) can differ per profile, so switching profiles must clear whatever was selected under the old one; clearSel is stable (empty-deps useCallback).
  useEffect(() => {
    clearSel();
  }, [profile]);

  const { data: encodeProfilesData } = useQuery({
    queryKey: ["profiles"],
    queryFn: api.profiles,
    staleTime: 60_000,
  });
  const encodeProfiles = encodeProfilesData?.items ?? [];

  // QueueConfirmDialog is generic over MediaFile; CompatibilityItem is a
  // MediaFile with `compatibility` narrowed to a single verdict instead of
  // one-per-profile, so drop it rather than widen the shared dialog's type.
  const selectedFiles: MediaFile[] = [...selectedIds]
    .map((id) => itemMapRef.current.get(id))
    .filter((f): f is CompatibilityItem => Boolean(f))
    .map(({ compatibility, ...f }) => f);
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
    }) =>
      api.createJobs(ids, profileId ?? undefined, {
        source: "compatibility",
        clientProfile: profile,
      }),
    onSuccess: (result) => {
      toast.success(`${result.queued.length} jobs queued`);
      clearSel();
      qc.invalidateQueries({ queryKey: ["jobs"] });
      qc.invalidateQueries({ queryKey: ["compatibility"] });
      qc.invalidateQueries({ queryKey: ["candidates"] });
    },
    onError: () => toast.error("Failed to queue jobs"),
  });

  const eligibleLoaded = allItems.filter((i) =>
    isCompatibilityQueueable(i.compatibility.recommended_action),
  );
  const allSelected =
    eligibleLoaded.length > 0 &&
    eligibleLoaded.every((i) => selectedIds.has(i.id));

  function toggleAll() {
    selectAllToggle(eligibleLoaded.map((i) => i.id));
  }

  return (
    <div className="flex flex-col min-w-0 h-screen overflow-hidden max-sm:h-full">
      <PageHeader
        title="Direct play"
        subtitle={
          data === undefined ? (
            <Skeleton className="h-3 w-52 mt-1" />
          ) : totalCount != null ? (
            `${formatInt(totalCount)} files predicted to transcode`
          ) : hasNextPage ? (
            `${formatInt(allItems.length)}+ files predicted to transcode`
          ) : (
            `${formatInt(allItems.length)} files predicted to transcode`
          )
        }
      />

      <CompatibilityBackfillBanner />

      <CompatibilityFilterBar
        profile={profile}
        profileOptions={profiles}
        onProfileChange={(v) => startTransition(() => setProfileParam(v))}
        search={search}
        onSearchChange={setSearch}
        sort={sort}
        onSortChange={(v) => startTransition(() => setSortRaw(v))}
        reason={reason}
        reasonOptions={reasonOptions}
        onReasonChange={(v) => startTransition(() => setReason(v))}
        codec={codec}
        codecOptions={codecOptions}
        onCodecChange={(v) => startTransition(() => setCodec(v))}
        resolution={resolution}
        resolutionOptions={resolutionOptions}
        onResolutionChange={(v) => startTransition(() => setResolution(v))}
        library={library}
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
        <CompatibilityFlatList
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
          onLoadMore={() => void fetchNextPage()}
        />
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
        profiles={encodeProfiles}
        subtitle="Re-encoding to HEVC often fixes both direct-play compatibility and savings. Nothing runs until you confirm."
        showSafetyNote
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

export default function Page() {
  return (
    <Suspense fallback={<CompatibilityPageSkeleton />}>
      <CompatibilityPage />
    </Suspense>
  );
}
