'use client';

import { useState } from 'react';
import Link from 'next/link';
import { usePathname, useRouter } from 'next/navigation';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { api, type ScanProgress } from '@/lib/api';
import { useWS } from '@/hooks/use-ws';
import { formatInt, windowInfo } from '@/lib/format';
import { toast } from 'sonner';
import { NotificationPanel, useUnreadCount } from '@/components/notification-panel';
import { FileDetailProvider } from '@/components/file-detail-sheet';
import { LogoMark } from '@/components/logo';

// ---------------------------------------------------------------------------
// WindowArc — 24-hour clock face showing the encode window period
// ---------------------------------------------------------------------------

function WindowArc({ start, end, isOpen }: { start: string; end: string; isOpen: boolean }) {
  const SIZE = 44;
  const CX = SIZE / 2;
  const CY = SIZE / 2;
  const R = 16;
  const SW = 3;

  function toRad(hhmm: string): number {
    const [h, m] = hhmm.split(':').map(Number);
    return ((h * 60 + (m ?? 0)) / 1440) * 2 * Math.PI - Math.PI / 2;
  }

  const now = new Date();
  const nowRad = ((now.getHours() * 60 + now.getMinutes()) / 1440) * 2 * Math.PI - Math.PI / 2;

  const a1 = toRad(start);
  let a2 = toRad(end);
  if (a2 <= a1) a2 += 2 * Math.PI;

  const largeArc = a2 - a1 > Math.PI ? 1 : 0;
  const x1 = (CX + R * Math.cos(a1)).toFixed(2);
  const y1 = (CY + R * Math.sin(a1)).toFixed(2);
  const x2 = (CX + R * Math.cos(a2)).toFixed(2);
  const y2 = (CY + R * Math.sin(a2)).toFixed(2);
  const nx = (CX + R * Math.cos(nowRad)).toFixed(2);
  const ny = (CY + R * Math.sin(nowRad)).toFixed(2);

  return (
    <svg width={SIZE} height={SIZE} viewBox={`0 0 ${SIZE} ${SIZE}`} className="shrink-0">
      <circle cx={CX} cy={CY} r={R} fill="none" stroke="var(--line)" strokeWidth={SW} />
      <path
        d={`M ${x1} ${y1} A ${R} ${R} 0 ${largeArc} 1 ${x2} ${y2}`}
        fill="none"
        stroke={isOpen ? 'var(--green)' : 'var(--surface-3)'}
        strokeWidth={SW}
        strokeLinecap="round"
        style={isOpen ? { filter: 'drop-shadow(0 0 3px var(--green))' } : undefined}
      />
      <circle cx={nx} cy={ny} r={2.2} fill="var(--brand)" />
    </svg>
  );
}

// ---------------------------------------------------------------------------
// Nav config
// ---------------------------------------------------------------------------

const NAV_ITEMS = [
  {
    path: '/',
    label: 'Overview',
    group: 'Library',
    icon: (
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" className="w-[18px] h-[18px] shrink-0">
        <rect x="3" y="3" width="7" height="9"/><rect x="14" y="3" width="7" height="5"/>
        <rect x="14" y="12" width="7" height="9"/><rect x="3" y="16" width="7" height="5"/>
      </svg>
    ),
  },
  {
    path: '/browse',
    label: 'Browse',
    group: 'Library',
    icon: (
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" className="w-[18px] h-[18px] shrink-0">
        <rect x="3" y="3" width="7" height="7" rx="1"/><rect x="14" y="3" width="7" height="7" rx="1"/>
        <rect x="3" y="14" width="7" height="7" rx="1"/><rect x="14" y="14" width="7" height="7" rx="1"/>
      </svg>
    ),
  },
  {
    path: '/library',
    label: 'Library',
    group: 'Library',
    icon: (
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" className="w-[18px] h-[18px] shrink-0">
        <path d="M3 6h18"/><path d="M3 12h18"/><path d="M3 18h18"/>
        <path d="M7 6v12"/>
      </svg>
    ),
  },
  {
    path: '/candidates',
    label: 'Candidates',
    group: 'Library',
    icon: (
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" className="w-[18px] h-[18px] shrink-0">
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
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" className="w-[18px] h-[18px] shrink-0">
        <path d="M9 11l3 3L22 4"/><path d="M21 12v7a2 2 0 01-2 2H5a2 2 0 01-2-2V5a2 2 0 012-2h11"/>
      </svg>
    ),
  },
  {
    path: '/queue',
    label: 'Queue',
    group: 'Encoding',
    icon: (
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" className="w-[18px] h-[18px] shrink-0">
        <polygon points="5 3 19 12 5 21 5 3"/>
      </svg>
    ),
  },
  {
    path: '/settings',
    label: 'Settings',
    group: 'Encoding',
    icon: (
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" className="w-[18px] h-[18px] shrink-0">
        <circle cx="12" cy="12" r="3"/>
        <path d="M19.4 15a1.65 1.65 0 00.33 1.82l.06.06a2 2 0 11-2.83 2.83l-.06-.06a1.65 1.65 0 00-1.82-.33 1.65 1.65 0 00-1 1.51V21a2 2 0 01-4 0v-.09A1.65 1.65 0 009 19.4a1.65 1.65 0 00-1.82.33l-.06.06a2 2 0 11-2.83-2.83l.06-.06a1.65 1.65 0 00.33-1.82 1.65 1.65 0 00-1.51-1H3a2 2 0 010-4h.09A1.65 1.65 0 004.6 9a1.65 1.65 0 00-.33-1.82l-.06-.06a2 2 0 112.83-2.83l.06.06a1.65 1.65 0 001.82.33H9a1.65 1.65 0 001-1.51V3a2 2 0 014 0v.09a1.65 1.65 0 001 1.51 1.65 1.65 0 001.82-.33l.06-.06a2 2 0 112.83 2.83l-.06.06a1.65 1.65 0 00-.33 1.82V9a1.65 1.65 0 001.51 1H21a2 2 0 010 4h-.09a1.65 1.65 0 00-1.51 1z"/>
      </svg>
    ),
  },
] as const;

