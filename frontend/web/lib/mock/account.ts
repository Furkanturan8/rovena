import type { Address } from "@/types";

// The backend has no address API yet: these are demo addresses shown in the account area and at checkout.
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
