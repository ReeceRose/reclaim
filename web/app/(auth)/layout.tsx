export default function AuthLayout({ children }: { children: React.ReactNode }) {
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
          <div
            className="relative flex-shrink-0 rounded-[9px]"
            style={{
              width: 38,
              height: 38,
              background: 'linear-gradient(145deg, var(--brand), var(--brand-2))',
              boxShadow: '0 0 0 1px var(--brand-line), 0 6px 18px var(--brand-soft)',
            }}
          >
            <span className="absolute inset-x-[7px] top-[7px] h-1 rounded-sm" style={{ background: 'var(--bg)', opacity: 0.6, boxShadow: '0 6px 0 var(--bg)' }} />
            <span className="absolute right-1.5 bottom-1.5 w-[5px] h-[5px] rounded-full" style={{ background: 'var(--bg)', opacity: 0.6 }} />
          </div>
          <div className="text-[1.7rem] font-extrabold tracking-tight">
            Re<span className="text-brand">claim</span>
          </div>
        </div>
        <p className="text-muted-fg text-sm mb-6">Media codec audit &amp; re-encode</p>
        {children}
      </div>
    </div>
  );
}