function NotificationBell({
  unreadCount,
  onClick,
}: {
  unreadCount: number;
  onClick: () => void;
}) {
  return (
    <button
      onClick={onClick}
      className="relative w-7 h-7 flex items-center justify-center rounded-[8px] text-muted-fg hover:text-text hover:bg-surface-2 transition-colors shrink-0 cursor-pointer"
      aria-label="Events"
    >
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" className="w-[15px] h-[15px]">
        <path d="M18 8A6 6 0 006 8c0 7-3 9-3 9h18s-3-2-3-9"/><path d="M13.73 21a2 2 0 01-3.46 0"/>
      </svg>
      {unreadCount > 0 && (
        <span
          className="absolute -top-1 -right-1 h-[15px] min-w-[15px] px-[3px] rounded-full text-[0.58rem] font-bold leading-none grid place-items-center tabular-nums"
          style={{ background: 'var(--brand)', color: 'var(--on-brand)' }}
        >
          {unreadCount > 9 ? '9+' : unreadCount}
        </span>
      )}
    </button>
  );
}

function BrandLink({ size = 30, className = '' }: { size?: number; className?: string }) {
  return (
    <Link
      href="/"
      className={`flex items-center gap-[11px] min-w-0 flex-1 rounded-[10px] transition-opacity hover:opacity-90 ${className}`}
    >
      <LogoMark size={size} className="shrink-0" style={{ boxShadow: '0 6px 18px var(--brand-soft)', borderRadius: 9 }} />
      <span className="font-extrabold tracking-tight text-[1.22rem] truncate">
        Re<span className="text-brand">claim</span>
      </span>
    </Link>
  );
}

// ---------------------------------------------------------------------------
// AppShell
// ---------------------------------------------------------------------------

