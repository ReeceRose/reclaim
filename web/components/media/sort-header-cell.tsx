"use client";

import type { ReactNode } from "react";
import { cn } from "@/lib/utils";

export function SortHeaderCell({
  active,
  arrow,
  onClick,
  className,
  align = "left",
  children,
}: {
  active: boolean;
  arrow: "↑" | "↓" | null;
  onClick: () => void;
  className?: string;
  align?: "left" | "right";
  children: ReactNode;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={cn(
        className,
        "inline-flex items-center gap-0 font-bold uppercase tracking-wider transition-colors",
        "cursor-pointer bg-transparent border-0 p-0 m-0",
        align === "right" ? "justify-end" : "justify-start",
        active
          ? "text-brand hover:text-brand"
          : "text-muted-fg hover:text-text",
      )}
    >
      <span>{children}</span>
      <span
        aria-hidden
        className="inline-block w-[0.75em] shrink-0 text-xs leading-none"
      >
        {arrow ?? ""}
      </span>
    </button>
  );
}
