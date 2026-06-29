import Link from 'next/link';
import { LogoMark } from '@/components/logo';

export function BrandLink({ size = 30, className = '' }: { size?: number; className?: string }) {
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
