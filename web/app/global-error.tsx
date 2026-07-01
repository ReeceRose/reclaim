"use client";

import { useEffect } from "react";
import "./globals.css";
import { StatusScreen } from "@/components/status-screen";

/**
 * Catches errors thrown in the root layout itself. It replaces the entire
 * document, so it must render its own <html>/<body> and pull in global styles.
 * Metadata exports aren't allowed in client components, hence the <title> tag.
 */
export default function GlobalError({
  error,
  reset,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}) {
  useEffect(() => {
    console.error(error);
  }, [error]);

  return (
    <html lang="en" className="dark h-full antialiased">
      <head>
        <title>Something went wrong · Reclaim</title>
      </head>
      <body className="min-h-full">
        <StatusScreen
          code="500"
          title="Something went wrong"
          description="The app hit an unexpected error while loading. Reloading usually clears it."
        >
          <button
            type="button"
            onClick={() => reset()}
            className="inline-flex items-center justify-center h-10 px-5 rounded-[11px] text-sm font-semibold text-on-brand"
            style={{
              background:
                "linear-gradient(145deg, var(--brand), var(--brand-2))",
              boxShadow: "0 4px 14px var(--brand-soft)",
            }}
          >
            Reload
          </button>
        </StatusScreen>
      </body>
    </html>
  );
}
