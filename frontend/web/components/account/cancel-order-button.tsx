"use client";

import { useState, useTransition } from "react";
import { useRouter } from "next/navigation";
import { cancelOrderAction } from "@/lib/orders/actions";
import { useDict, useLocale } from "@/lib/i18n/provider";

export function CancelOrderButton({ orderId }: { orderId: string }) {
  const d = useDict();
  const locale = useLocale();
  const t = d.account.orders;
  const router = useRouter();
  const [pending, start] = useTransition();
  const [error, setError] = useState(false);

  return (
    <div className="mt-10">
      <button
        disabled={pending}
        onClick={() =>
          start(async () => {
            const res = await cancelOrderAction(orderId, locale);
            if (res.ok) router.refresh();
            else setError(true);
          })
        }
        className="btn btn-secondary"
      >
        {pending ? t.cancelling : t.cancel}
      </button>
      {error && <p role="alert" className="mt-3 text-[13px] text-danger">{t.cancelError}</p>}
    </div>
  );
}
