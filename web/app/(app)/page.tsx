'use client';

import { useQuery, useSuspenseQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { api } from '@/lib/api';
import { formatBytes, formatInt } from '@/lib/format';
import { Skeleton } from '@/components/ui/skeleton';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { toast } from 'sonner';
import Link from 'next/link';
import { Suspense } from 'react';

const CODEC_COLORS: Record<string, string> = {
  h264: 'var(--gold)',
  hevc: 'var(--green)',
  mpeg2: 'var(--rose)',
  vc1: 'var(--violet)',
};

function codecColor(codec: string): string {
  return CODEC_COLORS[codec.toLowerCase()] ?? 'var(--slate)';
}

function codecClass(codec: string): string {
  const map: Record<string, string> = {
    h264: 'text-gold',
    hevc: 'text-green',
    mpeg2: 'text-rose',
    vc1: 'text-violet',
  };
  return map[codec.toLowerCase()] ?? 'text-slate';
}

function DashboardSkeleton() {
  return (
    <div className="px-7 py-[26px] w-full pb-14">
      <div className="rounded-[18px] border border-line px-7 py-[26px] mb-[18px]" style={{ background: 'var(--surface)' }}>
        <Skeleton className="h-16 w-44 mb-3" />
        <Skeleton className="h-4 w-72 mb-6" />
        <Skeleton className="h-8 w-full rounded-[11px]" />
        <div className="flex gap-5 mt-3">
          <Skeleton className="h-3 w-32" />
          <Skeleton className="h-3 w-32" />
          <Skeleton className="h-3 w-28" />
        </div>
      </div>
      <div className="grid grid-cols-3 gap-[18px] mb-[18px] max-sm:grid-cols-1">
        {[0, 1, 2].map((i) => (
          <div key={i} className="border border-line rounded-[var(--radius)] px-[18px] py-[17px]" style={{ background: 'var(--surface)' }}>
            <Skeleton className="h-3 w-24 mb-3" />
            <Skeleton className="h-9 w-20" />
          </div>
        ))}
      </div>
      <div className="grid grid-cols-[1.55fr_1fr] gap-[18px] max-sm:grid-cols-1">
        <div className="border border-line rounded-[var(--radius)] p-5" style={{ background: 'var(--surface)' }}>
          <Skeleton className="h-3 w-32 mb-5" />
          {[0, 1, 2, 3].map((i) => (
            <div key={i} className="flex items-center gap-3 mb-3.5">
              <Skeleton className="h-5 w-14 rounded-[7px]" />
              <Skeleton className="flex-1 h-[10px] rounded-[6px]" />
              <Skeleton className="h-4 w-24" />
            </div>
          ))}
        </div>
        <div className="border border-line rounded-[var(--radius)] p-5" style={{ background: 'var(--surface)' }}>
          <Skeleton className="h-3 w-24 mb-5" />
          {[0, 1, 2].map((i) => (
            <div key={i} className="flex items-center gap-3 mb-3.5">
              <Skeleton className="h-4 w-14" />
              <Skeleton className="flex-1 h-[10px] rounded-[6px]" />
              <Skeleton className="h-4 w-16" />
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}

function DashboardContent() {
  const { data: stats } = useSuspenseQuery({ queryKey: ['stats'], queryFn: api.stats, staleTime: 30_000 });

  const total = stats.total_bytes;
  const recoverable = stats.total_recoverable_bytes;
  const hevcBytes = stats.by_codec.find((c) => c.codec === 'hevc')?.total_bytes ?? 0;
  const kept = total - recoverable - hevcBytes;
  const reclaimPct = total > 0 ? Math.round((recoverable / total) * 100) : 0;
  const keptPct = total > 0 ? Math.round((kept / total) * 100) : 0;
  const hevcPct = total > 0 ? Math.round((hevcBytes / total) * 100) : 0;
  const hevcCount = stats.by_codec.find((c) => c.codec === 'hevc')?.file_count ?? 0;
  const candidateCount = stats.total_files - hevcCount;
  const maxCodecFiles = Math.max(...stats.by_codec.map((c) => c.file_count), 1);
  const maxResFiles = Math.max(...stats.by_resolution.map((r) => r.file_count), 1);

  return (
    <div className="px-7 py-[26px] w-full pb-14">
      <div
        className="rounded-[18px] border border-line px-7 py-[26px] mb-[18px] relative overflow-hidden"
        style={{ background: 'radial-gradient(120% 150% at 100% 0%, var(--brand-soft), transparent 55%), var(--surface)' }}
      >
        <div className="absolute inset-0 pointer-events-none" style={{ background: 'repeating-linear-gradient(0deg, rgba(244,244,244,.022) 0 1px, transparent 1px 4px)' }} />
        <div className="flex items-end justify-between gap-6 flex-wrap">
          <div>
            <div className="text-[3.8rem] font-extrabold leading-none tracking-tight text-brand" style={{ textShadow: '0 4px 26px var(--brand-soft)' }}>
              {formatBytes(recoverable, 1).replace(' ', '')}
            </div>
            <div className="text-[0.86rem] text-muted-fg mt-2">
              estimated recoverable across{' '}
              <b className="text-text font-semibold">{formatInt(candidateCount)} candidates</b>
              <Badge className="ml-2 text-[0.66rem] font-bold tracking-widest text-brand bg-brand-soft border-brand-line rounded-[6px] uppercase">estimate</Badge>
            </div>
          </div>
          <div className="text-right">
            <div className="text-[0.74rem] text-muted-fg uppercase tracking-wider">Library total</div>
            <div className="text-[1.45rem] font-bold tracking-tight mt-0.5">{formatBytes(total)}</div>
          </div>
        </div>
        <div className="mt-6">
          <div className="h-8 rounded-[11px] bg-surface-2 flex overflow-hidden shadow-[inset_0_0_0_1px_var(--line)]">
            <div className="h-full transition-[width_1.2s_cubic-bezier(.34,1.4,.5,1)]" style={{ width: `${reclaimPct}%`, background: 'linear-gradient(180deg, var(--brand), var(--brand-2))', boxShadow: '0 0 22px var(--brand-soft)' }} />
            <div className="h-full bg-surface-3" style={{ width: `${keptPct}%` }} />
            <div className="h-full" style={{ width: `${hevcPct}%`, background: 'color-mix(in srgb, var(--green) 32%, transparent)' }} />
          </div>
          <div className="flex gap-5 mt-3 text-[0.8rem] text-muted-fg flex-wrap">
            <span className="flex items-center gap-[7px]"><span className="w-[10px] h-[10px] rounded-[4px] bg-brand inline-block" />Reclaimable · {formatBytes(recoverable)}</span>
            <span className="flex items-center gap-[7px]"><span className="w-[10px] h-[10px] rounded-[4px] bg-surface-3 inline-block" />Stays after encode · {formatBytes(kept)}</span>
            <span className="flex items-center gap-[7px]"><span className="w-[10px] h-[10px] rounded-[4px] inline-block" style={{ background: 'color-mix(in srgb, var(--green) 45%, transparent)' }} />Already HEVC · {formatBytes(hevcBytes)}</span>
          </div>
        </div>
      </div>

      <div className="grid grid-cols-3 gap-[18px] mb-[18px] max-sm:grid-cols-1">
        {[
          { k: 'Candidates', v: formatInt(candidateCount), sub: 'non-HEVC, not queued' },
          { k: 'Already HEVC', v: formatInt(hevcCount), sub: 'excluded from candidates' },
          { k: 'Reclaimed to date', v: '—', sub: 'jobs completed' },
        ].map(({ k, v, sub }) => (
          <div key={k} className="border border-line rounded-[var(--radius)] px-[18px] py-[17px]" style={{ background: 'var(--surface)' }}>
            <div className="text-[0.74rem] text-muted-fg uppercase tracking-wider font-bold">{k}</div>
            <div className="text-[1.9rem] font-bold tracking-tight mt-1.5">{v}</div>
            <div className="text-[0.78rem] text-muted-dim mt-0.5">{sub}</div>
          </div>
        ))}
      </div>

      <div className="grid grid-cols-[1.55fr_1fr] gap-[18px] max-sm:grid-cols-1">
        <div className="border border-line rounded-[var(--radius)] p-5" style={{ background: 'var(--surface)' }}>
          <div className="text-[0.72rem] uppercase tracking-[0.11em] text-muted-fg font-bold mb-4">Codec breakdown</div>
          {stats.by_codec.map((c) => (
            <div key={c.codec} className="flex items-center gap-3 mb-3.5 last:mb-0 text-[0.84rem]">
              <div className="min-w-[78px] font-semibold flex items-center gap-1.5 flex-wrap">
                <Badge
                  className={`font-mono text-[0.7rem] rounded-[7px] font-semibold ${codecClass(c.codec)}`}
                  style={{ borderColor: `color-mix(in srgb, ${codecColor(c.codec)} 30%, transparent)`, background: `color-mix(in srgb, ${codecColor(c.codec)} 10%, transparent)` }}
                >
                  {c.codec}
                </Badge>
                {c.ratio_source === 'learned' && (
                  <Badge
                    className="text-[0.6rem] font-bold tracking-wide rounded-[5px] uppercase px-1 py-0 text-green border-green/30 bg-green/10"
                    title={`Savings ratio learned from ${c.learned_sample_count} completed jobs — more accurate than the seed estimate`}
                  >
                    learned
                  </Badge>
                )}
              </div>
              <div className="flex-1 h-[10px] bg-surface-2 rounded-[6px] overflow-hidden">
                <div className="h-full rounded-[6px]" style={{ width: `${Math.round((c.file_count / maxCodecFiles) * 100)}%`, background: codecColor(c.codec) }} />
              </div>
              <div className="w-[104px] text-right text-muted-fg text-[0.8rem]">{formatInt(c.file_count)} · {formatBytes(c.total_bytes)}</div>
            </div>
          ))}
        </div>
        <div className="border border-line rounded-[var(--radius)] p-5" style={{ background: 'var(--surface)' }}>
          <div className="text-[0.72rem] uppercase tracking-[0.11em] text-muted-fg font-bold mb-4">Resolution</div>
          {stats.by_resolution.map((r) => (
            <div key={r.band} className="flex items-center gap-3 mb-3.5 last:mb-0 text-[0.84rem]">
              <div className="w-[78px] font-semibold">{r.band}</div>
              <div className="flex-1 h-[10px] bg-surface-2 rounded-[6px] overflow-hidden">
                <div className="h-full rounded-[6px] bg-sky" style={{ width: `${Math.round((r.file_count / maxResFiles) * 100)}%` }} />
              </div>
              <div className="w-[104px] text-right text-muted-fg text-[0.8rem]">{formatInt(r.file_count)}</div>
            </div>
          ))}
        </div>
      </div>

      <Link href="/candidates" className="mt-4 text-[0.8rem] text-brand hover:underline inline-block">
        Review candidates →
      </Link>
    </div>
  );
}

export default function Page() {
  const qc = useQueryClient();
  const { data: stats } = useQuery({ queryKey: ['stats'], queryFn: api.stats, staleTime: 30_000 });
  const recoverable = stats?.total_recoverable_bytes ?? 0;

  const scanMutation = useMutation({
    mutationFn: api.scan,
    onSuccess: () => {
      toast.success('Scan started');
      setTimeout(() => qc.invalidateQueries({ queryKey: ['stats'] }), 2000);
    },
    onError: () => toast.error('Scan failed to start'),
  });

  return (
    <div className="flex flex-col min-w-0">
      <div
        className="flex items-center gap-4 px-7 py-[18px] border-b border-line sticky top-0 z-10"
        style={{ background: 'rgba(22,22,22,.82)', backdropFilter: 'blur(10px)' }}
      >
        <div>
          <div className="text-[1.38rem] font-bold tracking-tight">Library overview</div>
          <div className="text-[0.82rem] text-muted-fg mt-px">{stats ? `${formatInt(stats.total_files)} files` : '—'}</div>
        </div>
        <div className="ml-auto flex items-center gap-3">
          {recoverable > 0 && (
            <Badge className="gap-2 text-[0.82rem] font-semibold px-[13px] py-[7px] rounded-[10px] border-brand-line bg-brand-soft text-brand">
              <span className="w-[7px] h-[7px] rounded-full bg-brand" />
              {formatBytes(recoverable)} reclaimable
            </Badge>
          )}
          <Button
            variant="outline"
            onClick={() => scanMutation.mutate()}
            disabled={scanMutation.isPending}
            className="rounded-[11px] text-[0.86rem]"
          >
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" className="w-[15px] h-[15px]">
              <polyline points="23 4 23 10 17 10"/><path d="M20.49 15a9 9 0 11-2.12-9.36L23 10"/>
            </svg>
            Rescan
          </Button>
        </div>
      </div>

      <Suspense fallback={<DashboardSkeleton />}>
        <DashboardContent />
      </Suspense>
    </div>
  );
}
