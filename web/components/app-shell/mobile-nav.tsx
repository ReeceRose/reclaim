import Link from "next/link";
import { BrandLink } from "./brand-link";
import { isNavActive, NAV_ITEMS } from "./nav-config";
import { NotificationBell } from "./notification-bell";

export function MobileHeader({
  wsConnected,
  unreadCount,
  onOpenNotifications,
}: {
  wsConnected: boolean;
  unreadCount: number;
  onOpenNotifications: () => void;
}) {
  return (
    <header
      className="hidden max-sm:flex sticky top-0 z-50 items-center gap-3 px-4 border-b border-line-soft"
      style={{
        background: "var(--surface)",
        paddingTop: "calc(12px + env(safe-area-inset-top))",
        paddingBottom: "12px",
      }}
    >
      <BrandLink size={28} />
      <div className="flex items-center gap-2 ml-auto">
        {!wsConnected && (
          <span className="text-xs font-semibold text-muted-fg">
            Reconnecting…
          </span>
        )}
        <NotificationBell
          unreadCount={unreadCount}
          onClick={onOpenNotifications}
        />
      </div>
    </header>
  );
}

export function MobileBottomNav({ pathname }: { pathname: string }) {
  return (
    <nav
      className="hidden max-sm:flex fixed bottom-0 left-0 right-0 z-60 border-t border-line"
      style={{
        background: "rgba(19,36,42,.96)",
        backdropFilter: "blur(14px)",
        paddingBottom: "calc(8px + env(safe-area-inset-bottom))",
        paddingTop: "8px",
        paddingLeft: "6px",
        paddingRight: "6px",
      }}
    >
      {NAV_ITEMS.map((item) => {
        const active = isNavActive(pathname, item.path);
        return (
          <Link
            key={item.path}
            href={item.path}
            className={[
              "flex-1 flex flex-col items-center gap-1 px-0.5 py-1.5 rounded-xl",
              "text-xs font-semibold transition-all duration-100 whitespace-nowrap overflow-hidden",
              active ? "text-brand bg-brand-soft" : "text-muted-fg",
            ].join(" ")}
          >
            {item.icon}
            {item.label}
          </Link>
        );
      })}
    </nav>
  );
}
