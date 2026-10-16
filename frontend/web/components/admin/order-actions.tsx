"use client";

import { useActionState } from "react";
import { deliverOrderAction, shipOrderAction, type AdminOrderActionState } from "@/lib/admin/order-actions";
import type { ApiOrderStatus } from "@/lib/api/types";
import { useDict, useLocale } from "@/lib/i18n/provider";

const idle: AdminOrderActionState = { status: "idle" };

function Feedback({ state, okText }: { state: AdminOrderActionState; okText: string }) {
  if (state.status === "error") return <p role="alert" className="mt-3 text-[13px] text-red-700">{state.message}</p>;
  if (state.status === "ok") return <p role="status" className="mt-3 text-[13px] text-bronze-deep">{okText}</p>;
  return null;
}

export function AdminOrderActions({ orderId, status }: { orderId: string; status: ApiOrderStatus }) {
  const a = useDict().admin.orders.actions;
  const locale = useLocale();
  const [shipState, ship, shipPending] = useActionState(shipOrderAction, idle);
  const [deliverState, deliver, deliverPending] = useActionState(deliverOrderAction, idle);

  return (
    <section aria-labelledby="order-actions" className="border border-travertine bg-paper p-6">
      <h2 id="order-actions" className="label-caps mb-4 text-[10px] text-umber">{a.title}</h2>

      {status === "confirmed" ? (
        <form action={ship} className="space-y-4">
          <input type="hidden" name="locale" value={locale} />
          <input type="hidden" name="orderId" value={orderId} />
          <p className="text-umber">{a.shipHint}</p>
          <div className="grid gap-4 sm:grid-cols-2">
            <div>
              <label className="field-label" htmlFor="carrier">{a.carrier}</label>
              <input id="carrier" name="carrier" required maxLength={80} className="field" />
            </div>
            <div>
              <label className="field-label" htmlFor="trackingCode">{a.trackingCode}</label>
              <input id="trackingCode" name="trackingCode" required maxLength={80} className="field" />
            </div>
          </div>
          <button type="submit" disabled={shipPending} className="btn btn-primary">{shipPending ? a.shipping : a.ship}</button>
          <Feedback state={shipState} okText={a.shipped} />
        </form>
      ) : status === "shipped" ? (
        <form action={deliver}>
          <input type="hidden" name="locale" value={locale} />
          <input type="hidden" name="orderId" value={orderId} />
          <p className="mb-4 text-umber">{a.deliverHint}</p>
          <button type="submit" disabled={deliverPending} className="btn btn-primary">{deliverPending ? a.delivering : a.deliver}</button>
          <Feedback state={deliverState} okText={a.delivered} />
        </form>
      ) : (
        <p className="text-umber">{a.none}</p>
      )}
    </section>
  );
}
