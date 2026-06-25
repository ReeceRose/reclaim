"use client";

import { useState } from "react";
import { Menu, X } from "lucide-react";
import { GitHubIcon } from "@/components/github-icon";
import { LogoMark, Wordmark } from "@/components/logo";
import { nav, site } from "@/lib/site";

export function Nav() {
  const [open, setOpen] = useState(false);

  return (
    <header className="sticky top-0 z-50 border-b border-line/70 bg-bg/80 backdrop-blur-md">
      <div className="mx-auto flex h-16 max-w-6xl items-center justify-between px-5 sm:px-7">
        <a href="#top" className="flex items-center gap-2.5 font-extrabold tracking-tight">
          <LogoMark className="h-7 w-7" />
          <Wordmark className="text-lg" />
        </a>

        <nav className="hidden items-center gap-7 md:flex">
          {nav.map((item) => (
            <a
              key={item.href}
              href={item.href}
              target={item.external ? "_blank" : undefined}
              rel={item.external ? "noreferrer" : undefined}
              className="text-sm text-muted-fg transition-colors hover:text-text"
            >
              {item.label}
            </a>
          ))}
        </nav>

        <div className="hidden items-center gap-3 md:flex">
          <a
            href={site.repo}
            target="_blank"
            rel="noreferrer"
            className="inline-flex h-9 items-center gap-2 rounded-md border border-line bg-surface px-3.5 text-sm font-medium text-text transition-colors hover:bg-surface-2"
          >
            <GitHubIcon className="h-4 w-4" />
            GitHub
          </a>
          <a
            href="#install"
            className="inline-flex h-9 items-center rounded-md bg-brand px-4 text-sm font-semibold text-on-brand transition-colors hover:bg-brand-2"
          >
            Get started
          </a>
        </div>

        <button
          type="button"
          aria-label="Toggle menu"
          onClick={() => setOpen((v) => !v)}
          className="inline-flex h-9 w-9 items-center justify-center rounded-md border border-line bg-surface text-text md:hidden"
        >
          {open ? <X className="h-5 w-5" /> : <Menu className="h-5 w-5" />}
        </button>
      </div>

      {open && (
        <div className="border-t border-line bg-bg px-5 py-4 md:hidden">
          <nav className="flex flex-col gap-1">
            {nav.map((item) => (
              <a
                key={item.href}
                href={item.href}
                target={item.external ? "_blank" : undefined}
                rel={item.external ? "noreferrer" : undefined}
                onClick={() => setOpen(false)}
                className="rounded-md px-2 py-2.5 text-sm text-muted-fg transition-colors hover:bg-surface hover:text-text"
              >
                {item.label}
              </a>
            ))}
            <a
              href={site.repo}
              target="_blank"
              rel="noreferrer"
              onClick={() => setOpen(false)}
              className="mt-2 inline-flex h-10 items-center justify-center gap-2 rounded-md border border-line bg-surface text-sm font-medium"
            >
              <GitHubIcon className="h-4 w-4" />
              GitHub
            </a>
            <a
              href="#install"
              onClick={() => setOpen(false)}
              className="mt-2 inline-flex h-10 items-center justify-center rounded-md bg-brand text-sm font-semibold text-on-brand"
            >
              Get started
            </a>
          </nav>
        </div>
      )}
    </header>
  );
}
