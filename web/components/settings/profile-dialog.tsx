'use client';

import { useState } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { api, type Profile } from '@/lib/api';
import { toast } from 'sonner';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Button } from '@/components/ui/button';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from '@/components/ui/dialog';
import { LabelWithHelp } from './help-tip';

export function DeleteProfileDialog({
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

export function ProfileDialog({
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
