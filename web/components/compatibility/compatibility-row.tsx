"use client";

import Link from "next/link";
import { Checkbox } from "@/components/ui/checkbox";
import type { IdToggleHandler } from "@/hooks/use-id-selection";
import type { CompatibilityItem } from "@/lib/api";
import { baseName, dirName, formatBytes } from "@/lib/format";
import { cn } from "@/lib/utils";
import { CodecBadge } from "../media/codec-badge";
import {
  ACTION_LABELS,
  compatibilityQueueBlockReason,
  isCompatibilityQueueable,
} from "./constants";
import { ReasonChips } from "./reason-chips";
import { RiskBadge } from "./risk-badge";

/**
 * CompatibilityRow extends the MediaFlatRow pattern (docs/COMPATIBILITY
 * PLAN.md §10 "Row component: CompatibilityRow"). Phase 1 shipped with no
 * checkbox column; Phase 2 (§3, §8) adds one, gated the same way the
 * Library view gates MediaFlatRow — only rows whose recommended_action is
 * queueable today (reencode_hevc) can be selected.
 */
export function CompatibilityRow({
  item,
  index,
  orderedIds,
  selected,
  onToggle,
  href,
}: {
  item: CompatibilityItem;
  index: number;
  orderedIds: readonly number[];
  selected: boolean;
  onToggle: IdToggleHandler;
  href: string;
}) {
  const missing = item.status === "missing";
  const queueable = isCompatibilityQueueable(
    item.compatibility.recommended_action,
  );
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
        className="relative z-10 w-[52px] flex justify-center shrink-0"
        title={
          queueable
            ? "Queue re-encode"
            : compatibilityQueueBlockReason(
                item.compatibility.recommended_action,
              )
        }
      >
        <Checkbox
          checked={selected}
          disabled={!queueable}
          onClick={(e) => {
            e.stopPropagation();
            if (queueable) onToggle(item.id, index, e.shiftKey, orderedIds);
          }}
          className="size-[17px] rounded-[5px]"
        />
      </div>
      <div className="flex-1 min-w-0 pr-3">
        <div
          className={cn(
            "font-semibold text-[0.88rem] truncate",
            missing && "line-through text-muted-fg",
          )}
        >
          {baseName(item.path)}
        </div>
        <div className="text-[0.74rem] text-muted-dim truncate font-mono">
          {dirName(item.path)}
        </div>
      </div>
      <div className="w-[64px] sm:w-[80px] shrink-0">
        <CodecBadge codec={item.video_codec} />
      </div>
      <div className="relative z-10 w-[72px] shrink-0 flex justify-center">
        <RiskBadge score={item.compatibility.risk_score} />
      </div>
      <div className="hidden md:flex flex-1 min-w-0 shrink-0 px-2">
        <ReasonChips reasons={item.compatibility.reasons} />
      </div>
      <div className="hidden lg:block w-[150px] shrink-0 text-[0.78rem] text-muted-fg pr-3 truncate">
        {ACTION_LABELS[item.compatibility.recommended_action]}
      </div>
      <div className="hidden sm:block w-[90px] shrink-0 text-right text-[0.82rem] text-muted-fg pr-4 font-mono">
        {formatBytes(item.size_bytes)}
      </div>
    </div>
  );
}
