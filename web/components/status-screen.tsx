import type { ReactNode } from "react";
import { LogoMark } from "@/components/logo";

/**
 * Full-screen centered status surface for terminal app states (404, runtime
 * errors). Mirrors the auth layout: radial brand glow over the Carbon bg, the
 * Reclaim mark + wordmark, then a code/title/description stack and actions.
 *
 * Presentational only (no hooks), so it's safe to render from both server
 * components (not-found) and client error boundaries (error.tsx).
 */
export function StatusScreen({
  code,
  title,
  description,
  children,
}: {
  code: string;
  title: string;
  description: ReactNode;
  children?: ReactNode;
}) {
  return (
    <div
      className="fixed inset-0 flex items-center justify-center p-6 z-200"
      style={{
        background:
          "radial-gradient(130% 120% at 50% 0%, var(--brand-soft), transparent 55%), var(--bg)",
      }}
    >
      <div className="w-full max-w-md text-center">
        <div className="flex items-center justify-center gap-3 mb-6">
          <LogoMark
            size={34}
            className="shrink-0"
            style={{
              boxShadow: "0 6px 18px var(--brand-soft)",
              borderRadius: 10,
            }}
          />
          <div className="text-2xl font-extrabold tracking-tight">
            Re<span className="text-brand">claim</span>
          </div>
        </div>

        <div
          className="rounded-2xl border border-line px-7 py-9"
          style={{ background: "var(--surface)" }}
        >
          <div
            className="font-mono font-semibold tnum leading-none mb-4 text-muted-dim"
            style={{ fontSize: "3.25rem" }}
          >
            {code}
          </div>
          <h1 className="text-title font-bold mb-2">{title}</h1>
          <p className="text-muted-fg text-sm leading-relaxed">{description}</p>

          {children && (
            <div className="mt-7 flex flex-col sm:flex-row items-center justify-center gap-2.5">
              {children}
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
