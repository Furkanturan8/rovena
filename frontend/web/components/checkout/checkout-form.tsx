"use client";

import { useRouter } from "next/navigation";
import { useState } from "react";
import { Check, CreditCard, Lock } from "lucide-react";
import { Link } from "@/components/ui/link";
import { useStore } from "@/components/cart/store";
import { OrderSummary } from "@/components/cart/order-summary";
import { AddressForm } from "@/components/account/address-form";
import type { ApiAddress } from "@/lib/api/types";
import { Photo } from "@/components/ui/photo";
import { useCartLines } from "@/components/cart/use-cart-lines";
import { placeOrderAction } from "@/lib/orders/actions";
import { useDict, useLocale } from "@/lib/i18n/provider";
import { cn, formatPrice } from "@/lib/utils";

const shippingIds = ["standard", "store"] as const;
type ShippingId = (typeof shippingIds)[number];

/** Maps a card number to a payment provider token. The number itself never leaves the browser. */
function paymentToken(cardNumber: string): string {
  const digits = cardNumber.replace(/\D/g, "");
  if (digits === "4000000000000002") return "tok_declined";
  if (digits === "4000000000009995") return "tok_insufficient_funds";
  if (digits === "4000000000000069") return "tok_expired";
  return "tok_visa";
}

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

