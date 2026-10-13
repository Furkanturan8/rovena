"use client";

import { createContext, useCallback, useContext, useEffect, useMemo, useState } from "react";
import type { CartItem } from "@/types";
import { productBases } from "@/lib/mock/catalog";

const CART_KEY = "rovena.cart.v2";
const WISH_KEY = "rovena.wish.v2";

interface StoreValue {
  items: CartItem[];
  count: number;
  subtotal: number;
  drawerOpen: boolean;
  openDrawer: () => void;
  closeDrawer: () => void;
  add: (item: CartItem, openAfter?: boolean) => void;
  setQty: (productId: string, size: string, color: string, qty: number) => void;
  remove: (productId: string, size: string, color: string) => void;
  clear: () => void;
  wishlist: string[];
  toggleWish: (productId: string) => void;
  hydrated: boolean;
}

const StoreContext = createContext<StoreValue | null>(null);

type CartKey = Pick<CartItem, "productId" | "size" | "color">;
const same = (a: CartKey, b: CartKey) => a.productId === b.productId && a.size === b.size && a.color === b.color;

function readStorage<T>(key: string, fallback: T): T {
  try {
    const raw = localStorage.getItem(key);
    return raw ? (JSON.parse(raw) as T) : fallback;
  } catch {
    return fallback;
  }
}

export function StoreProvider({ children }: { children: React.ReactNode }) {
  const [items, setItems] = useState<CartItem[]>([]);
  const [wishlist, setWishlist] = useState<string[]>([]);
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [hydrated, setHydrated] = useState(false);

  // localStorage yalnızca istemcide okunabilir; SSR çıktısıyla uyuşsun diye mount sonrası yüklenir.
  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setItems(readStorage<CartItem[]>(CART_KEY, []));
    setWishlist(readStorage<string[]>(WISH_KEY, ["p-009", "p-016"]));
    setHydrated(true);
  }, []);

  useEffect(() => {
    if (!hydrated) return;
    try {
      localStorage.setItem(CART_KEY, JSON.stringify(items));
      localStorage.setItem(WISH_KEY, JSON.stringify(wishlist));
    } catch {}
  }, [items, wishlist, hydrated]);

  useEffect(() => {
    document.body.style.overflow = drawerOpen ? "hidden" : "";
    return () => {
      document.body.style.overflow = "";
    };
  }, [drawerOpen]);

  const add = useCallback((item: CartItem, openAfter = true) => {
    setItems((prev) => {
      const idx = prev.findIndex((i) => same(i, item));
      if (idx === -1) return [...prev, item];
      const next = [...prev];
      next[idx] = { ...next[idx], qty: next[idx].qty + item.qty };
      return next;
    });
    if (openAfter) setDrawerOpen(true);
  }, []);

  const setQty = useCallback((productId: string, size: string, color: string, qty: number) => {
    setItems((prev) =>
      prev.map((i) => (same(i, { productId, size, color }) ? { ...i, qty } : i)).filter((i) => i.qty > 0),
    );
  }, []);

  const remove = useCallback((productId: string, size: string, color: string) => {
    setItems((prev) => prev.filter((i) => !same(i, { productId, size, color })));
  }, []);

  const clear = useCallback(() => setItems([]), []);

  const toggleWish = useCallback((productId: string) => {
    setWishlist((prev) => (prev.includes(productId) ? prev.filter((id) => id !== productId) : [...prev, productId]));
  }, []);

  const value = useMemo<StoreValue>(() => {
    const count = items.reduce((n, i) => n + i.qty, 0);
    const subtotal = items.reduce((sum, i) => sum + (productBases.find((p) => p.id === i.productId)?.price ?? 0) * i.qty, 0);
    return {
      items,
      count,
      subtotal,
      drawerOpen,
      openDrawer: () => setDrawerOpen(true),
      closeDrawer: () => setDrawerOpen(false),
      add,
      setQty,
      remove,
      clear,
      wishlist,
      toggleWish,
      hydrated,
    };
  }, [items, drawerOpen, wishlist, hydrated, add, setQty, remove, clear, toggleWish]);

  return <StoreContext.Provider value={value}>{children}</StoreContext.Provider>;
}

export function useStore() {
  const ctx = useContext(StoreContext);
  if (!ctx) throw new Error("useStore must be used within StoreProvider");
  return ctx;
}
