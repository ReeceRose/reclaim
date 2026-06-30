import { type MediaFile } from '@/lib/api';
import { baseName, formatBytes, resolutionLabel } from '@/lib/format';
import { CodecBadge } from './codec-badge';

export function MovieRow({ file, onClick }: { file: MediaFile; onClick: () => void }) {
  const title = baseName(file.path).replace(/\.[^/.]+$/, '');
  const isConverted = file.candidate_state === 'already_hevc' || file.candidate_state === 'completed';
  const isCandidate = file.candidate_state === 'candidate';
  return (
    <div
      onClick={onClick}
      className="grid items-center gap-3 px-4 py-2.5 border-b border-line-soft last:border-b-0 cursor-pointer hover:bg-surface-2 transition-colors"
      style={{ gridTemplateColumns: '1fr auto auto auto auto' }}
    >
      <div className="min-w-0 truncate font-medium text-sm">{title}</div>
      <CodecBadge codec={file.video_codec} showUnknown />
      <span className="text-xs text-muted-dim hidden sm:inline">
        {file.width && file.height ? resolutionLabel(file.width, file.height) : '—'}
      </span>
      <span className="font-mono text-xs text-muted-fg hidden md:inline">{formatBytes(file.size_bytes)}</span>
      <div className="text-right w-24">
        {isCandidate && file.predicted_savings_bytes > 0
          ? <span className="text-xs font-semibold text-brand font-mono">-{formatBytes(file.predicted_savings_bytes)}</span>
          : isConverted
            ? <span className="text-xs font-medium text-green">Converted</span>
            : <span className="text-xs text-muted-dim">—</span>
        }
      </div>
    </div>
  );
}
