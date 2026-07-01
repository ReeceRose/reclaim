"use client";

import { useInfiniteQuery, useQuery } from "@tanstack/react-query";
import { Suspense, useEffect, useRef, useState, useTransition } from "react";
import { CompatibilityBackfillBanner } from "@/components/compatibility/compatibility-backfill-banner";
import { CompatibilityFilterBar } from "@/components/compatibility/compatibility-filter-bar";
import { CompatibilityFlatList } from "@/components/compatibility/compatibility-flat-list";
import { CompatibilityPageSkeleton } from "@/components/compatibility/compatibility-page-skeleton";
import {
  COMPATIBILITY_PAGE_SIZE,
  type CompatibilitySortKey,
  reasonLabel,
} from "@/components/compatibility/constants";
import { PageHeader } from "@/components/ui/page-header";
import { Skeleton } from "@/components/ui/skeleton";
import {
  parseQueryEnum,
  useQueryParam,
  useQueryParams,
} from "@/hooks/use-query-params";
import { api } from "@/lib/api";
import {
  codecFilterOptions,
  libraryFilterOptions,
  resolutionFilterOptions,
} from "@/lib/filter-options";
import { formatInt } from "@/lib/format";
import { cn } from "@/lib/utils";

const DEFAULT_PROFILE_FALLBACK = "apple_tv_4k";

function CompatibilityPage() {
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
  const totalCount = data?.pages[0]?.total_count;

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
          showError={isError && !data}
          error={error}
          onRetry={() => void refetch()}
          isInitialLoading={data === undefined}
          hasNextPage={hasNextPage}
          isFetchingNextPage={isFetchingNextPage}
          onLoadMore={() => void fetchNextPage()}
        />
      </div>
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
