'use client';

import { useCallback, useEffect, useMemo, useRef } from 'react';
import { usePathname, useSearchParams } from 'next/navigation';

export function parseQueryEnum<T extends string>(
  value: string | null,
  allowed: readonly T[],
  defaultValue: T,
): T {
  if (value && (allowed as readonly string[]).includes(value)) return value as T;
  return defaultValue;
}

export function useQueryParams() {
  const searchParams = useSearchParams();
  const pathname = usePathname();

  // Keep the latest values in refs so `set` can stay referentially stable.
  const searchParamsRef = useRef(searchParams);
  const pathnameRef = useRef(pathname);
  useEffect(() => {
    searchParamsRef.current = searchParams;
    pathnameRef.current = pathname;
  });

  const get = useCallback((key: string) => searchParams.get(key), [searchParams]);

  // Next.js 16 docs recommend window.history.replaceState for search-param-only
  // updates ("shallow routing") — this integrates with useSearchParams() in both
  // dev and static-export production, whereas router.replace() does not reliably
  // trigger useSearchParams() re-renders in the static export.
  const set = useCallback(
    (updates: Record<string, string | null | undefined>) => {
      const current = searchParamsRef.current.toString();
      const params = new URLSearchParams(current);
      for (const [key, value] of Object.entries(updates)) {
        if (value == null || value === '') params.delete(key);
        else params.set(key, value);
      }
      const qs = params.toString();
      if (qs === current) return;
      const url = qs ? `${pathnameRef.current}?${qs}` : pathnameRef.current;
      window.history.replaceState(null, '', url);
    },
    [],
  );

  return useMemo(() => ({ get, set, searchParams }), [get, set, searchParams]);
}

export function useQueryParam(
  key: string,
  defaultValue = '',
): [string, (value: string) => void] {
  const { get, set } = useQueryParams();
  const value = get(key) ?? defaultValue;
  const setValue = useCallback(
    (next: string) => {
      set({ [key]: !next || next === defaultValue ? null : next });
    },
    [key, defaultValue, set],
  );
  return [value, setValue];
}

export function buildQueryString(entries: Record<string, string | null | undefined>): string {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(entries)) {
    if (value) params.set(key, value);
  }
  return params.toString();
}
