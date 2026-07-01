import type { ReactNode } from "react";

export function DetailRow({
  label,
  value,
  mono,
}: {
  label: string;
  value: ReactNode;
  mono?: boolean;
}) {
  if (value == null || value === "" || value === "—") return null;
  return (
    <div className="flex items-baseline justify-between gap-4 py-2 border-b border-line-soft last:border-b-0">
      <span className="text-xs text-muted-fg shrink-0">{label}</span>
      <span className={`text-sm text-right ${mono ? "font-mono" : ""}`}>
        {value}
      </span>
    </div>
  );
}

export function DetailSection({
  title,
  children,
}: {
  title: string;
  children: ReactNode;
}) {
  return (
    <div className="mb-5 last:mb-0">
      <div className="text-xs uppercase tracking-widest text-muted-dim font-bold mb-2">
        {title}
      </div>
      <div
        className="rounded-xl border border-line px-3"
        style={{ background: "var(--surface-2)" }}
      >
        {children}
      </div>
    </div>
  );
}
