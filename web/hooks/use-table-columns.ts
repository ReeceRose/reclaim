"use client";

import { useCallback, useMemo, useSyncExternalStore } from "react";
import {
  type ColumnDef,
  columnOrder,
  isColumnVisible,
  isDefaultLayout,
  moveInOrder,
  orderedColumns,
  reorderColumns,
  setColumnVisible,
  type TableLayout,
} from "@/lib/table-columns";
import {
  clearLayout,
  readLayout,
  serverLayout,
  subscribeLayout,
  writeLayout,
} from "@/lib/table-layout-store";

export interface TableColumns<T, S extends string> {
  columns: ColumnDef<T, S>[];
  ordered: ColumnDef<T, S>[];
  layout: TableLayout;
  isDefault: boolean;
  isVisible: (id: string) => boolean;
  setVisible: (id: string, visible: boolean) => void;
  setOrder: (order: readonly string[]) => void;
  move: (id: string, toIndex: number) => void;
  reset: () => void;
}

export function useTableColumns<T, S extends string>(
  tableId: string,
  defs: readonly ColumnDef<T, S>[],
): TableColumns<T, S> {
  const subscribe = useCallback(
    (listener: () => void) => subscribeLayout(tableId, listener),
    [tableId],
  );
  const snapshot = useCallback(() => readLayout(tableId), [tableId]);
  const layout = useSyncExternalStore(subscribe, snapshot, serverLayout);

  const ordered = useMemo(() => orderedColumns(defs, layout), [defs, layout]);
  const columns = useMemo(
    () => ordered.filter((col) => isColumnVisible(col, layout)),
    [ordered, layout],
  );

  const isVisible = useCallback(
    (id: string) => {
      const col = defs.find((d) => d.id === id);
      return col ? isColumnVisible(col, layout) : false;
    },
    [defs, layout],
  );

  const setVisible = useCallback(
    (id: string, visible: boolean) => {
      writeLayout(tableId, setColumnVisible(defs, layout, id, visible));
    },
    [tableId, defs, layout],
  );

  const setOrder = useCallback(
    (order: readonly string[]) => {
      writeLayout(tableId, reorderColumns(defs, layout, order));
    },
    [tableId, defs, layout],
  );

  const move = useCallback(
    (id: string, toIndex: number) => {
      const next = moveInOrder(columnOrder(defs, layout), id, toIndex);
      writeLayout(tableId, reorderColumns(defs, layout, next));
    },
    [tableId, defs, layout],
  );

  const reset = useCallback(() => clearLayout(tableId), [tableId]);

  return {
    columns,
    ordered,
    layout,
    isDefault: isDefaultLayout(defs, layout),
    isVisible,
    setVisible,
    setOrder,
    move,
    reset,
  };
}
