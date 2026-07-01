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
import type { CompatibilityProfile } from "@/lib/api";
import { cn } from "@/lib/utils";
import {
  COMPATIBILITY_SORT_OPTIONS,
  type CompatibilitySortKey,
} from "./constants";

export function CompatibilityFilterBar({
  profile,
  profileOptions,
  onProfileChange,
  search,
  onSearchChange,
  sort,
  onSortChange,
  reason,
  reasonOptions,
  onReasonChange,
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
  profile: string;
  profileOptions: CompatibilityProfile[];
  onProfileChange: (v: string) => void;
  search: string;
  onSearchChange: (v: string) => void;
  sort: CompatibilitySortKey;
  onSortChange: (v: CompatibilitySortKey) => void;
  reason: string;
  reasonOptions: { value: string; label: string }[];
  onReasonChange: (v: string) => void;
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
      <div className="flex items-center gap-2 px-4 py-3 flex-wrap sm:px-7">
        <Select value={profile} onValueChange={onProfileChange}>
          <SelectTrigger className="rounded-[11px] bg-surface text-sm h-auto py-[7px] gap-1 min-w-[190px]">
            <svg
              aria-hidden="true"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              strokeWidth="2"
              className="w-[13px] h-[13px] text-muted-dim shrink-0"
            >
              <rect x="2" y="4" width="20" height="14" rx="2" />
              <line x1="8" y1="22" x2="16" y2="22" />
              <line x1="12" y1="18" x2="12" y2="22" />
            </svg>
            <span className="text-xs text-muted-dim shrink-0">Profile</span>
            <span className="text-muted-dim mx-px">·</span>
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {profileOptions.map((p) => (
              <SelectItem key={p.id} value={p.id}>
                {p.name}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>

        <div className="flex-1 min-w-[160px] relative">
          <svg
            aria-hidden="true"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            strokeWidth="2"
            className="w-[14px] h-[14px] absolute left-[11px] top-1/2 -translate-y-1/2 text-muted-dim pointer-events-none"
          >
            <circle cx="11" cy="11" r="8" />
            <line x1="21" y1="21" x2="16.65" y2="16.65" />
          </svg>
          <Input
            value={search}
            onChange={(e) => onSearchChange(e.target.value)}
            placeholder="Search by title or path…"
            aria-label="Search direct-play list"
            className="rounded-[11px] pl-[34px] text-sm"
          />
        </div>
      </div>
      <div className="flex items-center gap-2 px-4 pb-3 flex-wrap sm:px-7">
        <Select
          value={sort}
          onValueChange={(v) => onSortChange(v as CompatibilitySortKey)}
        >
          <SelectTrigger className="rounded-[11px] bg-surface text-sm h-auto py-[7px] gap-1 min-w-[175px]">
            <svg
              aria-hidden="true"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              strokeWidth="2"
              className="w-[13px] h-[13px] text-muted-dim shrink-0"
            >
              <path d="M3 8h18M6 12h12M10 16h4" />
            </svg>
            <span className="text-xs text-muted-dim shrink-0">Sort</span>
            <span className="text-muted-dim mx-px">·</span>
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {COMPATIBILITY_SORT_OPTIONS.map((o) => (
              <SelectItem key={o.value} value={o.value}>
                {o.label}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        <FilterSelect
          label="Reason"
          value={reason}
          options={reasonOptions}
          onChange={onReasonChange}
          className="min-w-[140px]"
        />
        <FilterSelect
          label="Codec"
          value={codec}
          options={codecOptions}
          onChange={onCodecChange}
          className="min-w-[120px]"
        />
        <FilterSelect
          label="Res"
          value={resolution}
          options={resolutionOptions}
          onChange={onResolutionChange}
          className="min-w-[100px]"
        />
        <FilterSelect
          label="Library"
          value={library}
          options={libraryOptions}
          onChange={onLibraryChange}
          className="min-w-[130px]"
        />
      </div>
    </div>
  );
}
