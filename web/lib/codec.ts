// Canonical codec → Tailwind class maps shared by every media surface
// (candidate browser, library, browse pages). Keeping a single source here
// avoids the badge palette drifting between screens.

export const CODEC_COLORS: Record<string, string> = {
  h264: 'text-gold',
  hevc: 'text-green',
  h265: 'text-green',
  mpeg2: 'text-rose',
  mpeg2video: 'text-rose',
  vc1: 'text-violet',
  av1: 'text-sky',
};

export const CODEC_CSS_COLORS: Record<string, string> = {
  h264: 'var(--gold)',
  hevc: 'var(--green)',
  h265: 'var(--green)',
  mpeg2: 'var(--rose)',
  mpeg2video: 'var(--rose)',
  vc1: 'var(--violet)',
  av1: 'var(--sky)',
};

export function codecCSSColor(codec: string): string {
  return CODEC_CSS_COLORS[codec.toLowerCase()] ?? 'var(--slate)';
}

export const CODEC_BORDER: Record<string, string> = {
  h264: 'border-[rgba(241,194,27,.3)] bg-[rgba(241,194,27,.1)]',
  hevc: 'border-green-soft bg-green-soft',
  h265: 'border-green-soft bg-green-soft',
  mpeg2: 'border-[rgba(255,126,182,.3)] bg-[rgba(255,126,182,.1)]',
  mpeg2video: 'border-[rgba(255,126,182,.3)] bg-[rgba(255,126,182,.1)]',
  vc1: 'border-[rgba(190,149,255,.3)] bg-[rgba(190,149,255,.1)]',
  av1: 'border-[rgba(51,177,255,.32)] bg-[rgba(51,177,255,.1)]',
};
