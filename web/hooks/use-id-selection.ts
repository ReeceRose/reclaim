"use client";

import { useCallback, useRef, useState } from "react";

type UseIdSelectionOptions = {
  isSelectable?: (id: number) => boolean;
};

export function useIdSelection(options?: UseIdSelectionOptions) {
  const [selectedIds, setSelectedIds] = useState<Set<number>>(new Set());
  const anchorIdRef = useRef<number | null>(null);
  const isSelectable = options?.isSelectable;

  const toggle = useCallback(
    (
      id: number,
      index: number,
      shiftKey: boolean,
      orderedIds: readonly number[],
    ) => {
      if (isSelectable && !isSelectable(id)) return;

      if (shiftKey && anchorIdRef.current !== null) {
        const anchorIndex = orderedIds.indexOf(anchorIdRef.current);
        if (anchorIndex !== -1) {
          const start = Math.min(anchorIndex, index);
          const end = Math.max(anchorIndex, index);
          setSelectedIds((prev) => {
            const next = new Set(prev);
            for (let i = start; i <= end; i++) {
              const rangeId = orderedIds[i];
              if (!isSelectable || isSelectable(rangeId)) next.add(rangeId);
            }
            return next;
          });
          return;
        }
      }

      setSelectedIds((prev) => {
        const next = new Set(prev);
        if (next.has(id)) next.delete(id);
        else next.add(id);
        return next;
      });
      anchorIdRef.current = id;
    },
    [isSelectable],
  );

  const clear = useCallback(() => {
    setSelectedIds(new Set());
    anchorIdRef.current = null;
  }, []);

  const toggleAll = useCallback((ids: number[]) => {
    setSelectedIds((prev) => {
      const allSelected = ids.length > 0 && ids.every((id) => prev.has(id));
      if (allSelected) {
        anchorIdRef.current = null;
        return new Set();
      }
      anchorIdRef.current = null;
      return new Set(ids);
    });
  }, []);

  return { selectedIds, setSelectedIds, toggle, clear, toggleAll };
}

export type IdToggleHandler = (
  id: number,
  index: number,
  shiftKey: boolean,
  orderedIds: readonly number[],
) => void;
