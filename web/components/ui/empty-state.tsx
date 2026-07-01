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
          className="w-12 h-12 rounded-[14px] border border-line grid place-items-center text-muted-dim"
          style={{ background: "var(--surface-2)" }}
        >
          {icon}
        </div>
      )}
      <div>
        <div className="text-[0.9rem] font-semibold text-text">{title}</div>
        {description && (
          <div className="text-[0.78rem] text-muted-dim mt-1 max-w-[260px]">
            {description}
          </div>
        )}
      </div>
      {children}
    </div>
  );
}
