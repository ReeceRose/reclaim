"use client";

import { Button } from "@/components/ui/button";
import type { Profile } from "@/lib/api";

export function ProfilesPanel({
  profiles,
  onNew,
  onEdit,
  onSetDefault,
  onDelete,
  isSettingDefault,
}: {
  profiles: Profile[];
  onNew: () => void;
  onEdit: (profile: Profile) => void;
  onSetDefault: (profile: Profile) => void;
  onDelete: (profile: Profile) => void;
  isSettingDefault: boolean;
}) {
  return (
    <div
      className="border border-line rounded-(--radius) p-5"
      style={{ background: "var(--surface)" }}
    >
      <div className="flex items-center mb-4">
        <div className="text-xs uppercase tracking-widest text-muted-fg font-bold">
          Transcode profiles
        </div>
        <Button
          variant="outline"
          size="sm"
          onClick={onNew}
          className="ml-auto rounded-xl"
        >
          + New profile
        </Button>
      </div>
      {profiles.map((p) => (
        <div
          key={p.id}
          className="flex flex-wrap items-center gap-x-3.5 gap-y-2 border rounded-xl px-4 py-3.5 mb-2.5 last:mb-0"
          style={{
            borderColor: p.is_default ? "var(--brand-line)" : "var(--line)",
            background: p.is_default ? "var(--brand-soft)" : undefined,
          }}
        >
          <div className="flex-1 min-w-0">
            <div className="font-semibold text-base">
              {p.name}
              {p.is_default && (
                <span className="ml-2 text-xs font-bold uppercase tracking-wider text-brand">
                  Default
                </span>
              )}
            </div>
            <div className="text-xs text-muted-fg font-mono mt-0.5 wrap-break-word">
              libx265 · CRF {p.crf} · preset {p.preset}
              {p.extra_args && ` · ${p.extra_args}`}
            </div>
          </div>
          <div className="flex gap-1 basis-full justify-end sm:basis-auto">
            <Button
              variant="ghost"
              size="sm"
              onClick={() => onEdit(p)}
              className="rounded-xl"
            >
              Edit
            </Button>
            {!p.is_default && (
              <>
                <Button
                  variant="ghost"
                  size="sm"
                  onClick={() => onSetDefault(p)}
                  disabled={isSettingDefault}
                  className="rounded-xl"
                >
                  Set default
                </Button>
                <Button
                  variant="ghost"
                  size="sm"
                  onClick={() => onDelete(p)}
                  className="rounded-xl text-red hover:bg-red-soft hover:text-red"
                >
                  Delete
                </Button>
              </>
            )}
          </div>
        </div>
      ))}
    </div>
  );
}
