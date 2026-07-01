import { Badge } from "@/components/ui/badge";
import { RISK_BAND_CLASSES, riskBand } from "./constants";

/**
 * RiskBadge renders a compatibility risk_score as a color-coded pill
 * (green/gold/red by band) — docs/COMPATIBILITY PLAN.md §10 "Risk badge
 * (green/yellow/red by score band)".
 */
export function RiskBadge({ score }: { score: number }) {
  const band = riskBand(score);
  return (
    <Badge
      className={`text-[0.72rem] rounded-[7px] font-bold tabular-nums ${RISK_BAND_CLASSES[band]}`}
    >
      {score}
    </Badge>
  );
}

export function DirectPlayBadge({ predicted }: { predicted: boolean }) {
  return predicted ? (
    <Badge className="text-[0.7rem] rounded-[7px] font-semibold text-green border-green-soft bg-green-soft">
      Direct play
    </Badge>
  ) : (
    <Badge className="text-[0.7rem] rounded-[7px] font-semibold text-red border-[rgba(255,120,120,.28)] bg-[rgba(255,120,120,.09)]">
      Predicted transcode
    </Badge>
  );
}
