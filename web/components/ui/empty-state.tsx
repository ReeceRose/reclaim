import type { ReactNode } from "react";
import { cn } from "@/lib/utils";

export function EmptyState({
  icon,
  title,
  description,
  children,
  className,
}: {
  icon?: ReactNode;
  title: string;
  description?: ReactNode;
  children?: ReactNode;
  className?: string;
}) {
  return (
    <div
      className={cn(
        "flex flex-col items-center justify-center gap-4 py-20 text-center",
        className,
      )}
    >
      {icon && (
        <div
          className="w-12 h-12 rounded-2xl border border-line grid place-items-center text-muted-dim"
          style={{ background: "var(--surface-2)" }}
        >
          {icon}
        </div>
      )}
      <div>
        <div className="text-sm font-semibold text-text">{title}</div>
        {description && (
          <div className="text-xs text-muted-dim mt-1 max-w-3xs">
            {description}
          </div>
        )}
      </div>
      {children}
    </div>
  );
}
