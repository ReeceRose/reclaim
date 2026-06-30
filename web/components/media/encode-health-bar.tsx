export function EncodeHealthBar({ fileCount, eligibleCount }: {
  fileCount: number;
  eligibleCount: number;
}) {
  const donePct = fileCount > 0 ? Math.round(Math.max(0, fileCount - eligibleCount) / fileCount * 100) : 100;
  return (
    <div className="w-full h-1 overflow-hidden rounded-full" style={{ background: 'var(--surface-3)' }}>
      <div
        className="h-full transition-[width] duration-500"
        style={{
          width: `${donePct}%`,
          background: donePct === 100 ? 'var(--green)' : 'linear-gradient(90deg, var(--green), var(--brand))',
        }}
      />
    </div>
  );
}
