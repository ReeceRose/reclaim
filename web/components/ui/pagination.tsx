"use client";

import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";

function ChevronIcon({ direction }: { direction: "left" | "right" }) {
  return (
    <svg
      aria-hidden="true"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="2.5"
      strokeLinecap="round"
      strokeLinejoin="round"
      className="w-3.5 h-3.5"
    >
      {direction === "left" ? (
        <polyline points="15 18 9 12 15 6" />
      ) : (
        <polyline points="9 18 15 12 9 6" />
      )}
    </svg>
  );
}

// pageWindow returns the page numbers to render, with -1 standing in for an
// elided run. The first and last page are always present so the ends of a long
// list stay one click away.
export function pageWindow(current: number, total: number): number[] {
  if (total <= 7) return Array.from({ length: total }, (_, i) => i + 1);

  const pages = new Set([1, total, current]);
  for (const p of [current - 1, current + 1]) {
    if (p >= 1 && p <= total) pages.add(p);
  }
  if (current <= 3) pages.add(2).add(3).add(4);
  if (current >= total - 2)
    pages
      .add(total - 1)
      .add(total - 2)
      .add(total - 3);

  const sorted = [...pages]
    .filter((p) => p >= 1 && p <= total)
    .sort((a, b) => a - b);
  const out: number[] = [];
  for (const p of sorted) {
    const prev = out.length > 0 ? out[out.length - 1] : null;
    if (prev != null && p - prev > 1) out.push(-1);
    out.push(p);
  }
  return out;
}

export function Pagination({
  page,
  pageCount,
  onPageChange,
  totalItems,
  itemLabel = "items",
  className,
}: {
  page: number;
  pageCount: number;
  onPageChange: (page: number) => void;
  totalItems?: number;
  itemLabel?: string;
  className?: string;
}) {
  if (pageCount <= 1) return null;
  const pages = pageWindow(page, pageCount);

  return (
    <nav
      aria-label="Pagination"
      className={cn(
        "flex flex-wrap items-center justify-between gap-3 mt-4",
        className,
      )}
    >
      <div className="text-xs text-muted-dim">
        Page {page} of {pageCount}
        {totalItems != null && (
          <>
            {" · "}
            {totalItems.toLocaleString()} {itemLabel}
          </>
        )}
      </div>
      <div className="flex items-center gap-1">
        <Button
          type="button"
          size="icon-sm"
          variant="outline"
          className="rounded-lg"
          disabled={page <= 1}
          onClick={() => onPageChange(page - 1)}
          aria-label="Previous page"
        >
          <ChevronIcon direction="left" />
        </Button>
        {pages.map((p, i) =>
          p === -1 ? (
            <span
              // biome-ignore lint/suspicious/noArrayIndexKey: ellipsis slots have no stable id
              key={`gap-${i}`}
              className="px-1 text-xs text-muted-dim select-none"
            >
              …
            </span>
          ) : (
            <Button
              key={p}
              type="button"
              size="icon-sm"
              variant={p === page ? "default" : "outline"}
              className={cn(
                "rounded-lg text-xs tabular-nums",
                p === page && "bg-brand-soft text-brand hover:bg-brand-soft",
              )}
              aria-current={p === page ? "page" : undefined}
              onClick={() => onPageChange(p)}
            >
              {p}
            </Button>
          ),
        )}
        <Button
          type="button"
          size="icon-sm"
          variant="outline"
          className="rounded-lg"
          disabled={page >= pageCount}
          onClick={() => onPageChange(page + 1)}
          aria-label="Next page"
        >
          <ChevronIcon direction="right" />
        </Button>
      </div>
    </nav>
  );
}
