"use client";

import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";

export function TimeSelect({
  value,
  onChange,
}: {
  value: string;
  onChange: (v: string) => void;
}) {
  const parts = value.split(":");
  const h24 = parseInt(parts[0] ?? "0", 10);
  const mins = parts[1] ?? "00";
  const isPM = h24 >= 12;
  const h12 = h24 % 12 || 12;

  function update(newH12: number, newIsPM: boolean) {
    const newH24 = newIsPM ? (newH12 % 12) + 12 : newH12 % 12;
    onChange(`${String(newH24).padStart(2, "0")}:${mins}`);
  }

  return (
    <div className="flex items-center gap-1.5">
      <Select
        value={String(h12)}
        onValueChange={(v) => update(Number(v), isPM)}
      >
        <SelectTrigger className="w-[90px] rounded-[10px] text-sm">
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          {[12, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11].map((h) => (
            <SelectItem key={h} value={String(h)}>
              {h}:00
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
      <Select
        value={isPM ? "PM" : "AM"}
        onValueChange={(v) => update(h12, v === "PM")}
      >
        <SelectTrigger className="w-[72px] rounded-[10px] text-sm">
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value="AM">AM</SelectItem>
          <SelectItem value="PM">PM</SelectItem>
        </SelectContent>
      </Select>
    </div>
  );
}
