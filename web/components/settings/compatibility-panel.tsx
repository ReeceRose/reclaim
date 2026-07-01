"use client";

import { Button } from "@/components/ui/button";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import type { CompatibilityProfile } from "@/lib/api";
import { LabelWithHelp } from "./help-tip";

export function CompatibilityPanel({
  profiles,
  value,
  onChange,
  onSave,
  isSaving,
}: {
  profiles: CompatibilityProfile[];
  value: string;
  onChange: (v: string) => void;
  onSave: () => void;
  isSaving: boolean;
}) {
  const selected = profiles.find((p) => p.id === value);
  return (
    <div
      className="border border-line rounded-(--radius) p-5 mt-[18px]"
      style={{ background: "var(--surface)" }}
    >
      <div className="text-[0.72rem] uppercase tracking-[0.11em] text-muted-fg font-bold mb-4">
        Compatibility
      </div>
      <LabelWithHelp
        label="Default client profile"
        help={
          <>
            Which device profile the{" "}
            <span className="font-mono">Direct play</span> page and overview
            stat use by default. Predictions are based on file metadata only —
            actual playback still depends on your server settings.
          </>
        }
      />
      <Select value={value} onValueChange={onChange}>
        <SelectTrigger className="w-full sm:max-w-xs">
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          {profiles.map((p) => (
            <SelectItem key={p.id} value={p.id}>
              {p.name}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
      {selected?.description && (
        <p className="text-[0.75rem] text-muted-dim mt-1.5 leading-relaxed">
          {selected.description}
        </p>
      )}
      <Button
        onClick={onSave}
        disabled={isSaving}
        className="rounded-[11px] mt-4"
        style={{
          background: "linear-gradient(145deg, var(--brand), var(--brand-2))",
        }}
      >
        {isSaving ? "Saving…" : "Save settings"}
      </Button>
    </div>
  );
}
