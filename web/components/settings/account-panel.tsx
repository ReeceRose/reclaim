'use client';

import { Label } from '@/components/ui/label';
import { Input } from '@/components/ui/input';
import { Button } from '@/components/ui/button';

export function AccountPanel({
  username,
  credPassword,
  credConfirm,
  onCredPasswordChange,
  onCredConfirmChange,
  onSave,
  isSaving,
  moviesPath,
  tvPath,
}: {
  username: string;
  credPassword: string;
  credConfirm: string;
  onCredPasswordChange: (v: string) => void;
  onCredConfirmChange: (v: string) => void;
  onSave: () => void;
  isSaving: boolean;
  moviesPath?: string;
  tvPath?: string;
}) {
  return (
    <div className="border border-line rounded-(--radius) p-5" style={{ background: 'var(--surface)' }}>
      <div className="text-[0.72rem] uppercase tracking-[0.11em] text-muted-fg font-bold mb-4">Account</div>
      <div className="mb-4">
        <Label className="text-[0.8rem] font-semibold mb-1.5 block">Username</Label>
        <Input value={username} disabled autoComplete="username" />
      </div>
      <div className="mb-4">
        <Label className="text-[0.8rem] font-semibold mb-1.5 block">New password</Label>
        <Input type="password" value={credPassword} onChange={(e) => onCredPasswordChange(e.target.value)} placeholder="Leave blank to keep current" autoComplete="new-password" />
      </div>
      <div className="mb-4">
        <Label className="text-[0.8rem] font-semibold mb-1.5 block">Confirm password</Label>
        <Input type="password" value={credConfirm} onChange={(e) => onCredConfirmChange(e.target.value)} autoComplete="new-password" />
      </div>
      <Button
        onClick={onSave}
        disabled={isSaving || !credPassword}
        className="rounded-[11px]"
        style={{ background: 'linear-gradient(145deg, var(--brand), var(--brand-2))' }}
      >
        {isSaving ? 'Saving…' : 'Save credentials'}
      </Button>
      <p className="text-[0.75rem] text-muted-dim mt-3">Re-hashed server-side. Takes effect on your next login.</p>

      <div className="border-t border-line-soft my-4" />
      <Label className="text-[0.8rem] font-semibold mb-2 block">
        Media mounts{' '}
        <span className="font-mono text-[0.72rem] text-muted-dim bg-surface-2 border border-line rounded-[6px] px-[7px] py-[2px] ml-1">read-only · set via env</span>
      </Label>
      <div className="font-mono text-[0.75rem] text-muted-dim break-all">
        {moviesPath && <div>{moviesPath} · rw</div>}
        {tvPath && <div>{tvPath} · rw</div>}
      </div>
    </div>
  );
}
