import type { ReactNode } from "react";
import { cn } from "@/lib/utils";

export type ColumnBreakpoint = "sm" | "md" | "lg" | "xl";

export interface ColumnDef<T, S extends string = never> {
  id: string;
  label: string;
  width: string;
  breakpoint?: ColumnBreakpoint;
  align?: "left" | "right";
  locked?: boolean;
  defaultVisible?: boolean;
  sort?: S;
  cell: (item: T) => ReactNode;
  cellClassName?: string;
  headerClassName?: string;
  title?: (item: T) => string | undefined;
  description?: string;
}

export interface TableLayout {
  order: string[];
  visible: Record<string, boolean>;
}

export const EMPTY_LAYOUT: TableLayout = { order: [], visible: {} };

const HEADER_AT: Record<ColumnBreakpoint, string> = {
  sm: "hidden sm:flex",
  md: "hidden md:flex",
  lg: "hidden lg:flex",
  xl: "hidden xl:flex",
};

const CELL_AT: Record<ColumnBreakpoint, string> = {
  sm: "hidden sm:block",
  md: "hidden md:block",
  lg: "hidden lg:block",
  xl: "hidden xl:block",
};

const BREAKPOINT_HINT: Record<ColumnBreakpoint, string> = {
  sm: "Hidden on phones",
  md: "Hidden on narrow windows",
  lg: "Only shown on wide windows",
  xl: "Only shown on very wide windows",
};

export function columnHeaderClass<T, S extends string>(
  col: ColumnDef<T, S>,
): string {
  return cn(
    "min-w-0",
    col.width,
    col.breakpoint ? HEADER_AT[col.breakpoint] : "flex",
    col.align === "right" ? "justify-end" : "justify-start",
    col.headerClassName,
  );
}

export function columnCellClass<T, S extends string>(
  col: ColumnDef<T, S>,
): string {
  return cn(
    "min-w-0",
    col.width,
    col.breakpoint ? CELL_AT[col.breakpoint] : "block",
    col.align === "right" && "text-right",
    col.cellClassName,
  );
}

export function columnHint<T, S extends string>(
  col: ColumnDef<T, S>,
): string | undefined {
  return col.breakpoint ? BREAKPOINT_HINT[col.breakpoint] : undefined;
}

export function columnOrder<T, S extends string>(
  defs: readonly ColumnDef<T, S>[],
  layout: TableLayout,
): string[] {
  const known = new Set(defs.map((d) => d.id));
  const placed = layout.order.filter((id) => known.has(id));
  const seen = new Set(placed);
  const order = [...placed];
  defs.forEach((def, i) => {
    if (seen.has(def.id)) return;
    let at = 0;
    for (let j = i - 1; j >= 0; j--) {
      const k = order.indexOf(defs[j].id);
      if (k >= 0) {
        at = k + 1;
        break;
      }
    }
    order.splice(at, 0, def.id);
    seen.add(def.id);
  });
  return order;
}

export function orderedColumns<T, S extends string>(
  defs: readonly ColumnDef<T, S>[],
  layout: TableLayout,
): ColumnDef<T, S>[] {
  const byId = new Map(defs.map((d) => [d.id, d]));
  return columnOrder(defs, layout)
    .map((id) => byId.get(id))
    .filter((d): d is ColumnDef<T, S> => d !== undefined);
}

export function isColumnVisible<T, S extends string>(
  col: ColumnDef<T, S>,
  layout: TableLayout,
): boolean {
  if (col.locked) return true;
  const saved = layout.visible[col.id];
  return saved === undefined ? col.defaultVisible !== false : saved;
}

export function isDefaultLayout<T, S extends string>(
  defs: readonly ColumnDef<T, S>[],
  layout: TableLayout,
): boolean {
  const order = columnOrder(defs, layout);
  if (order.some((id, i) => defs[i]?.id !== id)) return false;
  return defs.every(
    (def) => isColumnVisible(def, layout) === (def.defaultVisible !== false),
  );
}

export function setColumnVisible<T, S extends string>(
  defs: readonly ColumnDef<T, S>[],
  layout: TableLayout,
  id: string,
  visible: boolean,
): TableLayout {
  return {
    order: columnOrder(defs, layout),
    visible: { ...layout.visible, [id]: visible },
  };
}

export function reorderColumns<T, S extends string>(
  defs: readonly ColumnDef<T, S>[],
  layout: TableLayout,
  order: readonly string[],
): TableLayout {
  const known = new Set(defs.map((d) => d.id));
  return {
    ...layout,
    order: columnOrder(defs, {
      ...layout,
      order: order.filter((id) => known.has(id)),
    }),
  };
}

export function moveInOrder(
  order: readonly string[],
  id: string,
  toIndex: number,
): string[] {
  const from = order.indexOf(id);
  if (from < 0) return [...order];
  const next = [...order];
  next.splice(from, 1);
  next.splice(Math.max(0, Math.min(next.length, toIndex)), 0, id);
  return next;
}
