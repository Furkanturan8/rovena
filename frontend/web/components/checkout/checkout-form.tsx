"use client";

import { useRouter } from "next/navigation";
import { useState } from "react";
import { Check, CreditCard, Lock } from "lucide-react";
import { Link } from "@/components/ui/link";
import { useStore } from "@/components/cart/store";
import { OrderSummary } from "@/components/cart/order-summary";
import { getProductById } from "@/lib/mock/catalog";
import { addresses, user } from "@/lib/mock/account";
import { Photo } from "@/components/ui/photo";
import { useDict, useLocale } from "@/lib/i18n/provider";
import { cn, formatPrice } from "@/lib/utils";

const shippingPrices = { standard: 0, express: 650, store: 0 } as const;
type ShippingId = keyof typeof shippingPrices;

function Step({ n, title, done, children }: { n: number; title: string; done?: boolean; children: React.ReactNode }) {
  return (
    <section className="border-t border-travertine py-10 first:border-t-0 first:pt-0">
      <div className="mb-6 flex items-center gap-4">
        <span className={cn("flex h-8 w-8 items-center justify-center border text-[12px]", done ? "border-ink bg-ink text-canvas" : "border-ink")}>
          {done ? <Check size={14} /> : n}
        </span>
        <h2 className="headline-md">{title}</h2>
      </div>
      {children}
    </section>
  );
}

