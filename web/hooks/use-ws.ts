'use client';

import { useEffect } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { wsURL, type AppEvent } from '@/lib/api';
import { toast } from 'sonner';

const SCAN_EVENTS = new Set(['scan_started', 'scan_completed', 'scan_failed']);
const JOB_MUTATE_EVENTS = new Set(['jobs_queued', 'job_completed', 'job_failed', 'job_cancelled']);

const MIN_DELAY_MS = 1_000;
const MAX_DELAY_MS = 30_000;

export function useWS() {
  const qc = useQueryClient();

  useEffect(() => {
    const url = wsURL();
    if (!url) return;

    let ws: WebSocket;
    let reconnectTimer: ReturnType<typeof setTimeout>;
    let alive = true;
    let delay = MIN_DELAY_MS;

    function connect() {
      ws = new WebSocket(url);

      ws.onopen = () => {
        delay = MIN_DELAY_MS;
      };

      ws.onmessage = (evt) => {
        let msg: { event: string; data?: unknown };
        try {
          msg = JSON.parse(evt.data as string);
        } catch {
          return;
        }

        const { event, data } = msg;

        if (SCAN_EVENTS.has(event)) {
          qc.setQueryData(['scanning'], event === 'scan_started');
          qc.invalidateQueries({ queryKey: ['stats'] });
          qc.invalidateQueries({ queryKey: ['candidates'] });
          if (event === 'scan_completed') {
            const d = data as { files_added?: number; files_updated?: number; files_removed?: number } | undefined;
            const added = d?.files_added ?? 0;
            const updated = d?.files_updated ?? 0;
            const removed = d?.files_removed ?? 0;
            const parts: string[] = [];
            if (added) parts.push(`${added} added`);
            if (updated) parts.push(`${updated} updated`);
            if (removed) parts.push(`${removed} removed`);
            toast.success('Scan complete', { description: parts.length ? parts.join(', ') : 'No changes' });
          } else if (event === 'scan_failed') {
            const d = data as { error?: string } | undefined;
            toast.error('Scan failed', { description: d?.error });
          }
        } else if (event === 'job_started' || JOB_MUTATE_EVENTS.has(event)) {
          qc.invalidateQueries({ queryKey: ['jobs'] });
          if (JOB_MUTATE_EVENTS.has(event)) {
            qc.invalidateQueries({ queryKey: ['candidates'] });
            qc.invalidateQueries({ queryKey: ['stats'] });
          }
        } else if (event === 'job_progress') {
          const { job_id, percent } = data as { job_id: number; percent: number };
          qc.setQueryData<Record<number, number>>(['job_progress'], (prev) => ({
            ...(prev ?? {}),
            [job_id]: percent,
          }));
        } else if (event === 'event_created') {
          const newEvent = data as AppEvent;
          qc.setQueryData<{ items: AppEvent[] }>(['events'], (prev) => ({
            items: [newEvent, ...(prev?.items ?? [])].slice(0, 200),
          }));
        }
      };

      ws.onclose = () => {
        if (!alive) return;
        reconnectTimer = setTimeout(() => {
          delay = Math.min(delay * 2, MAX_DELAY_MS);
          connect();
        }, delay);
      };
    }

    connect();

    return () => {
      alive = false;
      clearTimeout(reconnectTimer);
      ws?.close();
    };
  }, [qc]);

}
