"use client";

import { useSuspenseQuery } from "@tanstack/react-query";
import Link from "next/link";
import { Skeleton } from "@/components/ui/skeleton";
import { api } from "@/lib/api";
import { formatInt, formatPct } from "@/lib/format";
import { reasonLabel } from "./constants";

export function CompatibilityOverviewCardSkeleton() {
  return (
    <div
      className="border border-line rounded-lg p-5"
      style={{ background: "var(--surface)" }}
    >
      <Skeleton className="h-3 w-40 mb-4" />
      <Skeleton className="h-8 w-24 mb-2" />
      <Skeleton className="h-3 w-48 mb-4" />
      <Skeleton className="h-2 w-full rounded-full mb-4" />
      {[0, 1, 2].map((i) => (
        <div key={i} className="flex items-center justify-between mb-2">
          <Skeleton className="h-3 w-28" />
          <Skeleton className="h-3 w-8" />
        </div>
      ))}
    </div>
  );
}

/**
 * CompatibilityOverviewCard is the overview's second stat block
 * (docs/COMPATIBILITY PLAN.md §10): direct-play vs at-risk counts for the
 * default client profile, a top-3 reason breakdown, and — mandatory per
 * §10/§14, not optional polish — a callout tying at-risk files back to the
 * existing savings-candidate stat so the two lenses (§1) don't read as two
 * unrelated features bolted together.
 */
export function CompatibilityOverviewCard() {
  const { data: settings } = useSuspenseQuery({
    queryKey: ["settings"],
    queryFn: api.settings,
  });
  const { data: profilesData } = useSuspenseQuery({
    queryKey: ["compatibility-profiles"],
    queryFn: api.compatibilityProfiles,
    staleTime: 5 * 60_000,
  });
  const profile = settings.default_client_profile || "apple_tv_4k";
  const { data: compatibilityStats } = useSuspenseQuery({
    queryKey: ["compatibility-stats", profile],
    queryFn: () => api.compatibilityStats(profile),
    staleTime: 30_000,
  });

  if (compatibilityStats.total_files === 0) return null;

  const profileName =
    profilesData.profiles.find((p) => p.id === profile)?.name ?? profile;
  const total = compatibilityStats.total_files;
  const atRisk = compatibilityStats.transcode_risk_count;
  const directPct =
    total > 0
      ? Math.round((compatibilityStats.direct_play_count / total) * 100)
      : 0;
  const atRiskPct = total > 0 ? 100 - directPct : 0;
  const topReasons = compatibilityStats.by_reason.slice(0, 3);

  return (
    <div
      className="border border-line rounded-lg p-5"
      style={{ background: "var(--surface)" }}
    >
      <div className="flex items-center justify-between mb-4">
        <div className="text-xs uppercase tracking-[0.11em] text-muted-fg font-bold">
          Direct play · {profileName}
        </div>
        <Link
          href="/compatibility"
          className="text-xs text-brand hover:underline"
        >
          View →
        </Link>
      </div>

      <div className="text-[1.9rem] font-extrabold tracking-tight text-red leading-none mb-1.5">
        {formatInt(atRisk)}
      </div>
      <div className="text-xs text-muted-fg mb-4">
        files may transcode · {formatPct(atRisk, total)} of evaluated library
      </div>

      <div className="h-2 rounded-full bg-surface-2 overflow-hidden flex mb-4">
        <div
          className="h-full"
          style={{ width: `${atRiskPct}%`, background: "var(--red)" }}
        />
        <div
          className="h-full"
          style={{ width: `${directPct}%`, background: "var(--green)" }}
        />
      </div>

      {compatibilityStats.savings_overlap_count > 0 && (
        <div
          className="text-xs text-muted-fg mb-4 rounded-lg border border-line-soft px-3 py-2 leading-relaxed"
          style={{ background: "var(--surface-2)" }}
        >
          <span className="text-text font-semibold">
            {formatInt(compatibilityStats.savings_overlap_count)}
          </span>{" "}
          of these are also predicted HEVC savings candidates — re-encoding them
          may fix both at once.
        </div>
      )}

      {topReasons.length > 0 && (
        <div className="space-y-1.5">
          {topReasons.map((r) => (
            <div
              key={r.code}
              className="flex items-center justify-between text-xs"
            >
              <span className="text-muted-fg">{reasonLabel(r.code)}</span>
              <span className="text-muted-dim tnum">
                {formatInt(r.file_count)}
              </span>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
