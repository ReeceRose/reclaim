import type { CSSProperties } from "react";

/**
 * Reclaim brand mark: a downward "reclaim" arrow landing in a drive/container
 * with a freed/verified savings meter. Self-contained SVG (no external fonts),
 * safe to render at any size from a 16px favicon up.
 *
 * `idPrefix` keeps the gradient/pattern ids unique when multiple marks render
 * on the same page (duplicate SVG ids would otherwise collide).
 */
export function LogoMark({
  size = 32,
  className,
  style,
  idPrefix = "reclaim",
}: {
  size?: number;
  className?: string;
  style?: CSSProperties;
  idPrefix?: string;
}) {
  const bg = `${idPrefix}-bg`;
  const scan = `${idPrefix}-scan`;
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 256 256"
      fill="none"
      xmlns="http://www.w3.org/2000/svg"
      role="img"
      aria-label="Reclaim"
      className={className}
      style={style}
    >
      <defs>
        <linearGradient id={bg} x1="0" y1="0" x2="0" y2="1">
          <stop offset="0" stopColor="#1e1e1e" />
          <stop offset="1" stopColor="#141414" />
        </linearGradient>
        <pattern id={scan} width="4" height="4" patternUnits="userSpaceOnUse">
          <rect width="4" height="2" fill="#ffffff" opacity="0.018" />
        </pattern>
      </defs>

      <rect x="0" y="0" width="256" height="256" rx="58" fill={`url(#${bg})`} />
      <rect
        x="0"
        y="0"
        width="256"
        height="256"
        rx="58"
        fill={`url(#${scan})`}
      />
      <rect
        x="0.75"
        y="0.75"
        width="254.5"
        height="254.5"
        rx="57.25"
        fill="none"
        stroke="#ffffff"
        strokeOpacity="0.06"
      />

      <rect
        x="60"
        y="62"
        width="136"
        height="132"
        rx="18"
        fill="none"
        stroke="#2e6ce0"
        strokeWidth="11"
      />

      <g
        stroke="#4589ff"
        strokeWidth="13"
        strokeLinecap="round"
        strokeLinejoin="round"
        fill="none"
      >
        <line x1="128" y1="86" x2="128" y2="138" />
        <polyline points="104,116 128,140 152,116" />
      </g>

      <rect x="80" y="162" width="96" height="16" rx="8" fill="#262626" />
      <rect x="80" y="162" width="44" height="16" rx="8" fill="#4589ff" />
      <rect x="80" y="162" width="20" height="16" rx="8" fill="#42be65" />
    </svg>
  );
}
