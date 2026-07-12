"use client";

import { useEffect, useState } from "react";

const TICK_MS = 30_000;

/** useNow re-renders the caller every TICK_MS so time-derived values (e.g. windowInfo) stay live. */
export function useNow(): Date {
  const [now, setNow] = useState(() => new Date());

  useEffect(() => {
    const id = setInterval(() => setNow(new Date()), TICK_MS);
    return () => clearInterval(id);
  }, []);

  return now;
}
