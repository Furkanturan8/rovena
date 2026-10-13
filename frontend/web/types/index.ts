export type CategorySlug = "coats" | "trousers" | "jackets" | "knitwear" | "accessories";

export interface Category {
  slug: CategorySlug;
  image: string;
}

export interface ColorOption {
  name: string;
  hex: string;
}

export interface SizeOption {
  label: string;
  stock: number;
}

export type ProductTag = "limited" | "new" | "sealed" | "custom";

export interface ProductText {
  name: string;
  kind: string;
  description: string;
  material: string;
  composition: string[];
}

/** Dilden bağımsız ürün verisi. */
export interface ProductBase {
  id: string;
  slug: string;
  category: CategorySlug;
  price: number;
  craftHours: number;
  edition?: string; // "No. 04/20"
  tag?: ProductTag;
  colors: ColorOption[];
  sizes: SizeOption[];
  images: string[];
  sku: string;
  rating: number;
  reviewCount: number;
  releasedAt: string;
  leadTimeDays?: [number, number];
  madeToOrderWeeks?: number;
  video?: "sewing" | "wool";
}

/** Aktif dile göre metinleri çözülmüş ürün. */
export type Product = ProductBase & ProductText;

export interface CartItem {
  productId: string;
  size: string;
  color: string;
  qty: number;
}

export type OrderStatus =
  | "pending"
  | "confirmed"
  | "preparing"
  | "shipped"
  | "delivered"
  | "cancelled"
  | "refunded";

export interface OrderLine {
  productId: string;
  size: string;
  color: string;
  qty: number;
  unitPrice: number;
}

export interface Order {
  id: string;
  number: string;
  placedAt: string;
  status: OrderStatus;
  lines: OrderLine[];
  shipping: number;
  total: number;
  address: string;
  paymentLabel: string;
  trackingCode?: string;
  eta?: string;
}

export interface Address {
  id: string;
  title: "home" | "office";
  name: string;
  lines: string[];
  phone: string;
  isDefault?: boolean;
}
