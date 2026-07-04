"use client";

import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { cn } from "@/lib/utils";

export function FilterSelect({
  label,
  value,
  options,
  onChange,
  className,
}: {
  label: string;
  value: string;
  options: { value: string; label: string }[];
  onChange: (v: string) => void;
  className?: string;
}) {
  const active = value !== "";
  const selectedLabel = options.find((o) => o.value === value)?.label;
  return (
    <Select
      value={value || "_all"}
      onValueChange={(v) => onChange(v === "_all" ? "" : v)}
    >
      <SelectTrigger
        className={cn(
          "rounded-xl text-sm h-auto py-2.5 gap-1 transition-colors",
          active ? "border-brand/45 bg-brand/7" : "bg-surface",
          className,
        )}
      >
        <span
          className={cn(
            "text-xs shrink-0",
            active ? "text-brand/60" : "text-muted-dim",
          )}
        >
          {label}
        </span>
        <SelectValue>
          {active ? (
            <>
              <span className="text-muted-dim mx-px">·</span>
              <span className="font-medium text-brand">{selectedLabel}</span>
            </>
          ) : (
            <span className="text-muted-fg">All</span>
          )}
        </SelectValue>
      </SelectTrigger>
      <SelectContent>
        <SelectItem value="_all">All</SelectItem>
        {options.map((o) => (
          <SelectItem key={o.value} value={o.value}>
            {o.label}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  );
}
