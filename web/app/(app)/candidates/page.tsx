'use client';

import { useInfiniteQuery, useQuery, useSuspenseQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { useVirtualizer } from '@tanstack/react-virtual';
import { api, type MediaFile, type CandidateFilters, type Profile, type Episode, type SeriesGroup } from '@/lib/api';
import { formatBytes, formatInt, resolutionLabel, baseName, dirName } from '@/lib/format';
import { cn } from '@/lib/utils';
import { Skeleton } from '@/components/ui/skeleton';
import { toast } from 'sonner';
import { useRef, useState, useEffect, useMemo, useCallback, useTransition, Suspense } from 'react';
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from '@/components/ui/dialog';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Checkbox } from '@/components/ui/checkbox';
import { Input } from '@/components/ui/input';

const PAGE_SIZE = 100;

type SortKey = 'savings_desc' | 'size_desc' | 'mtime_asc' | 'codec';
type ViewMode = 'flat' | 'grouped';

const SORT_OPTIONS: { value: SortKey; label: string }[] = [
  { value: 'savings_desc', label: 'Predicted savings' },
  { value: 'size_desc', label: 'Largest file' },
  { value: 'mtime_asc', label: 'Oldest file' },
  { value: 'codec', label: 'Source codec' },
];

const CODEC_OPTIONS = [
  { value: 'h264', label: 'H.264' },
  { value: 'hevc', label: 'HEVC' },
  { value: 'mpeg2', label: 'MPEG-2' },
  { value: 'vc1', label: 'VC-1' },
];

const RESOLUTION_OPTIONS = [
  { value: 'uhd', label: '4K' },
  { value: 'hd', label: 'HD' },
  { value: 'sd', label: 'SD' },
];

const LIBRARY_OPTIONS = [
  { value: 'movies', label: 'Movies' },
  { value: 'tv', label: 'TV' },
];

function FilterSelect({
  label,
  value,
  options,
  onChange,
  className,
}: {
  label: string;
  value: string;
  options: { value: string; label: string }[];
  onChange: (v: string) => void;
  className?: string;
}) {
  const active = value !== '';
  const selectedLabel = options.find((o) => o.value === value)?.label;
  return (
    <Select value={value || '_all'} onValueChange={(v) => onChange(v === '_all' ? '' : v)}>
      <SelectTrigger
        className={cn(
          'rounded-[11px] text-[0.84rem] h-auto py-[9px] gap-1 transition-colors',
          active
            ? 'border-[color-mix(in_srgb,var(--brand)_45%,transparent)] bg-[color-mix(in_srgb,var(--brand)_7%,transparent)]'
            : 'bg-surface',
          className,
        )}
      >
        <span className={cn('text-[0.78rem] flex-shrink-0', active ? 'text-brand/60' : 'text-muted-dim')}>{label}</span>
        {active ? (
          <>
            <span className="text-muted-dim mx-px">·</span>
            <span className={cn('font-medium', active && 'text-brand')}>{selectedLabel}</span>
          </>
        ) : (
          <span className="text-muted-fg">All</span>
        )}
      </SelectTrigger>
      <SelectContent>
        <SelectItem value="_all">All</SelectItem>
        {options.map((o) => (
          <SelectItem key={o.value} value={o.value}>{o.label}</SelectItem>
        ))}
      </SelectContent>
    </Select>
  );
}

const CODEC_COLORS: Record<string, string> = {
  h264: 'text-gold',
  hevc: 'text-green',
  mpeg2: 'text-rose',
  vc1: 'text-violet',
};
const CODEC_BORDER: Record<string, string> = {
  h264: 'border-[rgba(241,194,27,.3)] bg-[rgba(241,194,27,.1)]',
  hevc: 'border-green-soft bg-green-soft',
  mpeg2: 'border-[rgba(255,126,182,.3)] bg-[rgba(255,126,182,.1)]',
  vc1: 'border-[rgba(190,149,255,.3)] bg-[rgba(190,149,255,.1)]',
};

