'use client';

import { useSuspenseQuery, useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { api } from '@/lib/api';
import { formatBytes, formatInt, formatPct, resolutionBucketLabel } from '@/lib/format';
import { CODEC_COLORS, codecCSSColor } from '@/lib/codec';
import { Skeleton } from '@/components/ui/skeleton';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { toast } from 'sonner';
import Link from 'next/link';
import { Suspense } from 'react';

function DashboardSkeleton() {
  return (
    <div className="px-4 pt-5 pb-14 w-full sm:px-7 sm:pt-7">
      <div className="rounded-[18px] border border-line px-5 py-6 mb-6 sm:px-7 sm:py-7" style={{ background: 'var(--surface)' }}>
        <div className="flex items-center justify-between mb-6">
          <Skeleton className="h-3 w-40" />
          <Skeleton className="h-7 w-20 rounded-[10px]" />
        </div>
        <Skeleton className="h-16 w-52 mb-3" />
        <Skeleton className="h-4 w-80 mb-7" />
        <Skeleton className="h-8 w-full rounded-[11px] mb-3" />
        <div className="flex gap-6 mb-7">
          <Skeleton className="h-3 w-36" />
          <Skeleton className="h-3 w-36" />
          <Skeleton className="h-3 w-32" />
        </div>
        <div className="border-t border-line-soft pt-5 grid grid-cols-3 gap-4">
          {[0, 1, 2].map((i) => (
            <div key={i}>
              <Skeleton className="h-3 w-24 mb-2" />
              <Skeleton className="h-9 w-20" />
            </div>
          ))}
        </div>
      </div>
      <div className="grid grid-cols-[1.55fr_1fr] gap-5 max-sm:grid-cols-1">
        <div className="border border-line rounded-lg p-5" style={{ background: 'var(--surface)' }}>
          <Skeleton className="h-3 w-32 mb-5" />
          {[0, 1, 2, 3].map((i) => (
            <div key={i} className="flex items-center gap-3 mb-4">
              <Skeleton className="h-5 w-14 rounded-[7px]" />
              <Skeleton className="flex-1 h-[10px] rounded-[6px]" />
              <Skeleton className="h-4 w-24" />
            </div>
          ))}
        </div>
        <div className="border border-line rounded-lg p-5" style={{ background: 'var(--surface)' }}>
          <Skeleton className="h-3 w-24 mb-5" />
          {[0, 1, 2].map((i) => (
            <div key={i} className="flex items-center gap-3 mb-4">
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
  const qc = useQueryClient();
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

  const { data: isScanning } = useQuery<boolean>({
    queryKey: ['scanning'],
    queryFn: () => false,
    initialData: false,
    staleTime: Infinity,
    gcTime: Infinity,
  });

  const scanMutation = useMutation({
    mutationFn: api.scan,
    onSuccess: () => {
      qc.setQueryData(['scanning'], true);
      qc.setQueryData(['scan_progress'], null);
      toast.success('Scan started');
    },
    onError: () => toast.error('Scan failed to start'),
  });

  if (stats.total_files === 0) {
    return (
      <div className="px-4 pt-5 pb-14 w-full flex flex-col items-center justify-center min-h-[50vh] gap-4 text-center sm:px-7 sm:pt-7">
        <div
          className="w-14 h-14 rounded-[16px] border border-line grid place-items-center text-muted-dim"
          style={{ background: 'var(--surface-2)' }}
        >
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" className="w-6 h-6">
            <path d="M22 19a2 2 0 01-2 2H4a2 2 0 01-2-2V5a2 2 0 012-2h5l2 3h9a2 2 0 012 2z"/>
          </svg>
        </div>
        <div>
          <div className="text-[1rem] font-semibold text-text">No files indexed yet</div>
          <div className="text-[0.82rem] text-muted-dim mt-1 max-w-[280px]">Trigger a scan to get started — Reclaim will walk your library and rank files by predicted HEVC savings.</div>
        </div>
        <Button
          variant="outline"
          onClick={() => scanMutation.mutate()}
          disabled={scanMutation.isPending || !!isScanning}
          className="rounded-[10px] text-sm h-8 gap-1.5 mt-1"
        >
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" className="w-3.5 h-3.5">
            <polyline points="23 4 23 10 17 10"/><path d="M20.49 15a9 9 0 11-2.12-9.36L23 10"/>
          </svg>
          {scanMutation.isPending || isScanning ? 'Scanning…' : 'Scan now'}
        </Button>
      </div>
    );
  }

  return (
    <div className="px-4 pt-5 pb-14 w-full sm:px-7 sm:pt-7">
      <div
        className="rounded-[18px] border border-line px-5 py-6 mb-6 relative overflow-hidden sm:px-7 sm:py-7"
        style={{ background: 'radial-gradient(120% 150% at 100% 0%, var(--brand-soft), transparent 55%), var(--surface)' }}
      >
        <div className="absolute inset-0 pointer-events-none" style={{ background: 'repeating-linear-gradient(0deg, rgba(244,244,244,.022) 0 1px, transparent 1px 4px)' }} />

        <div className="flex items-center justify-between mb-5">
          <div className="text-xs text-muted-fg uppercase tracking-[0.13em] font-bold">Estimated recoverable</div>
          <Button
            variant="outline"
            size="sm"
            onClick={() => scanMutation.mutate()}
            disabled={scanMutation.isPending || !!isScanning}
            className="rounded-[10px] text-xs h-7 gap-1.5"
          >
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" className="w-3 h-3">
              <polyline points="23 4 23 10 17 10"/><path d="M20.49 15a9 9 0 11-2.12-9.36L23 10"/>
            </svg>
            {scanMutation.isPending || isScanning ? 'Scanning…' : 'Rescan'}
          </Button>
        </div>

        <div className="flex items-end justify-between gap-6 flex-wrap">
          <div>
            <div
              className="text-[2.8rem] sm:text-hero font-extrabold leading-none tracking-tight text-brand"
              style={{ textShadow: '0 4px 26px var(--brand-soft)' }}
            >
              {formatBytes(recoverable, 1).replace(' ', '')}
            </div>
            <div className="text-sm text-muted-fg mt-2">
              across{' '}
              <b className="text-text font-semibold">{formatInt(candidateCount)} candidates</b>
              {' '}· {formatInt(hevcCount)} already HEVC
              <Badge className="ml-2 text-[0.66rem] font-bold tracking-widest text-brand bg-brand-soft border-brand-line rounded-[6px] uppercase">estimate</Badge>
            </div>
          </div>
          <div className="text-right">
            <div className="text-xs text-muted-fg uppercase tracking-wider">Library total</div>
            <div className="text-[1.45rem] font-bold tracking-tight mt-0.5">{formatBytes(total)}</div>
          </div>
        </div>

        <div className="mt-6">
          <div className="h-8 rounded-[11px] bg-surface-2 flex overflow-hidden shadow-[inset_0_0_0_1px_var(--line)]">
            <div className="h-full transition-[width_1.2s_cubic-bezier(.34,1.4,.5,1)]" style={{ width: `${reclaimPct}%`, background: 'linear-gradient(180deg, var(--brand), var(--brand-2))', boxShadow: '0 0 22px var(--brand-soft)' }} />
            <div className="h-full bg-surface-3" style={{ width: `${keptPct}%` }} />
            <div className="h-full" style={{ width: `${hevcPct}%`, background: 'color-mix(in srgb, var(--green) 32%, transparent)' }} />
          </div>
          <div className="flex gap-5 mt-3 text-xs text-muted-fg flex-wrap">
            <span className="flex items-center gap-[7px]"><span className="w-[10px] h-[10px] rounded-[4px] bg-brand inline-block" />Reclaimable · {formatBytes(recoverable)} · {reclaimPct}%</span>
            <span className="flex items-center gap-[7px]"><span className="w-[10px] h-[10px] rounded-[4px] bg-surface-3 inline-block" />After encode · {formatBytes(kept)} · {keptPct}%</span>
            <span className="flex items-center gap-[7px]"><span className="w-[10px] h-[10px] rounded-[4px] inline-block" style={{ background: 'color-mix(in srgb, var(--green) 45%, transparent)' }} />Already HEVC · {formatBytes(hevcBytes)} · {hevcPct}%</span>
          </div>
        </div>

        <div className="mt-6 pt-5 border-t border-line-soft grid grid-cols-3 gap-4 max-sm:grid-cols-1">
          <div>
            <div className="text-xs text-muted-fg uppercase tracking-wider font-bold">Total files</div>
            <div className="text-stat font-bold tracking-tight mt-1">{formatInt(stats.total_files)}</div>
          </div>
          <div>
            <div className="text-xs text-muted-fg uppercase tracking-wider font-bold">Candidates</div>
            <div className="text-stat font-bold tracking-tight mt-1 text-brand">{formatInt(candidateCount)}</div>
            <div className="text-xs text-muted-dim mt-0.5">{formatPct(candidateCount, stats.total_files)} of library</div>
          </div>
          <div>
            <div className="text-xs text-muted-fg uppercase tracking-wider font-bold">Already HEVC</div>
            <div className="text-stat font-bold tracking-tight mt-1 text-green">{formatInt(hevcCount)}</div>
            <div className="text-xs text-muted-dim mt-0.5">{formatPct(hevcCount, stats.total_files)} of library</div>
          </div>
        </div>
      </div>

      <div className="grid grid-cols-[1.55fr_1fr] gap-5 max-sm:grid-cols-1">
        <div className="border border-line rounded-lg p-5" style={{ background: 'var(--surface)' }}>
          <div className="text-xs uppercase tracking-[0.11em] text-muted-fg font-bold mb-4">Codec breakdown</div>
          {stats.by_codec.map((c) => (
            <div key={c.codec} className="flex items-center gap-3 mb-3.5 last:mb-0 text-sm">
              <div className="min-w-[78px] shrink-0 font-semibold flex items-center gap-1.5 flex-wrap">
                <Badge
                  className={`font-mono text-xs rounded-[7px] font-semibold ${CODEC_COLORS[c.codec.toLowerCase()] ?? 'text-slate'}`}
                  style={{ borderColor: `color-mix(in srgb, ${codecCSSColor(c.codec)} 30%, transparent)`, background: `color-mix(in srgb, ${codecCSSColor(c.codec)} 10%, transparent)` }}
                >
                  {c.codec}
                </Badge>
              </div>
              <div className="flex-1 h-[10px] bg-surface-2 rounded-[6px] overflow-hidden">
                <div className="h-full rounded-[6px]" style={{ width: `${Math.round((c.file_count / maxCodecFiles) * 100)}%`, background: codecCSSColor(c.codec) }} />
              </div>
              <div className="w-[112px] sm:w-[148px] shrink-0 text-right text-muted-fg text-xs tnum">
                {formatInt(c.file_count)}<span className="hidden sm:inline"> ({formatPct(c.file_count, stats.total_files)})</span> · {formatBytes(c.total_bytes)}<span className="hidden sm:inline"> ({formatPct(c.total_bytes, total)})</span>
              </div>
            </div>
          ))}
        </div>
        <div className="border border-line rounded-lg p-5" style={{ background: 'var(--surface)' }}>
          <div className="text-xs uppercase tracking-[0.11em] text-muted-fg font-bold mb-4">Resolution</div>
          {stats.by_resolution.map((r) => (
            <div key={r.band} className="flex items-center gap-3 mb-3.5 last:mb-0 text-sm">
              <div className="w-[64px] sm:w-[78px] shrink-0 font-semibold">{resolutionBucketLabel(r.band)}</div>
              <div className="flex-1 h-[10px] bg-surface-2 rounded-[6px] overflow-hidden">
                <div className="h-full rounded-[6px] bg-sky" style={{ width: `${Math.round((r.file_count / maxResFiles) * 100)}%` }} />
              </div>
              <div className="w-[92px] sm:w-[104px] shrink-0 text-right text-muted-fg text-xs tnum">{formatInt(r.file_count)} ({formatPct(r.file_count, stats.total_files)})</div>
            </div>
          ))}
        </div>
      </div>

      <Link href="/candidates" className="mt-4 text-sm text-brand hover:underline inline-block">
        Review candidates →
      </Link>
    </div>
  );
}

export default function Page() {
  return (
    <div className="flex flex-col min-w-0">
      <Suspense fallback={<DashboardSkeleton />}>
        <DashboardContent />
      </Suspense>
    </div>
  );
}
