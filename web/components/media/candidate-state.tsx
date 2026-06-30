import { Badge } from '@/components/ui/badge';
import type { CandidateState, MediaFile } from '@/lib/api';

export const STATE_OPTIONS: { value: CandidateState; label: string }[] = [
  { value: 'candidate', label: 'Candidate' },
  { value: 'already_hevc', label: 'Already HEVC' },
  { value: 'probe_failed', label: 'Probe failed' },
  { value: 'unknown_codec', label: 'Unknown codec' },
  { value: 'queued', label: 'Queued' },
  { value: 'completed', label: 'Completed' },
  { value: 'missing', label: 'Missing' },
];

/** isQueueable reports whether a file is eligible to be queued for encoding. */
export function isQueueable(file: MediaFile): boolean {
  return file.candidate_state === 'candidate';
}

/** stateLabel resolves a candidate state to its human-readable label. */
export function stateLabel(state: CandidateState): string {
  return STATE_OPTIONS.find((o) => o.value === state)?.label ?? state;
}

/** queueBlockReason explains, in a few words, why a file cannot be queued. */
export function queueBlockReason(file: MediaFile): string {
  switch (file.candidate_state) {
    case 'already_hevc':
      return 'Already HEVC';
    case 'probe_failed':
      return 'Probe failed';
    case 'unknown_codec':
      return 'Unknown codec';
    case 'queued':
      return 'Already queued';
    case 'completed':
      return 'Already completed';
    case 'missing':
      return 'Missing from disk';
    default:
      return '';
  }
}

/** StateBadge renders a candidate state as a colour-coded pill. */
export function StateBadge({ state }: { state: CandidateState }) {
  const cls =
    state === 'candidate'
      ? 'text-brand border-brand-line bg-brand-soft'
      : state === 'already_hevc' || state === 'completed'
        ? 'text-green border-green-soft bg-green-soft'
        : state === 'probe_failed'
          ? 'text-red border-[rgba(255,120,120,.28)] bg-[rgba(255,120,120,.09)]'
          : state === 'queued'
            ? 'text-sky border-[rgba(51,177,255,.32)] bg-[rgba(51,177,255,.1)]'
            : 'text-muted-fg border-line bg-surface-3';
  return <Badge className={`text-[0.7rem] rounded-[7px] font-semibold ${cls}`}>{stateLabel(state)}</Badge>;
}
