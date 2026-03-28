import { useCallback, useRef, useState, useMemo } from "react";

export interface OptimisticItem {
  _pending?: boolean;
  _optimistic?: boolean;
}

interface Override<T> {
  patch?: Partial<T>;
  hidden?: boolean;
  pending?: boolean;
  optimistic?: T; // full item for optimistic creates
}

/**
 * Wraps polling data with optimistic overrides and stable ordering.
 *
 * - `addOptimistic(item)` — add an item that doesn't exist in external yet
 * - `hide(id)` — mark an item as deleted (hidden from UI)
 * - `patch(id, partial)` — override fields (rename, status change)
 * - `setPending(id, flag)` — mark as pending (show spinner)
 * - `clear(id)` — remove all overrides for an id
 *
 * Overrides auto-clean when external data catches up.
 */
export function useOptimistic<T>(
  externalData: T[],
  getId: (item: T) => string,
) {
  const [overrides, setOverrides] = useState<Map<string, Override<T>>>(new Map());
  const orderRef = useRef<Map<string, number>>(new Map());
  const nextOrder = useRef(0);

  // Auto-cleanup: remove overrides that external has caught up to
  const items = useMemo(() => {
    const result: (T & OptimisticItem)[] = [];
    const externalIds = new Set<string>();
    const stalePatch: string[] = [];
    const staleHidden: string[] = [];

    // Process external items
    for (const item of externalData) {
      const id = getId(item);
      externalIds.add(id);
      const ov = overrides.get(id);

      // Hidden → skip, but check if truly gone
      if (ov?.hidden) continue;

      // Apply patches
      let merged: T & OptimisticItem = { ...item } as T & OptimisticItem;
      if (ov?.patch) {
        // Check if patch matches external — if so, it's stale
        const allMatch = Object.entries(ov.patch).every(
          ([k, v]) => (item as Record<string, unknown>)[k] === v,
        );
        if (allMatch) {
          stalePatch.push(id);
        } else {
          merged = { ...merged, ...ov.patch };
        }
      }
      if (ov?.pending) merged._pending = true;
      result.push(merged);
    }

    // Add optimistic items not yet in external
    for (const [id, ov] of overrides) {
      if (ov.optimistic && !externalIds.has(id)) {
        result.push({ ...ov.optimistic, _optimistic: true, _pending: ov.pending });
      }
      // Optimistic item appeared in external → stale
      if (ov.optimistic && externalIds.has(id)) {
        stalePatch.push(id);
      }
      // Hidden item disappeared from external → stale
      if (ov.hidden && !externalIds.has(id)) {
        staleHidden.push(id);
      }
    }

    // Schedule cleanup of stale overrides
    const toClean = [...stalePatch, ...staleHidden];
    if (toClean.length > 0) {
      // Use setTimeout to avoid setState during render
      setTimeout(() => {
        setOverrides((prev) => {
          const m = new Map(prev);
          toClean.forEach((id) => m.delete(id));
          return m;
        });
      }, 0);
    }

    // Stable ordering: assign order numbers to known ids, sort by them
    const order = orderRef.current;
    for (const item of result) {
      const id = getId(item);
      if (!order.has(id)) {
        order.set(id, nextOrder.current++);
      }
    }
    // Clean up order entries for ids no longer present
    const presentIds = new Set(result.map((item) => getId(item)));
    for (const id of order.keys()) {
      if (!presentIds.has(id)) order.delete(id);
    }

    result.sort((a, b) => (order.get(getId(a)) ?? 0) - (order.get(getId(b)) ?? 0));

    return result;
  }, [externalData, overrides, getId]);

  const addOptimistic = useCallback((item: T) => {
    const id = getId(item);
    setOverrides((prev) => {
      const m = new Map(prev);
      m.set(id, { ...prev.get(id), optimistic: item, pending: true });
      return m;
    });
  }, [getId]);

  const hide = useCallback((id: string) => {
    setOverrides((prev) => {
      const m = new Map(prev);
      m.set(id, { ...prev.get(id), hidden: true });
      return m;
    });
  }, []);

  const patch = useCallback((id: string, p: Partial<T>) => {
    setOverrides((prev) => {
      const m = new Map(prev);
      const existing = prev.get(id);
      m.set(id, { ...existing, patch: { ...existing?.patch, ...p } as Partial<T> });
      return m;
    });
  }, []);

  const setPending = useCallback((id: string, pending: boolean) => {
    setOverrides((prev) => {
      const m = new Map(prev);
      const existing = prev.get(id);
      if (!pending && !existing?.patch && !existing?.hidden && !existing?.optimistic) {
        m.delete(id);
      } else {
        m.set(id, { ...existing, pending });
      }
      return m;
    });
  }, []);

  const clear = useCallback((id: string) => {
    setOverrides((prev) => {
      const m = new Map(prev);
      m.delete(id);
      return m;
    });
  }, []);

  return { items, addOptimistic, hide, patch, setPending, clear };
}
