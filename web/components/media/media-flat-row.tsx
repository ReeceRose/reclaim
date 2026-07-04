"use client";

import Link from "next/link";
import { Checkbox } from "@/components/ui/checkbox";
import type { IdToggleHandler } from "@/hooks/use-id-selection";
import type { MediaFile } from "@/lib/api";
import { baseName, dirName, formatBytes, resolutionLabel } from "@/lib/format";
import { cn } from "@/lib/utils";
import { isQueueable, queueBlockReason, StateBadge } from "./candidate-state";
import { CodecBadge } from "./codec-badge";

export function MediaFlatRow({
  item,
  index,
  orderedIds,
  selected,
  onToggle,
  href,
  showState = false,
  gateSelection = false,
}: {
  item: MediaFile;
  index: number;
  orderedIds: readonly number[];
  selected: boolean;
  onToggle: IdToggleHandler;
  href: string;
  showState?: boolean;
  gateSelection?: boolean;
}) {
  const queueable = !gateSelection || isQueueable(item);
  const missing = item.status === "missing";
  return (
    <div
      className={cn(
        "relative flex items-center gap-0 border-b border-line-soft hover:bg-surface-2 transition-colors",
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
        className="relative z-10 w-14 flex justify-center shrink-0"
        title={
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
      <div className="flex-1 min-w-0 pr-3">
        <div
          className={cn(
            "font-semibold text-sm truncate",
            missing && "line-through text-muted-fg",
          )}
        >
          {baseName(item.path)}
        </div>
        <div className="text-xs text-muted-dim truncate font-mono">
          {dirName(item.path)}
        </div>
      </div>
      <div className="w-16 sm:w-20 shrink-0">
        <CodecBadge codec={item.video_codec} showUnknown={showState} />
      </div>
      <div className="hidden sm:block w-16 shrink-0 text-sm text-muted-fg">
        {resolutionLabel(item.width, item.height)}
      </div>
      {showState && (
        <div className="hidden lg:block w-28 shrink-0">
          <StateBadge state={item.candidate_state} />
        </div>
      )}
      <div className="hidden sm:block w-24 shrink-0 text-right text-sm text-muted-fg pr-2 font-mono">
        {formatBytes(item.size_bytes)}
      </div>
      <div className="w-20 sm:w-28 shrink-0 text-right text-sm sm:text-sm pr-3 sm:pr-4 font-mono">
        {queueable ? (
          <span className="text-brand font-semibold">
            {formatBytes(item.predicted_savings_bytes)}
          </span>
        ) : (
          <span className="text-muted-dim">-</span>
        )}
      </div>
    </div>
  );
}