export function CheckoutForm() {
  const d = useDict();
  const locale = useLocale();
  const t = d.checkout;
  const router = useRouter();
  const { items, clear, hydrated } = useStore();
  const [shipping, setShipping] = useState<ShippingId>("standard");
  const [addressId, setAddressId] = useState(addresses[0].id);
  const [payment, setPayment] = useState<"card" | "transfer">("card");
  const [giftNote, setGiftNote] = useState(false);
  const [busy, setBusy] = useState(false);

  const shippingPrice = shippingPrices[shipping];

  if (hydrated && items.length === 0) {
    return (
      <div className="py-24 text-center">
        <p className="headline-lg">{t.emptyTitle}</p>
        <Link href="/collection" className="btn btn-primary mt-8">{d.cart.discoverCollection}</Link>
      </div>
    );
  }

  function submit(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    const number = `RV-${new Date().toISOString().slice(2, 10).replace(/-/g, "")}-${String(Math.floor(Math.random() * 9000) + 1000)}`;
    setTimeout(() => {
      clear();
      router.push(`/${locale}/order-confirmation/${number}`);
    }, 1100);
  }

  return (
    <form onSubmit={submit} className="grid gap-12 lg:grid-cols-12">
      <div className="lg:col-span-7">
        <Step n={1} title={t.contact} done>
          <div className="flex items-center justify-between border border-travertine p-4">
            <div>
              <p className="text-[14px]">{user.firstName} {user.lastName}</p>
              <p className="text-[12px] text-umber">{user.email}</p>
            </div>
            <Link href="/login" className="text-[12px] text-umber underline underline-offset-4 hover:text-ink">{t.changeAccount}</Link>
          </div>
        </Step>

        <Step n={2} title={t.address}>
          <div className="grid gap-3 sm:grid-cols-2">
            {addresses.map((a) => (
              <label key={a.id} className={cn("cursor-pointer border p-5 transition-colors", addressId === a.id ? "border-ink bg-paper" : "border-travertine hover:border-ink")}>
                <input type="radio" name="address" className="sr-only" checked={addressId === a.id} onChange={() => setAddressId(a.id)} />
                <div className="flex items-center justify-between">
                  <span className="label-caps text-[10px]">{d.account.addresses.titles[a.title]}</span>
                  {addressId === a.id && <Check size={14} />}
                </div>
                <p className="mt-3 text-[13px]">{a.name}</p>
                {a.lines.map((l) => <p key={l} className="text-[13px] text-umber">{l}</p>)}
                <p className="mt-2 text-[12px] text-umber">{a.phone}</p>
              </label>
            ))}
          </div>
          <details className="mt-4 border border-travertine p-5">
            <summary className="label-caps cursor-pointer text-[10px]">{t.newAddress}</summary>
            <div className="mt-5 grid gap-4 sm:grid-cols-2">
              <div><label className="field-label">{t.firstName}</label><input className="field" /></div>
              <div><label className="field-label">{t.lastName}</label><input className="field" /></div>
              <div className="sm:col-span-2"><label className="field-label">{t.addressLine}</label><input className="field" /></div>
              <div><label className="field-label">{t.district}</label><input className="field" /></div>
              <div><label className="field-label">{t.city}</label><input className="field" /></div>
            </div>
          </details>
        </Step>

        <Step n={3} title={t.delivery}>
          <div className="space-y-3">
            {(Object.keys(shippingPrices) as ShippingId[]).map((id) => (
              <label key={id} className={cn("flex cursor-pointer items-center justify-between gap-4 border p-5 transition-colors", shipping === id ? "border-ink bg-paper" : "border-travertine hover:border-ink")}>
                <input type="radio" name="shipping" className="sr-only" checked={shipping === id} onChange={() => setShipping(id)} />
                <div>
                  <p className="text-[14px]">{t.shippingOptions[id].label}</p>
                  <p className="text-[12px] text-umber">{t.shippingOptions[id].note}</p>
                </div>
                <span className="label-mono shrink-0">{shippingPrices[id] === 0 ? d.common.free : formatPrice(shippingPrices[id], locale)}</span>
              </label>
            ))}
          </div>
          <label className="mt-4 flex cursor-pointer items-start gap-3 text-[13px]">
            <input type="checkbox" checked={giftNote} onChange={(e) => setGiftNote(e.target.checked)} className="mt-0.5 h-4 w-4 appearance-none border border-ink checked:bg-ink" />
            <span>{t.giftNote} <span className="text-umber">{t.giftNoteFree}</span></span>
          </label>
          {giftNote && <textarea rows={3} className="field mt-3" placeholder={t.giftNotePlaceholder} maxLength={200} />}
        </Step>

        <Step n={4} title={t.payment}>
          <div className="mb-5 grid grid-cols-2 gap-3">
            {([["card", t.card], ["transfer", t.transfer]] as const).map(([id, label]) => (
              <button type="button" key={id} onClick={() => setPayment(id)} className={cn("label-caps border px-4 py-4 text-[10px]", payment === id ? "border-ink bg-ink text-canvas" : "border-travertine hover:border-ink")}>
                {label}
              </button>
            ))}
          </div>
          {payment === "card" ? (
            <div className="grid gap-4 sm:grid-cols-2">
              <div className="sm:col-span-2">
                <label className="field-label" htmlFor="cc-num">{t.cardNumber}</label>
                <div className="relative">
                  <input id="cc-num" required inputMode="numeric" autoComplete="cc-number" placeholder="0000 0000 0000 0000" className="field !pr-11" />
                  <CreditCard size={16} className="absolute right-4 top-1/2 -translate-y-1/2 text-umber" />
                </div>
              </div>
              <div className="sm:col-span-2">
                <label className="field-label" htmlFor="cc-name">{t.cardName}</label>
                <input id="cc-name" required autoComplete="cc-name" className="field" />
              </div>
              <div>
                <label className="field-label" htmlFor="cc-exp">{t.cardExpiry}</label>
                <input id="cc-exp" required placeholder="MM / YY" autoComplete="cc-exp" className="field" />
              </div>
              <div>
                <label className="field-label" htmlFor="cc-cvc">{t.cardCvc}</label>
                <input id="cc-cvc" required placeholder="000" autoComplete="cc-csc" className="field" />
              </div>
              <div className="sm:col-span-2">
                <label className="field-label" htmlFor="cc-inst">{t.installments}</label>
                <select id="cc-inst" className="field">
                  {t.installmentOptions.map((o) => <option key={o}>{o}</option>)}
                </select>
              </div>
            </div>
          ) : (
            <div className="border border-travertine bg-paper p-5 text-[13px] leading-6 text-umber">{t.transferNote}</div>
          )}
          <p className="mt-4 flex items-center gap-2 text-[12px] text-umber"><Lock size={13} /> {t.secureNote}</p>
        </Step>
      </div>

      <aside className="lg:col-span-5">
        <OrderSummary shipping={shippingPrice} className="lg:sticky lg:top-24">
          <ul className="mt-6 space-y-4 border-t border-travertine pt-6">
            {items.map((i) => {
              const p = getProductById(i.productId, locale);
              if (!p) return null;
              return (
                <li key={`${i.productId}${i.size}${i.color}`} className="flex gap-4">
                  <div className="relative w-16 shrink-0">
                    <Photo src={p.images[0]} alt={p.name} sizes="64px" imgClassName="object-top" />
                    <span className="absolute -right-2 -top-2 flex h-5 w-5 items-center justify-center bg-ink text-[10px] text-canvas">{i.qty}</span>
                  </div>
                  <div className="flex-1">
                    <p className="font-serif text-[15px] leading-5">{p.name}</p>
                    <p className="text-[12px] text-umber">{d.colors[i.color] ?? i.color} · {d.sizes[i.size] ?? i.size}</p>
                  </div>
                  <p className="label-mono">{formatPrice(p.price * i.qty, locale)}</p>
                </li>
              );
            })}
          </ul>
          <button disabled={busy} className="btn btn-primary mt-8 w-full">{busy ? t.confirming : t.confirm}</button>
          <p className="mt-4 text-center text-[11px] leading-5 text-umber">{t.terms}</p>
        </OrderSummary>
      </aside>
    </form>
  );
}
