import Image from 'next/image';
import { tmdbImageURL, type MediaFile } from '@/lib/api';
import { baseName, formatBytes, resolutionLabel } from '@/lib/format';
import { CodecBadge } from './codec-badge';

export function MovieCard({ file, onClick }: { file: MediaFile; onClick: () => void }) {
  const title = baseName(file.path).replace(/\.[^/.]+$/, '');
  const isConverted = file.candidate_state === 'already_hevc' || file.candidate_state === 'completed';
  const isCandidate = file.candidate_state === 'candidate';
  const imageURL = tmdbImageURL(file.backdrop_path, 'w780') ?? tmdbImageURL(file.poster_path, 'w342');

  return (
    <div
      onClick={onClick}
      className="relative bg-surface border border-line rounded-2xl overflow-hidden cursor-pointer hover:border-[var(--brand-line)] transition-[border-color] group"
    >
      <div className="relative h-48 overflow-hidden" style={{ background: 'var(--surface-2)' }}>
        {imageURL ? (
          <>
            <Image
              src={imageURL}
              alt={title}
              fill
              sizes="(max-width: 640px) 50vw, (max-width: 1024px) 33vw, 240px"
              className="object-cover transition-transform duration-300 group-hover:scale-105"
            />
            <div className="absolute inset-0" style={{ background: 'linear-gradient(to bottom, transparent 35%, rgba(10,10,10,0.88) 100%)' }} />
            <div className="absolute bottom-0 left-0 right-0 px-3 pb-2.5">
              <div className="font-bold text-sm leading-snug line-clamp-2 text-white drop-shadow">{title}</div>
            </div>
          </>
        ) : (
          <>
            <div className="w-full h-full flex items-center justify-center gap-1.5 flex-wrap px-3">
              <CodecBadge codec={file.video_codec} showUnknown />
              {file.width && file.height && (
                <span className="text-xs text-muted-dim">{resolutionLabel(file.width, file.height)}</span>
              )}
            </div>
            <div className="absolute bottom-0 left-0 right-0 px-3 pb-2.5">
              <div className="font-bold text-sm leading-snug line-clamp-2">{title}</div>
            </div>
          </>
        )}
      </div>

      <div className="px-3 pt-2 pb-3 flex flex-col gap-1">
        <div className="flex items-center gap-1.5 flex-wrap">
          <CodecBadge codec={file.video_codec} showUnknown />
          {file.width && file.height && (
            <span className="text-xs text-muted-dim">{resolutionLabel(file.width, file.height)}</span>
          )}
        </div>
        <div className="flex items-center justify-between gap-2">
          <span className="text-xs text-muted-fg font-mono">{formatBytes(file.size_bytes)}</span>
          {isCandidate && file.predicted_savings_bytes > 0
            ? <span className="text-xs font-semibold text-brand">-{formatBytes(file.predicted_savings_bytes)}</span>
            : isConverted
              ? <span className="text-xs font-medium text-green">Converted</span>
              : null
          }
        </div>
      </div>
      <div className="h-1 w-full" style={{ background: isConverted ? 'var(--green)' : isCandidate ? 'var(--brand)' : 'var(--surface-3)' }} />
    </div>
  );
}
