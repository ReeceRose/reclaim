"use client";

import { useSyncExternalStore } from "react";

const TICK_MS = 30_000;

let now = new Date();
let intervalId: ReturnType<typeof setInterval> | null = null;
const listeners = new Set<() => void>();

function subscribe(listener: () => void): () => void {
  if (listeners.size === 0) {
    intervalId = setInterval(() => {
      now = new Date();
      for (const l of listeners) l();
    }, TICK_MS);
  }
  listeners.add(listener);
  return () => {
    listeners.delete(listener);
    if (listeners.size === 0 && intervalId != null) {
      clearInterval(intervalId);
      intervalId = null;
    }
  };
}

function getSnapshot(): Date {
  return now;
}

/** useNow shares a single ticking clock across all callers so time-derived values (e.g. windowInfo) stay live and in sync. */
export function useNow(): Date {
  return useSyncExternalStore(subscribe, getSnapshot, getSnapshot);
}
