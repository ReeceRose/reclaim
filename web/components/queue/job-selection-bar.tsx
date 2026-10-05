"use client";

import {
  ArrowDownIcon,
  ArrowDownToLineIcon,
  ArrowUpIcon,
  ArrowUpToLineIcon,
} from "lucide-react";
import type { ReactNode } from "react";
import { Button } from "@/components/ui/button";
import type { QueuePosition } from "@/lib/api";
import { formatInt } from "@/lib/format";

export const QUEUE_MOVES: {
  to: QueuePosition;
  label: string;
  icon: ReactNode;
}[] = [
  { to: "top", label: "Move to top", icon: <ArrowUpToLineIcon /> },
  { to: "up", label: "Move up one", icon: <ArrowUpIcon /> },
  { to: "down", label: "Move down one", icon: <ArrowDownIcon /> },
  { to: "bottom", label: "Move to bottom", icon: <ArrowDownToLineIcon /> },
];

/**
 * JobSelectionBar is the sticky action bar shown while queued jobs are
 * selected: it moves, forces, or cancels them as one batch.
 */
export function JobSelectionBar({
  count,
  onClear,
  onMove,
  onForce,
  onCancel,
  movePending,
  forcePending,
  cancelPending,
}: {
  count: number;
  onClear: () => void;
  onMove: (to: QueuePosition) => void;
  onForce: () => void;
  onCancel: () => void;
  movePending: boolean;
  forcePending: boolean;
  cancelPending: boolean;
}) {
  return (
    <div
      className="mb-3 flex flex-wrap items-center gap-x-4 gap-y-2 rounded-xl px-4 py-3 border border-brand-line sticky bottom-3 z-10 sm:px-5"
      style={{
        background: "var(--surface-2)",
        boxShadow: "0 10px 30px rgba(0,0,0,.35)",
      }}
    >
      <div className="font-bold">
        <b className="text-brand">{formatInt(count)}</b> selected
      </div>
      <Button variant="ghost" onClick={onClear} className="rounded-xl text-sm">
        Clear
      </Button>
      <div className="ml-auto flex flex-wrap gap-2 items-center">
        <div className="flex">
          {QUEUE_MOVES.map((m) => (
            <Button
              key={m.to}
              size="icon-sm"
              variant="ghost"
              onClick={() => onMove(m.to)}
              disabled={movePending}
              aria-label={`${m.label} (selected)`}
              data-tooltip={m.label}
              className="text-muted-fg hover:bg-surface-3 hover:text-text"
            >
              {m.icon}
            </Button>
          ))}
        </div>
        <Button
          size="sm"
          variant="outline"
          onClick={onForce}
          disabled={forcePending}
          className="rounded-xl text-xs"
          data-tooltip="Run the selected jobs now, bypassing the encode window"
        >
          Run now
        </Button>
        <Button
          size="sm"
          variant="outline"
          onClick={onCancel}
          disabled={cancelPending}
          className="rounded-xl text-xs text-red border-red/30 hover:bg-red-soft hover:text-red"
        >
          Cancel selected
        </Button>
      </div>
    </div>
  );
}
