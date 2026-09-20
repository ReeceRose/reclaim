import Link from "next/link";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import type { Settings } from "@/lib/api";
import type { windowInfo } from "@/lib/format";
import { BrandLink } from "./brand-link";
import { isNavActive, NAV_GROUPS, NAV_ITEMS } from "./nav-config";
import { NotificationBell } from "./notification-bell";
import { ScanBanner } from "./scan-banner";
import { WindowArc } from "./window-arc";
import { WsDisconnectedBanner } from "./ws-disconnected-banner";

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
  version,
  hasNewRelease,
  onOpenNotifications,
  onOpenReleaseNotes,
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
  version: string | null;
  hasNewRelease: boolean;
  onOpenNotifications: () => void;
  onOpenReleaseNotes: () => void;
  onLogout: () => void;
}) {
  return (
    <aside
      className="flex flex-col sticky top-0 h-screen border-r border-line max-sm:hidden"
      style={{ background: "var(--surface)" }}
    >
      <div className="flex items-center gap-3 px-5 py-5 border-b border-line-soft">
        <BrandLink />
        <NotificationBell
          unreadCount={unreadCount}
          onClick={onOpenNotifications}
        />
      </div>

      <ScanBanner visible={isScanning} detail={scanProgressDetail} />
      <WsDisconnectedBanner visible={!wsConnected} />

      <nav className="flex flex-col gap-1 px-3 py-3.5 flex-1">
        {NAV_GROUPS.map((group) => (
          <div key={group}>
            <span className="px-2.5 pt-3 pb-1.5 text-xs uppercase tracking-widest text-muted-dim font-bold block">
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
                    "flex items-center gap-3 px-3 py-2.5 rounded-xl w-full",
                    "text-sm font-medium border transition-all duration-130",
                    active
                      ? "bg-brand-soft text-brand border-brand-line font-semibold"
                      : "text-muted-fg border-transparent hover:bg-surface-2 hover:text-text",
                  ].join(" ")}
                >
                  {item.icon}
                  <span className="flex-1">{item.label}</span>
                  {badge && (
                    <span
                      className={`text-xs font-bold px-2.5 py-px rounded-3xl ${active ? "bg-brand-soft text-brand" : "bg-surface-3 text-muted-fg"}`}
                    >
                      {badge}
                    </span>
                  )}
                </Link>
              );
            })}
          </div>
        ))}
      </nav>

      <div className="border-t border-line-soft px-4 py-3.5">
        {version && (
          <Button
            variant="ghost"
            size="xs"
            onClick={onOpenReleaseNotes}
            data-tooltip="View release notes"
            className="mb-3 w-full justify-start gap-2 px-1.5 text-xs font-normal text-muted-dim hover:text-text cursor-pointer"
          >
            <span className="truncate">Version: {version}</span>
            {hasNewRelease && (
              <Badge className="ml-auto bg-brand-soft px-2 py-px font-bold text-brand">
                New
              </Badge>
            )}
          </Button>
        )}
        {encodeWindow && settings && (
          <div className="flex items-center gap-3 mb-3">
            <WindowArc
              start={settings.encode_window_start}
              end={settings.encode_window_end}
              isOpen={encodeWindow.open}
            />
            <div className="min-w-0">
              <div className="text-xs font-semibold text-text leading-tight">
                {encodeWindow.label}
              </div>
              <div
                className={`text-xs leading-tight mt-0.5 ${encodeWindow.open ? "text-green" : "text-muted-dim"}`}
              >
                {encodeWindow.detail}
              </div>
            </div>
          </div>
        )}
        <div className="flex items-center gap-2.5 text-sm">
          <div
            className="w-8 h-8 rounded-full grid place-items-center font-bold text-xs text-on-brand shrink-0"
            style={{
              background: "linear-gradient(145deg, var(--brand), var(--green))",
            }}
          >
            {initials}
          </div>
          <span className="flex-1 min-w-0 truncate">{username}</span>
          <button
            type="button"
            onClick={() => void onLogout()}
            className="text-xs text-muted-dim hover:text-red transition-colors cursor-pointer"
          >
            Log out
          </button>
        </div>
      </div>
    </aside>
  );
}