export function CheckoutForm({ user, addresses }: { user: { firstName: string; lastName: string; email: string }; addresses: ApiAddress[] }) {
  const d = useDict();
  const locale = useLocale();
  const t = d.checkout;
  const router = useRouter();
  const { items, hydrated } = useStore();
  const { lines, ready } = useCartLines();
  const [error, setError] = useState<string | null>(null);
  const [needsLogin, setNeedsLogin] = useState(false);
  const [shipping, setShipping] = useState<ShippingId>("standard");
  const [picked, setPicked] = useState<string | null>(null);
  const [adding, setAdding] = useState(false);
  // Falls back to the default address (listed first) until the customer picks another one.
  const addressId = addresses.find((a) => a.id === picked)?.id ?? addresses[0]?.id ?? null;
  const [giftNote, setGiftNote] = useState(false);
  const [busy, setBusy] = useState(false);

  if (hydrated && items.length === 0) {
    return (
      <div className="py-24 text-center">
        <p className="headline-lg">{t.emptyTitle}</p>
        <Link href="/collection" className="btn btn-primary mt-8">{d.cart.discoverCollection}</Link>
      </div>
    );
  }

  async function submit(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    if (!addressId) {
      setError(t.noAddress);
      return;
    }
    const cardNumber = String(new FormData(e.currentTarget).get("card_number") ?? "");
    setBusy(true);
    setError(null);
    const result = await placeOrderAction({
      locale,
      paymentMethod: paymentToken(cardNumber),
      addressId,
      items: items.map((i) => ({ variantId: i.variantId, quantity: i.qty })),
    });
    if (result.ok) {
      // The cart is emptied on the confirmation page once the payment went through, so a failed payment keeps it.
      router.push(`/${locale}/order-confirmation/${result.orderId}`);
      return;
    }
    setBusy(false);
    if (result.reason === "auth") setNeedsLogin(true);
    else setError(result.message ?? t.errors.generic);
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

        <Step n={2} title={t.address} done={Boolean(addressId)}>
          {addresses.length > 0 && (
            <div className="grid gap-3 sm:grid-cols-2">
              {addresses.map((a) => (
                <label key={a.id} className={cn("cursor-pointer border p-5 transition-colors", addressId === a.id ? "border-ink bg-paper" : "border-travertine hover:border-ink")}>
                  <input type="radio" name="address" className="sr-only" checked={addressId === a.id} onChange={() => setPicked(a.id)} />
                  <div className="flex items-center justify-between">
                    <span className="label-caps text-[10px]">{a.title}</span>
                    {addressId === a.id && <Check size={14} />}
                  </div>
                  <p className="mt-3 text-[13px]">{a.full_name}</p>
                  <p className="text-[13px] text-umber">{a.line1}{a.line2 ? `, ${a.line2}` : ""}</p>
                  <p className="text-[13px] text-umber">{a.district} / {a.city} {a.postal_code}</p>
                  <p className="mt-2 text-[12px] text-umber">{a.phone}</p>
                </label>
              ))}
            </div>
          )}
          {addresses.length === 0 && !adding && <p className="mb-4 text-[13px] text-umber">{t.noAddress}</p>}
          {adding || addresses.length === 0 ? (
            <div className="mt-4 border border-travertine p-5">
              <AddressForm
                offerDefault={addresses.length > 0}
                defaultName={`${user.firstName} ${user.lastName}`.trim()}
                onSaved={(id) => {
                  setPicked(id);
                  setAdding(false);
                  router.refresh();
                }}
                onCancel={addresses.length > 0 ? () => setAdding(false) : undefined}
              />
            </div>
          ) : (
            <button type="button" onClick={() => setAdding(true)} className="label-caps mt-4 text-[10px] underline underline-offset-4 hover:text-bronze-deep">{t.newAddress}</button>
          )}
        </Step>

        <Step n={3} title={t.delivery}>
          <div className="space-y-3">
            {shippingIds.map((id) => (
              <label key={id} className={cn("flex cursor-pointer items-center justify-between gap-4 border p-5 transition-colors", shipping === id ? "border-ink bg-paper" : "border-travertine hover:border-ink")}>
                <input type="radio" name="shipping" className="sr-only" checked={shipping === id} onChange={() => setShipping(id)} />
                <div>
                  <p className="text-[14px]">{t.shippingOptions[id].label}</p>
                  <p className="text-[12px] text-umber">{t.shippingOptions[id].note}</p>
                </div>
                <span className="label-mono shrink-0">{d.common.free}</span>
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
          <div className="grid gap-4 sm:grid-cols-2">
            <div className="sm:col-span-2">
              <label className="field-label" htmlFor="cc-num">{t.cardNumber}</label>
              <div className="relative">
                <input id="cc-num" name="card_number" required inputMode="numeric" autoComplete="cc-number" placeholder="4242 4242 4242 4242" className="field !pr-11" />
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
          </div>
          <p className="mt-4 flex items-start gap-2 text-[12px] text-umber"><Lock size={13} className="mt-0.5 shrink-0" /> {t.secureNote}</p>
          <p className="mt-2 text-[12px] text-umber">{t.testCardNote}</p>
        </Step>
      </div>

      <aside className="lg:col-span-5">
        <OrderSummary className="lg:sticky lg:top-24">
          <ul className="mt-6 space-y-4 border-t border-travertine pt-6">
            {lines.map(({ item: i, product: p, lineTotal }) => (
              <li key={i.variantId} className="flex gap-4">
                <div className="relative w-16 shrink-0">
                  <Photo src={p.images[0]} alt={p.name} sizes="64px" imgClassName="object-top" />
                  <span className="absolute -right-2 -top-2 flex h-5 w-5 items-center justify-center bg-ink text-[10px] text-canvas">{i.qty}</span>
                </div>
                <div className="flex-1">
                  <p className="font-serif text-[15px] leading-5">{p.name}</p>
                  <p className="text-[12px] text-umber">{d.colors[i.color] ?? i.color} · {d.sizes[i.size] ?? i.size}</p>
                </div>
                <p className="label-mono">{formatPrice(lineTotal, locale)}</p>
              </li>
            ))}
          </ul>
          {error && <p role="alert" className="mt-6 border border-danger/40 bg-danger/5 p-3 text-[13px] text-danger">{error}</p>}
          {needsLogin && (
            <p role="alert" className="mt-6 border border-travertine p-3 text-[13px]">
              {t.loginRequired}{" "}
              <Link href="/login?next=/checkout" className="underline underline-offset-4">{t.signIn}</Link>
            </p>
          )}
          <button disabled={busy || !ready || lines.length === 0} className="btn btn-primary mt-8 w-full">{busy ? t.confirming : t.confirm}</button>
          <p className="mt-4 text-center text-[11px] leading-5 text-umber">{t.terms}</p>
        </OrderSummary>
      </aside>
    </form>
  );
}
