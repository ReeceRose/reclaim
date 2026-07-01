"use client";

import { useInfiniteQuery } from "@tanstack/react-query";
import { useEffect, useMemo } from "react";
import { BROWSE_ROUTES } from "@/app/(app)/browse/browse";
import { MediaFlatRow } from "@/components/media/media-flat-row";
import { Button } from "@/components/ui/button";
import type { IdToggleHandler } from "@/hooks/use-id-selection";
import {
  api,
  type CandidateFilters,
  type Episode,
  type MediaFile,
} from "@/lib/api";
import { CANDIDATES_PAGE_SIZE } from "./constants";

function EpisodeRow(props: {
  ep: Episode;
  index: number;
  orderedIds: readonly number[];
  selected: boolean;
  onToggle: IdToggleHandler;
}) {
  return (
    <div className="pl-[42px]">
      <MediaFlatRow
        item={props.ep}
        index={props.index}
        orderedIds={props.orderedIds}
        selected={props.selected}
        onToggle={props.onToggle}
        href={BROWSE_ROUTES.FILE(props.ep.id)}
      />
    </div>
  );
}

export function SeasonEpisodes({
  seriesTitle,
  season,
  filters,
  selectedIds,
  onToggle,
  onEpisodesLoaded,
}: {
  seriesTitle: string;
  season: number;
  filters: CandidateFilters;
  selectedIds: Set<number>;
  onToggle: IdToggleHandler;
  onEpisodesLoaded: (files: MediaFile[]) => void;
}) {
  const { data, fetchNextPage, hasNextPage, isFetchingNextPage, isLoading } =
    useInfiniteQuery({
      queryKey: [
        "candidates",
        "grouped",
        "episodes",
        filters,
        seriesTitle,
        season,
      ],
      queryFn: ({ pageParam }: { pageParam: number }) =>
        api.groupedSeasonEpisodes({
          ...filters,
          series: seriesTitle,
          season,
          limit: CANDIDATES_PAGE_SIZE,
          offset: pageParam,
        }),
      initialPageParam: 0,
      getNextPageParam: (lastPage, allPages) => {
        const loaded = allPages.flatMap((p) => p.episodes).length;
        if (lastPage.total_count != null)
          return loaded < lastPage.total_count ? loaded : undefined;
        return lastPage.episodes.length === CANDIDATES_PAGE_SIZE
          ? loaded
          : undefined;
      },
    });

  const episodes = useMemo(
    () => data?.pages.flatMap((p) => p.episodes) ?? [],
    [data],
  );
  const orderedIds = useMemo(() => episodes.map((ep) => ep.id), [episodes]);

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
      {episodes.map((ep, index) => (
        <EpisodeRow
          key={ep.id}
          ep={ep}
          index={index}
          orderedIds={orderedIds}
          selected={selectedIds.has(ep.id)}
          onToggle={onToggle}
        />
      ))}
      {(hasNextPage || isFetchingNextPage) && (
        <div className="px-4 py-2 pl-[42px] border-b border-line-soft">
          <Button
            variant="ghost"
            size="sm"
            disabled={isFetchingNextPage}
            onClick={() => void fetchNextPage()}
            className="text-xs text-muted-fg"
          >
            {isFetchingNextPage ? "Loading more…" : "Load more episodes"}
          </Button>
        </div>
      )}
    </>
  );
}
