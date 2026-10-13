// Shapes returned by the Rovena gateway (see backend/services/*/internal/transport/http/dto.go).

export interface ApiEnvelope<T> {
  success: boolean;
  data: T;
  message?: string;
}

export interface ApiPage<T> {
  items: T[];
  total: number;
  page: number;
  page_size: number;
}

export interface ApiCategory {
  id: string;
  parent_id: string | null;
  name: string;
  slug: string;
  description: string;
  translations: Record<string, { name?: string; description?: string }>;
}

export interface ApiImage {
  url: string;
  alt: string;
}

export interface ApiVariant {
  id: string;
  sku: string;
  size: string;
  color: string;
  price_cents: number;
  currency: string;
  active: boolean;
}

export interface ApiLocalizedText {
  name?: string;
  description?: string;
  kind?: string;
  material?: string;
  composition?: string[];
}

export interface ApiProductAttributes {
  sku_base?: string;
  craft_hours?: number;
  edition?: string;
  tag?: string;
  rating?: number;
  review_count?: number;
  released_at?: string;
  video?: string;
  lead_time_days?: [number, number];
  made_to_order_weeks?: number;
  colors?: { name: string; hex: string }[];
}

/** List items and product detail share these fields; detail adds timestamps. */
export interface ApiProduct {
  id: string;
  category_id: string;
  name: string;
  slug: string;
  description?: string;
  brand: string;
  status: string;
  price_from_cents?: number | null;
  translations: Record<string, ApiLocalizedText>;
  attributes: ApiProductAttributes;
  images: ApiImage[];
  variants: ApiVariant[];
}

export interface ApiAvailability {
  variant_id: string;
  available: number;
  in_stock: boolean;
}

export interface ApiTokens {
  access_token: string;
  access_expires_at: string;
  refresh_token: string;
  refresh_expires_at: string;
}

export interface ApiUser {
  id: string;
  email: string;
  first_name: string;
  last_name: string;
  role: "customer" | "admin";
  created_at: string;
}

export interface ApiAuthResult {
  user: ApiUser;
  tokens: ApiTokens;
}

export type ApiOrderStatus = "pending" | "awaiting_payment" | "paid" | "confirmed" | "shipped" | "delivered" | "cancelled";

export interface ApiAddress {
  id: string;
  title: string;
  full_name: string;
  phone: string;
  line1: string;
  line2: string;
  district: string;
  city: string;
  postal_code: string;
  country: string;
  is_default: boolean;
}

export interface ApiShippingAddress {
  full_name: string;
  phone: string;
  line1: string;
  line2?: string;
  district: string;
  city: string;
  postal_code?: string;
  country?: string;
}

export interface ApiOrderItem {
  variant_id: string;
  product_id: string;
  sku: string;
  product_name: string;
  size: string;
  color: string;
  unit_price_cents: number;
  quantity: number;
  line_total_cents: number;
}

export interface ApiOrder {
  id: string;
  status: ApiOrderStatus;
  total_cents: number;
  currency: string;
  items: ApiOrderItem[];
  failure_code?: string;
  failure_message?: string;
  refund?: "pending" | "refunded";
  shipping_address?: ApiShippingAddress;
  carrier?: string;
  tracking_code?: string;
  shipped_at?: string;
  delivered_at?: string;
  created_at: string;
  updated_at: string;
  history?: { status: ApiOrderStatus; at: string }[];
}
