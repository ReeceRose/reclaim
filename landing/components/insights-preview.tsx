const codecs = [
  { key: "h264", files: 28, saved: "2.1 TB", width: 100, color: "var(--gold)" },
  {
    key: "mpeg2video",
    saved: "912 GB",
    width: 43,
    color: "var(--rose)",
  },
  { key: "vc1", files: 4, saved: "418 GB", width: 20, color: "var(--violet)" },
  {
    key: "unknown",
    saved: "394 GB",
    width: 19,
    color: "var(--slate)",
  },
];

const CURVE =
  "M0,86 L34,86 L34,74 L62,74 L62,69 L96,69 L96,54 L128,54 L128,50 L163,50 L163,41 L198,41 L198,37 L232,37 L232,26 L268,26 L268,20 L300,20";

export function InsightsPreview() {
  return (
    <div className="glow-brand rounded-lg border border-line bg-surface p-5">
      <div className="mb-4 flex items-baseline justify-between">
        <div className="text-2xs font-bold uppercase tracking-wider text-muted-fg">
          Storage reclaimed
        </div>
        <div className="text-2xs text-muted-dim">last 90 days</div>
      </div>

      <div className="flex items-end justify-between gap-4">
        <div>
          <div className="text-4xl font-extrabold leading-none tracking-tight text-brand">
            3.8TB
          </div>
          <div className="mt-1.5 text-xs text-muted-fg">
            across <span className="font-semibold text-text">41 encodes</span>
          </div>
        </div>
        <div className="text-right">
          <div className="text-2xs uppercase tracking-wider text-muted-fg">
            Avg reduction
          </div>
          <div className="text-xl font-bold tracking-tight tnum">46%</div>
        </div>
      </div>

      <svg
        viewBox="0 0 300 96"
        className="mt-4 block h-24 w-full"
        aria-hidden="true"
        preserveAspectRatio="none"
      >
        <title>Cumulative storage reclaimed</title>
        <defs>
          <linearGradient id="insights-fill" x1="0" y1="0" x2="0" y2="1">
            <stop offset="0%" stopColor="var(--brand)" stopOpacity="0.32" />
            <stop offset="100%" stopColor="var(--brand)" stopOpacity="0.02" />
          </linearGradient>
        </defs>
        <path d={`${CURVE} L300,96 L0,96 Z`} fill="url(#insights-fill)" />
        <path
          d={CURVE}
          fill="none"
          stroke="var(--brand)"
          strokeWidth="2"
          strokeLinecap="round"
          strokeLinejoin="round"
          vectorEffect="non-scaling-stroke"
        />
      </svg>

      <div className="mt-4 space-y-2.5 border-t border-line-soft pt-4">
        <div className="text-2xs font-bold uppercase tracking-wider text-muted-fg">
          By source codec
        </div>
        {codecs.map((c) => (
          <div key={c.key} className="flex items-center gap-3">
            <div className="w-20 shrink-0 font-mono text-2xs text-muted-fg">
              {c.key}
            </div>
            <div className="h-2 flex-1 overflow-hidden rounded bg-surface-2">
              <div
                className="h-full rounded"
                style={{ width: `${c.width}%`, background: c.color }}
              />
            </div>
            <div className="w-16 shrink-0 text-right text-2xs text-muted-fg tnum">
              {c.saved}
            </div>
          </div>
        ))}
      </div>

      <div className="mt-4 border-t border-line-soft pt-3.5 text-xs leading-relaxed text-muted-dim">
        Actual reclaim ran at{" "}
        <span className="font-semibold text-green">103%</span> of the estimate
        across 38 jobs.
      </div>
    </div>
  );
}
