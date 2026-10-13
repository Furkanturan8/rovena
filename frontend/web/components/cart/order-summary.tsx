"use client";

import { useState } from "react";
import { Tag } from "lucide-react";
import { useStore } from "./store";
import { FREE_SHIPPING_THRESHOLD } from "./cart-drawer";
import { useDict, useLocale } from "@/lib/i18n/provider";
import { cn, formatPrice } from "@/lib/utils";

export function useTotals(shippingOverride?: number, promoApplied = false) {
  const { subtotal } = useStore();
  const discount = promoApplied ? Math.round(subtotal * 0.05) : 0;
  const baseShipping = subtotal >= FREE_SHIPPING_THRESHOLD || subtotal === 0 ? 0 : 450;
  const shipping = shippingOverride ?? baseShipping;
  return { subtotal, discount, shipping, total: subtotal - discount + shipping };
}

interface Props {
  shipping?: number;
  children?: React.ReactNode;
  className?: string;
  promo?: boolean;
  onPromo?: (applied: boolean) => void;
  promoApplied?: boolean;
}

export function OrderSummary({ shipping, children, className, promo, onPromo, promoApplied }: Props) {
  const d = useDict();
  const locale = useLocale();
  const s = d.summary;
  const totals = useTotals(shipping, promoApplied);
  const [code, setCode] = useState("");
  const [msg, setMsg] = useState<string | null>(null);
  const money = (v: number) => formatPrice(v, locale);

  return (
    <div className={cn("border border-travertine bg-paper p-6 md:p-8", className)}>
      <h2 className="headline-sm">{s.title}</h2>
      {promo && (
        <form
          className="mt-6"
          onSubmit={(e) => {
            e.preventDefault();
            const ok = code.trim().toUpperCase() === "ROVENA5";
            onPromo?.(ok);
            setMsg(ok ? s.promoApplied : s.promoInvalid);
          }}
        >
          <label className="field-label" htmlFor="promo">{s.promoLabel}</label>
          <div className="flex gap-2">
            <div className="relative flex-1">
              <Tag size={14} className="absolute left-3.5 top-1/2 -translate-y-1/2 text-umber" />
              <input id="promo" value={code} onChange={(e) => setCode(e.target.value)} className="field !pl-10" placeholder={s.promoPlaceholder} />
            </div>
            <button className="btn btn-secondary">{d.common.apply}</button>
          </div>
          {msg && <p className={cn("mt-2 text-[12px]", promoApplied ? "text-success" : "text-danger")}>{msg}</p>}
        </form>
      )}
      <dl className="mt-6 space-y-3 text-[14px]">
        <div className="flex justify-between"><dt className="text-umber">{s.subtotal}</dt><dd>{money(totals.subtotal)}</dd></div>
        {totals.discount > 0 && (
          <div className="flex justify-between text-success"><dt>{s.discount}</dt><dd>−{money(totals.discount)}</dd></div>
        )}
        <div className="flex justify-between">
          <dt className="text-umber">{s.shipping}</dt>
          <dd>{totals.shipping === 0 ? d.common.free : money(totals.shipping)}</dd>
        </div>
        <div className="flex items-baseline justify-between border-t border-travertine pt-4">
          <dt className="label-caps">{s.total}</dt>
          <dd className="font-serif text-[26px] leading-none">{money(totals.total)}</dd>
        </div>
        <p className="text-[11px] text-umber">{s.note}</p>
      </dl>
      {children}
    </div>
  );
}
