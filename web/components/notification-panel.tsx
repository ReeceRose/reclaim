'use client';

import { useState, useEffect } from 'react';
import { useQuery } from '@tanstack/react-query';
import { api, type AppEvent } from '@/lib/api';
import { Sheet, SheetContent, SheetHeader, SheetTitle } from '@/components/ui/sheet';
import { Skeleton } from '@/components/ui/skeleton';

const WATERMARK_KEY = 'reclaim_last_seen_event_id';

function relativeTime(unixSec: number): string {
  const diff = Math.floor(Date.now() / 1000) - unixSec;
  if (diff < 60) return 'just now';
  if (diff < 3600) return `${Math.floor(diff / 60)}m ago`;
  if (diff < 86400) return `${Math.floor(diff / 3600)}h ago`;
  return `${Math.floor(diff / 86400)}d ago`;
}

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

function EventRow({ event }: { event: AppEvent }) {
  return (
    <div className="flex gap-3 px-6 py-3.5 border-b border-line-soft last:border-0">
      <div className="pt-[5px] flex-shrink-0">
        <span className={`block w-2 h-2 rounded-full ${SEVERITY_DOT[event.severity] ?? 'bg-muted-dim'}`} />
      </div>
      <div className="min-w-0 flex-1">
        <div className="flex items-baseline justify-between gap-2">
          <p className="text-[0.85rem] leading-snug text-text break-words">{event.message}</p>
          <span className="text-[0.7rem] text-muted-dim whitespace-nowrap flex-shrink-0">
            {relativeTime(event.created_at)}
          </span>
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

export function NotificationPanel({ open, onOpenChange }: Props) {
  const { data, isLoading } = useQuery({
    queryKey: ['events'],
    queryFn: () => api.events({ limit: 50 }),
    staleTime: 30_000,
  });

  const events = data?.items ?? [];

  // Update watermark when panel opens
  useEffect(() => {
    if (!open || events.length === 0) return;
    const maxId = Math.max(...events.map((e) => e.id));
    localStorage.setItem(WATERMARK_KEY, String(maxId));
  }, [open, events]);

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent>
        <SheetHeader>
          <SheetTitle>Events</SheetTitle>
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
            events.map((event) => <EventRow key={event.id} event={event} />)
          )}
        </div>
      </SheetContent>
    </Sheet>
  );
}

/** Returns the number of events with id > the stored watermark. */
export function useUnreadCount(events: AppEvent[]): number {
  const [watermark, setWatermark] = useState(0);

  useEffect(() => {
    const stored = parseInt(localStorage.getItem(WATERMARK_KEY) ?? '0', 10);
    setWatermark(isNaN(stored) ? 0 : stored);
  }, []);

  return events.filter((e) => e.id > watermark).length;
}
