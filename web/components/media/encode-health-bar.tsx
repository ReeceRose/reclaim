export function EncodeHealthBar({
  fileCount,
  eligibleCount,
  queuedCount = 0,
  missingCount = 0,
}: {
  fileCount: number;
  eligibleCount: number;
  queuedCount?: number;
  missingCount?: number;
}) {
  const activeCount = fileCount - missingCount;
  const pct = (n: number) =>
    activeCount > 0 ? Math.round((Math.max(0, n) / activeCount) * 100) : 0;
  const donePct = pct(activeCount - eligibleCount - queuedCount);
  const queuedPct = pct(queuedCount);
  return (
    <div
      className="flex w-full h-1 overflow-hidden rounded-full"
      style={{ background: "var(--surface-3)" }}
    >
      <div
        className="h-full transition-[width] duration-500"
        style={{
          width: `${donePct}%`,
          background:
            donePct === 100
              ? "var(--green)"
              : "linear-gradient(90deg, var(--green), var(--brand))",
        }}
      />
      <div
        className="h-full transition-[width] duration-500"
        style={{ width: `${queuedPct}%`, background: "var(--sky)" }}
      />
    </div>
  );
}
