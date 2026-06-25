"use client";

import { useState } from "react";
import { Check, Copy } from "lucide-react";

export function CopyButton({ text, label }: { text: string; label?: string }) {
  const [copied, setCopied] = useState(false);

  async function copy() {
    try {
      await navigator.clipboard.writeText(text);
      setCopied(true);
      setTimeout(() => setCopied(false), 1800);
    } catch {
      // Clipboard API unavailable (e.g. non-secure context) — no-op.
    }
  }

  return (
    <button
      type="button"
      onClick={copy}
      aria-label={label ?? "Copy to clipboard"}
      className="inline-flex h-8 items-center gap-1.5 rounded-md border border-line bg-surface-2 px-2.5 text-xs font-medium text-muted-fg transition-colors hover:text-text"
    >
      {copied ? (
        <>
          <Check className="h-3.5 w-3.5 text-green" />
          Copied
        </>
      ) : (
        <>
          <Copy className="h-3.5 w-3.5" />
          Copy
        </>
      )}
    </button>
  );
}
