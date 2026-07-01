/** 24-hour clock face showing the encode window period. */
export function WindowArc({
  start,
  end,
  isOpen,
}: {
  start: string;
  end: string;
  isOpen: boolean;
}) {
  const SIZE = 44;
  const CX = SIZE / 2;
  const CY = SIZE / 2;
  const R = 16;
  const SW = 3;

  function toRad(hhmm: string): number {
    const [h, m] = hhmm.split(":").map(Number);
    return ((h * 60 + (m ?? 0)) / 1440) * 2 * Math.PI - Math.PI / 2;
  }

  const now = new Date();
  const nowRad =
    ((now.getHours() * 60 + now.getMinutes()) / 1440) * 2 * Math.PI -
    Math.PI / 2;

  const a1 = toRad(start);
  let a2 = toRad(end);
  if (a2 <= a1) a2 += 2 * Math.PI;

  const largeArc = a2 - a1 > Math.PI ? 1 : 0;
  const x1 = (CX + R * Math.cos(a1)).toFixed(2);
  const y1 = (CY + R * Math.sin(a1)).toFixed(2);
  const x2 = (CX + R * Math.cos(a2)).toFixed(2);
  const y2 = (CY + R * Math.sin(a2)).toFixed(2);
  const nx = (CX + R * Math.cos(nowRad)).toFixed(2);
  const ny = (CY + R * Math.sin(nowRad)).toFixed(2);

  return (
    <svg
      aria-hidden="true"
      width={SIZE}
      height={SIZE}
      viewBox={`0 0 ${SIZE} ${SIZE}`}
      className="shrink-0"
    >
      <circle
        cx={CX}
        cy={CY}
        r={R}
        fill="none"
        stroke="var(--line)"
        strokeWidth={SW}
      />
      <path
        d={`M ${x1} ${y1} A ${R} ${R} 0 ${largeArc} 1 ${x2} ${y2}`}
        fill="none"
        stroke={isOpen ? "var(--green)" : "var(--surface-3)"}
        strokeWidth={SW}
        strokeLinecap="round"
        style={
          isOpen ? { filter: "drop-shadow(0 0 3px var(--green))" } : undefined
        }
      />
      <circle cx={nx} cy={ny} r={2.2} fill="var(--brand)" />
    </svg>
  );
}
