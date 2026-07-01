"use client";

import { useVirtualizer } from "@tanstack/react-virtual";
import { type RefObject, useEffect } from "react";
import { BROWSE_ROUTES } from "@/app/(app)/browse/browse";
import { EmptyState } from "@/components/ui/empty-state";
import { QueryErrorState } from "@/components/ui/query-error-state";
import { Skeleton } from "@/components/ui/skeleton";
import type { CompatibilityItem } from "@/lib/api";
import { CompatibilityRow } from "./compatibility-row";

export function CompatibilityFlatList({
  parentRef,
  allItems,
  showError,
  error,
  onRetry,
  isInitialLoading,
  hasNextPage,
  isFetchingNextPage,
  onLoadMore,
}: {
  parentRef: RefObject<HTMLDivElement | null>;
  allItems: CompatibilityItem[];
  showError: boolean;
  error: unknown;
  onRetry: () => void;
  isInitialLoading: boolean;
  hasNextPage: boolean;
  isFetchingNextPage: boolean;
  onLoadMore: () => void;
}) {
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
      onLoadMore();
    }
  }, [
    virtualItems,
    allItems.length,
    hasNextPage,
    isFetchingNextPage,
    onLoadMore,
  ]);

  return (
    <div className="bg-surface border border-line rounded-(--radius) overflow-hidden flex flex-col h-full">
      <div className="flex items-center text-[0.7rem] uppercase tracking-wider text-muted-fg font-bold bg-surface-2 border-b border-line shrink-0">
        <div className="flex-1 py-3 pl-4 pr-3">File</div>
        <div className="w-[64px] sm:w-[80px] py-3">Codec</div>
        <div className="w-[56px] py-3 text-center text-brand">Risk</div>
        <div className="hidden md:block flex-1 py-3 px-2">Reasons</div>
        <div className="hidden lg:block w-[150px] py-3 pr-3">
          Recommended action
        </div>
        <div className="hidden sm:block w-[90px] py-3 text-right pr-4">
          Size
        </div>
      </div>

      {showError ? (
        <QueryErrorState
          error={error}
          onRetry={onRetry}
          title="Failed to load direct-play data"
        />
      ) : isInitialLoading ? (
        <div className="flex-1 overflow-auto">
          {Array.from({ length: 10 }).map((_, i) => (
            <div
              // biome-ignore lint/suspicious/noArrayIndexKey: reason: static, fixed-length skeleton placeholder list with no stable identity
              key={i}
              className="flex items-center gap-0 border-b border-line-soft px-0"
              style={{ height: 52 }}
            >
              <div className="flex-1 min-w-0 pl-4 pr-3">
                <Skeleton className="h-4 w-48 mb-1.5" />
                <Skeleton className="h-3 w-64" />
              </div>
              <Skeleton className="w-[64px] sm:w-[80px] h-5 rounded-[7px] shrink-0" />
              <Skeleton className="w-[56px] h-5 rounded-[7px] shrink-0 mx-2" />
              <Skeleton className="hidden md:block flex-1 h-5 rounded-[6px] shrink-0 mx-2" />
              <Skeleton className="hidden lg:block w-[150px] h-3 shrink-0 mr-3" />
              <Skeleton className="hidden sm:block w-[90px] h-3 shrink-0 mr-4" />
            </div>
          ))}
        </div>
      ) : allItems.length === 0 ? (
        <EmptyState
          className="flex-1"
          icon={
            <svg
              aria-hidden="true"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              strokeWidth="1.5"
              className="w-5 h-5"
            >
              <path d="M22 19a2 2 0 01-2 2H4a2 2 0 01-2-2V5a2 2 0 012-2h5l2 3h9a2 2 0 012 2z" />
            </svg>
          }
          title="No direct-play risks found"
          description="Every probed file is predicted to direct-play on this client profile, or your filters exclude everything."
        />
      ) : (
        <div ref={parentRef} className="flex-1 overflow-auto">
          <div
            style={{ height: virtualizer.getTotalSize(), position: "relative" }}
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
                  <CompatibilityRow
                    item={allItems[vRow.index]}
                    href={BROWSE_ROUTES.FILE(allItems[vRow.index].id)}
                  />
                ) : (
                  <div className="flex items-center justify-center h-full text-muted-dim text-sm">
                    {isFetchingNextPage
                      ? "Loading more…"
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
  );
}
