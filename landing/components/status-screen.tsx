import type { ReactNode } from "react";
import Link from "next/link";
import { LogoMark, Wordmark } from "@/components/logo";

/**
 * Full-screen centered status surface for terminal states (404, runtime
 * errors). Reuses the hero's visual language — grid backdrop, brand glow, the
 * Reclaim mark/wordmark — so an error still feels on-brand.
 *
 * Presentational only (no hooks), so it renders from both the server
 * not-found page and the client error boundaries.
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
    <main className="relative flex min-h-screen items-center justify-center overflow-hidden px-5 py-16">
      <div className="bg-grid pointer-events-none absolute inset-0 opacity-60" />
      <div
        className="pointer-events-none absolute inset-0"
        style={{
          background:
            "radial-gradient(120% 90% at 50% 0%, var(--brand-soft), transparent 55%)",
        }}
      />

      <div className="relative w-full max-w-lg text-center">
        <Link
          href="/"
          className="mb-8 inline-flex items-center gap-2.5 font-extrabold tracking-tight"
        >
          <LogoMark className="h-8 w-8" />
          <Wordmark className="text-xl" />
        </Link>

        <div className="glow-brand rounded-[18px] border border-line bg-surface px-7 py-10">
          <div className="mb-4 font-mono text-6xl font-semibold leading-none tracking-tight text-muted-dim tnum">
            {code}
          </div>
          <h1 className="mb-2.5 text-title font-bold tracking-tight">{title}</h1>
          <p className="mx-auto max-w-sm text-sm leading-relaxed text-muted-fg">
            {description}
          </p>

          {children && (
            <div className="mt-8 flex flex-col items-center justify-center gap-2.5 sm:flex-row">
              {children}
            </div>
          )}
        </div>
      </div>
    </main>
  );
}
