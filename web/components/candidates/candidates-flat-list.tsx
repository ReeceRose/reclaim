"use client";

import { useVirtualizer } from "@tanstack/react-virtual";
import { type RefObject, useEffect } from "react";
import { BROWSE_ROUTES } from "@/app/(app)/browse/browse";
import { MediaFlatRow } from "@/components/media/media-flat-row";
import { SortHeaderCell } from "@/components/media/sort-header-cell";
import { Checkbox } from "@/components/ui/checkbox";
import { EmptyState } from "@/components/ui/empty-state";
import { QueryErrorState } from "@/components/ui/query-error-state";
import { Skeleton } from "@/components/ui/skeleton";
import type { IdToggleHandler } from "@/hooks/use-id-selection";
import type { MediaFile } from "@/lib/api";
import {
  type CandidateSortColumn,
  type CandidateSortKey,
  candidateSortArrow,
  candidateSortColumn,
  toggleCandidateSort,
} from "./constants";

function CandidateSortHeader({
  column,
  sort,
  className,
  align = "left",
  onSortChange,
  children,
}: {
  column: CandidateSortColumn;
  sort: CandidateSortKey;
  className?: string;
  align?: "left" | "right";
  onSortChange: (sort: CandidateSortKey) => void;
  children: React.ReactNode;
}) {
  const active = candidateSortColumn(sort) === column;
  return (
    <SortHeaderCell
      active={active}
      arrow={active ? candidateSortArrow(sort) : null}
      onClick={() => onSortChange(toggleCandidateSort(sort, column))}
      className={className}
      align={align}
    >
      {children}
    </SortHeaderCell>
  );
}

export function CandidatesFlatList({
  sort,
  onSortChange,
  parentRef,
  allItems,
  orderedIds,
  selectedIds,
  onToggle,
  allSelected,
  onToggleAll,
  showError,
  error,
  onRetry,
  isInitialLoading,
  hasNextPage,
  isFetchingNextPage,
  onLoadMore,
}: {
  sort: CandidateSortKey;
  onSortChange: (sort: CandidateSortKey) => void;
  parentRef: RefObject<HTMLDivElement | null>;
  allItems: MediaFile[];
  orderedIds: number[];
  selectedIds: Set<number>;
  onToggle: IdToggleHandler;
  allSelected: boolean;
  onToggleAll: () => void;
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
      <div className="flex items-center text-xs uppercase tracking-wider text-muted-fg font-bold bg-surface-2 border-b border-line shrink-0">
        <div className="w-14 flex justify-center py-3">
          <Checkbox
            checked={allSelected}
            onCheckedChange={onToggleAll}
            className="size-4 rounded-md cursor-pointer"
          />
        </div>
        <CandidateSortHeader
          column="file"
          sort={sort}
          onSortChange={onSortChange}
          className="flex-1 py-3 pr-3 min-w-0"
        >
          File
        </CandidateSortHeader>
        <CandidateSortHeader
          column="codec"
          sort={sort}
          onSortChange={onSortChange}
          className="w-16 sm:w-20 py-3 shrink-0"
        >
          Codec
        </CandidateSortHeader>
        <div className="hidden sm:block w-16 py-3 shrink-0">Res</div>
        <CandidateSortHeader
          column="size"
          sort={sort}
          align="right"
          onSortChange={onSortChange}
          className="hidden sm:flex w-24 py-3 pr-2 shrink-0"
        >
          Size
        </CandidateSortHeader>
        <CandidateSortHeader
          column="savings"
          sort={sort}
          align="right"
          onSortChange={onSortChange}
          className="w-20 sm:w-28 py-3 pr-3 sm:pr-4 shrink-0"
        >
          Est. savings
        </CandidateSortHeader>
      </div>

      {showError ? (
        <QueryErrorState
          error={error}
          onRetry={onRetry}
          title="Failed to load candidates"
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
              <div className="w-14 flex justify-center shrink-0">
                <Skeleton className="w-4 h-4 rounded-md" />
              </div>
              <div className="flex-1 min-w-0 pr-3">
                <Skeleton className="h-4 w-48 mb-1.5" />
                <Skeleton className="h-3 w-64" />
              </div>
              <Skeleton className="w-16 sm:w-20 h-5 rounded-lg shrink-0" />
              <Skeleton className="hidden sm:block w-16 h-3 shrink-0 mx-1" />
              <Skeleton className="hidden sm:block w-24 h-3 shrink-0 mr-2" />
              <Skeleton className="w-20 sm:w-28 h-4 shrink-0 mr-3 sm:mr-4" />
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
              <circle cx="11" cy="11" r="8" />
              <line x1="21" y1="21" x2="16.65" y2="16.65" />
            </svg>
          }
          title="No candidates match"
          description="Try adjusting your filters or trigger a scan to index new files."
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
                  <MediaFlatRow
                    item={allItems[vRow.index]}
                    index={vRow.index}
                    orderedIds={orderedIds}
                    selected={selectedIds.has(allItems[vRow.index].id)}
                    onToggle={onToggle}
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
