"use client";

import { useQuery } from "@tanstack/react-query";
import { api, type ClockFormat } from "@/lib/api";

/**
 * useClockFormat reads the instance-wide 12h/24h display preference. It shares
 * the cached settings query every screen already loads, so it costs no extra
 * request and every consumer flips together when the setting is saved.
 */
export function useClockFormat(): ClockFormat {
  const { data } = useQuery({
    queryKey: ["settings"],
    queryFn: api.settings,
  });
  return data?.clock_format === "24h" ? "24h" : "12h";
}
