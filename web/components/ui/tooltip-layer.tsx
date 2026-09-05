"use client";

import {
  useCallback,
  useEffect,
  useLayoutEffect,
  useRef,
  useState,
} from "react";
import { createPortal } from "react-dom";
import { cn } from "@/lib/utils";

const ATTR = "data-tooltip";
const TIP_ID = "reclaim-tooltip";
const DELAY_MS = 320;
const EDGE = 8;
const GAP = 8;

type Anchored = { text: string; rect: DOMRect };
type Placed = { left: number; top: number; below: boolean };

function tooltipTarget(node: EventTarget | null): HTMLElement | null {
  if (!(node instanceof Element)) return null;
  const found = node.closest(`[${ATTR}]`);
  return found instanceof HTMLElement && found.getAttribute(ATTR)
    ? found
    : null;
}

/**
 * TooltipLayer renders one tooltip for the whole app, driven by delegated
 * hover and focus events rather than a component per trigger — the media
 * tables put a tooltip on every cell, and a virtualised list would otherwise
 * mount hundreds of them. Opt in from anywhere with a `data-tooltip`
 * attribute; the innermost one under the pointer wins.
 */
export function TooltipLayer() {
  const [mounted, setMounted] = useState(false);
  const [anchored, setAnchored] = useState<Anchored | null>(null);
  const [placed, setPlaced] = useState<Placed | null>(null);
  const tipRef = useRef<HTMLDivElement | null>(null);
  const anchorRef = useRef<HTMLElement | null>(null);
  const timerRef = useRef<number | undefined>(undefined);

  useEffect(() => setMounted(true), []);

  const hide = useCallback(() => {
    window.clearTimeout(timerRef.current);
    anchorRef.current?.removeAttribute("aria-describedby");
    anchorRef.current = null;
    setAnchored(null);
    setPlaced(null);
  }, []);

  const show = useCallback(
    (target: HTMLElement) => {
      if (target === anchorRef.current) return;
      hide();
      anchorRef.current = target;
      timerRef.current = window.setTimeout(() => {
        const anchor = anchorRef.current;
        const text = anchor?.getAttribute(ATTR);
        if (!anchor?.isConnected || !text) return;
        anchor.setAttribute("aria-describedby", TIP_ID);
        setAnchored({ text, rect: anchor.getBoundingClientRect() });
      }, DELAY_MS);
    },
    [hide],
  );

  useEffect(() => {
    function onPointerOver(event: PointerEvent) {
      if (event.pointerType === "touch") return;
      const target = tooltipTarget(event.target);
      if (target) show(target);
      else if (anchorRef.current) hide();
    }
    function onFocusIn(event: FocusEvent) {
      const target = tooltipTarget(event.target);
      if (target) show(target);
      else if (anchorRef.current) hide();
    }
    function onKeyDown(event: KeyboardEvent) {
      if (event.key === "Escape") hide();
    }
    document.addEventListener("pointerover", onPointerOver);
    document.addEventListener("pointerdown", hide);
    document.addEventListener("focusin", onFocusIn);
    document.addEventListener("keydown", onKeyDown);
    window.addEventListener("scroll", hide, true);
    window.addEventListener("resize", hide);
    window.addEventListener("blur", hide);
    return () => {
      window.clearTimeout(timerRef.current);
      document.removeEventListener("pointerover", onPointerOver);
      document.removeEventListener("pointerdown", hide);
      document.removeEventListener("focusin", onFocusIn);
      document.removeEventListener("keydown", onKeyDown);
      window.removeEventListener("scroll", hide, true);
      window.removeEventListener("resize", hide);
      window.removeEventListener("blur", hide);
    };
  }, [hide, show]);

  useLayoutEffect(() => {
    const tip = tipRef.current;
    if (!anchored || !tip) return;
    const { width, height } = tip.getBoundingClientRect();
    const below = anchored.rect.top < height + GAP + EDGE;
    setPlaced({
      below,
      top: below
        ? anchored.rect.bottom + GAP
        : anchored.rect.top - GAP - height,
      left: Math.min(
        Math.max(
          EDGE,
          anchored.rect.left + anchored.rect.width / 2 - width / 2,
        ),
        Math.max(EDGE, window.innerWidth - EDGE - width),
      ),
    });
  }, [anchored]);

  if (!mounted || !anchored) return null;

  return createPortal(
    <div
      id={TIP_ID}
      role="tooltip"
      ref={tipRef}
      style={{ left: placed?.left ?? 0, top: placed?.top ?? 0 }}
      className={cn(
        "fixed z-60 pointer-events-none max-w-80 rounded-lg px-2.5 py-1.5",
        "border border-line bg-surface-3 shadow-xl",
        "text-xs leading-snug text-text break-words",
        "transition-opacity duration-100",
        placed ? "opacity-100" : "opacity-0",
      )}
    >
      {anchored.text}
    </div>,
    document.body,
  );
}
