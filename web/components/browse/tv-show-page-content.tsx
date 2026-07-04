"use client";

import {
  useIsMutating,
  useMutation,
  useQuery,
  useQueryClient,
} from "@tanstack/react-query";
import Image from "next/image";
import Link from "next/link";
import { useState } from "react";
import { toast } from "sonner";
import { BROWSE_ROUTES, LIBRARY_TYPE } from "@/app/(app)/browse/browse";
import { EncodeHealthBar } from "@/components/media/encode-health-bar";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { useQueryParams } from "@/hooks/use-query-params";
import { api, tmdbImageURL } from "@/lib/api";
import { formatBytes, formatInt } from "@/lib/format";
import { cn } from "@/lib/utils";
import { EditPosterDialog } from "./edit-poster-dialog";
import { TvSeasonSection } from "./tv-season-section";

export function TvShowPageContent() {
  const { get } = useQueryParams();
  const title = get("show") ?? "";
  const view = get("view") ?? undefined;
  const queryClient = useQueryClient();

  const [editOpen, setEditOpen] = useState(false);
  const [refreshing, setRefreshing] = useState(false);

  const { data: showData, isLoading: showLoading } = useQuery({
    queryKey: ["browse", "show", title],
    queryFn: async () => {
      if (!title) return null;
      const result = await api.groupedFiles({
        library_type: LIBRARY_TYPE.TV,
        search: title,
        limit: 10,
      });
      return result.series.find((s) => s.title === title) ?? null;
    },
    enabled: Boolean(title),
  });

  const { data: metadata } = useQuery({
    queryKey: ["metadata", title],
    queryFn: () => api.getMetadata(title),
    enabled: Boolean(title),
  });

  const { data: seasonsData, isLoading: seasonsLoading } = useQuery({
    queryKey: ["browse", "seasons", title],
    queryFn: () => api.groupedFileSeasons(title),
    enabled: Boolean(title) && Boolean(showData),
  });

  const isLoading = showLoading || (Boolean(showData) && seasonsLoading);

  const allEpisodeIds =
    seasonsData?.seasons.flatMap((s) => s.episode_ids) ?? [];

  const rescanMutationKey = ["rescan-files", title] as const;
  const rescanMutation = useMutation({
    mutationKey: rescanMutationKey,
    mutationFn: () => api.rescanFiles(allEpisodeIds),
    onSuccess: () => {
      toast.success(`${showData?.title ?? "Show"} rescanned`);
      void queryClient.invalidateQueries({
        queryKey: ["browse", "show", title],
      });
      void queryClient.invalidateQueries({
        queryKey: ["browse", "seasons", title],
      });
      void queryClient.invalidateQueries({
        queryKey: ["browse", "episodes", title],
      });
    },
    onError: () => toast.error("Rescan failed"),
  });
  const isRescanning = useIsMutating({ mutationKey: rescanMutationKey }) > 0;

  const posterPath = showData?.poster_path ?? metadata?.poster_path;
  const backdropPath = showData?.backdrop_path ?? metadata?.backdrop_path;
  const posterURL = tmdbImageURL(posterPath, "w342");
  const backdropURL = tmdbImageURL(backdropPath, "w1280");

  async function handleRefresh() {
    setRefreshing(true);
    try {
      await api.refreshMetadata(title, "tv");
      await queryClient.invalidateQueries({
        queryKey: ["browse", "show", title],
      });
      await queryClient.invalidateQueries({ queryKey: ["metadata", title] });
    } finally {
      setRefreshing(false);
    }
  }

  function handleSaved() {
    void queryClient.invalidateQueries({ queryKey: ["browse", "show", title] });
    void queryClient.invalidateQueries({ queryKey: ["metadata", title] });
  }

  if (!title) {
    return (
      <div className="flex flex-col items-center justify-center h-full py-32 text-center">
        <div className="text-sm font-semibold text-text">No show selected</div>
        <Link
          href={BROWSE_ROUTES.ROOT(view)}
          className="text-sm text-brand hover:underline mt-3 cursor-pointer"
        >
          ← Back to Browse
        </Link>
      </div>
    );
  }

  if (isLoading) {
    return (
      <div className="flex flex-col min-w-0">
        <div className="px-4 py-4 border-b border-line sm:px-7">
          <Skeleton className="h-3 w-16 mb-4" />
          <Skeleton className="h-8 w-64 mb-3" />
          <Skeleton className="h-3 w-48 mb-5" />
          <Skeleton className="h-1 w-full rounded-full" />
        </div>
        <div className="px-4 pt-5 sm:px-7 space-y-4">
          {Array.from({ length: 2 }).map((_, i) => (
            <div
              // biome-ignore lint/suspicious/noArrayIndexKey: reason: static, fixed-length skeleton placeholder list with no stable identity
              key={i}
              className="bg-surface border border-line rounded-xl overflow-hidden"
            >
              <div className="px-4 py-3 bg-surface-2 border-b border-line">
                <Skeleton className="h-4 w-24" />
              </div>
              <div className="px-4 py-3 space-y-3">
                {Array.from({ length: 4 }).map((_, j) => (
                  // biome-ignore lint/suspicious/noArrayIndexKey: reason: static, fixed-length skeleton placeholder list with no stable identity
                  <Skeleton key={j} className="h-4 w-full" />
                ))}
              </div>
            </div>
          ))}
        </div>
      </div>
    );
  }

  if (!showData) {
    return (
      <div className="flex flex-col items-center justify-center h-full py-32 text-center">
        <div className="text-sm font-semibold text-text">Show not found</div>
        <div className="text-xs text-muted-dim mt-2 mb-5">
          &quot;{title}&quot; could not be found in the library.
        </div>
        <Link
          href={BROWSE_ROUTES.ROOT(view)}
          className="text-sm text-brand hover:underline cursor-pointer"
        >
          ← Back to Browse
        </Link>
      </div>
    );
  }

  const activeCount = showData.file_count - showData.missing_count;
  const convertedCount = Math.max(0, activeCount - showData.eligible_count);
  const donePct =
    activeCount > 0 ? Math.round((convertedCount / activeCount) * 100) : 0;

  const genres = metadata?.genres?.length ? metadata.genres : null;

  return (
    <div className="flex flex-col min-w-0 h-screen overflow-hidden max-sm:h-full">
      <div className="relative px-4 py-4 border-b border-line shrink-0 sm:px-7 overflow-hidden">
        {backdropURL && (
          <div
            className="absolute inset-0"
            style={{
              backgroundImage: `url(${backdropURL})`,
              backgroundSize: "cover",
              backgroundPosition: "center 25%",
              filter: "blur(3px) brightness(0.28)",
              transform: "scale(1.06)",
            }}
          />
        )}
        <div
          className="absolute inset-0"
          style={{
            background: backdropURL
              ? "rgba(10,10,10,.55)"
              : "rgba(22,22,22,.82)",
            backdropFilter: "blur(10px)",
          }}
        />

        <div className="relative">
          <div className="flex items-center justify-between mb-3">
            <Link
              href={BROWSE_ROUTES.ROOT(view)}
              className="inline-flex items-center gap-1 text-xs text-muted-dim hover:text-text transition-colors cursor-pointer"
            >
              <svg
                aria-hidden="true"
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                strokeWidth="2.5"
                className="w-3 h-3"
              >
                <path d="M19 12H5M12 5l-7 7 7 7" />
              </svg>
              Browse
            </Link>

            <div className="flex items-center gap-2">
              <Button
                variant="ghost"
                size="sm"
                onClick={() => rescanMutation.mutate()}
                disabled={isRescanning || allEpisodeIds.length === 0}
                className="h-7 text-xs text-muted-fg hover:text-text gap-1.5"
                title="Re-probe every episode in this show with ffprobe"
              >
                <svg
                  aria-hidden="true"
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke="currentColor"
                  strokeWidth="2"
                  className={cn("w-3.5 h-3.5", isRescanning && "animate-spin")}
                >
                  <circle cx="11" cy="11" r="8" />
                  <path d="m21 21-4.3-4.3" />
                </svg>
                {isRescanning ? "Rescanning…" : "Rescan"}
              </Button>
              <Button
                variant="ghost"
                size="sm"
                onClick={() => void handleRefresh()}
                disabled={refreshing}
                className="h-7 text-xs text-muted-fg hover:text-text gap-1.5"
                title="Re-fetch poster, title, and year from TMDB"
              >
                <svg
                  aria-hidden="true"
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke="currentColor"
                  strokeWidth="2"
                  className={cn("w-3.5 h-3.5", refreshing && "animate-spin")}
                >
                  <path d="M1 4v6h6M23 20v-6h-6" />
                  <path d="M20.49 9A9 9 0 0 0 5.64 5.64L1 10m22 4l-4.64 4.36A9 9 0 0 1 3.51 15" />
                </svg>
                {refreshing ? "Refreshing…" : "Refresh metadata"}
              </Button>
              <Button
                variant="ghost"
                size="sm"
                onClick={() => setEditOpen(true)}
                className="h-7 text-xs text-muted-fg hover:text-text gap-1.5"
              >
                <svg
                  aria-hidden="true"
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke="currentColor"
                  strokeWidth="2"
                  className="w-3.5 h-3.5"
                >
                  <path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7" />
                  <path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z" />
                </svg>
                Edit Poster
              </Button>
            </div>
          </div>

          <div className="flex items-start gap-4">
            {posterURL && (
              <Image
                src={posterURL}
                alt={showData.title}
                width={64}
                height={96}
                className="w-16 h-auto rounded-lg shrink-0 shadow-lg hidden sm:block"
              />
            )}

            <div className="min-w-0 flex-1">
              <div className="flex items-center gap-2 mb-1 flex-wrap">
                <Badge className="text-xs rounded-md text-sky border-[rgba(51,177,255,.32)] bg-[rgba(51,177,255,.1)]">
                  TV
                </Badge>
                {metadata?.release_year && (
                  <span className="text-xs text-muted-dim">
                    {metadata.release_year}
                  </span>
                )}
                {metadata?.vote_average && metadata.vote_average > 0 && (
                  <span className="text-xs text-muted-dim">
                    ★ {metadata.vote_average.toFixed(1)}
                  </span>
                )}
                {metadata?.network && (
                  <span className="text-xs text-muted-dim">
                    {metadata.network}
                  </span>
                )}
              </div>

              <h1 className="text-2xl font-bold tracking-tight leading-tight mb-1">
                {showData.title}
              </h1>

              {metadata?.tagline && (
                <p className="text-sm text-muted-fg italic mb-1">
                  {metadata.tagline}
                </p>
              )}

              {genres && (
                <div className="flex items-center gap-1 flex-wrap mb-1">
                  {genres.map((g, i) => (
                    <span key={g} className="text-xs text-muted-dim">
                      {g}
                      {i < genres.length - 1 ? " ·" : ""}
                    </span>
                  ))}
                </div>
              )}

              {metadata?.overview && (
                <p className="text-xs text-muted-fg leading-relaxed line-clamp-2 mb-2">
                  {metadata.overview}
                </p>
              )}

              <div className="flex items-center gap-1.5 flex-wrap text-xs text-muted-fg">
                <span>
                  {showData.season_count}{" "}
                  {showData.season_count === 1 ? "season" : "seasons"}
                </span>
                <span className="text-muted-dim">·</span>
                <span>{formatInt(showData.file_count)} episodes</span>
                <span className="text-muted-dim">·</span>
                <span className="font-mono">
                  {formatBytes(showData.total_bytes)}
                </span>
                {showData.predicted_savings_bytes > 0 && (
                  <>
                    <span className="text-muted-dim">·</span>
                    <span className="text-brand font-semibold">
                      -{formatBytes(showData.predicted_savings_bytes)}{" "}
                      recoverable
                    </span>
                  </>
                )}
              </div>
            </div>
          </div>

          <div className="mt-4">
            <EncodeHealthBar
              fileCount={showData.file_count}
              eligibleCount={showData.eligible_count}
              missingCount={showData.missing_count}
            />
            <div className="flex justify-between text-xs text-muted-dim mt-1">
              <span>
                {formatInt(convertedCount)} converted · {donePct}%
              </span>
              <span>
                {showData.missing_count > 0 &&
                  `${formatInt(showData.missing_count)} missing · `}
                {formatInt(showData.eligible_count)} remaining
              </span>
            </div>
          </div>
        </div>
      </div>

      <div className="flex-1 overflow-auto px-4 pt-5 pb-8 sm:px-7">
        {(seasonsData?.seasons ?? []).map((s) => (
          <TvSeasonSection
            key={s.season}
            seriesTitle={showData.title}
            seasonData={s}
          />
        ))}
      </div>

      <EditPosterDialog
        open={editOpen}
        onOpenChange={setEditOpen}
        showTitle={title}
        currentPosterPath={posterPath}
        currentBackdropPath={backdropPath}
        onSaved={handleSaved}
      />
    </div>
  );
}
