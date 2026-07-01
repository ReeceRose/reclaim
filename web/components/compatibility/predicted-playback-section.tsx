"use client";

import { useQuery } from "@tanstack/react-query";
import { useState } from "react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { DetailSection } from "@/components/ui/detail";
import {
  api,
  type CandidateState,
  type CompatibilityInfo,
  type StreamInfo,
} from "@/lib/api";
import { formatBytes } from "@/lib/format";
import { cn } from "@/lib/utils";
import {
  ACTION_LABELS,
  isCompatibilityQueueable,
  SEVERITY_LABELS,
} from "./constants";
import { DirectPlayBadge, RiskBadge } from "./risk-badge";

function StreamTable({ streams }: { streams: StreamInfo[] }) {
  if (streams.length === 0) return null;
  return (
    <div className="overflow-x-auto -mx-3 px-3 mt-3 pt-3 border-t border-line-soft">
      <table className="w-full text-xs">
        <thead>
          <tr className="text-muted-dim text-left">
            <th className="py-1.5 pr-3 font-medium">#</th>
            <th className="py-1.5 pr-3 font-medium">Type</th>
            <th className="py-1.5 pr-3 font-medium">Codec</th>
            <th className="py-1.5 pr-3 font-medium">Profile</th>
            <th className="py-1.5 pr-3 font-medium">Channels</th>
            <th className="py-1.5 pr-3 font-medium">Lang</th>
            <th className="py-1.5 font-medium">Default</th>
          </tr>
        </thead>
        <tbody>
          {streams.map((s) => (
            <tr key={s.index} className="border-t border-line-soft">
              <td className="py-1.5 pr-3 font-mono text-muted-fg">{s.index}</td>
              <td className="py-1.5 pr-3 capitalize">{s.codec_type}</td>
              <td className="py-1.5 pr-3 font-mono">{s.codec_name ?? "—"}</td>
              <td className="py-1.5 pr-3">{s.profile ?? "—"}</td>
              <td className="py-1.5 pr-3">{s.channels ?? "—"}</td>
              <td className="py-1.5 pr-3">{s.language ?? "—"}</td>
              <td className="py-1.5">{s.is_default ? "✓" : ""}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

/**
 * PredictedPlaybackSection is the file-detail piece of docs/COMPATIBILITY
 * PLAN.md §10: a per-profile verdict toggle, reason list, and stream table.
 * Renders nothing when the file hasn't been evaluated yet (probed before
 * the compatibility engine shipped and not yet through a full rescan —
 * see §5 "Backfill").
 *
 * `onQueueReencode` is Phase 2's "Queue re-encode" button (§10 — previously
 * deferred): shown only when the active profile's verdict recommends
 * reencode_hevc and the file has no blocking job already. It's gated on
 * `candidateState` rather than `is_already_hevc` — the compatibility rule
 * (server-validated too, see docs/COMPATIBILITY PLAN.md §8) can recommend a
 * re-encode for a file that's already HEVC (e.g. wrong bit depth for a
 * future profile).
 */
export function PredictedPlaybackSection({
  compatibility,
  streams,
  candidateState,
  predictedSavingsBytes,
  onQueueReencode,
}: {
  compatibility: CompatibilityInfo[];
  streams: StreamInfo[];
  candidateState?: CandidateState;
  predictedSavingsBytes?: number;
  onQueueReencode?: (clientProfile: string) => void;
}) {
  const { data: profilesData } = useQuery({
    queryKey: ["compatibility-profiles"],
    queryFn: api.compatibilityProfiles,
    staleTime: 5 * 60_000,
  });
  const profiles = profilesData?.profiles ?? [];
  const [selected, setSelected] = useState<string | null>(null);

  if (compatibility.length === 0) return null;

  const active =
    compatibility.find((c) => c.client_profile === selected) ??
    compatibility[0];

  return (
    <DetailSection title="Predicted playback">
      <div className="py-3">
        <div className="flex items-center gap-1.5 flex-wrap mb-3">
          {compatibility.map((c) => {
            const name =
              profiles.find((p) => p.id === c.client_profile)?.name ??
              c.client_profile;
            const isActive = c.client_profile === active.client_profile;
            return (
              <Button
                key={c.client_profile}
                type="button"
                variant={isActive ? "default" : "outline"}
                size="sm"
                onClick={() => setSelected(c.client_profile)}
                className={cn(
                  "h-7 text-xs rounded-[8px] px-2.5",
                  isActive &&
                    "bg-[linear-gradient(145deg,var(--brand),var(--brand-2))]",
                )}
              >
                {name}
              </Button>
            );
          })}
        </div>

        <div className="flex items-center gap-2 flex-wrap mb-3">
          <DirectPlayBadge predicted={active.direct_play_predicted} />
          <RiskBadge score={active.risk_score} />
          <span className="text-xs text-muted-dim">
            Recommended: {ACTION_LABELS[active.recommended_action]}
          </span>
          {onQueueReencode &&
            isCompatibilityQueueable(active.recommended_action) &&
            candidateState !== "queued" &&
            candidateState !== "completed" && (
              <div className="ml-auto flex items-center gap-2">
                {!!predictedSavingsBytes && predictedSavingsBytes > 0 && (
                  <span className="text-[0.7rem] text-brand">
                    also reclaims {formatBytes(predictedSavingsBytes)}
                  </span>
                )}
                <Button
                  type="button"
                  size="sm"
                  onClick={() => onQueueReencode(active.client_profile)}
                  className="h-6 text-xs gap-1 rounded-[7px]"
                  style={{
                    background:
                      "linear-gradient(145deg, var(--brand), var(--brand-2))",
                  }}
                >
                  Queue re-encode
                </Button>
              </div>
            )}
        </div>

        {active.reasons.length > 0 ? (
          <ul className="space-y-2">
            {active.reasons.map((r) => (
              <li
                key={`${r.code}-${r.stream ?? ""}`}
                className="text-xs text-muted-fg flex items-start gap-2"
              >
                <Badge
                  className={cn(
                    "text-[0.65rem] rounded-[6px] font-semibold shrink-0 mt-px",
                    r.severity === "hard"
                      ? "text-red border-[rgba(255,120,120,.28)] bg-[rgba(255,120,120,.09)]"
                      : "text-muted-dim border-line-soft bg-surface-2",
                  )}
                >
                  {SEVERITY_LABELS[r.severity]}
                </Badge>
                <span className="leading-relaxed">{r.message}</span>
              </li>
            ))}
          </ul>
        ) : (
          <p className="text-xs text-muted-dim">
            No compatibility issues predicted for this profile.
          </p>
        )}

        <p className="text-[0.7rem] text-muted-dim italic mt-3">
          Predicted from file metadata only — actual playback depends on your
          server settings and device. Not a guarantee.
        </p>

        <StreamTable streams={streams} />
      </div>
    </DetailSection>
  );
}
