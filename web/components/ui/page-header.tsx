import { type ReactNode } from 'react';

export function PageHeader({ title, subtitle, children }: {
  title: string;
  subtitle?: ReactNode;
  children?: ReactNode;
}) {
  return (
    <div
      className="flex flex-col gap-2 px-4 py-[14px] border-b border-line shrink-0 sm:flex-row sm:items-center sm:gap-4 sm:px-7 sm:py-[18px]"
      style={{ background: 'rgba(22,22,22,.82)', backdropFilter: 'blur(10px)' }}
    >
      <div className="min-w-0">
        <div className="text-title font-bold tracking-tight">{title}</div>
        {subtitle != null && <div className="text-[0.82rem] text-muted-fg mt-px">{subtitle}</div>}
      </div>
      {children}
    </div>
  );
}
