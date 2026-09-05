"use client";

import Link from "next/link";
import { SortHeaderCell } from "@/components/media/sort-header-cell";
import {
  type ColumnDef,
  columnCellClass,
  columnHeaderClass,
} from "@/lib/table-columns";
import { cn } from "@/lib/utils";

export function ColumnHeaders<T, S extends string>({
  columns,
  sortColumn,
  sortArrow,
  onSort,
}: {
  columns: readonly ColumnDef<T, S>[];
  sortColumn?: S | null;
  sortArrow?: "↑" | "↓" | null;
  onSort?: (column: S) => void;
}) {
  return columns.map((col) => {
    const sort = col.sort;
    if (!sort || !onSort) {
      return (
        <div
          key={col.id}
          className={columnHeaderClass(col)}
          data-tooltip={col.description}
        >
          {col.label}
        </div>
      );
    }
    const active = sortColumn === sort;
    return (
      <SortHeaderCell
        key={col.id}
        active={active}
        arrow={active ? (sortArrow ?? null) : null}
        onClick={() => onSort(sort)}
        className={columnHeaderClass(col)}
        align={col.align === "right" ? "right" : "left"}
        tooltip={col.description}
      >
        {col.label}
      </SortHeaderCell>
    );
  });
}

/**
 * ColumnCells renders one cell per visible column. `href` is for rows that
 * navigate from an absolutely positioned overlay link: a positioned sibling
 * paints above static cells and swallows their hover, so each cell becomes its
 * own link instead, stacked above the overlay. Rows that are themselves a link
 * must leave `href` unset — nesting anchors is invalid.
 */
export function ColumnCells<T, S extends string>({
  columns,
  item,
  href,
}: {
  columns: readonly ColumnDef<T, S>[];
  item: T;
  href?: string;
}) {
  return columns.map((col) => {
    const className = columnCellClass(col);
    const title = col.title?.(item);
    if (!href) {
      return (
        <div key={col.id} className={className} data-tooltip={title}>
          {col.cell(item)}
        </div>
      );
    }
    return (
      <Link
        key={col.id}
        href={href}
        tabIndex={-1}
        className={cn(className, "relative z-10 cursor-pointer")}
        data-tooltip={title}
      >
        {col.cell(item)}
      </Link>
    );
  });
}
