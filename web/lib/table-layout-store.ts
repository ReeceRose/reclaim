import { EMPTY_LAYOUT, type TableLayout } from "@/lib/table-columns";

const PREFIX = "reclaim:columns:";

const cache = new Map<string, TableLayout>();
const listeners = new Map<string, Set<() => void>>();
let bound = false;

function storageKey(tableId: string): string {
  return `${PREFIX}${tableId}`;
}

function parseLayout(raw: string | null): TableLayout {
  if (!raw) return EMPTY_LAYOUT;
  try {
    const parsed: unknown = JSON.parse(raw);
    if (!parsed || typeof parsed !== "object") return EMPTY_LAYOUT;
    const { order, visible } = parsed as Partial<TableLayout>;
    const nextOrder = Array.isArray(order)
      ? order.filter((id): id is string => typeof id === "string")
      : [];
    const nextVisible: Record<string, boolean> = {};
    if (visible && typeof visible === "object") {
      for (const [id, on] of Object.entries(visible)) {
        if (typeof on === "boolean") nextVisible[id] = on;
      }
    }
    if (nextOrder.length === 0 && Object.keys(nextVisible).length === 0) {
      return EMPTY_LAYOUT;
    }
    return { order: nextOrder, visible: nextVisible };
  } catch {
    return EMPTY_LAYOUT;
  }
}

function emit(tableId: string) {
  const set = listeners.get(tableId);
  if (!set) return;
  for (const listener of set) listener();
}

function onStorage(event: StorageEvent) {
  if (event.key === null) {
    for (const tableId of [...cache.keys()]) {
      cache.delete(tableId);
      emit(tableId);
    }
    return;
  }
  if (!event.key.startsWith(PREFIX)) return;
  const tableId = event.key.slice(PREFIX.length);
  cache.delete(tableId);
  emit(tableId);
}

export function subscribeLayout(tableId: string, listener: () => void) {
  const existing = listeners.get(tableId);
  const set = existing ?? new Set<() => void>();
  if (!existing) listeners.set(tableId, set);
  set.add(listener);
  if (!bound && typeof window !== "undefined") {
    window.addEventListener("storage", onStorage);
    bound = true;
  }
  return () => {
    set.delete(listener);
    if (set.size === 0) listeners.delete(tableId);
  };
}

export function readLayout(tableId: string): TableLayout {
  const cached = cache.get(tableId);
  if (cached) return cached;
  if (typeof window === "undefined") return EMPTY_LAYOUT;
  let layout = EMPTY_LAYOUT;
  try {
    layout = parseLayout(window.localStorage.getItem(storageKey(tableId)));
  } catch {
    layout = EMPTY_LAYOUT;
  }
  cache.set(tableId, layout);
  return layout;
}

export function serverLayout(): TableLayout {
  return EMPTY_LAYOUT;
}

export function writeLayout(tableId: string, layout: TableLayout) {
  cache.set(tableId, layout);
  try {
    window.localStorage.setItem(storageKey(tableId), JSON.stringify(layout));
  } catch {}
  emit(tableId);
}

export function clearLayout(tableId: string) {
  cache.set(tableId, EMPTY_LAYOUT);
  try {
    window.localStorage.removeItem(storageKey(tableId));
  } catch {}
  emit(tableId);
}
