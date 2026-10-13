"use client";

import { useEffect, useMemo, useState } from "react";
import { fetchProduct } from "@/lib/api/client";
import { toProduct } from "@/lib/api/mappers";
import type { ApiProduct } from "@/lib/api/types";
import { useLocale } from "@/lib/i18n/provider";
import type { Product } from "@/types";

/** Loads products by id (cart, wishlist) from the catalog API; unknown ids are left out. */
export function useProducts(ids: string[]) {
  const locale = useLocale();
  const key = useMemo(() => Array.from(new Set(ids)).sort().join(","), [ids]);
  const [loaded, setLoaded] = useState<Record<string, ApiProduct>>({});
  const [doneKey, setDoneKey] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    if (!key) return;
    const unique = key.split(",");
    Promise.all(unique.map(fetchProduct)).then((list) => {
      if (cancelled) return;
      setLoaded((prev) => {
        const next = { ...prev };
        for (const p of list) if (p) next[p.id] = p;
        return next;
      });
      setDoneKey(key);
    });
    return () => {
      cancelled = true;
    };
  }, [key]);

  const products = useMemo(() => {
    const map = new Map<string, Product>();
    for (const id of ids) {
      const api = loaded[id];
      if (api) map.set(id, toProduct(api, locale, ""));
    }
    return map;
  }, [ids, loaded, locale]);

  // An empty id list is trivially loaded; otherwise ready once the lookup for the current ids finished.
  return { products, ready: key === "" || doneKey === key };
}
