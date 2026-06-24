'use client';

import { useQuery, useSuspenseQuery } from '@tanstack/react-query';
import { api, type CandidateFilters } from '@/lib/api';
import { formatBytes, formatInt } from '@/lib/format';
import { useState, Suspense } from 'react';
import Link from 'next/link';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';

function DryRunSkeleton() {
  return (
    <div className="px-7 py-[26px] w-full pb-14">
      <div className="border border-line rounded-[var(--radius)] p-5 mb-[18px]" style={{ background: 'var(--surface)' }}>
        <Skeleton className="h-3 w-24 mb-4" />
        <div className="flex items-center gap-[10px] flex-wrap">
          <Skeleton className="h-9 w-32 rounded-[11px]" />
          <Skeleton className="h-9 w-32 rounded-[11px]" />
          <Skeleton className="h-9 w-40 rounded-[11px]" />
          <Skeleton className="h-9 w-28 rounded-[11px]" />
        </div>
      </div>
      <div className="text-center py-12">
        <Skeleton className="h-4 w-64 mx-auto" />
      </div>
    </div>
  );
}

function DryRunContent() {
  const [codec, setCodec] = useState('');
  const [library, setLibrary] = useState('');
  const [profileId, setProfileId] = useState<number | null>(null);

  const [queryFilters, setQueryFilters] = useState<CandidateFilters | null>(null);

  const { data: profilesData } = useSuspenseQuery({
    queryKey: ['profiles'],
    queryFn: api.profiles,
    staleTime: 60_000,
  });
  const profiles = profilesData.items ?? [];

  const { data: result, isFetching } = useQuery({
    queryKey: ['dry-run', queryFilters, profileId],
    queryFn: () => api.dryRun({
      video_codec: queryFilters?.video_codec,
      library_type: queryFilters?.library_type,
    }),
    enabled: queryFilters !== null,
    staleTime: 0,
  });

  function handleCalculate() {
    setQueryFilters({ video_codec: codec || undefined, library_type: library || undefined });
  }

  const total = result?.total_bytes ?? 0;
  const savings = result?.predicted_savings_bytes ?? 0;
  const reclaimPct = total > 0 ? Math.round((savings / total) * 100) : 0;
  const keptPct = 100 - reclaimPct;
  const selectedProfile = profiles.find((p) => p.id === profileId) ?? profiles.find((p) => p.is_default) ?? profiles[0];

  return (
      <div className="px-7 py-[26px] w-full pb-14">
        <div className="border border-line rounded-[var(--radius)] p-5 mb-[18px]" style={{ background: 'var(--surface)' }}>
          <div className="text-[0.72rem] uppercase tracking-[0.11em] text-muted-fg font-bold mb-4">Define a set</div>
          <div className="flex items-center gap-[10px] flex-wrap">
            <Select value={codec || 'all'} onValueChange={(v) => setCodec(v === 'all' ? '' : v)}>
              <SelectTrigger className="rounded-[11px] text-[0.84rem] h-auto py-[9px]">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="all">All codecs</SelectItem>
                <SelectItem value="h264">h264 only</SelectItem>
                <SelectItem value="mpeg2">mpeg2 only</SelectItem>
                <SelectItem value="vc1">vc1 only</SelectItem>
              </SelectContent>
            </Select>
            <Select value={library || 'all'} onValueChange={(v) => setLibrary(v === 'all' ? '' : v)}>
              <SelectTrigger className="rounded-[11px] text-[0.84rem] h-auto py-[9px]">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="all">All libraries</SelectItem>
                <SelectItem value="movies">Movies only</SelectItem>
                <SelectItem value="tv">TV only</SelectItem>
              </SelectContent>
            </Select>
            {profiles.length > 0 && (
              <Select
                value={String(profileId ?? profiles[0]?.id ?? '')}
                onValueChange={(v) => setProfileId(v ? Number(v) : null)}
              >
                <SelectTrigger className="rounded-[11px] text-[0.84rem] h-auto py-[9px]">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {profiles.map((p) => (
                    <SelectItem key={p.id} value={String(p.id)}>
                      Profile: {p.name}{p.is_default ? ' (default)' : ''}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            )}
            <Button
              variant="outline"
              onClick={handleCalculate}
              disabled={isFetching}
              className="rounded-[11px] text-[0.86rem] h-auto py-[9px]"
            >
              {isFetching ? 'Calculating…' : 'Recalculate'}
            </Button>
          </div>
        </div>

        {result && (
          <>
            <div
              className="rounded-[18px] border border-line px-7 py-[26px] relative overflow-hidden"
              style={{
                background:
                  'radial-gradient(120% 150% at 100% 0%, var(--brand-soft), transparent 55%), var(--surface)',
              }}
            >
              <div
                className="absolute inset-0 pointer-events-none"
                style={{ background: 'repeating-linear-gradient(0deg, rgba(244,244,244,.022) 0 1px, transparent 1px 4px)' }}
              />
              <div className="flex items-end justify-between gap-6 flex-wrap">
                <div>
                  <div
                    className="text-[3.8rem] font-extrabold leading-none tracking-tight text-brand"
                    style={{ textShadow: '0 4px 26px var(--brand-soft)' }}
                  >
                    {formatBytes(savings, 1).replace(' ', '')}
                  </div>
                  <div className="text-[0.86rem] text-muted-fg mt-2">
                    projected recoverable from{' '}
                    <b className="text-text font-semibold">{formatInt(result.file_count)} files</b>
                    {selectedProfile && <> · {selectedProfile.name}</>}
                    <Badge className="ml-2 text-[0.66rem] font-bold tracking-widest text-brand bg-brand-soft border-brand-line rounded-[6px] uppercase">
                      estimate
                    </Badge>
                  </div>
                </div>
                <div className="text-right">
                  <div className="text-[0.74rem] text-muted-fg uppercase tracking-wider">Set total</div>
                  <div className="text-[1.45rem] font-bold tracking-tight mt-0.5">
                    {formatBytes(total)} → {formatBytes(total - savings)}
                  </div>
                </div>
              </div>
              <div className="mt-6">
                <div className="h-8 rounded-[11px] bg-surface-2 flex overflow-hidden shadow-[inset_0_0_0_1px_var(--line)]">
                  <div
                    className="h-full"
                    style={{ width: `${reclaimPct}%`, background: 'linear-gradient(180deg, var(--brand), var(--brand-2))', boxShadow: '0 0 22px var(--brand-soft)' }}
                  />
                  <div className="h-full bg-surface-3" style={{ width: `${keptPct}%` }} />
                </div>
                <div className="flex gap-5 mt-3 text-[0.8rem] text-muted-fg flex-wrap">
                  <span className="flex items-center gap-[7px]">
                    <span className="w-[10px] h-[10px] rounded-[4px] bg-brand inline-block" />
                    Recovered · {formatBytes(savings)}
                  </span>
                  <span className="flex items-center gap-[7px]">
                    <span className="w-[10px] h-[10px] rounded-[4px] bg-surface-3 inline-block" />
                    Resulting size · {formatBytes(total - savings)}
                  </span>
                </div>
              </div>
              <div className="mt-5 flex justify-end">
                <Button
                  asChild
                  className="rounded-[11px]"
                  style={{ background: 'linear-gradient(145deg, var(--brand), var(--brand-2))', boxShadow: '0 4px 14px var(--brand-soft)' }}
                >
                  <Link href="/candidates">Review &amp; queue this set →</Link>
                </Button>
              </div>
            </div>
            <p className="text-[0.8rem] text-muted-dim mt-3">
              Estimates use per-codec ratios that refine as your own jobs complete. Nothing here touches a file.
            </p>
          </>
        )}

        {queryFilters === null && (
          <div className="text-center text-muted-dim text-sm py-12">
            Select a set and click Recalculate to see projections.
          </div>
        )}
      </div>
  );
}

export default function Page() {
  return (
    <div className="flex flex-col min-w-0">
      <div
        className="flex items-center gap-4 px-7 py-[18px] border-b border-line"
        style={{ background: 'rgba(22,22,22,.82)', backdropFilter: 'blur(10px)' }}
      >
        <div>
          <div className="text-[1.38rem] font-bold tracking-tight">Dry-run savings</div>
          <div className="text-[0.82rem] text-muted-fg mt-px">Project a batch before committing. Queues nothing.</div>
        </div>
      </div>
      <Suspense fallback={<DryRunSkeleton />}>
        <DryRunContent />
      </Suspense>
    </div>
  );
}
