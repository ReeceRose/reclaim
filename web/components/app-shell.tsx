'use client';

import Link from 'next/link';
import { usePathname, useRouter } from 'next/navigation';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { api } from '@/lib/api';
import { useWS } from '@/hooks/use-ws';
import { windowInfo } from '@/lib/format';
import { toast } from 'sonner';

// ---------------------------------------------------------------------------
// Nav config
// ---------------------------------------------------------------------------

const NAV_ITEMS = [
  {
    path: '/',
    label: 'Overview',
    group: 'Library',
    icon: (
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" className="w-[18px] h-[18px] flex-shrink-0">
        <rect x="3" y="3" width="7" height="9"/><rect x="14" y="3" width="7" height="5"/>
        <rect x="14" y="12" width="7" height="9"/><rect x="3" y="16" width="7" height="5"/>
      </svg>
    ),
  },
  {
    path: '/candidates',
    label: 'Candidates',
    group: 'Library',
    icon: (
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" className="w-[18px] h-[18px] flex-shrink-0">
        <line x1="8" y1="6" x2="21" y2="6"/><line x1="8" y1="12" x2="21" y2="12"/><line x1="8" y1="18" x2="21" y2="18"/>
        <line x1="3" y1="6" x2="3.01" y2="6"/><line x1="3" y1="12" x2="3.01" y2="12"/><line x1="3" y1="18" x2="3.01" y2="18"/>
      </svg>
    ),
  },
  {
    path: '/dry-run',
    label: 'Dry-run',
    group: 'Library',
    icon: (
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" className="w-[18px] h-[18px] flex-shrink-0">
        <path d="M9 11l3 3L22 4"/><path d="M21 12v7a2 2 0 01-2 2H5a2 2 0 01-2-2V5a2 2 0 012-2h11"/>
      </svg>
    ),
  },
  {
    path: '/queue',
    label: 'Queue',
    group: 'Encoding',
    icon: (
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" className="w-[18px] h-[18px] flex-shrink-0">
        <polygon points="5 3 19 12 5 21 5 3"/>
      </svg>
    ),
  },
  {
    path: '/settings',
    label: 'Settings',
    group: 'Encoding',
    icon: (
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" className="w-[18px] h-[18px] flex-shrink-0">
        <circle cx="12" cy="12" r="3"/>
        <path d="M19.4 15a1.65 1.65 0 00.33 1.82l.06.06a2 2 0 11-2.83 2.83l-.06-.06a1.65 1.65 0 00-1.82-.33 1.65 1.65 0 00-1 1.51V21a2 2 0 01-4 0v-.09A1.65 1.65 0 009 19.4a1.65 1.65 0 00-1.82.33l-.06.06a2 2 0 11-2.83-2.83l.06-.06a1.65 1.65 0 00.33-1.82 1.65 1.65 0 00-1.51-1H3a2 2 0 010-4h.09A1.65 1.65 0 004.6 9a1.65 1.65 0 00-.33-1.82l-.06-.06a2 2 0 112.83-2.83l.06.06a1.65 1.65 0 001.82.33H9a1.65 1.65 0 001-1.51V3a2 2 0 014 0v.09a1.65 1.65 0 001 1.51 1.65 1.65 0 001.82-.33l.06-.06a2 2 0 112.83 2.83l-.06.06a1.65 1.65 0 00-.33 1.82V9a1.65 1.65 0 001.51 1H21a2 2 0 010 4h-.09a1.65 1.65 0 00-1.51 1z"/>
      </svg>
    ),
  },
] as const;

// ---------------------------------------------------------------------------
// AppShell
// ---------------------------------------------------------------------------

