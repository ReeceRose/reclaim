'use client';

import { useState } from 'react';
import { NotificationPanel } from '@/components/notification-panel';
import { MobileBottomNav, MobileHeader } from './mobile-nav';
import { Sidebar } from './sidebar';
import { useShellData } from './use-shell-data';

export function AppShell({ children }: { children: React.ReactNode }) {
  const [notifOpen, setNotifOpen] = useState(false);
  const shell = useShellData();

  return (
    <div className="grid grid-cols-[230px_1fr] sm:min-h-screen max-sm:grid-cols-1 max-sm:grid-rows-[auto_1fr] max-sm:h-dvh">
      <Sidebar
        pathname={shell.pathname}
        isScanning={shell.isScanning}
        scanProgressDetail={shell.scanProgressDetail}
        unreadCount={shell.unreadCount}
        navBadges={shell.navBadges}
        settings={shell.settings}
        encodeWindow={shell.encodeWindow}
        username={shell.username}
        initials={shell.initials}
        onOpenNotifications={() => setNotifOpen(true)}
        onLogout={shell.handleLogout}
      />

      <MobileHeader
        unreadCount={shell.unreadCount}
        onOpenNotifications={() => setNotifOpen(true)}
      />

      <main className="flex flex-col min-w-0 max-sm:min-h-0 max-sm:overflow-y-auto max-sm:pb-20">
        {children}
      </main>

      <NotificationPanel open={notifOpen} onOpenChange={setNotifOpen} />

      <MobileBottomNav pathname={shell.pathname} />
    </div>
  );
}
