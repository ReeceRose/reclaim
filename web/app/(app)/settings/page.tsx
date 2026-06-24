'use client';

import { useSuspenseQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { api, type Profile } from '@/lib/api';
import { useState, Suspense } from 'react';
import { Skeleton } from '@/components/ui/skeleton';
import { toast } from 'sonner';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Button } from '@/components/ui/button';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from '@/components/ui/dialog';

function ProfileDialog({
  open,
  onClose,
  initial,
}: {
  open: boolean;
  onClose: () => void;
  initial: Partial<Profile> | null;
}) {
  const qc = useQueryClient();
  const [name, setName] = useState(initial?.name ?? '');
  const [crf, setCrf] = useState(initial?.crf ?? 26);
  const [preset, setPreset] = useState(initial?.preset ?? 'medium');
  const [extra, setExtra] = useState(initial?.extra_args ?? '-c:a copy -c:s copy');
  const [loading, setLoading] = useState(false);

  async function handleSave() {
    setLoading(true);
    try {
      const body = { name, crf, preset, extra_args: extra, is_default: initial?.is_default ?? false };
      if (initial?.id) {
        await api.updateProfile(initial.id, body);
        toast.success('Profile updated');
      } else {
        await api.createProfile(body);
        toast.success('Profile created');
      }
      qc.invalidateQueries({ queryKey: ['profiles'] });
      onClose();
    } catch {
      toast.error('Save failed');
    } finally {
      setLoading(false);
    }
  }

  return (
    <Dialog open={open} onOpenChange={(o) => !o && onClose()}>
      <DialogContent className="max-w-md border-line p-0 overflow-hidden" style={{ background: 'var(--surface)' }}>
        <DialogHeader className="px-6 pt-[22px] pb-4 border-b border-line">
          <DialogTitle className="text-[1.1rem] font-bold">{initial?.id ? 'Edit profile' : 'New profile'}</DialogTitle>
        </DialogHeader>
        <div className="px-6 py-5 space-y-4">
          <div>
            <Label className="text-[0.8rem] font-semibold mb-1.5 block">Name</Label>
            <Input value={name} onChange={(e) => setName(e.target.value)} />
          </div>
          <div className="grid grid-cols-2 gap-4">
            <div>
              <Label className="text-[0.8rem] font-semibold mb-1.5 block">CRF</Label>
              <Input type="number" min={0} max={51} value={crf} onChange={(e) => setCrf(Number(e.target.value))} />
            </div>
            <div>
              <Label className="text-[0.8rem] font-semibold mb-1.5 block">Preset</Label>
              <Select value={preset} onValueChange={setPreset}>
                <SelectTrigger className="rounded-lg text-sm">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {['ultrafast', 'superfast', 'veryfast', 'faster', 'fast', 'medium', 'slow', 'slower', 'veryslow'].map((p) => (
                    <SelectItem key={p} value={p}>{p}</SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
          </div>
          <div>
            <Label className="text-[0.8rem] font-semibold mb-1.5 block">Extra args</Label>
            <Input className="font-mono text-sm" value={extra} onChange={(e) => setExtra(e.target.value)} />
            <p className="text-[0.75rem] text-muted-dim mt-1">Audio/subtitle passthrough etc.</p>
          </div>
        </div>
        <DialogFooter className="px-6 py-4 border-t border-line flex justify-end gap-2">
          <Button variant="outline" onClick={onClose} className="rounded-[11px]">
            Cancel
          </Button>
          <Button
            onClick={() => void handleSave()}
            disabled={loading || !name}
            className="rounded-[11px]"
            style={{ background: 'linear-gradient(145deg, var(--brand), var(--brand-2))' }}
          >
            {loading ? 'Saving…' : 'Save'}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

function SettingsSkeleton() {
  return (
    <div className="px-7 py-[26px] max-w-[1180px] w-full pb-14">
      <div className="grid grid-cols-2 gap-[18px] mb-[18px] max-sm:grid-cols-1">
        {[0, 1].map((i) => (
          <div key={i} className="border border-line rounded-[var(--radius)] p-5" style={{ background: 'var(--surface)' }}>
            <Skeleton className="h-3 w-24 mb-5" />
            {[0, 1, 2].map((j) => (
              <div key={j} className="mb-4">
                <Skeleton className="h-3 w-28 mb-2" />
                <Skeleton className="h-9 w-full rounded-[10px]" />
              </div>
            ))}
            <Skeleton className="h-9 w-28 rounded-[11px]" />
          </div>
        ))}
      </div>
      <div className="border border-line rounded-[var(--radius)] p-5" style={{ background: 'var(--surface)' }}>
        <div className="flex items-center mb-4">
          <Skeleton className="h-3 w-36" />
          <Skeleton className="ml-auto h-8 w-28 rounded-[11px]" />
        </div>
        {[0, 1].map((i) => (
          <div key={i} className="flex items-center gap-3.5 border border-line rounded-[12px] px-4 py-[14px] mb-2.5">
            <div className="flex-1 min-w-0">
              <Skeleton className="h-4 w-32 mb-1.5" />
              <Skeleton className="h-3 w-48" />
            </div>
            <Skeleton className="h-7 w-12 rounded-[11px]" />
            <Skeleton className="h-7 w-14 rounded-[11px]" />
          </div>
        ))}
      </div>
    </div>
  );
}

function SettingsContent() {
  const qc = useQueryClient();

  const { data: settings } = useSuspenseQuery({ queryKey: ['settings'], queryFn: api.settings });
  const { data: profilesData } = useSuspenseQuery({ queryKey: ['profiles'], queryFn: api.profiles, staleTime: 30_000 });
  const profiles = profilesData.items ?? [];

  const [windowStart, setWindowStart] = useState(settings.encode_window_start);
  const [windowEnd, setWindowEnd] = useState(settings.encode_window_end);
  const [scanInterval, setScanInterval] = useState(settings.scan_interval);
  const [probeConcurrency, setProbeConcurrency] = useState(settings.probe_concurrency);

  const [credUsername, setCredUsername] = useState('');
  const [credPassword, setCredPassword] = useState('');
  const [credConfirm, setCredConfirm] = useState('');

  const settingsMutation = useMutation({
    mutationFn: () =>
      api.updateSettings({
        encode_window_start: windowStart,
        encode_window_end: windowEnd,
        scan_interval: scanInterval,
        probe_concurrency: probeConcurrency,
      }),
    onSuccess: () => {
      toast.success('Settings saved');
      qc.invalidateQueries({ queryKey: ['settings'] });
    },
    onError: () => toast.error('Failed to save settings'),
  });

  const credMutation = useMutation({
    mutationFn: () => api.changeCredentials(credUsername, credPassword),
    onSuccess: () => {
      toast.success('Credentials updated');
      setCredUsername('');
      setCredPassword('');
      setCredConfirm('');
    },
    onError: () => toast.error('Failed to update credentials'),
  });

  const deleteProfileMutation = useMutation({
    mutationFn: (id: number) => api.deleteProfile(id),
    onSuccess: () => {
      toast.success('Profile deleted');
      qc.invalidateQueries({ queryKey: ['profiles'] });
    },
    onError: () => toast.error('Delete failed'),
  });

  const [profileDialog, setProfileDialog] = useState<{ open: boolean; initial: Partial<Profile> | null }>({
    open: false,
    initial: null,
  });

  function handleCredSave() {
    if (credPassword !== credConfirm) {
      toast.error('Passwords do not match');
      return;
    }
    credMutation.mutate();
  }

  return (
    <>
      <div
        className="flex items-center gap-4 px-7 py-[18px] border-b border-line"
        style={{ background: 'rgba(22,22,22,.82)', backdropFilter: 'blur(10px)' }}
      >
        <div>
          <div className="text-[1.38rem] font-bold tracking-tight">Settings</div>
          <div className="text-[0.82rem] text-muted-fg mt-px">Changes apply live — no restart</div>
        </div>
      </div>

      <div className="px-7 py-[26px] max-w-[1180px] w-full pb-14">
        <div className="grid grid-cols-2 gap-[18px] mb-[18px] max-sm:grid-cols-1">
          <div className="border border-line rounded-[var(--radius)] p-5" style={{ background: 'var(--surface)' }}>
            <div className="text-[0.72rem] uppercase tracking-[0.11em] text-muted-fg font-bold mb-4">Encoding</div>
            <div className="mb-4">
              <Label className="text-[0.8rem] font-semibold mb-1.5 block">
                Encode window <span className="text-muted-dim font-normal">· when jobs may run</span>
              </Label>
              <div className="flex items-center gap-3">
                <Input
                  type="time"
                  value={windowStart}
                  onChange={(e) => setWindowStart(e.target.value)}
                  className="w-auto rounded-[10px]"
                />
                <span className="text-muted-fg">to</span>
                <Input
                  type="time"
                  value={windowEnd}
                  onChange={(e) => setWindowEnd(e.target.value)}
                  className="w-auto rounded-[10px]"
                />
              </div>
              <p className="text-[0.75rem] text-muted-dim mt-1.5">A running job finishes even if the window closes — only new pulls stop.</p>
            </div>
            <div className="mb-4">
              <Label className="text-[0.8rem] font-semibold mb-1.5 block">Probe concurrency</Label>
              <Input
                type="number"
                min={1}
                max={32}
                value={probeConcurrency}
                onChange={(e) => setProbeConcurrency(Number(e.target.value))}
              />
              <p className="text-[0.75rem] text-muted-dim mt-1.5">Parallel ffprobe cap during scans.</p>
            </div>
            <div className="mb-4">
              <Label className="text-[0.8rem] font-semibold mb-1.5 block">Scan interval</Label>
              <Input value={scanInterval} onChange={(e) => setScanInterval(e.target.value)} />
              <p className="text-[0.75rem] text-muted-dim mt-1.5">Scheduled safety-net rescan, independent of the window.</p>
            </div>
            <Button
              variant="outline"
              onClick={() => settingsMutation.mutate()}
              disabled={settingsMutation.isPending}
              className="rounded-[11px]"
            >
              {settingsMutation.isPending ? 'Saving…' : 'Save settings'}
            </Button>
          </div>

          <div className="border border-line rounded-[var(--radius)] p-5" style={{ background: 'var(--surface)' }}>
            <div className="text-[0.72rem] uppercase tracking-[0.11em] text-muted-fg font-bold mb-4">Account</div>
            <div className="mb-4">
              <Label className="text-[0.8rem] font-semibold mb-1.5 block">Username</Label>
              <Input value={credUsername} onChange={(e) => setCredUsername(e.target.value)} autoComplete="username" />
            </div>
            <div className="mb-4">
              <Label className="text-[0.8rem] font-semibold mb-1.5 block">New password</Label>
              <Input type="password" value={credPassword} onChange={(e) => setCredPassword(e.target.value)} placeholder="Leave blank to keep current" autoComplete="new-password" />
            </div>
            <div className="mb-4">
              <Label className="text-[0.8rem] font-semibold mb-1.5 block">Confirm password</Label>
              <Input type="password" value={credConfirm} onChange={(e) => setCredConfirm(e.target.value)} autoComplete="new-password" />
            </div>
            <Button
              onClick={handleCredSave}
              disabled={credMutation.isPending || !credUsername || !credPassword}
              className="rounded-[11px]"
              style={{ background: 'linear-gradient(145deg, var(--brand), var(--brand-2))' }}
            >
              {credMutation.isPending ? 'Saving…' : 'Save credentials'}
            </Button>
            <p className="text-[0.75rem] text-muted-dim mt-3">Re-hashed server-side. Takes effect on your next login.</p>

            <div className="border-t border-line-soft my-4" />
            <Label className="text-[0.8rem] font-semibold mb-2 block">
              Media mounts{' '}
              <span className="font-mono text-[0.72rem] text-muted-dim bg-surface-2 border border-line rounded-[6px] px-[7px] py-[2px] ml-1">read-only · set via env</span>
            </Label>
            <div className="font-mono text-[0.75rem] text-muted-dim">
              {settings.movies_path && <div>{settings.movies_path} · rw</div>}
              {settings.tv_path && <div>{settings.tv_path} · rw</div>}
            </div>
          </div>
        </div>

        <div className="border border-line rounded-[var(--radius)] p-5" style={{ background: 'var(--surface)' }}>
          <div className="flex items-center mb-4">
            <div className="text-[0.72rem] uppercase tracking-[0.11em] text-muted-fg font-bold">Transcode profiles</div>
            <Button
              variant="outline"
              size="sm"
              onClick={() => setProfileDialog({ open: true, initial: null })}
              className="ml-auto rounded-[11px]"
            >
              + New profile
            </Button>
          </div>
          {profiles.map((p) => (
            <div
              key={p.id}
              className="flex items-center gap-3.5 border rounded-[12px] px-4 py-[14px] mb-2.5 last:mb-0"
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
                <div className="text-[0.78rem] text-muted-fg font-mono mt-0.5">
                  libx265 · CRF {p.crf} · preset {p.preset}
                  {p.extra_args && ` · ${p.extra_args}`}
                </div>
              </div>
              <Button
                variant="ghost"
                size="sm"
                onClick={() => setProfileDialog({ open: true, initial: p })}
                className="rounded-[11px]"
              >
                Edit
              </Button>
              {!p.is_default && (
                <Button
                  variant="ghost"
                  size="sm"
                  onClick={() => {
                    if (confirm(`Delete profile "${p.name}"?`)) deleteProfileMutation.mutate(p.id);
                  }}
                  className="rounded-[11px] text-red hover:bg-red-soft hover:text-red"
                >
                  Delete
                </Button>
              )}
            </div>
          ))}
        </div>
      </div>

      <ProfileDialog
        key={`${String(profileDialog.open)}-${profileDialog.initial?.id ?? 'new'}`}
        open={profileDialog.open}
        onClose={() => setProfileDialog({ open: false, initial: null })}
        initial={profileDialog.initial}
      />
    </>
  );
}

export default function Page() {
  return (
    <div className="flex flex-col min-w-0">
      <Suspense fallback={
        <>
          <div
            className="flex items-center gap-4 px-7 py-[18px] border-b border-line"
            style={{ background: 'rgba(22,22,22,.82)', backdropFilter: 'blur(10px)' }}
          >
            <div>
              <div className="text-[1.38rem] font-bold tracking-tight">Settings</div>
              <Skeleton className="h-3 w-48 mt-1.5" />
            </div>
          </div>
          <SettingsSkeleton />
        </>
      }>
        <SettingsContent />
      </Suspense>
    </div>
  );
}
