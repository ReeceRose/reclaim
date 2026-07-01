import Link from 'next/link';
import type { Settings } from '@/lib/api';
import type { windowInfo } from '@/lib/format';
import { BrandLink } from './brand-link';
import { NotificationBell } from './notification-bell';
import { ScanBanner } from './scan-banner';
import { WsDisconnectedBanner } from './ws-disconnected-banner';
import { WindowArc } from './window-arc';
import { NAV_GROUPS, NAV_ITEMS, isNavActive } from './nav-config';

type EncodeWindow = ReturnType<typeof windowInfo>;

export function Sidebar({
  pathname,
  isScanning,
  wsConnected,
  scanProgressDetail,
  unreadCount,
  navBadges,
  settings,
  encodeWindow,
  username,
  initials,
  onOpenNotifications,
  onLogout,
}: {
  pathname: string;
  isScanning: boolean;
  wsConnected: boolean;
  scanProgressDetail: string | null;
  unreadCount: number;
  navBadges: Record<string, string | null>;
  settings: Settings | undefined;
  encodeWindow: EncodeWindow | null;
  username: string;
  initials: string;
  onOpenNotifications: () => void;
  onLogout: () => void;
}) {
  return (
    <aside
      className="flex flex-col sticky top-0 h-screen border-r border-line max-sm:hidden"
      style={{ background: 'var(--surface)' }}
    >
      <div className="flex items-center gap-[11px] px-5 py-5 border-b border-line-soft">
        <BrandLink />
        <NotificationBell unreadCount={unreadCount} onClick={onOpenNotifications} />
      </div>

      <ScanBanner visible={isScanning} detail={scanProgressDetail} />
      <WsDisconnectedBanner visible={!wsConnected} />

      <nav className="flex flex-col gap-[3px] px-3 py-[14px] flex-1">
        {NAV_GROUPS.map((group) => (
          <div key={group}>
            <span className="px-[10px] pt-[11px] pb-[6px] text-[0.66rem] uppercase tracking-[0.13em] text-muted-dim font-bold block">
              {group}
            </span>
            {NAV_ITEMS.filter((n) => n.group === group).map((item) => {
              const active = isNavActive(pathname, item.path);
              const badge = navBadges[item.path];
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
        {encodeWindow && settings && (
          <div className="flex items-center gap-3 mb-3">
            <WindowArc
              start={settings.encode_window_start}
              end={settings.encode_window_end}
              isOpen={encodeWindow.open}
            />
            <div className="min-w-0">
              <div className="text-[0.76rem] font-semibold text-text leading-tight">{encodeWindow.label}</div>
              <div className={`text-[0.7rem] leading-tight mt-[2px] ${encodeWindow.open ? 'text-green' : 'text-muted-dim'}`}>
                {encodeWindow.detail}
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
          <button onClick={() => void onLogout()} className="text-[0.76rem] text-muted-dim hover:text-red transition-colors cursor-pointer">
            Log out
          </button>
        </div>
      </div>
    </aside>
  );
}
