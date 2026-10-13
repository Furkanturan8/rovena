import type { Address, Order, OrderStatus } from "@/types";

export const user = {
  firstName: "Elif",
  lastName: "Demirci",
  email: "elif.demirci@example.com",
  phone: "+90 532 000 00 00",
  memberSince: "2023",
};

export const addresses: Address[] = [
  {
    id: "a-1",
    title: "home",
    name: "Elif Demirci",
    lines: ["Bebek Mah. Cevdet Paşa Cad. No: 42 D: 6", "Beşiktaş / İstanbul 34342"],
    phone: "+90 532 000 00 00",
    isDefault: true,
  },
  {
    id: "a-2",
    title: "office",
    name: "Elif Demirci",
    lines: ["Levent Mah. Büyükdere Cad. No: 201 Kat: 14", "Şişli / İstanbul 34394"],
    phone: "+90 532 000 00 00",
  },
];

export const orders: Order[] = [
  {
    id: "o-1",
    number: "RV-250914-0042",
    placedAt: "2025-10-09T14:22:00",
    status: "preparing",
    lines: [{ productId: "p-001", size: "S", color: "Dark Brown", qty: 1, unitPrice: 168000 }],
    shipping: 0,
    total: 168000,
    address: "Bebek Mah. Cevdet Paşa Cad. No: 42 D: 6, Beşiktaş / İstanbul",
    paymentLabel: "Visa •••• 4821",
    eta: "2025-10-21",
  },
  {
    id: "o-2",
    number: "RV-250830-0017",
    placedAt: "2025-09-02T10:05:00",
    status: "delivered",
    lines: [
      { productId: "p-004", size: "S", color: "Bark", qty: 1, unitPrice: 66000 },
      { productId: "p-013", size: "OS", color: "Midnight Navy", qty: 1, unitPrice: 154000 },
    ],
    shipping: 0,
    total: 220000,
    address: "Levent Mah. Büyükdere Cad. No: 201 Kat: 14, Şişli / İstanbul",
    paymentLabel: "Mastercard •••• 1190",
    trackingCode: "RVN-TR-448120",
  },
  {
    id: "o-3",
    number: "RV-250611-0009",
    placedAt: "2025-06-11T19:40:00",
    status: "refunded",
    lines: [{ productId: "p-012", size: "38", color: "Caramel", qty: 1, unitPrice: 59000 }],
    shipping: 0,
    total: 59000,
    address: "Bebek Mah. Cevdet Paşa Cad. No: 42 D: 6, Beşiktaş / İstanbul",
    paymentLabel: "Visa •••• 4821",
  },
];

export const statusTone: Record<OrderStatus, "neutral" | "active" | "done" | "bad"> = {
  pending: "neutral",
  confirmed: "active",
  preparing: "active",
  shipped: "active",
  delivered: "done",
  cancelled: "bad",
  refunded: "bad",
};
