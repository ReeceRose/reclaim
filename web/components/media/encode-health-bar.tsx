export function EncodeHealthBar({
  fileCount,
  eligibleCount,
  missingCount = 0,
}: {
  fileCount: number;
  eligibleCount: number;
  missingCount?: number;
}) {
  const activeCount = fileCount - missingCount;
  const donePct =
    activeCount > 0
      ? Math.round(
          (Math.max(0, activeCount - eligibleCount) / activeCount) * 100,
        )
      : 0;
  return (
    <div
      className="w-full h-1 overflow-hidden rounded-full"
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
    </div>
  );
}
