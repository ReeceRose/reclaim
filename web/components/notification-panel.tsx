'use client';

import { useState, useEffect, useMemo, useSyncExternalStore } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { api, type AppEvent } from '@/lib/api';
import { relativeTime } from '@/lib/format';
import { Sheet, SheetContent, SheetHeader, SheetTitle } from '@/components/ui/sheet';
import { Skeleton } from '@/components/ui/skeleton';
import { Button } from '@/components/ui/button';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';

const WATERMARK_KEY = 'reclaim_last_seen_event_id';

const SEVERITY_DOT: Record<string, string> = {
  info: 'bg-[var(--green)]',
  warn: 'bg-yellow-400',
  error: 'bg-[var(--red)]',
};

function ExpandableMetadata({ metadata }: { metadata: Record<string, unknown> | null }) {
  const [open, setOpen] = useState(false);
  if (!metadata) return null;
  const vr = metadata.verification_result as Record<string, unknown> | undefined;
  if (!vr) return null;
  return (
    <div className="mt-1.5">
      <button
        onClick={() => setOpen((v) => !v)}
        className="text-[0.7rem] text-muted-dim hover:text-muted-fg transition-colors"
      >
        {open ? '▾ Hide details' : '▸ Show details'}
      </button>
      {open && (
        <div className="mt-1 text-[0.72rem] font-mono text-muted-fg bg-surface-2 rounded-[6px] px-3 py-2 space-y-0.5">
          {Object.entries(vr).map(([k, v]) => (
            <div key={k} className="flex gap-2">
              <span className="text-muted-dim">{k}:</span>
              <span>{String(v)}</span>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

function EventRow({ event, onDelete, deleting }: { event: AppEvent; onDelete: (id: number) => void; deleting: boolean }) {
  return (
    <div className="group flex gap-3 px-6 py-3.5 border-b border-line-soft last:border-0">
      <div className="pt-[5px] flex-shrink-0">
        <span className={`block w-2 h-2 rounded-full ${SEVERITY_DOT[event.severity] ?? 'bg-muted-dim'}`} />
      </div>
      <div className="min-w-0 flex-1">
        <div className="flex items-baseline justify-between gap-2">
          <p className="text-[0.85rem] leading-snug text-text break-words">{event.message}</p>
          <div className="flex items-center gap-1 flex-shrink-0">
            <span className="text-[0.7rem] text-muted-dim whitespace-nowrap">
              {relativeTime(event.created_at)}
            </span>
            <button
              type="button"
              onClick={() => onDelete(event.id)}
              disabled={deleting}
              aria-label="Dismiss event"
              className="rounded-[6px] p-1 text-muted-dim opacity-0 transition-opacity hover:bg-surface-2 hover:text-text group-hover:opacity-100 focus:opacity-100 disabled:opacity-40"
            >
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" className="w-3.5 h-3.5">
                <line x1="18" y1="6" x2="6" y2="18" /><line x1="6" y1="6" x2="18" y2="18" />
              </svg>
            </button>
          </div>
        </div>
        {event.type === 'job_failed' && <ExpandableMetadata metadata={event.metadata} />}
      </div>
    </div>
  );
}

function EventListSkeleton() {
  return (
    <div className="px-6 py-4 space-y-4">
      {Array.from({ length: 6 }).map((_, i) => (
        <div key={i} className="flex gap-3">
          <Skeleton className="w-2 h-2 rounded-full mt-1.5 flex-shrink-0" />
          <div className="flex-1 space-y-1.5">
            <Skeleton className="h-3.5 w-[80%]" />
            <Skeleton className="h-3 w-[40%]" />
          </div>
        </div>
      ))}
    </div>
  );
}

interface Props {
  open: boolean;
  onOpenChange: (v: boolean) => void;
}

function ClearAllDialog({
  open,
  onClose,
  onConfirm,
  pending,
}: {
  open: boolean;
  onClose: () => void;
  onConfirm: () => void;
  pending: boolean;
}) {
  return (
    <Dialog open={open} onOpenChange={(o) => !o && onClose()}>
      <DialogContent
        className="max-w-sm border-line p-0 overflow-hidden"
        style={{ background: 'var(--surface)' }}
      >
        <DialogHeader className="px-6 pt-[22px] pb-4 border-b border-line">
          <DialogTitle className="text-[1.1rem] font-bold">Clear all events?</DialogTitle>
          <DialogDescription className="text-[0.85rem] text-muted-fg mt-1">
            This removes every event from the list. New activity will still be logged going forward.
          </DialogDescription>
        </DialogHeader>

        <DialogFooter className="px-6 pb-5 pt-1 flex justify-end gap-2">
          <Button variant="outline" onClick={onClose} disabled={pending} className="rounded-[11px]">
            Cancel
          </Button>
          <Button
            onClick={onConfirm}
            disabled={pending}
            variant="destructive"
            className="rounded-[11px]"
          >
            {pending ? 'Clearing…' : 'Clear all'}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

export function NotificationPanel({ open, onOpenChange }: Props) {
  const qc = useQueryClient();
  const { data, isLoading } = useQuery({
    queryKey: ['events'],
    queryFn: () => api.events({ limit: 50 }),
    staleTime: 30_000,
  });

  const events = useMemo(() => data?.items ?? [], [data]);
  const [clearDialogOpen, setClearDialogOpen] = useState(false);

  const deleteMutation = useMutation({
    mutationFn: (id: number) => api.deleteEvent(id),
    onMutate: async (id) => {
      await qc.cancelQueries({ queryKey: ['events'] });
      const prev = qc.getQueryData<{ items: AppEvent[] }>(['events']);
      qc.setQueryData<{ items: AppEvent[] }>(['events'], (old) => ({
        items: (old?.items ?? []).filter((e) => e.id !== id),
      }));
      return { prev };
    },
    onError: (_err, _id, ctx) => {
      if (ctx?.prev) qc.setQueryData(['events'], ctx.prev);
    },
    onSettled: () => qc.invalidateQueries({ queryKey: ['events'] }),
  });

  const clearMutation = useMutation({
    mutationFn: () => api.clearEvents(),
    onMutate: async () => {
      await qc.cancelQueries({ queryKey: ['events'] });
      const prev = qc.getQueryData<{ items: AppEvent[] }>(['events']);
      qc.setQueryData<{ items: AppEvent[] }>(['events'], { items: [] });
      localStorage.setItem(WATERMARK_KEY, '0');
      return { prev };
    },
    onError: (_err, _vars, ctx) => {
      if (ctx?.prev) qc.setQueryData(['events'], ctx.prev);
    },
    onSuccess: () => setClearDialogOpen(false),
    onSettled: () => qc.invalidateQueries({ queryKey: ['events'] }),
  });

  // Update watermark when panel opens
  useEffect(() => {
    if (!open || events.length === 0) return;
    const maxId = Math.max(...events.map((e) => e.id));
    localStorage.setItem(WATERMARK_KEY, String(maxId));
  }, [open, events]);

  return (
    <>
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent>
        <SheetHeader className="flex-row items-center justify-between pr-12">
          <SheetTitle>Events</SheetTitle>
          {events.length > 0 && (
            <Button
              variant="ghost"
              size="sm"
              onClick={() => setClearDialogOpen(true)}
              disabled={clearMutation.isPending}
              className="rounded-[8px] h-7 px-2.5 text-[0.78rem] text-muted-dim hover:text-text"
            >
              Clear all
            </Button>
          )}
        </SheetHeader>

        <div className="flex-1 overflow-y-auto">
          {isLoading ? (
            <EventListSkeleton />
          ) : events.length === 0 ? (
            <div className="flex flex-col items-center justify-center h-full gap-2 text-muted-dim py-20">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" className="w-10 h-10 opacity-30">
                <path d="M18 8A6 6 0 006 8c0 7-3 9-3 9h18s-3-2-3-9"/><path d="M13.73 21a2 2 0 01-3.46 0"/>
              </svg>
              <p className="text-[0.85rem]">No events yet</p>
            </div>
          ) : (
            events.map((event) => (
              <EventRow
                key={event.id}
                event={event}
                onDelete={(id) => deleteMutation.mutate(id)}
                deleting={deleteMutation.isPending}
              />
            ))
          )}
        </div>
      </SheetContent>
    </Sheet>

    <ClearAllDialog
      open={clearDialogOpen}
      onClose={() => setClearDialogOpen(false)}
      onConfirm={() => clearMutation.mutate()}
      pending={clearMutation.isPending}
    />
    </>
  );
}

function subscribeWatermark(callback: () => void): () => void {
  window.addEventListener('storage', callback);
  return () => window.removeEventListener('storage', callback);
}

function readWatermark(): number {
  const stored = parseInt(localStorage.getItem(WATERMARK_KEY) ?? '0', 10);
  return isNaN(stored) ? 0 : stored;
}

/** Returns the number of events with id > the stored watermark. */
export function useUnreadCount(events: AppEvent[]): number {
  const watermark = useSyncExternalStore(subscribeWatermark, readWatermark, () => 0);
  return events.filter((e) => e.id > watermark).length;
}
