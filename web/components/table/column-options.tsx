"use client";

import { type DragEvent, type KeyboardEvent, useId, useState } from "react";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/components/ui/popover";
import type { TableColumns } from "@/hooks/use-table-columns";
import { type ColumnDef, columnHint, moveInOrder } from "@/lib/table-columns";
import { cn } from "@/lib/utils";

function ColumnsIcon() {
  return (
    <svg
      aria-hidden="true"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
      className="w-3.5 h-3.5"
    >
      <rect x="3" y="4" width="5" height="16" rx="1" />
      <rect x="10" y="4" width="5" height="16" rx="1" />
      <path d="M19 4v16" strokeDasharray="2 3" />
    </svg>
  );
}

function GripIcon() {
  return (
    <svg
      aria-hidden="true"
      viewBox="0 0 24 24"
      fill="currentColor"
      className="w-3 h-3"
    >
      <circle cx="9" cy="6" r="1.4" />
      <circle cx="15" cy="6" r="1.4" />
      <circle cx="9" cy="12" r="1.4" />
      <circle cx="15" cy="12" r="1.4" />
      <circle cx="9" cy="18" r="1.4" />
      <circle cx="15" cy="18" r="1.4" />
    </svg>
  );
}

export function ColumnOptions<T, S extends string>({
  table,
  label = "Columns",
  variant = "outline",
  className,
}: {
  table: TableColumns<T, S>;
  label?: string;
  variant?: "outline" | "ghost";
  className?: string;
}) {
  const { ordered, columns, isDefault, isVisible, setVisible, setOrder, move } =
    table;
  const fieldId = useId();
  const [dragId, setDragId] = useState<string | null>(null);
  const [preview, setPreview] = useState<string[] | null>(null);

  const byId = new Map(ordered.map((col) => [col.id, col]));
  const rows = (preview ?? ordered.map((col) => col.id))
    .map((id) => byId.get(id))
    .filter((col): col is ColumnDef<T, S> => col !== undefined);

  function handleDragStart(event: DragEvent<HTMLLIElement>, id: string) {
    setDragId(id);
    setPreview(ordered.map((col) => col.id));
    event.dataTransfer.effectAllowed = "move";
    event.dataTransfer.setData("text/plain", id);
  }

  function handleDragOver(event: DragEvent<HTMLLIElement>, targetId: string) {
    event.preventDefault();
    event.dataTransfer.dropEffect = "move";
    if (!dragId || dragId === targetId) return;
    setPreview((prev) => {
      const base = prev ?? ordered.map((col) => col.id);
      const to = base.indexOf(targetId);
      return to < 0 ? base : moveInOrder(base, dragId, to);
    });
  }

  function handleDrop(event: DragEvent<HTMLLIElement>) {
    event.preventDefault();
    if (preview) setOrder(preview);
    setPreview(null);
    setDragId(null);
  }

  function handleDragEnd() {
    setPreview(null);
    setDragId(null);
  }

  function handleGripKeyDown(
    event: KeyboardEvent<HTMLButtonElement>,
    id: string,
    index: number,
  ) {
    if (event.key === "ArrowUp" && index > 0) {
      event.preventDefault();
      move(id, index - 1);
    }
    if (event.key === "ArrowDown" && index < rows.length - 1) {
      event.preventDefault();
      move(id, index + 1);
    }
  }

  return (
    <Popover>
      <PopoverTrigger asChild>
        <Button
          variant={variant}
          size="sm"
          className={cn(
            "gap-1.5 text-xs font-semibold text-muted-fg hover:text-text",
            variant === "outline" &&
              "rounded-xl border-line bg-surface h-auto py-2.5 px-3",
            className,
          )}
        >
          <ColumnsIcon />
          {label}
          {!isDefault && (
            <span className="size-1.5 rounded-full bg-brand shrink-0" />
          )}
        </Button>
      </PopoverTrigger>
      <PopoverContent
        align="end"
        className="w-64 p-0 rounded-xl border-line bg-surface overflow-hidden shadow-xl"
      >
        <div className="flex items-center justify-between px-3 py-2.5 bg-surface-2 border-b border-line">
          <span className="text-2xs uppercase tracking-wider font-bold text-muted-dim">
            Columns
          </span>
          <span className="text-2xs font-mono text-muted-dim">
            {columns.length}/{ordered.length}
          </span>
        </div>

        <ul className="p-1.5 max-h-[19rem] overflow-y-auto">
          {rows.map((col, index) => {
            const checked = isVisible(col.id);
            const id = `${fieldId}-${col.id}`;
            return (
              <li
                key={col.id}
                draggable
                onDragStart={(e) => handleDragStart(e, col.id)}
                onDragOver={(e) => handleDragOver(e, col.id)}
                onDrop={handleDrop}
                onDragEnd={handleDragEnd}
                className={cn(
                  "group flex items-center gap-1 rounded-lg pl-1 pr-2.5 py-1.5 transition-colors",
                  dragId === col.id
                    ? "bg-surface-2 ring-1 ring-brand-line"
                    : "hover:bg-surface-2",
                )}
              >
                <button
                  type="button"
                  aria-label={`Reorder ${col.label}`}
                  onKeyDown={(e) => handleGripKeyDown(e, col.id, index)}
                  className="shrink-0 p-1 rounded-md text-muted-dim/50 group-hover:text-muted-dim cursor-grab active:cursor-grabbing focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/50"
                >
                  <GripIcon />
                </button>
                <label
                  htmlFor={id}
                  data-tooltip={[col.description, columnHint(col)]
                    .filter(Boolean)
                    .join(" · ")}
                  className={cn(
                    "flex-1 min-w-0 truncate text-sm cursor-pointer select-none transition-colors",
                    checked ? "text-text" : "text-muted-dim",
                    col.locked && "cursor-default",
                  )}
                >
                  {col.label}
                </label>
                <Checkbox
                  id={id}
                  checked={checked}
                  disabled={col.locked}
                  onCheckedChange={(next) => setVisible(col.id, next === true)}
                  className="size-4 rounded-md shrink-0 cursor-pointer disabled:opacity-35"
                />
              </li>
            );
          })}
        </ul>

        <div className="flex items-center justify-between gap-2 px-3 py-2 border-t border-line bg-surface-2">
          <span className="text-2xs text-muted-dim">
            Drag to reorder · saved in this browser
          </span>
          <Button
            variant="ghost"
            size="xs"
            disabled={isDefault}
            onClick={table.reset}
            className="text-2xs text-muted-dim hover:text-text -mr-1.5 shrink-0"
          >
            Reset
          </Button>
        </div>
      </PopoverContent>
    </Popover>
  );
}
