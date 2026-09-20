"use client";

import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import type { ClockFormat } from "@/lib/api";

const SLOTS = Array.from({ length: 96 }, (_, i) => {
  const h = Math.floor(i / 4);
  const m = (i % 4) * 15;
  return `${String(h).padStart(2, "0")}:${String(m).padStart(2, "0")}`;
});

function normalize(value: string): string {
  const [h = "0", m = "0"] = value.split(":");
  return `${h.padStart(2, "0")}:${m.padStart(2, "0")}`;
}

export function TimeSelect({
  value,
  onChange,
  format,
}: {
  value: string;
  onChange: (v: string) => void;
  format: ClockFormat;
}) {
  const current = normalize(value);
  const options = SLOTS.includes(current) ? SLOTS : [...SLOTS, current].sort();

  return (
    <Select value={current} onValueChange={onChange}>
      <SelectTrigger className="w-32 rounded-xl text-sm tabular-nums">
        <SelectValue />
      </SelectTrigger>
      <SelectContent className="max-h-72">
        {options.map((slot) => (
          <SelectItem key={slot} value={slot} className="tabular-nums">
            {format === "24h" ? slot : formatSlot12(slot)}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  );
}

function formatSlot12(slot: string): string {
  const [h, m] = slot.split(":");
  const h24 = Number(h);
  return `${h24 % 12 || 12}:${m} ${h24 >= 12 ? "PM" : "AM"}`;
}
