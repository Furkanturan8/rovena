"use client";

import { useActionState } from "react";
import { adjustStockAction, receiveStockAction, type AdminStockActionState } from "@/lib/admin/stock-actions";
import { useDict, useLocale } from "@/lib/i18n/provider";

const idle: AdminStockActionState = { status: "idle" };

function Feedback({ state, okText }: { state: AdminStockActionState; okText: string }) {
  if (state.status === "error") return <p role="alert" className="mt-3 text-[13px] text-red-700">{state.message}</p>;
  if (state.status === "ok") return <p role="status" className="mt-3 text-[13px] text-bronze-deep">{okText}</p>;
  return null;
}

export function StockForms({ variantId }: { variantId: string }) {
  const f = useDict().admin.stock.forms;
  const locale = useLocale();
  const [receiveState, receive, receivePending] = useActionState(receiveStockAction, idle);
  const [adjustState, adjust, adjustPending] = useActionState(adjustStockAction, idle);

  return (
    <div className="grid gap-6 md:grid-cols-2">
      <form action={receive} className="space-y-4 border border-travertine bg-paper p-6">
        <input type="hidden" name="locale" value={locale} />
        <input type="hidden" name="variantId" value={variantId} />
        <h3 className="label-caps text-[10px] text-umber">{f.receiveTitle}</h3>
        <p className="text-umber">{f.receiveHint}</p>
        <div>
          <label className="field-label" htmlFor="receive-quantity">{f.quantity}</label>
          <input id="receive-quantity" name="quantity" type="number" inputMode="numeric" min={1} step={1} required className="field" />
        </div>
        <div>
          <label className="field-label" htmlFor="receive-reason">{f.reason}</label>
          <input id="receive-reason" name="reason" maxLength={200} className="field" />
        </div>
        <button type="submit" disabled={receivePending} className="btn btn-primary">{receivePending ? f.receiving : f.receive}</button>
        <Feedback state={receiveState} okText={f.received} />
      </form>

      <form action={adjust} className="space-y-4 border border-travertine bg-paper p-6">
        <input type="hidden" name="locale" value={locale} />
        <input type="hidden" name="variantId" value={variantId} />
        <h3 className="label-caps text-[10px] text-umber">{f.adjustTitle}</h3>
        <p className="text-umber">{f.adjustHint}</p>
        <div>
          <label className="field-label" htmlFor="adjust-delta">{f.delta}</label>
          <input id="adjust-delta" name="delta" type="number" inputMode="numeric" step={1} required className="field" />
        </div>
        <div>
          <label className="field-label" htmlFor="adjust-reason">{f.adjustReason}</label>
          <input id="adjust-reason" name="reason" required maxLength={200} className="field" />
        </div>
        <button type="submit" disabled={adjustPending} className="btn btn-primary">{adjustPending ? f.adjusting : f.adjust}</button>
        <Feedback state={adjustState} okText={f.adjusted} />
      </form>
    </div>
  );
}
