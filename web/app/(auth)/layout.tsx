'use client';

import { useQuery } from '@tanstack/react-query';
import { usePathname, useRouter } from 'next/navigation';
import { useEffect } from 'react';
import { api } from '@/lib/api';
import { LogoMark } from '@/components/logo';

export default function AuthLayout({ children }: { children: React.ReactNode }) {
  const router = useRouter();
  const pathname = usePathname();

  const { data: session } = useQuery({
    queryKey: ['session'],
    queryFn: api.session,
    retry: false,
  });

  useEffect(() => {
    if (!session) return;
    // Already authenticated — no business on the setup or login pages.
    if (session.authenticated) {
      router.replace('/');
      return;
    }
    // Setup is done — the setup page is off-limits; send them to login.
    if (session.setup_complete && pathname === '/setup') {
      router.replace('/login');
    }
  }, [session, pathname, router]);

  return (
    <div
      className="fixed inset-0 flex items-center justify-center p-6 z-[200]"
      style={{
        background:
          'radial-gradient(130% 120% at 50% 0%, var(--brand-soft), transparent 55%), var(--bg)',
      }}
    >
      <div className="w-full max-w-[380px] text-center">
        <div className="flex items-center justify-center gap-3 mb-2">
          <LogoMark size={38} className="shrink-0" style={{ boxShadow: '0 6px 18px var(--brand-soft)', borderRadius: 11 }} />
          <div className="text-[1.7rem] font-extrabold tracking-tight">
            Re<span className="text-brand">claim</span>
          </div>
        </div>
        <p className="text-muted-fg text-sm mb-6">Codec audit, library browse, and safe re-encode</p>
        {children}
      </div>
    </div>
  );
}
