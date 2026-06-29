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
import { Tooltip, TooltipTrigger, TooltipContent } from '@/components/ui/tooltip';
import { CircleHelpIcon } from 'lucide-react';

function HelpTip({ children }: { children: React.ReactNode }) {
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <button
          type="button"
          tabIndex={-1}
          aria-label="More info"
          className="inline-flex align-middle text-muted-dim hover:text-muted-fg transition-colors focus:outline-none cursor-pointer"
        >
          <CircleHelpIcon className="size-3.5" />
        </button>
      </TooltipTrigger>
      <TooltipContent className="leading-relaxed">{children}</TooltipContent>
    </Tooltip>
  );
}

function LabelWithHelp({ label, help }: { label: string; help: React.ReactNode }) {
  return (
    <Label className="text-[0.8rem] font-semibold mb-1.5 flex items-center gap-1.5">
      {label}
      <HelpTip>{help}</HelpTip>
    </Label>
  );
}

function TimeSelect({ value, onChange }: { value: string; onChange: (v: string) => void }) {
  const parts = value.split(':');
  const h24 = parseInt(parts[0] ?? '0', 10);
  const mins = parts[1] ?? '00';
  const isPM = h24 >= 12;
  const h12 = h24 % 12 || 12;

  function update(newH12: number, newIsPM: boolean) {
    const newH24 = newIsPM ? (newH12 % 12) + 12 : newH12 % 12;
    onChange(`${String(newH24).padStart(2, '0')}:${mins}`);
  }

  return (
    <div className="flex items-center gap-1.5">
      <Select value={String(h12)} onValueChange={(v) => update(Number(v), isPM)}>
        <SelectTrigger className="w-[90px] rounded-[10px] text-sm">
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          {[12, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11].map((h) => (
            <SelectItem key={h} value={String(h)}>{h}:00</SelectItem>
          ))}
        </SelectContent>
      </Select>
      <Select value={isPM ? 'PM' : 'AM'} onValueChange={(v) => update(h12, v === 'PM')}>
        <SelectTrigger className="w-[72px] rounded-[10px] text-sm">
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value="AM">AM</SelectItem>
          <SelectItem value="PM">PM</SelectItem>
        </SelectContent>
      </Select>
    </div>
  );
}

