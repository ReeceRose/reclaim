"use client";

import { useInfiniteQuery } from "@tanstack/react-query";
import { useMemo } from "react";
import { BROWSE_ROUTES, EPISODES_PER_PAGE } from "@/app/(app)/browse/browse";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { api, type LibrarySeasonGroup } from "@/lib/api";
import { formatBytes, formatInt } from "@/lib/format";
import { TvEpisodeRow } from "./tv-episode-row";

export function TvSeasonSection({
  seriesTitle,
  seasonData,
}: {
  seriesTitle: string;
  seasonData: LibrarySeasonGroup;
}) {
  const { data, fetchNextPage, hasNextPage, isFetchingNextPage, isLoading } =
    useInfiniteQuery({
      queryKey: ["browse", "episodes", seriesTitle, seasonData.season],
      queryFn: ({ pageParam }: { pageParam: number }) =>
        api.groupedFileEpisodes({
          series: seriesTitle,
          season: seasonData.season,
          limit: EPISODES_PER_PAGE,
          offset: pageParam,
        }),
      initialPageParam: 0,
      getNextPageParam: (_lastPage, allPages) => {
        const loaded = allPages.flatMap((p) => p.episodes).length;
        return loaded < seasonData.file_count ? loaded : undefined;
      },
    });

  const episodes = useMemo(
    () => data?.pages.flatMap((p) => p.episodes) ?? [],
    [data],
  );

  return (
    <section className="mb-5">
      <div className="flex items-center gap-3 px-4 py-3 bg-surface-2 border border-line rounded-t-xl border-b-0">
        <h2 className="font-bold text-sm flex-1">Season {seasonData.season}</h2>
        <span className="text-xs text-muted-dim">
          {formatInt(seasonData.file_count)} files
        </span>
        <span className="text-muted-dim">·</span>
        <span className="font-mono text-xs text-muted-fg">
          {formatBytes(seasonData.total_bytes)}
        </span>
        {seasonData.predicted_savings_bytes > 0 && (
          <>
            <span className="text-muted-dim">·</span>
            <span className="text-xs font-semibold text-brand font-mono">
              -{formatBytes(seasonData.predicted_savings_bytes)}
            </span>
          </>
        )}
      </div>

      <div className="bg-surface border border-line rounded-b-xl overflow-hidden">
        <div className="flex items-center gap-3 px-4 py-2 border-b border-line text-xs uppercase tracking-wider text-muted-dim font-bold">
          <span className="flex-1">File</span>
          <span>Codec</span>
          <span className="hidden sm:inline w-12 text-right">Res</span>
          <span className="hidden md:inline w-16 text-right">Size</span>
          <span className="w-20 text-right">Savings</span>
        </div>

        {isLoading ? (
          <div className="px-4 py-3 space-y-2">
            {Array.from({ length: 4 }).map((_, i) => (
              // biome-ignore lint/suspicious/noArrayIndexKey: reason: static, fixed-length skeleton placeholder list with no stable identity
              <Skeleton key={i} className="h-4 w-full" />
            ))}
          </div>
        ) : (
          <>
            {episodes.map((ep) => (
              <TvEpisodeRow
                key={ep.id}
                ep={ep}
                href={BROWSE_ROUTES.FILE(ep.id)}
              />
            ))}
            {hasNextPage && (
              <div className="px-4 py-3 border-t border-line-soft">
                <Button
                  variant="ghost"
                  size="sm"
                  disabled={isFetchingNextPage}
                  onClick={() => void fetchNextPage()}
                  className="text-xs text-muted-fg"
                >
                  {isFetchingNextPage ? "Loading…" : "Load more episodes"}
                </Button>
              </div>
            )}
          </>
        )}
      </div>
    </section>
  );
}
