"use client";

import Link from "next/link";
import { ColumnCells } from "@/components/table/column-cells";
import { Checkbox } from "@/components/ui/checkbox";
import type { IdToggleHandler } from "@/hooks/use-id-selection";
import type { MediaFile } from "@/lib/api";
import type { ColumnDef } from "@/lib/table-columns";
import { cn } from "@/lib/utils";
import { isQueueable, queueBlockReason } from "./candidate-state";

export const FLAT_ROW_CLASS = "flex items-center gap-2 pr-3 sm:pr-4";

export function MediaFlatRow<S extends string>({
  item,
  index,
  orderedIds,
  columns,
  selected,
  onToggle,
  href,
  gateSelection = false,
}: {
  item: MediaFile;
  index: number;
  orderedIds: readonly number[];
  columns: readonly ColumnDef<MediaFile, S>[];
  selected: boolean;
  onToggle: IdToggleHandler;
  href: string;
  gateSelection?: boolean;
}) {
  const queueable = !gateSelection || isQueueable(item);
  const missing = item.status === "missing";
  return (
    <div
      className={cn(
        FLAT_ROW_CLASS,
        "relative border-b border-line-soft hover:bg-surface-2 transition-colors",
        selected && "bg-brand-soft",
        missing && "opacity-70",
      )}
      style={{ height: 52 }}
    >
      <Link
        href={href}
        className="absolute inset-0 cursor-pointer"
        tabIndex={-1}
        aria-hidden
      />
      <div
        className="relative z-10 w-11 flex justify-center shrink-0"
        data-tooltip={
          gateSelection
            ? queueable
              ? "Queue candidate"
              : queueBlockReason(item)
            : undefined
        }
      >
        <Checkbox
          checked={selected}
          disabled={!queueable}
          onClick={(e) => {
            e.stopPropagation();
            if (queueable) onToggle(item.id, index, e.shiftKey, orderedIds);
          }}
          className="size-4 rounded-md"
        />
      </div>
      <ColumnCells columns={columns} item={item} href={href} />
    </div>
  );
}
