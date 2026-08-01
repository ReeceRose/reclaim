"use client";

import { useMemo } from "react";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";

function zoneList(current: string): string[] {
  // supportedValuesOf lands in ES2022; the tsconfig lib target predates it, and
  // it is missing entirely on old browsers — either way the current zone plus
  // UTC keeps the control usable.
  const intl = Intl as typeof Intl & {
    supportedValuesOf?: (key: string) => string[];
  };
  let zones: string[] = [];
  try {
    zones = intl.supportedValuesOf?.("timeZone") ?? [];
  } catch {
    zones = [];
  }
  const seen = new Set(["UTC", current, ...zones].filter(Boolean));
  return ["UTC", ...[...seen].filter((z) => z !== "UTC").sort()];
}

export function TimezoneSelect({
  value,
  onChange,
}: {
  value: string;
  onChange: (v: string) => void;
}) {
  const zones = useMemo(() => zoneList(value), [value]);

  return (
    <Select value={value} onValueChange={onChange}>
      <SelectTrigger className="w-full rounded-xl text-sm sm:w-72">
        <SelectValue />
      </SelectTrigger>
      <SelectContent className="max-h-72">
        {zones.map((z) => (
          <SelectItem key={z} value={z}>
            {z.replace(/_/g, " ")}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  );
}
