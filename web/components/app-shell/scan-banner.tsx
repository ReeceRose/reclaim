export function ScanBanner({
  visible,
  detail,
}: {
  visible: boolean;
  detail: string | null;
}) {
  return (
    <div
      className="grid transition-[grid-template-rows] duration-200 ease-out"
      style={{ gridTemplateRows: visible ? '1fr' : '0fr' }}
      aria-hidden={!visible}
    >
      <div className="overflow-hidden">
        <div
          className="flex items-center gap-2 px-5 py-[9px] text-[0.75rem] font-medium text-brand border-b border-brand-line"
          style={{ background: 'var(--brand-soft)' }}
        >
          <svg
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            strokeWidth="2.5"
            className="w-3.5 h-3.5 shrink-0 animate-spin"
            style={{ animationDuration: '1.1s' }}
          >
            <path d="M21 12a9 9 0 11-6.219-8.56" strokeLinecap="round"/>
          </svg>
          <div className="min-w-0">
            <div>Scanning library…</div>
            {detail && <div className="truncate text-[0.66rem] font-normal opacity-80">{detail}</div>}
          </div>
        </div>
      </div>
    </div>
  );
}
