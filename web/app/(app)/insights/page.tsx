"use client";

import { useSuspenseQuery } from "@tanstack/react-query";
import Link from "next/link";
import { Suspense, useMemo, useState, useTransition } from "react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { PageHeader } from "@/components/ui/page-header";
import { Skeleton } from "@/components/ui/skeleton";
import {
  api,
  type ReplacementEntry,
  type ReplacementReport,
  type SavingsBucket,
  type SavingsDay,
} from "@/lib/api";
import { CODEC_COLORS, codecCSSColor } from "@/lib/codec";
import {
  baseName,
  formatBytes,
  formatDayFull,
  formatDayLabel,
  formatDuration,
  formatDurationCompact,
  formatInt,
  formatPct,
  relativeTime,
  resolutionBucketLabel,
} from "@/lib/format";

const RANGES = [30, 90, 365] as const;

type Point = {
  day: string;
  encoded: number;
  replaced: number;
  delta: number;
  cumulative: number;
};

function dayKey(d: Date): string {
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}-${String(d.getDate()).padStart(2, "0")}`;
}

function buildSeries(
  daily: SavingsDay[],
  days: number,
  lifetimeReclaimed: number,
): Point[] {
  const byDay = new Map(daily.map((d) => [d.day, d]));
  const inWindow = daily.reduce(
    (a, d) => a + d.bytes_saved + d.bytes_replaced,
    0,
  );
  let running = Math.max(0, lifetimeReclaimed - inWindow);

  const out: Point[] = [];
  const today = new Date();
  for (let i = days - 1; i >= 0; i--) {
    const dt = new Date(
      today.getFullYear(),
      today.getMonth(),
      today.getDate() - i,
    );
    const key = dayKey(dt);
    const row = byDay.get(key);
    const encoded = row?.bytes_saved ?? 0;
    const replaced = row?.bytes_replaced ?? 0;
    running += encoded + replaced;
    out.push({
      day: key,
      encoded,
      replaced,
      delta: encoded + replaced,
      cumulative: running,
    });
  }
  return out;
}

function SavingsChart({ series }: { series: Point[] }) {
  const [hover, setHover] = useState<number | null>(null);

  const W = 760;
  const H = 200;
  const padBottom = 24;
  const padTop = 10;
  const plotH = H - padBottom - padTop;

  const max = Math.max(...series.map((p) => p.cumulative), 1);
  const x = (i: number) =>
    series.length <= 1 ? 0 : (i / (series.length - 1)) * W;
  const y = (v: number) => padTop + plotH - (v / max) * plotH;

  const line = series
    .map(
      (p, i) =>
        `${i === 0 ? "M" : "L"}${x(i).toFixed(2)},${y(p.cumulative).toFixed(2)}`,
    )
    .join(" ");
  const area = `${line} L${W},${padTop + plotH} L0,${padTop + plotH} Z`;

  const gridValues = [0, 0.25, 0.5, 0.75, 1].map((f) => f * max);
  const labelStep = Math.max(1, Math.floor(series.length / 6));
  const active = hover != null ? series[hover] : null;

  return (
    <div className="relative">
      <div className="flex justify-between text-2xs text-muted-dim mb-1.5 tnum">
        <span>{series.length > 0 ? formatDayFull(series[0].day) : ""}</span>
        <span>
          {series.length > 0
            ? formatDayFull(series[series.length - 1].day)
            : ""}
        </span>
      </div>
      <svg
        viewBox={`0 0 ${W} ${H}`}
        className="w-full h-auto block overflow-visible"
        role="img"
        aria-label="Cumulative storage reclaimed over time"
        onMouseLeave={() => setHover(null)}
        onMouseMove={(e) => {
          const rect = e.currentTarget.getBoundingClientRect();
          if (rect.width <= 0 || series.length === 0) return;
          const rel = (e.clientX - rect.left) / rect.width;
          const idx = Math.round(rel * (series.length - 1));
          setHover(Math.min(series.length - 1, Math.max(0, idx)));
        }}
      >
        <defs>
          <linearGradient id="savings-fill" x1="0" y1="0" x2="0" y2="1">
            <stop offset="0%" stopColor="var(--brand)" stopOpacity="0.34" />
            <stop offset="100%" stopColor="var(--brand)" stopOpacity="0.02" />
          </linearGradient>
        </defs>

        {gridValues.map((v) => (
          <g key={v}>
            <line
              x1="0"
              x2={W}
              y1={y(v).toFixed(2)}
              y2={y(v).toFixed(2)}
              stroke="var(--line-soft)"
              strokeWidth="1"
            />
            <text
              x="4"
              y={(y(v) - 4).toFixed(2)}
              fill="var(--muted-dim)"
              fontSize="10"
            >
              {formatBytes(v)}
            </text>
          </g>
        ))}

        <path d={area} fill="url(#savings-fill)" />
        <path
          d={line}
          fill="none"
          stroke="var(--brand)"
          strokeWidth="2"
          strokeLinejoin="round"
          strokeLinecap="round"
          style={{ filter: "drop-shadow(0 0 6px var(--brand-soft))" }}
        />

        {series.map((p, i) =>
          i % labelStep === 0 || i === series.length - 1 ? (
            <text
              key={p.day}
              x={x(i).toFixed(2)}
              y={H - 6}
              fill="var(--muted-dim)"
              fontSize="10"
              textAnchor={
                i === 0 ? "start" : i === series.length - 1 ? "end" : "middle"
              }
            >
              {formatDayLabel(p.day)}
            </text>
          ) : null,
        )}

        {active && hover != null && (
          <g>
            <line
              x1={x(hover).toFixed(2)}
              x2={x(hover).toFixed(2)}
              y1={padTop}
              y2={padTop + plotH}
              stroke="var(--brand)"
              strokeWidth="1"
              strokeDasharray="3 3"
              opacity="0.7"
            />
            <circle
              cx={x(hover).toFixed(2)}
              cy={y(active.cumulative).toFixed(2)}
              r="3.5"
              fill="var(--brand)"
              stroke="var(--surface)"
              strokeWidth="1.5"
            />
          </g>
        )}
      </svg>

      <div className="h-9 mt-1">
        {active ? (
          <div className="text-xs text-muted-fg tnum">
            <b className="text-text font-semibold">
              {formatDayFull(active.day)}
            </b>{" "}
            · total {formatBytes(active.cumulative)}
            {active.encoded > 0 && (
              <span className="text-brand">
                {" "}
                · +{formatBytes(active.encoded)} encoded
              </span>
            )}
            {active.replaced !== 0 && (
              <span
                className={active.replaced > 0 ? "text-green" : "text-gold"}
              >
                {" "}
                · {active.replaced > 0 ? "+" : "−"}
                {formatBytes(Math.abs(active.replaced))} replaced
              </span>
            )}
          </div>
        ) : (
          <div className="text-xs text-muted-dim">
            Hover the chart for a daily breakdown.
          </div>
        )}
      </div>
    </div>
  );
}

function BucketBars({
  buckets,
  label,
  colorFor,
  labelFor,
  footnote,
}: {
  buckets: SavingsBucket[];
  label: string;
  colorFor?: (key: string) => string;
  labelFor?: (key: string) => string;
  footnote?: string;
}) {
  const max = Math.max(...buckets.map((b) => b.bytes_saved), 1);
  return (
    <div
      className="border border-line rounded-lg p-5"
      style={{ background: "var(--surface)" }}
    >
      <div className="text-xs uppercase tracking-widest text-muted-fg font-bold mb-4">
        {label}
      </div>
      {buckets.length === 0 ? (
        <div className="text-sm text-muted-dim">No encodes recorded yet.</div>
      ) : (
        buckets.map((b) => (
          <div
            key={b.key}
            className="flex items-center gap-3 mb-3.5 last:mb-0 text-sm"
          >
            <div className="w-20 shrink-0 font-semibold truncate">
              {colorFor ? (
                <Badge
                  className={`font-mono text-xs rounded-lg font-semibold ${CODEC_COLORS[b.key.toLowerCase()] ?? "text-slate"}`}
                  style={{
                    borderColor: `color-mix(in srgb, ${colorFor(b.key)} 30%, transparent)`,
                    background: `color-mix(in srgb, ${colorFor(b.key)} 10%, transparent)`,
                  }}
                >
                  {b.key}
                </Badge>
              ) : (
                (labelFor?.(b.key) ?? b.key)
              )}
            </div>
            <div className="flex-1 h-2.5 bg-surface-2 rounded-md overflow-hidden">
              <div
                className="h-full rounded-md"
                style={{
                  width: `${Math.round((b.bytes_saved / max) * 100)}%`,
                  background: colorFor
                    ? colorFor(b.key)
                    : "linear-gradient(90deg, var(--brand), var(--brand-2))",
                }}
              />
            </div>
            <div className="w-32 sm:w-44 shrink-0 text-right text-muted-fg text-xs tnum">
              {formatBytes(b.bytes_saved)}
              <span className="hidden sm:inline">
                {" "}
                · {formatInt(b.files_encoded)} files ·{" "}
                {Math.round((1 - b.compression_ratio) * 100)}% smaller
              </span>
            </div>
          </div>
        ))
      )}
      {footnote && buckets.some((b) => b.key === "unknown") && (
        <div className="text-2xs text-muted-dim mt-4 pt-3 border-t border-line-soft">
          {footnote}
        </div>
      )}
    </div>
  );
}

// libraryLabel names a library_type bucket for display.
function libraryLabel(key: string): string {
  if (key === "tv") return "TV";
  if (key === "movies") return "Movies";
  return key;
}

// signedBytes renders a delta with an explicit direction, so a replacement that
// cost space never reads as a saving.
function plural(n: number, one: string, many: string): string {
  return `${formatInt(n)} ${n === 1 ? one : many}`;
}

function signedBytes(n: number): string {
  if (n === 0) return "0 B";
  return `${n > 0 ? "−" : "+"}${formatBytes(Math.abs(n))}`;
}

function ReplacementRow({ r }: { r: ReplacementEntry }) {
  const gained = r.bytes_saved > 0;
  return (
    <div className="flex flex-wrap items-center gap-x-3 gap-y-1.5 px-4 py-3 border border-line rounded-xl bg-surface">
      <div className="min-w-0 flex-1">
        <div className="text-sm font-medium truncate">
          {baseName(r.path) || "(removed)"}
        </div>
        <div className="text-xs text-muted-dim truncate">
          replaced {baseName(r.previous_path) || "(unknown)"}
        </div>
        <div className="text-xs text-muted-dim tnum">
          {formatBytes(r.original_size_bytes)} →{" "}
          {formatBytes(r.output_size_bytes)} · {relativeTime(r.completed_at)}
        </div>
      </div>
      {r.source_codec && (
        <div className="flex items-center gap-1.5 shrink-0">
          <Badge
            className={`font-mono text-xs rounded-lg font-semibold ${CODEC_COLORS[r.source_codec] ?? "text-slate"}`}
            style={{
              borderColor: `color-mix(in srgb, ${codecCSSColor(r.source_codec)} 30%, transparent)`,
              background: `color-mix(in srgb, ${codecCSSColor(r.source_codec)} 10%, transparent)`,
            }}
          >
            {r.source_codec}
          </Badge>
          <span className="text-muted-dim text-xs">→</span>
          <Badge
            className={`font-mono text-xs rounded-lg font-semibold ${CODEC_COLORS[r.result_codec ?? ""] ?? "text-slate"}`}
            style={{
              borderColor: `color-mix(in srgb, ${codecCSSColor(r.result_codec ?? "")} 30%, transparent)`,
              background: `color-mix(in srgb, ${codecCSSColor(r.result_codec ?? "")} 10%, transparent)`,
            }}
          >
            {r.result_codec ?? "?"}
          </Badge>
        </div>
      )}
      <div
        className={`text-sm font-semibold tnum shrink-0 ${gained ? "text-brand" : "text-gold"}`}
      >
        {signedBytes(r.bytes_saved)}
      </div>
    </div>
  );
}

function ReplacementsCard({ report }: { report: ReplacementReport }) {
  const s = report.summary;
  return (
    <div
      className="border border-line rounded-lg p-5"
      style={{ background: "var(--surface)" }}
    >
      <div className="flex items-baseline justify-between gap-3 flex-wrap mb-4">
        <div className="text-xs uppercase tracking-widest text-muted-fg font-bold">
          Replaced, not encoded
        </div>
        <div className="text-2xs text-muted-dim">
          files deleted and re-acquired elsewhere
        </div>
      </div>

      {s.files_replaced === 0 ? (
        <div className="text-sm text-muted-dim">
          Nothing recorded yet. When a file disappears and another copy of the
          same episode or movie is indexed later, Reclaim pairs the two and
          books the size difference here — so deleting a bloated release and
          grabbing a leaner one counts toward the same total as a re-encode.
        </div>
      ) : (
        <>
          <div className="grid grid-cols-4 gap-4 max-sm:grid-cols-2">
            <StatTile
              label="Net reclaimed"
              value={signedBytes(s.bytes_delta)}
              sub={`across ${plural(s.files_replaced, "replacement", "replacements")}`}
              tone={s.bytes_delta >= 0 ? "text-brand" : "text-gold"}
            />
            <StatTile
              label="Freed"
              value={formatBytes(s.bytes_reclaimed)}
              sub={plural(
                s.replacements_smaller,
                "leaner release",
                "leaner releases",
              )}
              tone="text-green"
            />
            <StatTile
              label="Given back"
              value={formatBytes(s.bytes_added)}
              sub={plural(s.replacements_larger, "upgrade", "upgrades")}
              tone={s.bytes_added > 0 ? "text-gold" : undefined}
            />
            <StatTile
              label="Biggest win"
              value={formatBytes(s.best_saved_bytes)}
              sub={s.best_path ? baseName(s.best_path) : undefined}
            />
          </div>

          {report.by_library.length > 0 && (
            <div className="flex flex-wrap gap-x-6 gap-y-1 mt-4 pt-4 border-t border-line-soft text-xs text-muted-fg tnum">
              {report.by_library.map((b) => (
                <span key={b.key}>
                  {libraryLabel(b.key)}: {signedBytes(b.bytes_saved)} over{" "}
                  {formatInt(b.files_encoded)}
                </span>
              ))}
            </div>
          )}

          {report.recent.length > 0 && (
            <div className="mt-5 pt-5 border-t border-line-soft">
              <div className="text-xs uppercase tracking-widest text-muted-fg font-bold mb-3">
                Recent replacements
              </div>
              <div className="flex flex-col gap-2.5">
                {report.recent.slice(0, 6).map((r) => (
                  <ReplacementRow
                    key={`${r.media_file_id}-${r.completed_at}`}
                    r={r}
                  />
                ))}
              </div>
            </div>
          )}
        </>
      )}
    </div>
  );
}

function StatTile({
  label,
  value,
  sub,
  tone,
}: {
  label: string;
  value: string;
  sub?: string;
  tone?: string;
}) {
  return (
    <div>
      <div className="text-xs text-muted-fg uppercase tracking-wider font-bold">
        {label}
      </div>
      <div
        className={`text-stat font-bold tracking-tight mt-1 tnum ${tone ?? ""}`}
      >
        {value}
      </div>
      {sub && <div className="text-xs text-muted-dim mt-0.5">{sub}</div>}
    </div>
  );
}

function InsightsSkeleton() {
  return (
    <div className="px-4 pt-5 pb-14 w-full sm:px-7 sm:pt-7">
      <div
        className="rounded-2xl border border-line px-5 py-6 mb-6 sm:px-7 sm:py-7"
        style={{ background: "var(--surface)" }}
      >
        <Skeleton className="h-3 w-40 mb-6" />
        <Skeleton className="h-16 w-52 mb-3" />
        <Skeleton className="h-4 w-80 mb-7" />
        <Skeleton className="h-48 w-full rounded-xl" />
      </div>
      <div className="grid grid-cols-2 gap-5 max-sm:grid-cols-1">
        {[0, 1].map((i) => (
          <div
            key={i}
            className="border border-line rounded-lg p-5"
            style={{ background: "var(--surface)" }}
          >
            <Skeleton className="h-3 w-32 mb-5" />
            {[0, 1, 2].map((j) => (
              <div key={j} className="flex items-center gap-3 mb-4">
                <Skeleton className="h-5 w-14 rounded-lg" />
                <Skeleton className="flex-1 h-2.5 rounded-md" />
                <Skeleton className="h-4 w-24" />
              </div>
            ))}
          </div>
        ))}
      </div>
    </div>
  );
}

function EmptyState() {
  return (
    <div className="px-4 pt-5 pb-14 w-full flex flex-col items-center justify-center min-h-[50vh] gap-4 text-center sm:px-7 sm:pt-7">
      <div
        className="w-14 h-14 rounded-2xl border border-line grid place-items-center text-muted-dim"
        style={{ background: "var(--surface-2)" }}
      >
        <svg
          aria-hidden="true"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          strokeWidth="1.5"
          className="w-6 h-6"
        >
          <path d="M3 3v18h18" />
          <path d="M7 15l4-4 3 3 5-6" />
        </svg>
      </div>
      <div>
        <div className="text-base font-semibold text-text">
          Nothing reclaimed yet
        </div>
        <div className="text-sm text-muted-dim mt-1 max-w-sm">
          Reclaim measures two things here: bytes freed by a completed
          re-encode, and bytes freed by deleting a file and re-acquiring a
          leaner copy of it. As soon as either happens, this page fills in with
          real numbers instead of estimates.
        </div>
      </div>
      <Link
        href="/candidates"
        className="text-sm text-brand hover:underline mt-1"
      >
        Review candidates →
      </Link>
    </div>
  );
}

function InsightsContent() {
  const [days, setDays] = useState<number>(90);
  const [isPending, startTransition] = useTransition();

  const { data: stats } = useSuspenseQuery({
    queryKey: ["stats"],
    queryFn: api.stats,
    staleTime: 30_000,
  });

  const { data: report } = useSuspenseQuery({
    queryKey: ["savings", days],
    queryFn: () => api.savings(days),
    staleTime: 30_000,
  });

  const summary = report.summary;
  const replacements = report.replacements.summary;

  // The headline is bytes reclaimed by any means. Replacements are netted in
  // signed, so a run of 4K upgrades pulls the number down rather than being
  // quietly dropped.
  const totalReclaimed = summary.bytes_saved + replacements.bytes_delta;

  const series = useMemo(
    () => buildSeries(report.daily, report.days, totalReclaimed),
    [report, totalReclaimed],
  );

  if (summary.files_encoded === 0 && replacements.files_replaced === 0) {
    return <EmptyState />;
  }

  const savingsAccuracy = summary.savings_estimate_ratio;
  const durationAccuracy = summary.duration_estimate_ratio;
  const libraryTotal = stats.total_bytes;
  const originalLibrary = libraryTotal + totalReclaimed;

  return (
    <div
      className="px-4 pt-5 pb-14 w-full sm:px-7 sm:pt-7 transition-opacity"
      style={{ opacity: isPending ? 0.6 : 1 }}
    >
      <div
        className="rounded-2xl border border-line px-5 py-6 mb-6 relative overflow-hidden sm:px-7 sm:py-7"
        style={{
          background:
            "radial-gradient(120% 150% at 100% 0%, var(--brand-soft), transparent 55%), var(--surface)",
        }}
      >
        <div className="flex items-center justify-between mb-5 gap-3 flex-wrap">
          <div className="text-xs text-muted-fg uppercase tracking-widest font-bold">
            Storage reclaimed
          </div>
          <div className="flex gap-1 rounded-xl border border-line p-0.5 bg-surface-2">
            {RANGES.map((r) => (
              <Button
                key={r}
                variant={days === r ? "secondary" : "ghost"}
                size="xs"
                onClick={() => startTransition(() => setDays(r))}
                className="rounded-lg px-2.5"
              >
                {r === 365 ? "1y" : `${r}d`}
              </Button>
            ))}
          </div>
        </div>

        <div className="flex items-end justify-between gap-6 flex-wrap mb-6">
          <div>
            <div
              className="text-5xl sm:text-hero font-extrabold leading-none tracking-tight text-brand"
              style={{ textShadow: "0 4px 26px var(--brand-soft)" }}
            >
              {formatBytes(totalReclaimed, 1).replace(" ", "")}
            </div>
            <div className="text-sm text-muted-fg mt-2">
              <b className="text-text font-semibold">
                {formatBytes(summary.bytes_saved)}
              </b>{" "}
              across {formatInt(summary.files_encoded)} encodes
              {replacements.files_replaced > 0 && (
                <>
                  {" · "}
                  <b className="text-text font-semibold">
                    {signedBytes(replacements.bytes_delta)}
                  </b>{" "}
                  across{" "}
                  {plural(
                    replacements.files_replaced,
                    "replacement",
                    "replacements",
                  )}
                </>
              )}
              <Badge className="ml-2 text-xs font-bold tracking-widest text-green bg-green-soft border-green-soft rounded-md uppercase">
                measured
              </Badge>
            </div>
          </div>
          <div className="text-right">
            <div className="text-xs text-muted-fg uppercase tracking-wider">
              Library shrunk by
            </div>
            <div className="text-2xl font-bold tracking-tight mt-0.5 tnum">
              {formatPct(summary.bytes_saved, originalLibrary)}
            </div>
            <div className="text-xs text-muted-dim mt-0.5 tnum">
              {formatBytes(originalLibrary)} → {formatBytes(libraryTotal)}
            </div>
          </div>
        </div>

        {series.length > 0 && <SavingsChart series={series} />}

        <div className="mt-5 pt-5 border-t border-line-soft grid grid-cols-4 gap-4 max-sm:grid-cols-2">
          <StatTile
            label="Avg reduction"
            value={`${Math.round((1 - summary.compression_ratio) * 100)}%`}
            sub="per encoded file"
            tone="text-brand"
          />
          <StatTile
            label="Encode time"
            value={formatDurationCompact(summary.encode_seconds_total)}
            sub={
              summary.mean_encode_seconds
                ? `${formatDurationCompact(Math.round(summary.mean_encode_seconds))} avg`
                : undefined
            }
          />
          <StatTile
            label="Reclaim rate"
            value={
              summary.bytes_saved_per_encode_hour
                ? `${formatBytes(summary.bytes_saved_per_encode_hour)}/h`
                : "—"
            }
            sub="per hour of encoding"
          />
          <StatTile
            label="Biggest win"
            value={formatBytes(summary.best_saved_bytes)}
            sub={summary.best_path ? baseName(summary.best_path) : undefined}
            tone="text-green"
          />
        </div>
      </div>

      <div className="grid grid-cols-2 gap-5 mb-5 max-sm:grid-cols-1">
        <div
          className="border border-line rounded-lg p-5"
          style={{ background: "var(--surface)" }}
        >
          <div className="text-xs uppercase tracking-widest text-muted-fg font-bold mb-4">
            Estimate accuracy
          </div>
          {savingsAccuracy == null && durationAccuracy == null ? (
            <div className="text-sm text-muted-dim">
              Not enough jobs with recorded predictions yet.
            </div>
          ) : (
            <div className="flex flex-col gap-4">
              {savingsAccuracy != null && (
                <div>
                  <div className="flex items-baseline justify-between gap-2">
                    <span className="text-sm text-muted-fg">
                      Predicted savings
                    </span>
                    <span
                      className={`text-sm font-semibold tnum ${savingsAccuracy >= 1 ? "text-green" : "text-gold"}`}
                    >
                      {savingsAccuracy >= 1 ? "beat by " : "short by "}
                      {Math.abs(Math.round((savingsAccuracy - 1) * 100))}%
                    </span>
                  </div>
                  <div className="text-xs text-muted-dim mt-1 tnum">
                    Actual reclaim ran at {Math.round(savingsAccuracy * 100)}%
                    of the estimate across{" "}
                    {formatInt(summary.savings_estimate_samples)} jobs.
                  </div>
                </div>
              )}
              {durationAccuracy != null && (
                <div>
                  <div className="flex items-baseline justify-between gap-2">
                    <span className="text-sm text-muted-fg">
                      Predicted encode time
                    </span>
                    <span
                      className={`text-sm font-semibold tnum ${durationAccuracy <= 1 ? "text-green" : "text-gold"}`}
                    >
                      {durationAccuracy <= 1 ? "faster by " : "slower by "}
                      {Math.abs(Math.round((durationAccuracy - 1) * 100))}%
                    </span>
                  </div>
                  <div className="text-xs text-muted-dim mt-1 tnum">
                    Encodes took {Math.round(durationAccuracy * 100)}% of the
                    estimated time across{" "}
                    {formatInt(summary.duration_estimate_samples)} jobs.
                  </div>
                </div>
              )}
            </div>
          )}
        </div>

        <div
          className="border border-line rounded-lg p-5"
          style={{ background: "var(--surface)" }}
        >
          <div className="text-xs uppercase tracking-widest text-muted-fg font-bold mb-4">
            Still on the table
          </div>
          <div className="flex items-end gap-6 flex-wrap">
            <div>
              <div className="text-stat font-bold tracking-tight text-brand tnum">
                {formatBytes(stats.total_recoverable_bytes)}
              </div>
              <div className="text-xs text-muted-dim mt-0.5">
                estimated across {formatInt(summary.remaining_candidates)}{" "}
                remaining candidates
              </div>
            </div>
          </div>
          {summary.projected_remaining_encode_seconds != null && (
            <div className="text-sm text-muted-fg mt-4 pt-4 border-t border-line-soft">
              At your observed pace that is roughly{" "}
              <b className="text-text font-semibold">
                {formatDurationCompact(
                  summary.projected_remaining_encode_seconds,
                )}
              </b>{" "}
              of encoding left.
            </div>
          )}
          <div className="flex gap-4 mt-4 text-xs text-muted-dim tnum">
            {Object.entries(report.job_outcomes).map(([k, v]) => (
              <span key={k}>
                {formatInt(v)} {k}
              </span>
            ))}
          </div>
        </div>
      </div>

      <div className="grid grid-cols-2 gap-5 mb-5 max-sm:grid-cols-1">
        <BucketBars
          buckets={report.by_codec}
          label="Reclaimed by source codec"
          colorFor={codecCSSColor}
          footnote="“unknown” covers encodes finished before this breakdown existed — the re-encode overwrites the file's codec, so the original can't be recovered after the fact — plus any file whose codec never probed. Their bytes still count toward every total on this page."
        />
        <BucketBars
          buckets={report.by_resolution}
          label="Reclaimed by resolution"
          labelFor={resolutionBucketLabel}
        />
      </div>

      <div
        className="border border-line rounded-lg p-5"
        style={{ background: "var(--surface)" }}
      >
        <div className="text-xs uppercase tracking-widest text-muted-fg font-bold mb-4">
          Biggest wins
        </div>
        {report.top_wins.length === 0 ? (
          <div className="text-sm text-muted-dim">No encodes recorded yet.</div>
        ) : (
          <div className="flex flex-col gap-2.5">
            {report.top_wins.map((w) => (
              <div
                key={w.job_id}
                className="flex flex-wrap items-center gap-x-3 gap-y-1.5 px-4 py-3 border border-line rounded-xl bg-surface"
              >
                <div className="min-w-0 flex-1">
                  <div className="text-sm font-medium truncate">
                    {baseName(w.path) || "(removed)"}
                  </div>
                  <div className="text-xs text-muted-dim tnum">
                    {formatBytes(w.original_size_bytes)} →{" "}
                    {formatBytes(w.output_size_bytes)} ·{" "}
                    {relativeTime(w.completed_at)}
                    {w.encode_seconds != null &&
                      ` · ${formatDuration(w.encode_seconds)} to encode`}
                  </div>
                </div>
                {w.source_codec && (
                  <Badge
                    className={`font-mono text-xs rounded-lg font-semibold ${CODEC_COLORS[w.source_codec] ?? "text-slate"}`}
                    style={{
                      borderColor: `color-mix(in srgb, ${codecCSSColor(w.source_codec)} 30%, transparent)`,
                      background: `color-mix(in srgb, ${codecCSSColor(w.source_codec)} 10%, transparent)`,
                    }}
                  >
                    {w.source_codec}
                  </Badge>
                )}
                <div className="text-sm font-semibold text-brand tnum shrink-0">
                  −{formatBytes(w.bytes_saved)}
                </div>
              </div>
            ))}
          </div>
        )}
      </div>

      <div className="mt-5">
        <ReplacementsCard report={report.replacements} />
      </div>
    </div>
  );
}

export default function Page() {
  return (
    <div className="flex flex-col min-w-0">
      <PageHeader
        title="Insights"
        subtitle="What your library has actually given back — measured from completed encodes and from files you replaced."
      />
      <Suspense fallback={<InsightsSkeleton />}>
        <InsightsContent />
      </Suspense>
    </div>
  );
}
