"use client";

import { useStore } from "./store";
import { useProducts } from "@/hooks/use-products";
import type { CartItem, Product, VariantInfo } from "@/types";

export interface CartLine {
  item: CartItem;
  product: Product;
  variant: VariantInfo;
  lineTotal: number;
}

/** Cart items joined with live catalog data (name, photo, current price). Prices always come from the catalog. */
export function useCartLines() {
  const { items, hydrated } = useStore();
  const { products, ready } = useProducts(items.map((i) => i.productId));

  const lines: CartLine[] = items.flatMap((item) => {
    const product = products.get(item.productId);
    const variant = product?.variants.find((v) => v.id === item.variantId);
    return product && variant ? [{ item, product, variant, lineTotal: variant.price * item.qty }] : [];
  });
  const subtotal = lines.reduce((n, l) => n + l.lineTotal, 0);

  return { lines, subtotal, ready: hydrated && ready };
}
