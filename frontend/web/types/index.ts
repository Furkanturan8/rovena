export interface Category {
  id: string;
  slug: string;
  label: string; // localized
  image: string; // first photo of a product in the category
}

export interface ColorOption {
  name: string; // catalog key, e.g. "Dark Brown"; display names live in the dictionary
  hex: string;
}

export interface SizeOption {
  label: string;
  stock: number; // sellable units across all colors; 0 = sold out
}

export interface VariantInfo {
  id: string;
  sku: string;
  size: string;
  color: string;
  price: number;
}

export type ProductTag = "limited" | "new" | "sealed" | "custom";

/** Storefront product: catalog API data resolved for one language, plus current stock. */
export interface Product {
  id: string;
  slug: string;
  category: string; // category slug
  name: string;
  kind: string;
  description: string;
  material: string;
  composition: string[];
  price: number; // TL (cheapest active variant)
  craftHours: number;
  edition?: string;
  tag?: ProductTag;
  colors: ColorOption[];
  sizes: SizeOption[];
  variants: VariantInfo[];
  /** Sellable units per variant id; empty when stock was not loaded (e.g. search suggestions). */
  stock: Record<string, number>;
  images: string[];
  sku: string;
  rating: number;
  reviewCount: number;
  releasedAt: string;
  leadTimeDays?: [number, number];
  madeToOrderWeeks?: number;
  video?: "sewing" | "wool";
}

export interface CartItem {
  productId: string;
  variantId: string;
  size: string;
  color: string;
  qty: number;
}
