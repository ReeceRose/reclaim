"use client";

import { FilterSelect } from "@/components/filter-select";
import { Input } from "@/components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { cn } from "@/lib/utils";
import { CANDIDATE_SORT_OPTIONS, type CandidateSortKey } from "./constants";

export function CandidatesFilterBar({
  search,
  onSearchChange,
  sort,
  onSortChange,
  codec,
  codecOptions,
  onCodecChange,
  resolution,
  resolutionOptions,
  onResolutionChange,
  library,
  libraryOptions,
  onLibraryChange,
  isPending,
}: {
  search: string;
  onSearchChange: (v: string) => void;
  sort: CandidateSortKey;
  onSortChange: (v: CandidateSortKey) => void;
  codec: string;
  codecOptions: { value: string; label: string }[];
  onCodecChange: (v: string) => void;
  resolution: string;
  resolutionOptions: { value: string; label: string }[];
  onResolutionChange: (v: string) => void;
  library: string;
  libraryOptions: { value: string; label: string }[];
  onLibraryChange: (v: string) => void;
  isPending?: boolean;
}) {
  return (
    <div
      className={cn(
        "border-b border-line-soft shrink-0",
        isPending && "opacity-50",
      )}
      style={{ background: "var(--bg)" }}
    >
      <div className="flex items-center gap-2 px-4 py-3 sm:px-7">
        <div className="flex-1 relative">
          <svg
            aria-hidden="true"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            strokeWidth="2"
            className="w-3.5 h-3.5 absolute left-3 top-1/2 -translate-y-1/2 text-muted-dim pointer-events-none"
          >
            <circle cx="11" cy="11" r="8" />
            <line x1="21" y1="21" x2="16.65" y2="16.65" />
          </svg>
          <Input
            value={search}
            onChange={(e) => onSearchChange(e.target.value)}
            placeholder="Search by title or path…"
            aria-label="Search candidates"
            className="rounded-xl pl-9 text-sm"
          />
        </div>
      </div>
      <div className="flex items-center gap-2 px-4 pb-3 flex-wrap sm:px-7">
        <Select
          value={sort}
          onValueChange={(v) => onSortChange(v as CandidateSortKey)}
        >
          <SelectTrigger className="rounded-xl bg-surface text-sm h-auto py-2 gap-1 min-w-48">
            <svg
              aria-hidden="true"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              strokeWidth="2"
              className="w-3.5 h-3.5 text-muted-dim shrink-0"
            >
              <path d="M3 8h18M6 12h12M10 16h4" />
            </svg>
            <span className="text-xs text-muted-dim shrink-0">Sort</span>
            <span className="text-muted-dim mx-px">·</span>
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {CANDIDATE_SORT_OPTIONS.map((o) => (
              <SelectItem key={o.value} value={o.value}>
                {o.label}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        <FilterSelect
          label="Codec"
          value={codec}
          options={codecOptions}
          onChange={onCodecChange}
          className="min-w-32"
        />
        <FilterSelect
          label="Res"
          value={resolution}
          options={resolutionOptions}
          onChange={onResolutionChange}
          className="min-w-24"
        />
        <FilterSelect
          label="Library"
          value={library}
          options={libraryOptions}
          onChange={onLibraryChange}
          className="min-w-32"
        />
      </div>
    </div>
  );
}
