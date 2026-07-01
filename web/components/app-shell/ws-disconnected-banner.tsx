export function WsDisconnectedBanner({ visible }: { visible: boolean }) {
  return (
    <div
      className="grid transition-[grid-template-rows] duration-200 ease-out"
      style={{ gridTemplateRows: visible ? '1fr' : '0fr' }}
      aria-hidden={!visible}
      aria-live="polite"
    >
      <div className="overflow-hidden">
        <div
          className="flex items-center gap-2 px-5 py-[9px] text-[0.75rem] font-medium border-b border-line"
          style={{ background: 'var(--surface-2)', color: 'var(--muted-fg)' }}
        >
          <span
            className="w-2 h-2 rounded-full shrink-0"
            style={{ background: 'var(--warn, #d97706)' }}
            aria-hidden
          />
          <span>Live updates disconnected — reconnecting…</span>
        </div>
      </div>
    </div>
  );
}
