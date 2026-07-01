"use client";

import { useInfiniteQuery } from "@tanstack/react-query";
import { useEffect, useMemo, useState } from "react";
import { BROWSE_ROUTES } from "@/app/(app)/browse/browse";
import { GroupedSkeleton } from "@/components/media/grouped-skeleton";
import { MediaFlatRow } from "@/components/media/media-flat-row";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { QueryErrorState } from "@/components/ui/query-error-state";
import type { IdToggleHandler } from "@/hooks/use-id-selection";
import {
  api,
  type CandidateFilters,
  type MediaFile,
  type SeriesGroup,
} from "@/lib/api";
import { formatBytes, formatCoverage } from "@/lib/format";
import { CANDIDATES_PAGE_SIZE } from "./constants";
import { SeasonEpisodes } from "./season-episodes";

export function GroupedContent({
  selectedIds,
  onToggle,
  onToggleSeries,
  filters,
  onEpisodesLoaded,
}: {
  selectedIds: Set<number>;
  onToggle: IdToggleHandler;
  onToggleSeries: (ids: number[]) => void;
  filters: CandidateFilters;
  onEpisodesLoaded: (files: MediaFile[]) => void;
}) {
  const {
    data,
    fetchNextPage: fetchNextSeries,
    hasNextPage: hasMoreSeries,
    isFetchingNextPage: isFetchingMoreSeries,
    isLoading,
    isError,
    error,
    refetch,
  } = useInfiniteQuery({
    queryKey: ["candidates", "grouped", filters],
    queryFn: ({ pageParam }: { pageParam: number }) =>
      api.groupedCandidates({
        ...filters,
        limit: CANDIDATES_PAGE_SIZE,
        offset: pageParam,
      }),
    initialPageParam: 0,
    getNextPageParam: (lastPage, allPages) => {
      const loaded = allPages.flatMap((p) => p.series).length;
      if (lastPage.total_count != null)
        return loaded < lastPage.total_count ? loaded : undefined;
      return lastPage.series.length === CANDIDATES_PAGE_SIZE
        ? loaded
        : undefined;
    },
  });
  const groupedSeries = useMemo(
    () => data?.pages.flatMap((p) => p.series) ?? [],
    [data],
  );

  const movieFilters = { ...filters, library_type: "movies" as const };
  const {
    data: movieData,
    fetchNextPage: fetchNextMovies,
    hasNextPage: hasMoreMovies,
    isFetchingNextPage: isFetchingMovies,
  } = useInfiniteQuery({
    queryKey: ["candidates", "grouped", "movies", movieFilters],
    queryFn: ({
      pageParam,
    }: {
      pageParam: Record<string, number | undefined>;
    }) =>
      api.candidates({
        ...movieFilters,
        sort: filters.sort,
        limit: CANDIDATES_PAGE_SIZE,
        ...pageParam,
      }),
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
    enabled: filters.library_type !== "tv",
  });

  const movies = useMemo(
    () => movieData?.pages.flatMap((p) => p.items) ?? [],
    [movieData],
  );
  const movieOrderedIds = useMemo(() => movies.map((f) => f.id), [movies]);

  useEffect(() => {
    if (movies.length > 0) onEpisodesLoaded(movies);
  }, [movies, onEpisodesLoaded]);

  const [expandedSeries, setExpandedSeries] = useState<Set<string>>(new Set());
  const [expandedSeasons, setExpandedSeasons] = useState<Set<string>>(
    new Set(),
  );

  function toggleSeriesExpand(title: string) {
    setExpandedSeries((prev) => {
      const next = new Set(prev);
      if (next.has(title)) {
        next.delete(title);
      } else {
        next.add(title);
      }
      return next;
    });
  }

  function toggleSeasonExpand(key: string) {
    setExpandedSeasons((prev) => {
      const next = new Set(prev);
      if (next.has(key)) {
        next.delete(key);
      } else {
        next.add(key);
      }
      return next;
    });
  }

  function seriesEpisodeIds(s: SeriesGroup): number[] {
    return s.seasons.flatMap((se) => se.episode_ids);
  }

  function seriesSelState(s: SeriesGroup): "none" | "partial" | "all" {
    const ids = seriesEpisodeIds(s);
    const selCount = ids.filter((id) => selectedIds.has(id)).length;
    if (selCount === 0) return "none";
    if (selCount === ids.length) return "all";
    return "partial";
  }

  if (isError && !data) {
    return (
      <QueryErrorState
        error={error}
        onRetry={() => void refetch()}
        title="Failed to load candidates"
      />
    );
  }

  return (
    <div className="bg-surface border border-line rounded-(--radius) overflow-hidden">
      {isLoading && groupedSeries.length === 0 ? (
        <GroupedSkeleton withCheckbox />
      ) : (
        groupedSeries.map((s) => {
          const expanded = expandedSeries.has(s.title);
          const selState = seriesSelState(s);
          const allIds = seriesEpisodeIds(s);
          return (
            <div key={s.title}>
              {/* biome-ignore lint/a11y/useSemanticElements: contains a nested Checkbox (itself a button), so this can't be a native <button> */}
              <div
                className="flex items-center gap-[11px] px-4 py-[13px] border-b border-line-soft hover:bg-surface-2 cursor-pointer transition-colors"
                role="button"
                tabIndex={0}
                onClick={() => toggleSeriesExpand(s.title)}
                onKeyDown={(e) => {
                  if (e.key === "Enter" || e.key === " ") {
                    e.preventDefault();
                    toggleSeriesExpand(s.title);
                  }
                }}
              >
                <Checkbox
                  checked={
                    selState === "all"
                      ? true
                      : selState === "partial"
                        ? "indeterminate"
                        : false
                  }
                  onCheckedChange={() => onToggleSeries(allIds)}
                  onClick={(e) => e.stopPropagation()}
                  className="size-[17px] rounded-[5px] shrink-0"
                />
                <span
                  className={`w-[18px] h-[18px] shrink-0 grid place-items-center text-muted-fg transition-transform ${expanded ? "rotate-90" : ""}`}
                >
                  <svg
                    aria-hidden="true"
                    viewBox="0 0 24 24"
                    fill="none"
                    stroke="currentColor"
                    strokeWidth="2.5"
                    className="w-3 h-3"
                  >
                    <path d="M9 18l6-6-6-6" />
                  </svg>
                </span>
                <div className="flex-1 min-w-0">
                  <div className="font-bold text-[0.92rem] flex items-center gap-2">
                    {s.title}
                    <Badge className="text-[0.7rem] rounded text-sky border-[rgba(51,177,255,.32)] bg-[rgba(51,177,255,.1)]">
                      TV
                    </Badge>
                  </div>
                  <div className="text-[0.76rem] text-muted-fg mt-0.5">
                    {s.season_count} seasons ·{" "}
                    {formatCoverage(s.file_count, s.candidate_count)} ·{" "}
                    {formatBytes(s.total_bytes)}
                  </div>
                </div>
                <div className="text-right shrink-0">
                  <div className="text-[0.78rem] text-muted-fg">
                    {formatBytes(s.total_bytes)}
                  </div>
                  <div className="text-[0.92rem] font-semibold text-brand">
                    {formatBytes(s.predicted_savings_bytes)}
                  </div>
                </div>
              </div>

              {expanded && (
                <div style={{ background: "var(--bg)" }}>
                  {s.seasons.map((se) => {
                    const seasonKey = `${s.title}-${se.season}`;
                    const seasonExpanded = expandedSeasons.has(seasonKey);
                    const seasonSelCount = se.episode_ids.filter((id) =>
                      selectedIds.has(id),
                    ).length;
                    return (
                      <div key={se.season}>
                        {/* biome-ignore lint/a11y/useSemanticElements: contains a nested Checkbox (itself a button), so this can't be a native <button> */}
                        <div
                          className="flex items-center gap-[10px] px-4 py-[9px] pl-[18px] text-[0.76rem] font-semibold text-muted-fg bg-surface-2 border-b border-line-soft cursor-pointer"
                          role="button"
                          tabIndex={0}
                          onClick={() => toggleSeasonExpand(seasonKey)}
                          onKeyDown={(e) => {
                            if (e.key === "Enter" || e.key === " ") {
                              e.preventDefault();
                              toggleSeasonExpand(seasonKey);
                            }
                          }}
                        >
                          <span
                            className={`w-[18px] h-[18px] shrink-0 grid place-items-center transition-transform ${seasonExpanded ? "rotate-90" : ""}`}
                          >
                            <svg
                              aria-hidden="true"
                              viewBox="0 0 24 24"
                              fill="none"
                              stroke="currentColor"
                              strokeWidth="2.5"
                              className="w-3 h-3"
                            >
                              <path d="M9 18l6-6-6-6" />
                            </svg>
                          </span>
                          Season {se.season}
                          <span className="text-muted-dim">
                            {formatCoverage(se.file_count, se.candidate_count)}
                          </span>
                          {seasonSelCount > 0 && (
                            <span className="text-brand">
                              ({seasonSelCount} sel)
                            </span>
                          )}
                          <span className="ml-auto text-brand font-semibold">
                            {formatBytes(se.predicted_savings_bytes)}
                          </span>
                        </div>
                        {seasonExpanded && (
                          <SeasonEpisodes
                            seriesTitle={s.title}
                            season={se.season}
                            filters={filters}
                            selectedIds={selectedIds}
                            onToggle={onToggle}
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
        })
      )}
      {(hasMoreSeries || isFetchingMoreSeries) && (
        <div className="px-4 py-3 text-center border-t border-line-soft">
          <Button
            variant="ghost"
            size="sm"
            disabled={isFetchingMoreSeries}
            onClick={() => void fetchNextSeries()}
            className="text-sm text-muted-fg"
          >
            {isFetchingMoreSeries ? "Loading more…" : "Load more series"}
          </Button>
        </div>
      )}
      {filters.library_type !== "tv" && movies.length > 0 && (
        <>
          <div className="text-[0.7rem] uppercase tracking-widest text-muted-dim font-bold px-4 pt-[15px] pb-[9px] border-b border-line-soft">
            Movies
          </div>
          {movies.map((f, index) => (
            <MediaFlatRow
              key={f.id}
              item={f}
              index={index}
              orderedIds={movieOrderedIds}
              selected={selectedIds.has(f.id)}
              onToggle={onToggle}
              href={BROWSE_ROUTES.FILE(f.id)}
            />
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
                {isFetchingMovies ? "Loading more…" : "Load more movies"}
              </Button>
            </div>
          )}
        </>
      )}
    </div>
  );
}
