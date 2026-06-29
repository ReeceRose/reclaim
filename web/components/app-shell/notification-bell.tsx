export function NotificationBell({
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
