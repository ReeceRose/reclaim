'use client';

import { useQueryClient } from '@tanstack/react-query';
import { api, ApiError } from '@/lib/api';
import { useState } from 'react';
import { useRouter } from 'next/navigation';
import { toast } from 'sonner';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Button } from '@/components/ui/button';

export default function SetupPage() {
  const qc = useQueryClient();
  const router = useRouter();
  const [step, setStep] = useState<1 | 2>(1);
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [tmdbKey, setTmdbKey] = useState('');
  const [loading, setLoading] = useState(false);

  async function handleSubmitCredentials() {
    setLoading(true);
    try {
      const { username: user } = await api.setup(username, password);
      qc.setQueryData(['session'], { setup_complete: true, authenticated: true, username: user });
      setStep(2);
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : 'Setup failed');
    } finally {
      setLoading(false);
    }
  }

  async function handleSaveAndFinish() {
    setLoading(true);
    try {
      if (tmdbKey.trim()) {
        await api.updateSettings({ tmdb_api_key: tmdbKey.trim() });
      }
      router.replace('/');
    } catch {
      toast.error('Failed to save API key');
    } finally {
      setLoading(false);
    }
  }

  if (step === 2) {
    return (
      <div
        className="text-left rounded-2xl p-[22px] border border-line"
        style={{ background: 'var(--surface)' }}
      >
        <span className="inline-block mb-4 px-2 py-0.5 rounded text-[0.7rem] font-bold uppercase tracking-widest text-brand bg-brand-soft border border-brand-line">
          Configure metadata
        </span>
        <div className="mb-4">
          <Label className="block text-[0.8rem] font-semibold mb-1.5">TMDB API key <span className="font-normal text-muted-dim">(optional)</span></Label>
          <Input
            type="text"
            value={tmdbKey}
            onChange={(e) => setTmdbKey(e.target.value)}
            placeholder="eyJhbGci…"
            autoComplete="off"
          />
          <p className="text-[0.75rem] text-muted-dim mt-1.5">
            Get a free key at <span className="font-mono">themoviedb.org</span> → Settings → API
          </p>
        </div>
        <Button
          onClick={() => void handleSaveAndFinish()}
          disabled={loading}
          className="w-full h-10 rounded-[11px] text-sm font-semibold text-on-brand mb-3"
          style={{ background: 'linear-gradient(145deg, var(--brand), var(--brand-2))', boxShadow: '0 4px 14px var(--brand-soft)' }}
        >
          {loading ? 'Saving…' : 'Save & finish'}
        </Button>
        <button
          type="button"
          onClick={() => router.replace('/')}
          className="w-full text-center text-[0.8rem] text-muted-dim hover:text-muted-fg transition-colors cursor-pointer"
        >
          Skip for now
        </button>
      </div>
    );
  }

  return (
    <form
      onSubmit={(e) => { e.preventDefault(); void handleSubmitCredentials(); }}
      className="text-left rounded-2xl p-[22px] border border-line"
      style={{ background: 'var(--surface)' }}
    >
      <span className="inline-block mb-4 px-2 py-0.5 rounded text-[0.7rem] font-bold uppercase tracking-widest text-brand bg-brand-soft border border-brand-line">
        First-run setup
      </span>
      <div className="mb-4">
        <Label className="block text-[0.8rem] font-semibold mb-1.5">Create a username</Label>
        <Input value={username} onChange={(e) => setUsername(e.target.value)} autoComplete="username" required />
      </div>
      <div className="mb-4">
        <Label className="block text-[0.8rem] font-semibold mb-1.5">Create a password</Label>
        <Input type="password" value={password} onChange={(e) => setPassword(e.target.value)} autoComplete="new-password" required />
      </div>
      <Button
        type="submit"
        disabled={loading}
        className="w-full h-10 rounded-[11px] text-sm font-semibold text-on-brand"
        style={{ background: 'linear-gradient(145deg, var(--brand), var(--brand-2))', boxShadow: '0 4px 14px var(--brand-soft)' }}
      >
        {loading ? 'Setting up…' : 'Complete setup'}
      </Button>
      <p className="text-[0.76rem] text-muted-dim mt-4">
        Stored bcrypt-hashed in the database. No password lives in your compose file.
      </p>
    </form>
  );
}