export function AppShell({ children }: { children: React.ReactNode }) {
  useWS();
  const { data: isScanning } = useQuery<boolean>({
    queryKey: ['scanning'],
    queryFn: () => false,
    initialData: false,
    staleTime: Infinity,
    gcTime: Infinity,
  });
  const { data: scanProgress } = useQuery<ScanProgress | null>({
    queryKey: ['scan_progress'],
    queryFn: () => null,
    initialData: null,
    staleTime: Infinity,
    gcTime: Infinity,
  });

  const [notifOpen, setNotifOpen] = useState(false);
  const pathname = usePathname();
  const router = useRouter();
  const qc = useQueryClient();

  const { data: session } = useQuery({ queryKey: ['session'], queryFn: api.session, retry: false });
  const { data: settings } = useQuery({ queryKey: ['settings'], queryFn: api.settings });
  const { data: stats } = useQuery({ queryKey: ['stats'], queryFn: api.stats, staleTime: 30_000 });
  const { data: runningJobs } = useQuery({
    queryKey: ['jobs', 'running-count'],
    queryFn: () => api.jobs({ status: 'running', limit: 1 }),
  });
  const { data: queuedJobs } = useQuery({
    queryKey: ['jobs', 'queued-count'],
    queryFn: () => api.jobs({ status: 'queued', limit: 1 }),
  });
  const { data: eventsData } = useQuery({
    queryKey: ['events'],
    queryFn: () => api.events({ limit: 50 }),
    staleTime: 30_000,
  });

  const unreadCount = useUnreadCount(eventsData?.items ?? []);

  const candidateCount = stats
    ? stats.by_codec.filter((c) => c.codec !== 'hevc').reduce((s, c) => s + c.file_count, 0)
    : null;

  const runningCount = runningJobs?.total_count ?? runningJobs?.items.length ?? 0;
  const queuedCount = queuedJobs?.total_count ?? queuedJobs?.items.length ?? 0;
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
  const scanProgressDetail = scanProgress
    ? [
        `${formatInt(scanProgress.files_processed)} indexed`,
        scanProgress.files_seen > scanProgress.files_processed ? `${formatInt(scanProgress.files_seen)} found` : null,
        scanProgress.errors > 0 ? `${formatInt(scanProgress.errors)} errors` : null,
      ]
        .filter(Boolean)
        .join(' · ')
    : null;

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
    <FileDetailProvider>
    <div className="grid grid-cols-[230px_1fr] sm:min-h-screen max-sm:grid-cols-1 max-sm:grid-rows-[auto_1fr] max-sm:h-dvh">
      <aside
        className="flex flex-col sticky top-0 h-screen border-r border-line max-sm:hidden"
        style={{ background: 'var(--surface)' }}
      >
        <div className="flex items-center gap-[11px] px-5 py-5 border-b border-line-soft">
          <BrandLink />
          <NotificationBell unreadCount={unreadCount} onClick={() => setNotifOpen(true)} />
        </div>

        <div
          className="grid transition-[grid-template-rows] duration-200 ease-out"
          style={{ gridTemplateRows: isScanning ? '1fr' : '0fr' }}
          aria-hidden={!isScanning}
        >
          <div className="overflow-hidden">
            <div className="flex items-center gap-2 px-5 py-[9px] text-[0.75rem] font-medium text-brand border-b border-brand-line" style={{ background: 'var(--brand-soft)' }}>
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" className="w-3.5 h-3.5 shrink-0 animate-spin" style={{ animationDuration: '1.1s' }}>
                <path d="M21 12a9 9 0 11-6.219-8.56" strokeLinecap="round"/>
              </svg>
              <div className="min-w-0">
                <div>Scanning library…</div>
                {scanProgressDetail && <div className="truncate text-[0.66rem] font-normal opacity-80">{scanProgressDetail}</div>}
              </div>
            </div>
          </div>
        </div>

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
                      'text-[0.93rem] font-medium border transition-all duration-130',
                      active
                        ? 'bg-brand-soft text-brand border-brand-line font-semibold'
                        : 'text-muted-fg border-transparent hover:bg-surface-2 hover:text-text',
                    ].join(' ')}
                  >
                    {item.icon}
                    <span className="flex-1">{item.label}</span>
                    {badge && (
                      <span className={`text-[0.7rem] font-bold px-[9px] py-px rounded-[20px] ${active ? 'bg-brand-soft text-brand' : 'bg-surface-3 text-muted-fg'}`}>
                        {badge}
                      </span>
                    )}
                  </Link>
                );
              })}
            </div>
          ))}
        </nav>

        <div className="border-t border-line-soft px-4 py-[14px]">
          {win && settings && (
            <div className="flex items-center gap-3 mb-3">
              <WindowArc
                start={settings.encode_window_start}
                end={settings.encode_window_end}
                isOpen={win.open}
              />
              <div className="min-w-0">
                <div className="text-[0.76rem] font-semibold text-text leading-tight">{win.label}</div>
                <div className={`text-[0.7rem] leading-tight mt-[2px] ${win.open ? 'text-green' : 'text-muted-dim'}`}>
                  {win.detail}
                </div>
              </div>
            </div>
          )}
          <div className="flex items-center gap-[10px] text-[0.84rem]">
            <div
              className="w-[30px] h-[30px] rounded-full grid place-items-center font-bold text-[0.8rem] text-on-brand shrink-0"
              style={{ background: 'linear-gradient(145deg, var(--brand), var(--green))' }}
            >
              {initials}
            </div>
            <span className="flex-1 min-w-0 truncate">{username}</span>
            <button onClick={() => void handleLogout()} className="text-[0.76rem] text-muted-dim hover:text-red transition-colors cursor-pointer">
              Log out
            </button>
          </div>
        </div>
      </aside>

      <header
        className="hidden max-sm:flex sticky top-0 z-50 items-center gap-3 px-4 border-b border-line-soft"
        style={{
          background: 'var(--surface)',
          paddingTop: 'calc(12px + env(safe-area-inset-top))',
          paddingBottom: '12px',
        }}
      >
        <BrandLink size={28} />
        <NotificationBell unreadCount={unreadCount} onClick={() => setNotifOpen(true)} />
      </header>

      <main className="flex flex-col min-w-0 max-sm:min-h-0 max-sm:overflow-y-auto max-sm:pb-20">
        {children}
      </main>

      <NotificationPanel open={notifOpen} onOpenChange={setNotifOpen} />

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
    </FileDetailProvider>
  );
}
