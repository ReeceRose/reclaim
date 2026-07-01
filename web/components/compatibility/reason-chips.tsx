import { Badge } from "@/components/ui/badge";
import type { CompatibilityReason } from "@/lib/api";
import { reasonLabel } from "./constants";

/**
 * ReasonChips renders up to `max` reason codes as pills, collapsing the rest
 * into a "+N" pill (docs/COMPATIBILITY PLAN.md §10 "Reason chips (max 2
 * visible + '+N')"). Advisory reasons render dimmer than Hard ones so the
 * "predicted, not guaranteed" framing (§1) survives into the list view.
 */
export function ReasonChips({
  reasons,
  max = 2,
}: {
  reasons: CompatibilityReason[];
  max?: number;
}) {
  if (reasons.length === 0) {
    return <span className="text-muted-dim text-xs">—</span>;
  }
  const visible = reasons.slice(0, max);
  const rest = reasons.length - visible.length;
  return (
    <div className="flex items-center gap-1 flex-wrap">
      {visible.map((r) => (
        <Badge
          key={`${r.code}-${r.stream ?? ""}`}
          title={r.message}
          className={
            r.severity === "hard"
              ? "text-[0.68rem] rounded-[6px] font-medium text-muted-fg border-line bg-surface-3"
              : "text-[0.68rem] rounded-[6px] font-medium text-muted-dim border-line-soft bg-surface-2"
          }
        >
          {reasonLabel(r.code)}
        </Badge>
      ))}
      {rest > 0 && (
        <span className="text-[0.68rem] text-muted-dim font-medium">
          +{rest}
        </span>
      )}
    </div>
  );
}
