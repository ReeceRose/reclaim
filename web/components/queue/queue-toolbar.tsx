"use client";

import { ArrowDownUpIcon, ChevronDownIcon, SearchIcon } from "lucide-react";
import { useState } from "react";
import { FilterSelect } from "@/components/filter-select";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/components/ui/popover";
import type { QueueSortKey } from "@/lib/api";

export const QUEUE_SORT_OPTIONS: {
  value: QueueSortKey;
  label: string;
  hint: string;
}[] = [
  {
    value: "savings_per_hour_desc",
    label: "Most savings per hour",
    hint: "Packs the most reclaimed space into each encode window.",
  },
  {
    value: "savings_desc",
    label: "Biggest savings first",
    hint: "Largest predicted savings run first, however long they take.",
  },
  {
    value: "duration_asc",
    label: "Shortest encode first",
    hint: "Quick jobs run first, so more files finish each night.",
  },
  {
    value: "size_desc",
    label: "Largest file first",
    hint: "Biggest source files run first.",
  },
  {
    value: "path_asc",
    label: "Title A–Z",
    hint: "Alphabetical by path, which keeps a show's episodes in order.",
  },
  {
    value: "queued_at_asc",
    label: "Order queued",
    hint: "The order jobs were added in, undoing any manual moves.",
  },
];

export type QueueFilterOption = { value: string; label: string };

export function QueueSearch({
  value,
  onChange,
  placeholder,
  label,
}: {
  value: string;
  onChange: (v: string) => void;
  placeholder: string;
  label: string;
}) {
  return (
    <div className="relative flex-1 min-w-48">
      <SearchIcon className="size-3.5 absolute left-3 top-1/2 -translate-y-1/2 text-muted-dim pointer-events-none" />
      <Input
        value={value}
        onChange={(e) => onChange(e.target.value)}
        placeholder={placeholder}
        aria-label={label}
        className="rounded-xl pl-9 text-sm"
      />
    </div>
  );
}

export function QueueToolbar({
  search,
  onSearchChange,
  library,
  libraryOptions,
  onLibraryChange,
  codec,
  codecOptions,
  onCodecChange,
  profile,
  profileOptions,
  onProfileChange,
  forced,
  onForcedChange,
  onSort,
  sortDisabled,
  filtered,
}: {
  search: string;
  onSearchChange: (v: string) => void;
  library: string;
  libraryOptions: QueueFilterOption[];
  onLibraryChange: (v: string) => void;
  codec: string;
  codecOptions: QueueFilterOption[];
  onCodecChange: (v: string) => void;
  profile: string;
  profileOptions: QueueFilterOption[];
  onProfileChange: (v: string) => void;
  forced: string;
  onForcedChange: (v: string) => void;
  onSort: (by: QueueSortKey) => void;
  sortDisabled: boolean;
  filtered: boolean;
}) {
  return (
    <div className="flex flex-col gap-2 mb-4">
      <QueueSearch
        value={search}
        onChange={onSearchChange}
        placeholder="Search the queue…"
        label="Search queued jobs"
      />
      <div className="flex items-center gap-2 flex-wrap">
        {libraryOptions.length > 1 && (
          <FilterSelect
            label="Library"
            value={library}
            options={libraryOptions}
            onChange={onLibraryChange}
            className="min-w-32"
          />
        )}
        {codecOptions.length > 0 && (
          <FilterSelect
            label="Source"
            value={codec}
            options={codecOptions}
            onChange={onCodecChange}
            className="min-w-32"
          />
        )}
        {profileOptions.length > 1 && (
          <FilterSelect
            label="Profile"
            value={profile}
            options={profileOptions}
            onChange={onProfileChange}
            className="min-w-32"
          />
        )}
        <FilterSelect
          label="Run now"
          value={forced}
          options={[{ value: "true", label: "Forced only" }]}
          onChange={onForcedChange}
          className="min-w-32"
        />
        <SortMenu onSort={onSort} disabled={sortDisabled} filtered={filtered} />
      </div>
    </div>
  );
}

function SortMenu({
  onSort,
  disabled,
  filtered,
}: {
  onSort: (by: QueueSortKey) => void;
  disabled: boolean;
  filtered: boolean;
}) {
  const [open, setOpen] = useState(false);
  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <Button
          variant="outline"
          disabled={disabled}
          className="rounded-xl border-line bg-surface h-auto py-2.5 px-3 gap-1.5 text-sm font-normal text-muted-fg hover:text-text min-w-56 justify-start sm:ml-auto"
        >
          <ArrowDownUpIcon className="size-3.5 text-muted-dim" />
          {filtered ? "Sort matches…" : "Sort queue…"}
          <ChevronDownIcon className="size-4 opacity-50 ml-auto" />
        </Button>
      </PopoverTrigger>
      <PopoverContent
        align="end"
        className="w-[28rem] max-w-[calc(100vw-2rem)] p-0 rounded-xl border-line bg-surface overflow-hidden shadow-xl"
      >
        <div className="px-3 py-2.5 bg-surface-2 border-b border-line text-2xs uppercase tracking-wider font-bold text-muted-dim">
          {filtered ? "Sort matching jobs" : "Sort the whole queue"}
        </div>
        <div className="p-1.5 flex flex-col">
          {QUEUE_SORT_OPTIONS.map((o) => (
            <Button
              key={o.value}
              variant="ghost"
              onClick={() => {
                setOpen(false);
                onSort(o.value);
              }}
              className="h-auto flex-col items-start gap-0.5 rounded-lg px-2.5 py-2 text-left whitespace-normal hover:bg-surface-2"
            >
              <span className="text-sm font-medium text-text">{o.label}</span>
              <span className="text-xs font-normal text-muted-dim">
                {o.hint}
              </span>
            </Button>
          ))}
        </div>
      </PopoverContent>
    </Popover>
  );
}

export function ConfirmQueueAction({
  open,
  title,
  description,
  confirmLabel,
  destructive,
  pending,
  onConfirm,
  onOpenChange,
}: {
  open: boolean;
  title: string;
  description: string;
  confirmLabel: string;
  destructive?: boolean;
  pending: boolean;
  onConfirm: () => void;
  onOpenChange: (open: boolean) => void;
}) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-md">
        <DialogHeader>
          <DialogTitle>{title}</DialogTitle>
          <DialogDescription>{description}</DialogDescription>
        </DialogHeader>
        <DialogFooter>
          <Button variant="ghost" onClick={() => onOpenChange(false)}>
            Keep as is
          </Button>
          <Button
            variant={destructive ? "destructive" : "default"}
            onClick={onConfirm}
            disabled={pending}
          >
            {confirmLabel}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
