"use client";

import { useEffect, useState } from "react";
import { Check, Loader2, X } from "lucide-react";
import { Link } from "@/components/ui/link";
import type { ApiOrder, ApiOrderStatus } from "@/lib/api/types";
import { useStore } from "@/components/cart/store";
import { useDict } from "@/lib/i18n/provider";
import { cn } from "@/lib/utils";

const FINAL: ApiOrderStatus[] = ["confirmed", "shipped", "delivered", "cancelled"];
const ORDER: ApiOrderStatus[] = ["pending", "awaiting_payment", "paid", "confirmed"];

/** Follows the checkout saga: polls the order until it is confirmed or cancelled. */
export function OrderProgress({ orderId, initial }: { orderId: string; initial: ApiOrder }) {
  const d = useDict();
  const c = d.confirmation;
  const [order, setOrder] = useState(initial);
  const { clear, hydrated } = useStore();

  // Only an order that got paid empties the cart; a cancelled one leaves it as it was.
  const paid = order.status !== "pending" && order.status !== "awaiting_payment" && order.status !== "cancelled";
  useEffect(() => {
    if (paid && hydrated) clear();
  }, [paid, hydrated, clear]);

  useEffect(() => {
    if (FINAL.includes(order.status)) return;
    let cancelled = false;
    const timer = setInterval(async () => {
      try {
        const res = await fetch(`/api/orders/${orderId}`, { cache: "no-store" });
        if (res.ok && !cancelled) setOrder((await res.json()) as ApiOrder);
      } catch {
        // keep the last known state; the next tick retries
      }
    }, 2000);
    return () => {
      cancelled = true;
      clearInterval(timer);
    };
  }, [orderId, order.status]);

  const orderNo = `#${order.id.slice(-8).toUpperCase()}`;

  if (order.status === "cancelled") {
    return (
      <>
        <span className="mx-auto flex h-14 w-14 items-center justify-center border border-danger text-danger"><X size={22} strokeWidth={1.4} /></span>
        <h1 className="headline-xl mt-8">{c.failedTitle}</h1>
        <p className="label-mono mt-8 inline-block border border-travertine bg-paper px-5 py-3">{c.orderNo(orderNo)}</p>
        <div className="mx-auto mt-10 max-w-md border border-danger/40 bg-danger/5 p-6 text-left" role="alert">
          <p className="text-[13px] leading-6 text-umber">{c.failedText}</p>
          <Link href="/cart" className="btn btn-primary mt-6">{c.tryAgain}</Link>
        </div>
      </>
    );
  }

  // Shipping steps come after this page's job is done: a shipped or delivered order has passed them all.
  const reachedIndex = ORDER.includes(order.status) ? ORDER.indexOf(order.status) : ORDER.length - 1;
  return (
    <>
    <span className="mx-auto flex h-14 w-14 items-center justify-center border border-ink"><Check size={22} strokeWidth={1.4} /></span>
    <p className="label-caps mt-8 text-umber">{c.eyebrow}</p>
    <h1 className="headline-xl mt-3">{c.title}</h1>
    <p className="mt-5 text-[15px] leading-7 text-umber">{c.text}</p>
    <p className="label-mono mt-8 inline-block border border-travertine bg-paper px-5 py-3">{c.orderNo(orderNo)}</p>
    <ol className="mx-auto mt-12 max-w-md space-y-0 text-left">
      {c.steps.slice(0, ORDER.length).map((s, i) => {
        const done = i <= reachedIndex;
        const current = i === reachedIndex + 1;
        return (
          <li key={s.status} className="relative flex gap-5 pb-8 last:pb-0">
            {i < ORDER.length - 1 && <span aria-hidden className="absolute left-[15px] top-8 h-[calc(100%-2rem)] w-px bg-travertine" />}
            <span className={cn("flex h-8 w-8 shrink-0 items-center justify-center border", done ? "border-ink bg-ink text-canvas" : "border-travertine bg-canvas")}>
              {done ? <Check size={14} /> : current ? <Loader2 size={14} className="animate-spin text-umber" /> : null}
            </span>
            <div className={cn(!done && !current && "opacity-50")}>
              <p className="text-[14px] font-medium">{s.label}</p>
              <p className="text-[12px] text-umber">{s.note}</p>
            </div>
          </li>
        );
      })}
    </ol>
  </>
  );
}
