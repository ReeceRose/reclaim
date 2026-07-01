'use client';

import { type Profile } from '@/lib/api';
import { Button } from '@/components/ui/button';

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
    <div className="border border-line rounded-(--radius) p-5" style={{ background: 'var(--surface)' }}>
      <div className="flex items-center mb-4">
        <div className="text-[0.72rem] uppercase tracking-[0.11em] text-muted-fg font-bold">Transcode profiles</div>
        <Button
          variant="outline"
          size="sm"
          onClick={onNew}
          className="ml-auto rounded-[11px]"
        >
          + New profile
        </Button>
      </div>
      {profiles.map((p) => (
        <div
          key={p.id}
          className="flex flex-wrap items-center gap-x-3.5 gap-y-2 border rounded-[12px] px-4 py-[14px] mb-2.5 last:mb-0"
          style={{
            borderColor: p.is_default ? 'var(--brand-line)' : 'var(--line)',
            background: p.is_default ? 'var(--brand-soft)' : undefined,
          }}
        >
          <div className="flex-1 min-w-0">
            <div className="font-semibold text-[0.95rem]">
              {p.name}
              {p.is_default && (
                <span className="ml-2 text-[0.66rem] font-bold uppercase tracking-wider text-brand">Default</span>
              )}
            </div>
            <div className="text-[0.78rem] text-muted-fg font-mono mt-0.5 wrap-break-word">
              libx265 · CRF {p.crf} · preset {p.preset}
              {p.extra_args && ` · ${p.extra_args}`}
            </div>
          </div>
          <div className="flex gap-1 basis-full justify-end sm:basis-auto">
            <Button
              variant="ghost"
              size="sm"
              onClick={() => onEdit(p)}
              className="rounded-[11px]"
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
                  className="rounded-[11px]"
                >
                  Set default
                </Button>
                <Button
                  variant="ghost"
                  size="sm"
                  onClick={() => onDelete(p)}
                  className="rounded-[11px] text-red hover:bg-red-soft hover:text-red"
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
