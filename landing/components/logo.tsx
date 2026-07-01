// Inline SVG marks (mirrors web/public/logo-mark.svg) so the nav/footer render
// instantly with no image request and inherit crisp rendering at any size.

export function LogoMark({ className }: { className?: string }) {
  return (
    <svg
      viewBox="0 0 256 256"
      className={className}
      role="img"
      aria-label="Reclaim"
    >
      <defs>
        <linearGradient id="reclaim-bg" x1="0" y1="0" x2="0" y2="1">
          <stop offset="0" stopColor="#1e1e1e" />
          <stop offset="1" stopColor="#141414" />
        </linearGradient>
      </defs>
      <rect
        x="0"
        y="0"
        width="256"
        height="256"
        rx="58"
        fill="url(#reclaim-bg)"
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

export function Wordmark({ className }: { className?: string }) {
  return (
    <span className={className}>
      <span className="text-text">Re</span>
      <span className="text-brand">claim</span>
    </span>
  );
}
