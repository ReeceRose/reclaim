"use client";

import { RotateCw } from "lucide-react";
import { useEffect } from "react";
import "./globals.css";
import { StatusScreen } from "@/components/status-screen";

/**
 * Catches errors thrown in the root layout itself. It replaces the whole
 * document, so it renders its own <html>/<body> and pulls in global styles.
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
        <title>Something went wrong — Reclaim</title>
      </head>
      <body className="min-h-full font-sans">
        <StatusScreen
          code="500"
          title="Something went wrong"
          description="The page hit an unexpected error while loading. Reloading usually clears it."
        >
          <button
            type="button"
            onClick={() => reset()}
            className="inline-flex h-10 items-center gap-2 rounded-md bg-brand px-4 text-sm font-semibold text-on-brand transition-colors hover:bg-brand-2"
          >
            <RotateCw className="h-4 w-4" />
            Reload
          </button>
        </StatusScreen>
      </body>
    </html>
  );
}