function CodecBadge({ codec }: { codec: string | null }) {
  if (!codec) return null;
  const c = codec.toLowerCase();
  return (
    <Badge
      className={`font-mono text-[0.7rem] rounded-[7px] font-semibold ${CODEC_COLORS[c] ?? 'text-slate'} ${CODEC_BORDER[c] ?? 'border-line bg-surface-3'}`}
    >
      {codec}
    </Badge>
  );
}

function ConfirmDialog({
  open,
  onClose,
  selectedFiles,
  profiles,
  onConfirm,
}: {
  open: boolean;
  onClose: () => void;
  selectedFiles: MediaFile[];
  profiles: Profile[];
  onConfirm: (profileId: number | null) => Promise<void>;
}) {
  const defaultProfile = profiles.find((p) => p.is_default) ?? profiles[0];
  const [profileId, setProfileId] = useState<number | null>(null);
  const [loading, setLoading] = useState(false);

  const totalSavings = selectedFiles.reduce((s, f) => s + f.predicted_savings_bytes, 0);
  const preview = selectedFiles.slice(0, 8);
  const more = selectedFiles.length - preview.length;

  async function handleConfirm() {
    setLoading(true);
    try {
      await onConfirm(profileId ?? defaultProfile?.id ?? null);
      onClose();
    } finally {
      setLoading(false);
    }
  }

  return (
    <Dialog open={open} onOpenChange={(o) => !o && onClose()}>
      <DialogContent
        className="max-w-[540px] p-0 overflow-hidden border-line"
        style={{ background: 'var(--surface)' }}
      >
        <DialogHeader className="px-6 pt-[22px] pb-4 border-b border-line">
          <DialogTitle className="text-[1.2rem] font-bold tracking-tight">Confirm queue</DialogTitle>
          <p className="text-[0.85rem] text-muted-fg mt-1">Review the selection. Nothing runs until you confirm.</p>
        </DialogHeader>

        <div className="px-6 py-5 max-h-[300px] overflow-auto">
          <div className="flex gap-6 mb-[18px] flex-wrap">
            <div>
              <div className="text-[0.72rem] uppercase tracking-wider text-muted-fg">Files</div>
              <div className="text-[1.55rem] font-bold tracking-tight mt-0.5">{formatInt(selectedFiles.length)}</div>
            </div>
            <div>
              <div className="text-[0.72rem] uppercase tracking-wider text-muted-fg">Est. recoverable</div>
              <div className="text-[1.55rem] font-bold tracking-tight mt-0.5 text-brand">{formatBytes(totalSavings)}</div>
            </div>
            <div>
              <div className="text-[0.72rem] uppercase tracking-wider text-muted-fg">Profile</div>
              {profiles.length > 1 ? (
                <Select
                  value={String(profileId ?? profiles[0]?.id ?? '')}
                  onValueChange={(v) => setProfileId(Number(v))}
                >
                  <SelectTrigger className="mt-0.5 h-8 rounded-lg text-sm">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {profiles.map((p) => (
                      <SelectItem key={p.id} value={String(p.id)}>
                        {p.name}{p.is_default ? ' (default)' : ''}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              ) : (
                <div className="text-[1.1rem] font-bold tracking-tight mt-0.5">{defaultProfile?.name ?? '—'}</div>
              )}
            </div>
          </div>

          <div className="text-[0.82rem]">
            {preview.map((f) => (
              <div key={f.id} className="flex justify-between gap-3 py-[7px] border-b border-line-soft last:border-b-0">
                <span className="truncate text-muted-fg">{baseName(f.path)}</span>
                <span className="text-brand font-medium flex-shrink-0">-{formatBytes(f.predicted_savings_bytes)}</span>
              </div>
            ))}
            {more > 0 && <div className="text-[0.78rem] text-muted-dim pt-2">…and {more} more</div>}
          </div>

          <div
            className="flex items-start gap-[9px] text-[0.8rem] text-muted-fg mt-4 rounded-[11px] px-[13px] py-[11px] border"
            style={{ background: 'var(--green-soft)', borderColor: 'color-mix(in srgb, var(--green) 26%, transparent)' }}
          >
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" className="w-4 h-4 text-green flex-shrink-0 mt-px">
              <path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z"/>
            </svg>
            <div>
              <b className="text-text font-semibold">Non-destructive.</b> Each original is kept until its re-encode passes verification, then swapped atomically. Jobs run only inside your encode window.
            </div>
          </div>
        </div>

        <DialogFooter className="px-6 py-4 border-t border-line flex justify-end gap-2">
          <Button variant="outline" onClick={onClose} className="rounded-[11px]">
            Cancel
          </Button>
          <Button
            onClick={() => void handleConfirm()}
            disabled={loading}
            className="rounded-[11px]"
            style={{ background: 'linear-gradient(145deg, var(--brand), var(--brand-2))' }}
          >
            Queue {formatInt(selectedFiles.length)} jobs
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

function FlatRow({
  item,
  selected,
  onToggle,
}: {
  item: MediaFile;
  selected: boolean;
  onToggle: (id: number) => void;
}) {
  return (
    <div
      className={`flex items-center gap-0 border-b border-line-soft hover:bg-surface-2 cursor-pointer transition-colors ${selected ? 'bg-brand-soft' : ''}`}
      style={{ height: 52 }}
      onClick={() => onToggle(item.id)}
    >
      <div className="w-[52px] flex justify-center flex-shrink-0">
        <Checkbox
          checked={selected}
          onCheckedChange={() => onToggle(item.id)}
          onClick={(e) => e.stopPropagation()}
          className="size-[17px] rounded-[5px]"
        />
      </div>
      <div className="flex-1 min-w-0 pr-3">
        <div className="font-semibold text-[0.88rem] truncate">{baseName(item.path)}</div>
        <div className="text-[0.74rem] text-muted-dim truncate font-mono">{dirName(item.path)}</div>
      </div>
      <div className="w-[80px] flex-shrink-0"><CodecBadge codec={item.video_codec} /></div>
      <div className="w-[60px] flex-shrink-0 text-[0.82rem] text-muted-fg">{resolutionLabel(item.width, item.height)}</div>
      <div className="w-[90px] flex-shrink-0 text-right text-[0.82rem] text-muted-fg pr-2 font-mono">{formatBytes(item.size_bytes)}</div>
      <div className="w-[110px] flex-shrink-0 text-right text-brand font-semibold text-[0.88rem] pr-4 font-mono">{formatBytes(item.predicted_savings_bytes)}</div>
    </div>
  );
}

function EpisodeRow({ ep, selected, onToggle }: { ep: Episode; selected: boolean; onToggle: (id: number) => void }) {
  return (
    <div
      className={`flex items-center gap-[10px] px-4 py-[10px] pl-[42px] border-b border-line-soft hover:bg-surface-2 cursor-pointer transition-colors ${selected ? 'bg-brand-soft' : ''}`}
      onClick={() => onToggle(ep.id)}
    >
      <Checkbox
        checked={selected}
        onCheckedChange={() => onToggle(ep.id)}
        onClick={(e) => e.stopPropagation()}
        className="size-[17px] rounded-[5px] flex-shrink-0"
      />
      <div className="flex-1 min-w-0">
        <div className="text-sm truncate">{baseName(ep.path)}</div>
      </div>
      <div className="flex items-center gap-2 flex-shrink-0">
        <CodecBadge codec={ep.video_codec} />
        <span className="text-[0.78rem] text-muted-fg font-mono">{formatBytes(ep.size_bytes)}</span>
        <span className="text-brand font-semibold text-[0.82rem] font-mono">-{formatBytes(ep.predicted_savings_bytes)}</span>
      </div>
    </div>
  );
}

function GroupedSkeleton() {
  return (
    <div className="bg-surface border border-line rounded-[var(--radius)] overflow-hidden">
      {[0, 1, 2, 3].map((i) => (
        <div key={i} className="flex items-center gap-[11px] px-4 py-[13px] border-b border-line-soft">
          <Skeleton className="w-[17px] h-[17px] rounded-[5px] flex-shrink-0" />
          <Skeleton className="w-[18px] h-[18px] flex-shrink-0 rounded" />
          <div className="flex-1 min-w-0">
            <Skeleton className="h-4 w-48 mb-1.5" />
            <Skeleton className="h-3 w-64" />
          </div>
          <div className="text-right flex-shrink-0">
            <Skeleton className="h-3 w-20 mb-1 ml-auto" />
            <Skeleton className="h-4 w-16 ml-auto" />
          </div>
        </div>
      ))}
    </div>
  );
}

function GroupedContent({
  selectedIds,
  onToggle,
  onToggleSeries,
  filters,
}: {
  selectedIds: Set<number>;
  onToggle: (id: number) => void;
  onToggleSeries: (ids: number[]) => void;
  filters: CandidateFilters;
}) {
  const { data } = useSuspenseQuery({
    queryKey: ['candidates', 'grouped', filters],
    queryFn: () => api.groupedCandidates(filters),
  });

  const [expandedSeries, setExpandedSeries] = useState<Set<string>>(new Set());
  const [expandedSeasons, setExpandedSeasons] = useState<Set<string>>(new Set());

  function toggleSeriesExpand(title: string) {
    setExpandedSeries((prev) => {
      const next = new Set(prev);
      if (next.has(title)) { next.delete(title); } else { next.add(title); }
      return next;
    });
  }

  function toggleSeasonExpand(key: string) {
    setExpandedSeasons((prev) => {
      const next = new Set(prev);
      if (next.has(key)) { next.delete(key); } else { next.add(key); }
      return next;
    });
  }

  function seriesEpisodeIds(s: SeriesGroup): number[] {
    return s.seasons.flatMap((se) => se.episodes.map((e) => e.id));
  }

  function seriesSelState(s: SeriesGroup): 'none' | 'partial' | 'all' {
    const ids = seriesEpisodeIds(s);
    const selCount = ids.filter((id) => selectedIds.has(id)).length;
    if (selCount === 0) return 'none';
    if (selCount === ids.length) return 'all';
    return 'partial';
  }

  return (
    <div className="bg-surface border border-line rounded-[var(--radius)] overflow-hidden">
      {data.series.map((s) => {
        const expanded = expandedSeries.has(s.title);
        const selState = seriesSelState(s);
        const allIds = seriesEpisodeIds(s);
        return (
          <div key={s.title}>
            <div
              className="flex items-center gap-[11px] px-4 py-[13px] border-b border-line-soft hover:bg-surface-2 cursor-pointer transition-colors"
              onClick={() => toggleSeriesExpand(s.title)}
            >
              <Checkbox
                checked={selState === 'all' ? true : selState === 'partial' ? 'indeterminate' : false}
                onCheckedChange={() => onToggleSeries(allIds)}
                onClick={(e) => e.stopPropagation()}
                className="size-[17px] rounded-[5px] flex-shrink-0"
              />
              <span className={`w-[18px] h-[18px] flex-shrink-0 grid place-items-center text-muted-fg transition-transform ${expanded ? 'rotate-90' : ''}`}>
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" className="w-3 h-3"><path d="M9 18l6-6-6-6"/></svg>
              </span>
              <div className="flex-1 min-w-0">
                <div className="font-bold text-[0.92rem] flex items-center gap-2">
                  {s.title}
                  <Badge className="text-[0.7rem] rounded text-sky border-[rgba(51,177,255,.32)] bg-[rgba(51,177,255,.1)]">TV</Badge>
                </div>
                <div className="text-[0.76rem] text-muted-fg mt-0.5">
                  {s.season_count} seasons · {formatInt(s.candidate_count)} candidates · {formatBytes(s.total_bytes)}
                </div>
              </div>
              <div className="text-right flex-shrink-0">
                <div className="text-[0.78rem] text-muted-fg">{formatBytes(s.total_bytes)}</div>
                <div className="text-[0.92rem] font-semibold text-brand">{formatBytes(s.predicted_savings_bytes)}</div>
              </div>
            </div>

            {expanded && (
              <div style={{ background: 'var(--bg)' }}>
                {s.seasons.map((se) => {
                  const seasonKey = `${s.title}-${se.season}`;
                  const seasonExpanded = expandedSeasons.has(seasonKey);
                  const seasonSelCount = se.episodes.filter((e) => selectedIds.has(e.id)).length;
                  return (
                    <div key={se.season}>
                      <div
                        className="flex items-center gap-[10px] px-4 py-[9px] pl-[18px] text-[0.76rem] font-semibold text-muted-fg bg-surface-2 border-b border-line-soft cursor-pointer"
                        onClick={() => toggleSeasonExpand(seasonKey)}
                      >
                        <span className={`w-[18px] h-[18px] flex-shrink-0 grid place-items-center transition-transform ${seasonExpanded ? 'rotate-90' : ''}`}>
                          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" className="w-3 h-3"><path d="M9 18l6-6-6-6"/></svg>
                        </span>
                        Season {se.season}
                        {seasonSelCount > 0 && <span className="text-brand">({seasonSelCount} sel)</span>}
                        <span className="ml-auto text-brand font-semibold">{formatBytes(se.predicted_savings_bytes)}</span>
                      </div>
                      {seasonExpanded && se.episodes.map((ep) => (
                        <EpisodeRow key={ep.id} ep={ep} selected={selectedIds.has(ep.id)} onToggle={onToggle} />
                      ))}
                    </div>
                  );
                })}
              </div>
            )}
          </div>
        );
      })}

      {data.movies.length > 0 && (
        <>
          <div className="text-[0.7rem] uppercase tracking-widest text-muted-dim font-bold px-4 pt-[15px] pb-[9px] border-b border-line-soft">
            Movies
          </div>
          {data.movies.map((f) => (
            <FlatRow key={f.id} item={f} selected={selectedIds.has(f.id)} onToggle={onToggle} />
          ))}
        </>
      )}
    </div>
  );
}

function GroupedView(props: {
  selectedIds: Set<number>;
  onToggle: (id: number) => void;
  onToggleSeries: (ids: number[]) => void;
  filters: CandidateFilters;
}) {
  return (
    <Suspense fallback={<GroupedSkeleton />}>
      <GroupedContent {...props} />
    </Suspense>
  );
}

export default function Page() {
  const qc = useQueryClient();
  const [isPending, startTransition] = useTransition();
  const [search, setSearch] = useState('');
  const [debouncedSearch, setDebouncedSearch] = useState('');
  const [sort, setSort] = useState<SortKey>('savings_desc');
  const [codec, setCodec] = useState('');
  const [resolution, setResolution] = useState('');
  const [library, setLibrary] = useState('');
  const [view, setView] = useState<ViewMode>('flat');
  const [selectedIds, setSelectedIds] = useState<Set<number>>(new Set());
  const [confirmOpen, setConfirmOpen] = useState(false);
  const fileMapRef = useRef<Map<number, MediaFile>>(new Map());
  const parentRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const t = setTimeout(() => startTransition(() => setDebouncedSearch(search)), 300);
    return () => clearTimeout(t);
  }, [search]);

  const filters: CandidateFilters = {
    sort,
    video_codec: codec || undefined,
    resolution_band: resolution || undefined,
    library_type: library || undefined,
    search: debouncedSearch || undefined,
  };

  const { data, fetchNextPage, hasNextPage, isFetchingNextPage } = useInfiniteQuery({
    queryKey: ['candidates', filters],
    queryFn: ({ pageParam }: { pageParam: Record<string, number | undefined> }) =>
      api.candidates({ ...filters, limit: PAGE_SIZE, ...pageParam }),
    initialPageParam: {} as Record<string, number | undefined>,
    getNextPageParam: (lastPage, allPages) => {
      if (lastPage.next_cursor) {
        return { after_savings: lastPage.next_cursor.after_savings, after_id: lastPage.next_cursor.after_id };
      }
      if (lastPage.items.length === PAGE_SIZE) {
        return { offset: allPages.flatMap((p) => p.items).length };
      }
      return undefined;
    },
    placeholderData: (prev) => prev,
  });

  const allItems = useMemo(
    () => data?.pages.flatMap((p) => p.items) ?? [],
    [data],
  );

  useEffect(() => {
    allItems.forEach((item) => fileMapRef.current.set(item.id, item));
  }, [allItems]);

  const virtualizer = useVirtualizer({
    count: hasNextPage ? allItems.length + 1 : allItems.length,
    getScrollElement: () => parentRef.current,
    estimateSize: () => 52,
    overscan: 15,
  });

  const virtualItems = virtualizer.getVirtualItems();
  useEffect(() => {
    const last = virtualItems[virtualItems.length - 1];
    if (!last) return;
    if (last.index >= allItems.length - 1 && hasNextPage && !isFetchingNextPage) {
      void fetchNextPage();
    }
  }, [virtualItems, allItems.length, hasNextPage, isFetchingNextPage, fetchNextPage]);

  const { data: profilesData } = useQuery({
    queryKey: ['profiles'],
    queryFn: api.profiles,
    staleTime: 60_000,
  });
  const profiles = profilesData?.items ?? [];

  function toggleId(id: number) {
    setSelectedIds((prev) => {
      const next = new Set(prev);
      if (next.has(id)) { next.delete(id); } else { next.add(id); }
      return next;
    });
  }

  function toggleSeries(ids: number[]) {
    setSelectedIds((prev) => {
      const next = new Set(prev);
      const allSelected = ids.every((id) => next.has(id));
      ids.forEach((id) => { if (allSelected) { next.delete(id); } else { next.add(id); } });
      return next;
    });
  }

  function toggleAll() {
    if (selectedIds.size === allItems.length && allItems.length > 0) {
      setSelectedIds(new Set());
    } else {
      setSelectedIds(new Set(allItems.map((i) => i.id)));
    }
  }

  const clearSel = useCallback(() => setSelectedIds(new Set()), []);

  const selectedFiles = useMemo(
    () => [...selectedIds].map((id) => fileMapRef.current.get(id)).filter(Boolean) as MediaFile[],
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [selectedIds, allItems],
  );

  const totalSavings = selectedFiles.reduce((s, f) => s + f.predicted_savings_bytes, 0);

  const queueMutation = useMutation({
    mutationFn: ({ ids, profileId }: { ids: number[]; profileId: number | null }) =>
      api.createJobs(ids, profileId ?? undefined),
    onSuccess: (result) => {
      toast.success(`${result.queued.length} jobs queued`);
      clearSel();
      qc.invalidateQueries({ queryKey: ['jobs'] });
      qc.invalidateQueries({ queryKey: ['candidates'] });
    },
    onError: () => toast.error('Failed to queue jobs'),
  });

  const allSelected = allItems.length > 0 && allItems.every((i) => selectedIds.has(i.id));

  return (
    <div className="flex flex-col min-w-0 h-screen overflow-hidden">
      <div
        className="flex items-center gap-4 px-7 py-[18px] border-b border-line flex-shrink-0"
        style={{ background: 'rgba(22,22,22,.82)', backdropFilter: 'blur(10px)' }}
      >
        <div>
          <div className="text-[1.38rem] font-bold tracking-tight">Candidate browser</div>
          <div className="text-[0.82rem] text-muted-fg mt-px">
            {data === undefined
              ? <Skeleton className="h-3 w-52 mt-1" />
              : `${formatInt(allItems.length)}+ files · ranked by predicted savings`}
          </div>
        </div>
        {profiles[0] && (
          <Badge variant="outline" className="ml-auto text-[0.82rem] font-semibold px-[13px] py-[7px] rounded-[10px] border-line bg-surface gap-1.5">
            <span className="font-mono text-[0.8rem]">Profile</span>
            {profiles.find((p) => p.is_default)?.name ?? profiles[0].name}
          </Badge>
        )}
      </div>

      <div className="flex items-center gap-[10px] px-7 py-3 flex-wrap border-b border-line-soft flex-shrink-0" style={{ background: 'var(--bg)' }}>
        <div className="flex-1 min-w-[200px] relative">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" className="w-[15px] h-[15px] absolute left-[11px] top-1/2 -translate-y-1/2 text-muted-dim pointer-events-none">
            <circle cx="11" cy="11" r="8"/><line x1="21" y1="21" x2="16.65" y2="16.65"/>
          </svg>
          <Input
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder="Filter by title or path…"
            className="rounded-[11px] pl-[34px] text-[0.86rem]"
          />
        </div>

        <div className="h-5 w-px bg-line flex-shrink-0" />

        <Select value={sort} onValueChange={(v) => startTransition(() => setSort(v as SortKey))}>
          <SelectTrigger className="rounded-[11px] bg-surface text-[0.84rem] h-auto py-[9px] gap-1 min-w-[195px]">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" className="w-[13px] h-[13px] text-muted-dim flex-shrink-0">
              <path d="M3 8h18M6 12h12M10 16h4"/>
            </svg>
            <span className="text-[0.78rem] text-muted-dim flex-shrink-0">Sort</span>
            <span className="text-muted-dim mx-px">·</span>
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {SORT_OPTIONS.map((o) => <SelectItem key={o.value} value={o.value}>{o.label}</SelectItem>)}
          </SelectContent>
        </Select>

        <FilterSelect label="Codec" value={codec} options={CODEC_OPTIONS} onChange={(v) => startTransition(() => setCodec(v))} className="min-w-[120px]" />
        <FilterSelect label="Res" value={resolution} options={RESOLUTION_OPTIONS} onChange={(v) => startTransition(() => setResolution(v))} className="min-w-[100px]" />
        <FilterSelect label="Library" value={library} options={LIBRARY_OPTIONS} onChange={(v) => startTransition(() => setLibrary(v))} className="min-w-[130px]" />

        <div className="h-5 w-px bg-line flex-shrink-0" />

        <div className="inline-flex bg-surface border border-line rounded-[11px] p-[3px] gap-[2px]">
          <Button
            variant="ghost"
            size="sm"
            onClick={() => setView('flat')}
            className={cn('rounded-[8px] text-[0.82rem] font-semibold h-auto py-[7px] px-[13px]', view === 'flat' ? 'bg-brand-soft text-brand hover:bg-brand-soft hover:text-brand' : 'text-muted-fg')}
          >
            Flat
          </Button>
          <Button
            variant="ghost"
            size="sm"
            onClick={() => setView('grouped')}
            className={cn('rounded-[8px] text-[0.82rem] font-semibold h-auto py-[7px] px-[13px]', view === 'grouped' ? 'bg-brand-soft text-brand hover:bg-brand-soft hover:text-brand' : 'text-muted-fg')}
          >
            By series
          </Button>
        </div>
      </div>

      <div className={cn('flex-1 overflow-hidden relative px-7 pt-3 pb-3 transition-opacity duration-150', isPending && 'opacity-50')}>
        {view === 'flat' ? (
          <div className="bg-surface border border-line rounded-[var(--radius)] overflow-hidden flex flex-col h-full">
            <div className="flex items-center text-[0.7rem] uppercase tracking-wider text-muted-fg font-bold bg-surface-2 border-b border-line flex-shrink-0">
              <div className="w-[52px] flex justify-center py-3">
                <Checkbox
                  checked={allSelected}
                  onCheckedChange={toggleAll}
                  className="size-[17px] rounded-[5px] cursor-pointer"
                />
              </div>
              <div className="flex-1 py-3 pr-3">File</div>
              <div className="w-[80px] py-3">Codec</div>
              <div className="w-[60px] py-3">Res</div>
              <div className="w-[90px] py-3 text-right pr-2">Size</div>
              <div className="w-[110px] py-3 text-right pr-4 text-brand">Est. savings ↓</div>
            </div>

            {data === undefined ? (
              <div className="flex-1 overflow-auto">
                {Array.from({ length: 10 }).map((_, i) => (
                  <div key={i} className="flex items-center gap-0 border-b border-line-soft px-0" style={{ height: 52 }}>
                    <div className="w-[52px] flex justify-center flex-shrink-0">
                      <Skeleton className="w-[17px] h-[17px] rounded-[5px]" />
                    </div>
                    <div className="flex-1 min-w-0 pr-3">
                      <Skeleton className="h-4 w-48 mb-1.5" />
                      <Skeleton className="h-3 w-64" />
                    </div>
                    <Skeleton className="w-[80px] h-5 rounded-[7px] flex-shrink-0" />
                    <Skeleton className="w-[60px] h-3 flex-shrink-0 mx-1" />
                    <Skeleton className="w-[90px] h-3 flex-shrink-0 mr-2" />
                    <Skeleton className="w-[110px] h-4 flex-shrink-0 mr-4" />
                  </div>
                ))}
              </div>
            ) : (
            <div ref={parentRef} className="flex-1 overflow-auto">
              <div style={{ height: virtualizer.getTotalSize(), position: 'relative' }}>
                {virtualItems.map((vRow) => (
                  <div
                    key={vRow.key}
                    style={{ position: 'absolute', top: vRow.start, height: vRow.size, width: '100%' }}
                  >
                    {vRow.index < allItems.length ? (
                      <FlatRow
                        item={allItems[vRow.index]}
                        selected={selectedIds.has(allItems[vRow.index].id)}
                        onToggle={toggleId}
                      />
                    ) : (
                      <div className="flex items-center justify-center h-full text-muted-dim text-sm">
                        {isFetchingNextPage ? 'Loading more…' : hasNextPage ? 'Scroll to load more' : 'End of list'}
                      </div>
                    )}
                  </div>
                ))}
              </div>
            </div>
            )}
          </div>
        ) : (
          <div className="h-full overflow-auto">
            <GroupedView
              selectedIds={selectedIds}
              onToggle={toggleId}
              onToggleSeries={toggleSeries}
              filters={filters}
            />
          </div>
        )}
      </div>

      {selectedIds.size > 0 && (
        <div
          className="mx-7 mb-3 flex items-center gap-4 rounded-[13px] px-[18px] py-[13px] border border-brand-line sticky bottom-3"
          style={{ background: 'var(--surface-2)', boxShadow: '0 10px 30px rgba(0,0,0,.35)' }}
        >
          <div className="font-bold">
            <b className="text-brand">{formatInt(selectedIds.size)}</b> selected
          </div>
          <div className="text-muted-fg text-[0.85rem]">
            ≈ <span className="text-brand font-semibold">{formatBytes(totalSavings)}</span> estimated recoverable
          </div>
          <div className="ml-auto flex gap-2.5 items-center">
            <Button variant="ghost" onClick={clearSel} className="rounded-[11px] text-sm">
              Clear
            </Button>
            <Button
              onClick={() => setConfirmOpen(true)}
              className="rounded-[11px] text-sm"
              style={{ background: 'linear-gradient(145deg, var(--brand), var(--brand-2))', boxShadow: '0 4px 14px var(--brand-soft)' }}
            >
              Queue selected →
            </Button>
          </div>
        </div>
      )}

      <ConfirmDialog
        open={confirmOpen}
        onClose={() => setConfirmOpen(false)}
        selectedFiles={selectedFiles}
        profiles={profiles}
        onConfirm={async (profileId) => {
          await queueMutation.mutateAsync({ ids: [...selectedIds], profileId });
        }}
      />
    </div>
  );
}
