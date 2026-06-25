import { ArrowRight, Terminal } from "lucide-react";
import { GitHubIcon } from "@/components/github-icon";
import { site } from "@/lib/site";

function DashboardMock() {
  const codecs = [
    { label: "h264", pct: 100, color: "var(--gold)", meta: "8,214 (68%) · 41.2 TB (71%)" },
    { label: "hevc", pct: 38, color: "var(--green)", meta: "3,110 (26%) · 9.8 TB (17%)" },
    { label: "mpeg2", pct: 14, color: "var(--rose)", meta: "612 (5%) · 5.1 TB (9%)" },
    { label: "vc1", pct: 7, color: "var(--violet)", meta: "204 (2%) · 2.3 TB (4%)" },
  ];

  return (
    <div className="glow-brand relative overflow-hidden rounded-[18px] border border-line bg-surface p-5 sm:p-6">
      <div className="scanlines pointer-events-none absolute inset-0" />
      <div
        className="pointer-events-none absolute inset-0"
        style={{
          background:
            "radial-gradient(120% 150% at 100% 0%, var(--brand-soft), transparent 55%)",
        }}
      />

      <div className="relative">
        <div className="mb-4 flex items-center justify-between">
          <span className="text-2xs font-bold uppercase tracking-[0.13em] text-muted-fg">
            Estimated recoverable
          </span>
          <span className="rounded-md border border-brand-line bg-brand-soft px-1.5 py-0.5 text-2xs font-bold uppercase tracking-widest text-brand">
            estimate
          </span>
        </div>

        <div className="flex items-end justify-between gap-4">
          <div
            className="text-stat font-extrabold leading-none tracking-tight text-brand tnum sm:text-[2.75rem]"
            style={{ textShadow: "0 4px 26px var(--brand-soft)" }}
          >
            18.4TB
          </div>
          <div className="text-right">
            <div className="text-2xs uppercase tracking-wider text-muted-fg">
              Library total
            </div>
            <div className="text-lg font-bold tracking-tight tnum">58.4 TB</div>
          </div>
        </div>

        <div className="mt-5">
          <div className="flex h-7 overflow-hidden rounded-[10px] bg-surface-2 shadow-[inset_0_0_0_1px_var(--line)]">
            <div
              className="h-full"
              style={{
                width: "31%",
                background: "linear-gradient(180deg, var(--brand), var(--brand-2))",
                boxShadow: "0 0 22px var(--brand-soft)",
              }}
            />
            <div className="h-full bg-surface-3" style={{ width: "52%" }} />
            <div
              className="h-full"
              style={{
                width: "17%",
                background: "color-mix(in srgb, var(--green) 32%, transparent)",
              }}
            />
          </div>
          <div className="mt-3 flex flex-wrap gap-x-5 gap-y-1.5 text-2xs text-muted-fg">
            <span className="flex items-center gap-1.5">
              <span className="inline-block h-2.5 w-2.5 rounded-[4px] bg-brand" />
              Reclaimable · 18.4 TB · 31%
            </span>
            <span className="flex items-center gap-1.5">
              <span className="inline-block h-2.5 w-2.5 rounded-[4px] bg-surface-3" />
              After encode · 30.2 TB · 52%
            </span>
            <span className="flex items-center gap-1.5">
              <span
                className="inline-block h-2.5 w-2.5 rounded-[4px]"
                style={{ background: "color-mix(in srgb, var(--green) 45%, transparent)" }}
              />
              Already HEVC · 9.8 TB · 17%
            </span>
          </div>
        </div>

        <div className="mt-5 border-t border-line-soft pt-4">
          <div className="mb-3 text-2xs font-bold uppercase tracking-[0.11em] text-muted-fg">
            Codec breakdown
          </div>
          <div className="flex flex-col gap-2.5">
            {codecs.map((c) => (
              <div key={c.label} className="flex items-center gap-3 text-xs">
                <span
                  className="min-w-[52px] rounded-md border px-1.5 py-0.5 text-center font-mono font-semibold"
                  style={{
                    color: c.color,
                    borderColor: `color-mix(in srgb, ${c.color} 30%, transparent)`,
                    background: `color-mix(in srgb, ${c.color} 10%, transparent)`,
                  }}
                >
                  {c.label}
                </span>
                <div className="h-2.5 flex-1 overflow-hidden rounded-[6px] bg-surface-2">
                  <div
                    className="h-full rounded-[6px]"
                    style={{ width: `${c.pct}%`, background: c.color }}
                  />
                </div>
                <span className="w-[148px] text-right text-2xs text-muted-fg tnum">
                  {c.meta}
                </span>
              </div>
            ))}
          </div>
        </div>
      </div>
    </div>
  );
}

export function Hero() {
  return (
    <section id="top" className="relative overflow-hidden border-b border-line">
      <div className="bg-grid pointer-events-none absolute inset-0 opacity-60" />
      <div
        className="pointer-events-none absolute inset-x-0 top-0 h-[420px]"
        style={{
          background:
            "radial-gradient(60% 100% at 50% 0%, var(--brand-soft), transparent 70%)",
        }}
      />

      <div className="relative mx-auto grid max-w-6xl items-center gap-12 px-5 py-16 sm:px-7 sm:py-24 lg:grid-cols-[1.05fr_1fr] lg:gap-10">
        <div className="animate-float-up">
          <a
            href={site.repo}
            target="_blank"
            rel="noreferrer"
            className="inline-flex items-center gap-2 rounded-full border border-line bg-surface px-3 py-1 text-xs text-muted-fg transition-colors hover:text-text"
          >
            <span className="inline-block h-1.5 w-1.5 rounded-full bg-green" />
            Open source · self-hosted · single container
          </a>

          <h1 className="mt-5 text-4xl font-extrabold leading-[1.05] tracking-tight sm:text-5xl lg:text-[3.4rem]">
            Reclaim disk space from your{" "}
            <span className="text-brand">media library</span> — safely.
          </h1>

          <p className="mt-5 max-w-xl text-base leading-relaxed text-muted-fg sm:text-lg">
            Point it at the same movie and TV folders Plex, Jellyfin, or Emby
            already use. Reclaim ranks files by predicted HEVC savings and lets
            you manually queue overnight <span className="font-mono text-text">ffmpeg</span>{" "}
            re-encodes — replacing files in place only after verification.
          </p>

          <div className="mt-7 flex flex-wrap items-center gap-3">
            <a
              href="#install"
              className="inline-flex h-11 items-center gap-2 rounded-lg bg-brand px-5 text-sm font-semibold text-on-brand transition-colors hover:bg-brand-2"
            >
              <Terminal className="h-4 w-4" />
              Quick start
            </a>
            <a
              href={site.repo}
              target="_blank"
              rel="noreferrer"
              className="inline-flex h-11 items-center gap-2 rounded-lg border border-line bg-surface px-5 text-sm font-semibold text-text transition-colors hover:bg-surface-2"
            >
              <GitHubIcon className="h-4 w-4" />
              View on GitHub
              <ArrowRight className="h-4 w-4 text-muted-fg" />
            </a>
          </div>

          <div className="mt-7 flex flex-wrap gap-x-6 gap-y-2 text-xs text-muted-fg">
            <span>No database server</span>
            <span className="text-line">·</span>
            <span>No Redis</span>
            <span className="text-line">·</span>
            <span>No sidecars</span>
            <span className="text-line">·</span>
            <span>Just Docker + your library</span>
          </div>
        </div>

        <div className="animate-float-up [animation-delay:120ms]">
          <DashboardMock />
        </div>
      </div>
    </section>
  );
}
