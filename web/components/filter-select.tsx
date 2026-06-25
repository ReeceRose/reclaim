'use client';

import { cn } from '@/lib/utils';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';

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
  const active = value !== '';
  const selectedLabel = options.find((o) => o.value === value)?.label;
  return (
    <Select value={value || '_all'} onValueChange={(v) => onChange(v === '_all' ? '' : v)}>
      <SelectTrigger
        className={cn(
          'rounded-[11px] text-[0.84rem] h-auto py-[9px] gap-1 transition-colors',
          active
            ? 'border-[color-mix(in_srgb,var(--brand)_45%,transparent)] bg-[color-mix(in_srgb,var(--brand)_7%,transparent)]'
            : 'bg-surface',
          className,
        )}
      >
        <span className={cn('text-[0.78rem] shrink-0', active ? 'text-brand/60' : 'text-muted-dim')}>{label}</span>
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
          <SelectItem key={o.value} value={o.value}>{o.label}</SelectItem>
        ))}
      </SelectContent>
    </Select>
  );
}