export function AppShell({ children }: { children: React.ReactNode }) {
  useWS();

  const pathname = usePathname();
  const router = useRouter();
  const qc = useQueryClient();

  const { data: session } = useQuery({ queryKey: ['session'], queryFn: api.session, retry: false });
  const { data: settings } = useQuery({ queryKey: ['settings'], queryFn: api.settings });
  const { data: stats } = useQuery({ queryKey: ['stats'], queryFn: api.stats, staleTime: 30_000 });
  const { data: jobsData } = useQuery({
    queryKey: ['jobs'],
    queryFn: () => api.jobs(),
    refetchInterval: 5000,
  });

  const candidateCount = stats
    ? stats.by_codec.filter((c) => c.codec !== 'hevc').reduce((s, c) => s + c.file_count, 0)
    : null;

  const runningCount = jobsData?.items.filter((j) => j.status === 'running').length ?? 0;
  const queuedCount = jobsData?.items.filter((j) => j.status === 'queued').length ?? 0;
  const queueBadge =
    runningCount + queuedCount > 0
      ? [runningCount > 0 ? String(runningCount) : '', queuedCount > 0 ? String(queuedCount) : '']
          .filter(Boolean)
          .join(' · ')
      : null;

  const candidateBadge =
    candidateCount != null
      ? candidateCount >= 1000
        ? `${(candidateCount / 1000).toFixed(1)}k`
        : String(candidateCount)
      : null;

  const badges: Record<string, string | null> = {
    '/candidates': candidateBadge,
    '/queue': queueBadge,
  };

  const win = settings ? windowInfo(settings.encode_window_start, settings.encode_window_end) : null;
  const username = session?.username ?? '';
  const initials = username.slice(0, 2).toUpperCase() || '?';

  function isActive(path: string) {
    return path === '/' ? pathname === '/' : pathname.startsWith(path);
  }

  async function handleLogout() {
    try {
      await api.logout();
      qc.removeQueries({ queryKey: ['session'] });
      router.replace('/login');
    } catch {
      toast.error('Logout failed');
    }
  }

  return (
    <div className="grid min-h-screen grid-cols-[230px_1fr] max-sm:grid-cols-1">
      {/* Sidebar */}
      <aside
        className="flex flex-col sticky top-0 h-screen border-r border-line max-sm:hidden"
        style={{ background: 'var(--surface)' }}
      >
        {/* Brand */}
        <div className="flex items-center gap-[11px] px-5 py-5 border-b border-line-soft">
          <div
            className="relative w-[30px] h-[30px] rounded-[9px] flex-shrink-0"
            style={{
              background: 'linear-gradient(145deg, var(--brand), var(--brand-2))',
              boxShadow: '0 0 0 1px var(--brand-line), 0 6px 18px var(--brand-soft)',
            }}
          >
            <span className="absolute inset-x-[7px] top-[7px] h-1 rounded-sm" style={{ background: 'var(--bg)', opacity: 0.6, boxShadow: '0 6px 0 var(--bg)' }} />
            <span className="absolute right-1.5 bottom-1.5 w-[5px] h-[5px] rounded-full" style={{ background: 'var(--bg)', opacity: 0.6 }} />
          </div>
          <span className="font-extrabold tracking-tight text-[1.22rem]">
            Re<span className="text-brand">claim</span>
          </span>
        </div>

        {/* Nav */}
        <nav className="flex flex-col gap-[3px] px-3 py-[14px] flex-1">
          {(['Library', 'Encoding'] as const).map((group) => (
            <div key={group}>
              <span className="px-[10px] pt-[11px] pb-[6px] text-[0.66rem] uppercase tracking-[0.13em] text-muted-dim font-bold block">
                {group}
              </span>
              {NAV_ITEMS.filter((n) => n.group === group).map((item) => {
                const active = isActive(item.path);
                const badge = badges[item.path];
                return (
                  <Link
                    key={item.path}
                    href={item.path}
                    className={[
                      'flex items-center gap-[11px] px-[11px] py-[10px] rounded-[11px] w-full',
                      'text-[0.93rem] font-medium border transition-all duration-[130ms]',
                      active
                        ? 'bg-brand-soft text-brand border-brand-line font-semibold'
                        : 'text-muted-fg border-transparent hover:bg-surface-2 hover:text-text',
                    ].join(' ')}
                  >
                    {item.icon}
                    <span className="flex-1">{item.label}</span>
                    {badge && (
                      <span className={`text-[0.7rem] font-bold px-[9px] py-[1px] rounded-[20px] ${active ? 'bg-brand-soft text-brand' : 'bg-surface-3 text-muted-fg'}`}>
                        {badge}
                      </span>
                    )}
                  </Link>
                );
              })}
            </div>
          ))}
        </nav>

        {/* Footer */}
        <div className="border-t border-line-soft px-4 py-[14px]">
          {win && (
            <div className="flex items-center gap-[9px] text-[0.78rem] text-muted-fg bg-surface-2 border border-line rounded-[11px] px-[11px] py-[9px] mb-3">
              <span
                className={`w-[7px] h-[7px] rounded-full flex-shrink-0 ${win.open ? 'bg-green' : 'bg-muted-dim'}`}
                style={win.open ? { boxShadow: '0 0 0 3px var(--green-soft)' } : undefined}
              />
              Window {win.label} · {win.detail}
            </div>
          )}
          <div className="flex items-center gap-[10px] text-[0.84rem]">
            <div
              className="w-[30px] h-[30px] rounded-full grid place-items-center font-bold text-[0.8rem] text-on-brand flex-shrink-0"
              style={{ background: 'linear-gradient(145deg, var(--brand), var(--green))' }}
            >
              {initials}
            </div>
            <span className="flex-1 min-w-0 truncate">{username}</span>
            <button onClick={() => void handleLogout()} className="text-[0.76rem] text-muted-dim hover:text-red transition-colors">
              Log out
            </button>
          </div>
        </div>
      </aside>

      {/* Main */}
      <main className="flex flex-col min-w-0 max-sm:pb-20">
        {children}
      </main>

      {/* Mobile bottom tab bar */}
      <nav
        className="hidden max-sm:flex fixed bottom-0 left-0 right-0 z-60 border-t border-line"
        style={{
          background: 'rgba(19,36,42,.96)',
          backdropFilter: 'blur(14px)',
          paddingBottom: 'calc(8px + env(safe-area-inset-bottom))',
          paddingTop: '8px',
          paddingLeft: '6px',
          paddingRight: '6px',
        }}
      >
        {NAV_ITEMS.map((item) => {
          const active = isActive(item.path);
          return (
            <Link
              key={item.path}
              href={item.path}
              className={[
                'flex-1 flex flex-col items-center gap-[3px] px-[2px] py-[6px] rounded-[11px]',
                'text-[0.64rem] font-semibold transition-[120ms] whitespace-nowrap overflow-hidden',
                active ? 'text-brand bg-brand-soft' : 'text-muted-fg',
              ].join(' ')}
            >
              {item.icon}
              {item.label}
            </Link>
          );
        })}
      </nav>
    </div>
  );
}
