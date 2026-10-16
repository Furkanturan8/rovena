import { StatusBadge } from "@/components/account/status-badge";
import { AdminOrderActions } from "@/components/admin/order-actions";
import { Link } from "@/components/ui/link";
import { requireAdmin } from "@/lib/auth/session";
import { getAdminOrder } from "@/lib/data/admin-orders";
import { getI18n } from "@/lib/i18n/server";
import { displayStatus } from "@/lib/order-status";
import { formatPrice } from "@/lib/utils";

export default async function AdminOrderDetailPage({ params }: { params: Promise<{ locale: string; id: string }> }) {
  const { locale, d } = await getI18n(params);
  const { id } = await params;
  const { token } = await requireAdmin(locale, `/${locale}/admin/orders/${id}`);
  const o = d.admin.orders;
  const t = o.detail;

  // Other failures bubble up to the route's error boundary.
  const order = await getAdminOrder(token, id);
  if (!order) {
    return (
      <div className="space-y-6">
        <Link href="/admin/orders" className="label-caps text-[10px] text-bronze-deep">← {o.back}</Link>
        <div className="border border-travertine bg-paper p-8">
          <h2 className="headline-md">{o.notFoundTitle}</h2>
          <p className="mt-3 text-umber">{o.notFoundText}</p>
        </div>
      </div>
    );
  }

  const intl = locale === "tr" ? "tr-TR" : "en-GB";
  const dateTime = new Intl.DateTimeFormat(intl, { day: "numeric", month: "long", year: "numeric", hour: "2-digit", minute: "2-digit" });
  const fmt = (iso?: string) => (iso ? dateTime.format(new Date(iso)) : "—");
  const money = (cents: number) => formatPrice(cents / 100, locale);
  const addr = order.shipping_address;

  return (
    <div className="space-y-10">
      <Link href="/admin/orders" className="label-caps text-[10px] text-bronze-deep">← {o.back}</Link>

      <header className="flex flex-wrap items-start justify-between gap-4">
        <div>
          <h2 className="headline-md break-all">{order.id}</h2>
          <p className="mt-2 text-umber">{t.placed}: {fmt(order.created_at)} · {t.updated}: {fmt(order.updated_at)}</p>
          <p className="mt-1 break-all text-[12px] text-umber">{t.customerId}: {order.customer_id}</p>
        </div>
        <StatusBadge status={displayStatus(order.status, order.refund)} />
      </header>

      <AdminOrderActions orderId={order.id} status={order.status} />

      <section aria-labelledby="order-items">
        <h3 id="order-items" className="label-caps mb-4 text-[10px] text-umber">{t.items}</h3>
        <div className="overflow-x-auto border border-travertine">
          <table className="w-full min-w-[560px] text-left text-[14px]">
            <thead className="bg-paper">
              <tr className="label-caps text-[10px] text-umber">
                <th scope="col" className="px-4 py-3">{t.items}</th>
                <th scope="col" className="px-4 py-3">{t.quantity}</th>
                <th scope="col" className="px-4 py-3">{t.unitPrice}</th>
                <th scope="col" className="px-4 py-3">{t.lineTotal}</th>
              </tr>
            </thead>
            <tbody>
              {order.items.map((i) => (
                <tr key={i.variant_id} className="border-t border-travertine">
                  <td className="px-4 py-3">
                    {i.product_name}
                    <span className="block text-[12px] text-umber">{[i.sku, i.size, i.color].filter(Boolean).join(" · ")}</span>
                  </td>
                  <td className="px-4 py-3">{i.quantity}</td>
                  <td className="px-4 py-3">{money(i.unit_price_cents)}</td>
                  <td className="px-4 py-3">{money(i.line_total_cents)}</td>
                </tr>
              ))}
            </tbody>
            <tfoot>
              <tr className="border-t border-travertine bg-paper">
                <th scope="row" colSpan={3} className="px-4 py-3 text-right">{t.total}</th>
                <td className="px-4 py-3">{money(order.total_cents)}</td>
              </tr>
            </tfoot>
          </table>
        </div>
      </section>

      <div className="grid gap-6 md:grid-cols-2">
        <section aria-labelledby="order-address" className="border border-travertine p-6">
          <h3 id="order-address" className="label-caps mb-3 text-[10px] text-umber">{t.address}</h3>
          {addr ? (
            <address className="not-italic leading-relaxed">
              {addr.full_name}<br />
              {addr.phone}<br />
              {addr.line1}{addr.line2 ? `, ${addr.line2}` : ""}<br />
              {[addr.postal_code, addr.district, addr.city].filter(Boolean).join(" ")}
              {addr.country ? <><br />{addr.country}</> : null}
            </address>
          ) : (
            <p className="text-umber">{t.noAddress}</p>
          )}
        </section>

        <section aria-labelledby="order-shipment" className="border border-travertine p-6">
          <h3 id="order-shipment" className="label-caps mb-3 text-[10px] text-umber">{t.shipment}</h3>
          {order.carrier || order.tracking_code ? (
            <dl className="space-y-1">
              <div><dt className="inline text-umber">{t.carrier}: </dt><dd className="inline">{order.carrier ?? "—"}</dd></div>
              <div><dt className="inline text-umber">{t.trackingCode}: </dt><dd className="inline break-all">{order.tracking_code ?? "—"}</dd></div>
              <div><dt className="inline text-umber">{t.shippedAt}: </dt><dd className="inline">{fmt(order.shipped_at)}</dd></div>
              <div><dt className="inline text-umber">{t.deliveredAt}: </dt><dd className="inline">{fmt(order.delivered_at)}</dd></div>
            </dl>
          ) : (
            <p className="text-umber">{t.notShipped}</p>
          )}
        </section>
      </div>

      <section aria-labelledby="order-saga" className="border border-travertine p-6">
        <h3 id="order-saga" className="label-caps mb-3 text-[10px] text-umber">{t.saga}</h3>
        <dl className="grid gap-2 sm:grid-cols-2">
          <div><dt className="inline text-umber">{t.reservation}: </dt><dd className="inline">{t.reservationStates[order.reservation_state]}</dd></div>
          <div><dt className="inline text-umber">{t.refund}: </dt><dd className="inline">{t.refundStates[order.refund_state]}</dd></div>
          <div><dt className="inline text-umber">{t.paymentRequests}: </dt><dd className="inline">{order.payment_requests}</dd></div>
          {order.failure_code || order.failure_message ? (
            <div><dt className="inline text-umber">{t.failure}: </dt><dd className="inline">{[order.failure_code, order.failure_message].filter(Boolean).join(" — ")}</dd></div>
          ) : null}
        </dl>
      </section>

      <section aria-labelledby="order-history">
        <h3 id="order-history" className="label-caps mb-3 text-[10px] text-umber">{t.history}</h3>
        <ul className="space-y-2">
          {order.history.map((h) => (
            <li key={`${h.to}-${h.at}`} className="flex flex-wrap justify-between gap-4 border-b border-travertine pb-2">
              <span>
                {h.from ? `${h.from} → ` : ""}{h.to}
                {h.reason ? <span className="text-umber"> ({h.reason})</span> : null}
              </span>
              <span className="text-umber">{fmt(h.at)}</span>
            </li>
          ))}
        </ul>
      </section>
    </div>
  );
}