function DeleteProfileDialog({
  profile,
  onClose,
  onConfirm,
}: {
  profile: Profile | null;
  onClose: () => void;
  onConfirm: () => void;
}) {
  return (
    <Dialog open={!!profile} onOpenChange={(o) => !o && onClose()}>
      <DialogContent className="max-w-sm p-0 overflow-hidden border-line" style={{ background: 'var(--surface)' }} showCloseButton={false}>
        <DialogHeader className="px-6 pt-[22px] pb-4 border-b border-line">
          <DialogTitle className="text-[1.1rem] font-bold tracking-tight">Delete profile</DialogTitle>
        </DialogHeader>
        <div className="px-6 py-5">
          <p className="text-[0.85rem] text-muted-fg">
            Delete <span className="font-semibold text-text">&ldquo;{profile?.name}&rdquo;</span>? This cannot be undone.
          </p>
        </div>
        <DialogFooter className="px-6 py-4 border-t border-line flex justify-end gap-2">
          <Button variant="outline" onClick={onClose} className="rounded-[11px]">
            Cancel
          </Button>
          <Button
            onClick={() => { onConfirm(); onClose(); }}
            className="rounded-[11px] bg-red hover:bg-red/90 text-white border-0"
          >
            Delete
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

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
              <LabelWithHelp
                label="CRF"
                help={
                  <>
                    <strong>Constant Rate Factor</strong> — the quality target for libx265.
                    Lower means higher quality and bigger files; higher means more compression
                    and smaller files. Range is 0–51; <strong>24–28</strong> is the sweet spot
                    for visually-lossless HEVC. Each +6 roughly halves the bitrate.
                  </>
                }
              />
              <Input type="number" min={0} max={51} value={crf} onChange={(e) => setCrf(Number(e.target.value))} />
            </div>
            <div>
              <LabelWithHelp
                label="Preset"
                help={
                  <>
                    Encoder speed vs. compression efficiency. Slower presets squeeze out more
                    savings at the same CRF but take longer to encode. <strong>medium</strong> is
                    a balanced default; <strong>slow</strong>/<strong>slower</strong> gain a few
                    extra percent, while <strong>fast</strong>/<strong>veryfast</strong> trade
                    file size for shorter encode times.
                  </>
                }
              />
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
            <LabelWithHelp
              label="Extra args"
              help={
                <>
                  Raw flags appended to the <span className="font-mono">ffmpeg</span> command.
                  Defaults to <span className="font-mono">-c:a copy -c:s copy</span>, which
                  passes audio and subtitle streams through untouched so only the video is
                  re-encoded. Add flags here to tweak the output (e.g.{' '}
                  <span className="font-mono">-pix_fmt yuv420p10le</span> for 10-bit).
                </>
              }
            />
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
    <div className="px-4 py-[22px] w-full pb-14 sm:px-7 sm:py-[26px]">
      <div className="grid grid-cols-2 gap-[18px] mb-[18px] max-sm:grid-cols-1">
        {[0, 1].map((i) => (
          <div key={i} className="border border-line rounded-(--radius) p-5" style={{ background: 'var(--surface)' }}>
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
      <div className="border border-line rounded-(--radius) p-5" style={{ background: 'var(--surface)' }}>
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
  const { data: session } = useSuspenseQuery({ queryKey: ['session'], queryFn: api.session });
  const { data: profilesData } = useSuspenseQuery({ queryKey: ['profiles'], queryFn: api.profiles, staleTime: 30_000 });
  const profiles = profilesData.items ?? [];

  const [windowStart, setWindowStart] = useState(settings.encode_window_start);
  const [windowEnd, setWindowEnd] = useState(settings.encode_window_end);
  const [scanIntervalHours, setScanIntervalHours] = useState(() => {
    const m = settings.scan_interval.match(/^(\d+)h/);
    return m ? parseInt(m[1], 10) : 24;
  });
  const [scanAnchor, setScanAnchor] = useState(settings.scan_anchor ?? '00:00');
  const [probeConcurrency, setProbeConcurrency] = useState(settings.probe_concurrency);

  const [credPassword, setCredPassword] = useState('');
  const [credConfirm, setCredConfirm] = useState('');

  const settingsMutation = useMutation({
    mutationFn: () =>
      api.updateSettings({
        encode_window_start: windowStart,
        encode_window_end: windowEnd,
        scan_interval: `${scanIntervalHours}h0m0s`,
        scan_anchor: scanAnchor,
        probe_concurrency: probeConcurrency,
      }),
    onSuccess: () => {
      toast.success('Settings saved');
      qc.invalidateQueries({ queryKey: ['settings'] });
    },
    onError: () => toast.error('Failed to save settings'),
  });

  const credMutation = useMutation({
    mutationFn: () => api.changeCredentials(session.username ?? '', credPassword),
    onSuccess: () => {
      toast.success('Credentials updated');
      setCredPassword('');
      setCredConfirm('');
    },
    onError: () => toast.error('Failed to update credentials'),
  });

  const refreshMetaMutation = useMutation({
    mutationFn: () => api.refreshMetadata(),
    onSuccess: () => toast.success('Metadata refresh queued'),
    onError: () => toast.error('Refresh failed'),
  });

  const deleteProfileMutation = useMutation({
    mutationFn: (id: number) => api.deleteProfile(id),
    onSuccess: () => {
      toast.success('Profile deleted');
      qc.invalidateQueries({ queryKey: ['profiles'] });
    },
    onError: () => toast.error('Delete failed'),
  });

  const defaultProfileMutation = useMutation({
    mutationFn: ({ id, ...profile }: Profile) => api.updateProfile(id, { ...profile, is_default: true }),
    onSuccess: (profile) => {
      toast.success(`"${profile.name}" is now the default`);
      qc.invalidateQueries({ queryKey: ['profiles'] });
    },
    onError: () => toast.error('Failed to update default profile'),
  });

  const [profileDialog, setProfileDialog] = useState<{ open: boolean; initial: Partial<Profile> | null }>({
    open: false,
    initial: null,
  });
  const [deleteProfile, setDeleteProfile] = useState<Profile | null>(null);

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
        className="flex items-center gap-4 px-4 py-[14px] border-b border-line sm:px-7 sm:py-[18px]"
        style={{ background: 'rgba(22,22,22,.82)', backdropFilter: 'blur(10px)' }}
      >
        <div>
          <div className="text-title font-bold tracking-tight">Settings</div>
          <div className="text-[0.82rem] text-muted-fg mt-px">Changes apply live — no restart</div>
        </div>
      </div>

      <div className="px-4 py-[22px] w-full pb-14 sm:px-7 sm:py-[26px]">
        <div className="grid grid-cols-2 gap-[18px] mb-[18px] max-sm:grid-cols-1">
          <div className="border border-line rounded-(--radius) p-5" style={{ background: 'var(--surface)' }}>
            <div className="text-[0.72rem] uppercase tracking-[0.11em] text-muted-fg font-bold mb-4">Encoding</div>
            <div className="mb-4">
              <Label className="text-[0.8rem] font-semibold mb-1.5 block">
                Encode window <span className="text-muted-dim font-normal">· when jobs may run</span>
              </Label>
              <div className="flex items-center gap-2 flex-wrap sm:gap-3">
                <TimeSelect value={windowStart} onChange={setWindowStart} />
                <span className="text-muted-fg">to</span>
                <TimeSelect value={windowEnd} onChange={setWindowEnd} />
              </div>
              <p className="text-[0.75rem] text-muted-dim mt-1.5">A running job finishes even if the window closes — only new pulls stop.</p>
            </div>
            <div className="mb-4">
              <LabelWithHelp
                label="Probe concurrency"
                help={
                  <>
                    How many <span className="font-mono">ffprobe</span> processes run in parallel
                    while indexing your library. Higher values scan faster but use more CPU and
                    disk I/O. <strong>4</strong> is a safe default; bump it up on fast NAS/SSD
                    storage, lower it if scans are saturating a spinning disk.
                  </>
                }
              />
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
              <LabelWithHelp
                label="Scan interval"
                help={
                  <>
                    How often Reclaim re-walks your libraries to pick up new or changed files.
                    The <strong>at</strong> time anchors the schedule, so a 24h interval anchored
                    to 12:00 AM rescans nightly at midnight. File changes are also caught live via
                    a filesystem watcher between scans.
                  </>
                }
              />
              <div className="flex items-center gap-3 flex-wrap">
                <div className="flex items-center gap-2">
                  <Input
                    type="number"
                    min={1}
                    max={168}
                    value={scanIntervalHours}
                    onChange={(e) => setScanIntervalHours(Number(e.target.value))}
                    className="w-24"
                  />
                  <span className="text-[0.85rem] text-muted-fg">hours · at</span>
                </div>
                <TimeSelect value={scanAnchor} onChange={setScanAnchor} />
              </div>
              <p className="text-[0.75rem] text-muted-dim mt-1.5">Rescans repeat every N hours, aligned to the chosen time.</p>
            </div>
            <Button
              onClick={() => settingsMutation.mutate()}
              disabled={settingsMutation.isPending}
              className="rounded-[11px]"
              style={{ background: 'linear-gradient(145deg, var(--brand), var(--brand-2))' }}
            >
              {settingsMutation.isPending ? 'Saving…' : 'Save settings'}
            </Button>
          </div>

          <div className="border border-line rounded-(--radius) p-5" style={{ background: 'var(--surface)' }}>
            <div className="text-[0.72rem] uppercase tracking-[0.11em] text-muted-fg font-bold mb-4">Account</div>
            <div className="mb-4">
              <Label className="text-[0.8rem] font-semibold mb-1.5 block">Username</Label>
              <Input value={session.username ?? ''} disabled autoComplete="username" />
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
              disabled={credMutation.isPending || !credPassword}
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
            <div className="font-mono text-[0.75rem] text-muted-dim break-all">
              {settings.movies_path && <div>{settings.movies_path} · rw</div>}
              {settings.tv_path && <div>{settings.tv_path} · rw</div>}
            </div>
          </div>
        </div>

        <div className="border border-line rounded-(--radius) p-5" style={{ background: 'var(--surface)' }}>
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
                  onClick={() => setProfileDialog({ open: true, initial: p })}
                  className="rounded-[11px]"
                >
                  Edit
                </Button>
                {!p.is_default && (
                  <>
                    <Button
                      variant="ghost"
                      size="sm"
                      onClick={() => defaultProfileMutation.mutate(p)}
                      disabled={defaultProfileMutation.isPending}
                      className="rounded-[11px]"
                    >
                      Set default
                    </Button>
                    <Button
                      variant="ghost"
                      size="sm"
                      onClick={() => setDeleteProfile(p)}
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
      </div>

      <div className="border border-line rounded-(--radius) p-5 mt-[18px]" style={{ background: 'var(--surface)' }}>
        <div className="text-[0.72rem] uppercase tracking-[0.11em] text-muted-fg font-bold mb-4">Metadata</div>
        <div className="flex items-center justify-between flex-wrap gap-3">
          <div>
            <Label className="text-[0.8rem] font-semibold block mb-0.5">TMDB API key</Label>
            {settings.tmdb_configured ? (
              <p className="text-[0.78rem] text-green font-mono">Configured · TMDB_API_KEY env var</p>
            ) : (
              <p className="text-[0.78rem] text-muted-dim font-mono">Not configured · set <span className="text-muted-fg">TMDB_API_KEY</span> env var</p>
            )}
          </div>
          {settings.tmdb_configured && (
            <Button
              variant="outline"
              onClick={() => refreshMetaMutation.mutate()}
              disabled={refreshMetaMutation.isPending}
              className="rounded-[11px]"
            >
              {refreshMetaMutation.isPending ? 'Refreshing…' : 'Refresh all metadata'}
            </Button>
          )}
        </div>
      </div>

      <ProfileDialog
        key={`${String(profileDialog.open)}-${profileDialog.initial?.id ?? 'new'}`}
        open={profileDialog.open}
        onClose={() => setProfileDialog({ open: false, initial: null })}
        initial={profileDialog.initial}
      />
      <DeleteProfileDialog
        profile={deleteProfile}
        onClose={() => setDeleteProfile(null)}
        onConfirm={() => deleteProfileMutation.mutate(deleteProfile!.id)}
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
            className="flex items-center gap-4 px-4 py-[14px] border-b border-line sm:px-7 sm:py-[18px]"
            style={{ background: 'rgba(22,22,22,.82)', backdropFilter: 'blur(10px)' }}
          >
            <div>
              <div className="text-title font-bold tracking-tight">Settings</div>
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
