"use client";

import Link from "next/link";
import type { CompatibilityItem } from "@/lib/api";
import { baseName, dirName, formatBytes } from "@/lib/format";
import { cn } from "@/lib/utils";
import { CodecBadge } from "../media/codec-badge";
import { ACTION_LABELS } from "./constants";
import { ReasonChips } from "./reason-chips";
import { RiskBadge } from "./risk-badge";

/**
 * CompatibilityRow extends the MediaFlatRow pattern (docs/COMPATIBILITY
 * PLAN.md §10 "Row component: CompatibilityRow") — no checkbox/selection
 * column since Phase 1 has no queue action from this view, and the savings
 * column is de-emphasized in favor of risk score + reasons + recommended
 * action.
 */
export function CompatibilityRow({
  item,
  href,
}: {
  item: CompatibilityItem;
  href: string;
}) {
  const missing = item.status === "missing";
  return (
    <div
      className={cn(
        "relative flex items-center gap-0 border-b border-line-soft hover:bg-surface-2 transition-colors",
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
      <div className="flex-1 min-w-0 pl-4 pr-3">
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
      <div className="w-[56px] shrink-0 flex justify-center">
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
