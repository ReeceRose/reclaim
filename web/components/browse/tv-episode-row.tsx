'use client';

import Link from 'next/link';
import { type Episode } from '@/lib/api';
import { baseName, formatBytes, resolutionLabel } from '@/lib/format';
import { cn } from '@/lib/utils';
import { CodecBadge } from '@/components/media/codec-badge';
import { StateBadge } from '@/components/media/candidate-state';

export function TvEpisodeRow({ ep, href }: { ep: Episode; href: string }) {
  const dimmed = ep.candidate_state === 'already_hevc' || ep.candidate_state === 'completed';
  return (
    <Link
      href={href}
      className={cn(
        'grid items-center gap-3 px-4 py-2.5 border-b border-line-soft last:border-b-0 text-sm',
        'grid-cols-[1fr_auto_auto_auto_auto]',
        'cursor-pointer hover:bg-surface-2 transition-colors',
        dimmed && 'opacity-60',
      )}
    >
      <div className="min-w-0 truncate font-medium">{baseName(ep.path)}</div>
      <CodecBadge codec={ep.video_codec} />
      <span className="text-muted-dim hidden sm:inline">
        {ep.width && ep.height ? resolutionLabel(ep.width, ep.height) : '—'}
      </span>
      <span className="text-muted-fg font-mono hidden md:inline">{formatBytes(ep.size_bytes)}</span>
      <div className="text-right w-20">
        {ep.candidate_state === 'candidate' && ep.predicted_savings_bytes > 0
          ? <span className="text-brand font-semibold font-mono">-{formatBytes(ep.predicted_savings_bytes)}</span>
          : <StateBadge state={ep.candidate_state} />
        }
      </div>
    </Link>
  );
}
