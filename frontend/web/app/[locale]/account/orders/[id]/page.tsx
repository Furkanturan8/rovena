import type { Metadata } from "next";
import { notFound } from "next/navigation";
import { Check, ChevronLeft } from "lucide-react";
import { Link } from "@/components/ui/link";
import { StatusBadge } from "@/components/account/status-badge";
import { displayStatus } from "@/lib/order-status";
import { CancelOrderButton } from "@/components/account/cancel-order-button";
import { Photo } from "@/components/ui/photo";
import { getI18n } from "@/lib/i18n/server";
import { requireUser } from "@/lib/auth/session";
import { getMyOrder } from "@/lib/data/orders";
import { productsForOrders } from "@/lib/data/order-products";
import { formatPrice } from "@/lib/utils";

export async function generateMetadata({ params }: PageProps<"/[locale]/account/orders/[id]">): Promise<Metadata> {
  const { d } = await getI18n(params);
  return { title: d.account.orders.detailTitle };
}

export default async function OrderDetailPage({ params }: PageProps<"/[locale]/account/orders/[id]">) {
  const { locale, d } = await getI18n(params);
  const { id } = await params;
  const { token } = await requireUser(locale, `/${locale}/account/orders/${id}`);
  const order = await getMyOrder(token, id);
  if (!order) notFound();

  const t = d.account.orders;
  const products = await productsForOrders([order], locale);
  const money = (cents: number) => formatPrice(cents / 100, locale);
  const intl = locale === "tr" ? "tr-TR" : "en-GB";
  const dateTime = new Intl.DateTimeFormat(intl, { day: "numeric", month: "long", year: "numeric", hour: "2-digit", minute: "2-digit" });
  const reached = new Set((order.history ?? []).map((h) => h.status));
  const cancelled = order.status === "cancelled";
  const canCancel = order.status === "pending" || order.status === "awaiting_payment";

  return (
    <div>
      <Link href="/account/orders" className="label-caps mb-6 inline-flex items-center gap-2 text-[10px] text-umber hover:text-ink">
        <ChevronLeft size={13} /> {t.back}
      </Link>
      <div className="flex flex-wrap items-end justify-between gap-4">
        <div>
          <h2 className="headline-md">{t.order(`#${order.id.slice(-8).toUpperCase()}`)}</h2>
          <p className="mt-1 text-[13px] text-umber">{dateTime.format(new Date(order.created_at))}</p>
        </div>
        <StatusBadge status={displayStatus(order.status, order.refund)} />
      </div>

      <section className="mt-10 border border-travertine bg-paper p-6 md:p-8">
        {cancelled ? (
          <p className="text-[14px] text-umber">
            {t.cancelled} {order.refund === "pending" ? t.refundPending : order.refund === "refunded" ? t.refunded : ""}
          </p>
        ) : (
          <ol className="grid gap-6 md:grid-cols-3 lg:grid-cols-6">
            {d.confirmation.steps.map((s, i) => {
              const done = reached.has(s.status);
              return (
                <li key={s.status}>
                  <div className="flex items-center gap-3 md:block">
                    <span className={`flex h-8 w-8 items-center justify-center border text-[12px] ${done ? "border-ink bg-ink text-canvas" : "border-travertine text-umber"}`}>
                      {done ? <Check size={14} /> : i + 1}
                    </span>
                    <div className="md:mt-4">
                      <p className={`text-[14px] ${done ? "" : "text-umber"}`}>{s.label}</p>
                      <p className="text-[12px] text-umber">{s.note}</p>
                    </div>
                  </div>
                </li>
              );
            })}
          </ol>
        )}
      </section>

      {(order.shipping_address || order.tracking_code) && (
        <div className="mt-10 grid gap-8 md:grid-cols-2">
          {order.shipping_address && (
            <section>
              <h3 className="headline-sm mb-4">{t.shipTo}</h3>
              <address className="space-y-0.5 border border-travertine p-5 text-[13px] not-italic">
                <p>{order.shipping_address.full_name}</p>
                <p className="text-umber">{order.shipping_address.line1}{order.shipping_address.line2 ? `, ${order.shipping_address.line2}` : ""}</p>
                <p className="text-umber">{order.shipping_address.district} / {order.shipping_address.city} {order.shipping_address.postal_code}</p>
                <p className="text-umber">{order.shipping_address.phone}</p>
              </address>
            </section>
          )}
          {order.tracking_code && (
            <section>
              <h3 className="headline-sm mb-4">{t.shipment}</h3>
              <dl className="space-y-2 border border-travertine p-5 text-[13px]">
                <div className="flex justify-between"><dt className="text-umber">{t.carrier}</dt><dd>{order.carrier}</dd></div>
                <div className="flex justify-between"><dt className="text-umber">{t.trackingCode}</dt><dd className="label-mono">{order.tracking_code}</dd></div>
                {order.shipped_at && <div className="flex justify-between"><dt className="text-umber">{t.shippedOn}</dt><dd>{dateTime.format(new Date(order.shipped_at))}</dd></div>}
                {order.delivered_at && <div className="flex justify-between"><dt className="text-umber">{t.deliveredOn}</dt><dd>{dateTime.format(new Date(order.delivered_at))}</dd></div>}
              </dl>
            </section>
          )}
        </div>
      )}

      <section className="mt-10">
        <h3 className="headline-sm mb-4">{t.products}</h3>
        <ul className="divide-y divide-travertine border-y border-travertine">
          {order.items.map((it) => {
            const p = products.get(it.product_id);
            return (
              <li key={it.variant_id} className="flex gap-5 py-5">
                <div className="w-24 shrink-0">
                  {p && (
                    <Link href={`/product/${p.slug}`}>
                      <Photo src={p.images[0]} alt={p.name} sizes="96px" imgClassName="object-top" />
                    </Link>
                  )}
                </div>
                <div className="flex-1">
                  {p && <p className="label-caps text-[10px] text-umber">{p.kind}</p>}
                  <p className="font-serif text-[18px]">{p?.name ?? it.product_name}</p>
                  <p className="mt-1 text-[13px] text-umber">{d.colors[it.color] ?? it.color} · {d.cart.sizeLabel(d.sizes[it.size] ?? it.size)} · x{it.quantity}</p>
                </div>
                <p className="label-mono">{money(it.line_total_cents)}</p>
              </li>
            );
          })}
        </ul>
      </section>

      <div className="mt-10 grid gap-8 md:grid-cols-2">
        <section>
          <h3 className="headline-sm mb-4">{t.historyTitle}</h3>
          <ul className="space-y-2 text-[13px]">
            {(order.history ?? []).map((h) => (
              <li key={`${h.status}-${h.at}`} className="flex justify-between gap-4 border-b border-travertine pb-2">
                <span>{d.status[h.status]}</span>
                <span className="text-umber">{dateTime.format(new Date(h.at))}</span>
              </li>
            ))}
          </ul>
        </section>
        <section>
          <h3 className="headline-sm mb-4">{t.summary}</h3>
          <dl className="space-y-2 text-[13px]">
            <div className="flex justify-between"><dt className="text-umber">{t.subtotal}</dt><dd>{money(order.total_cents)}</dd></div>
            <div className="flex justify-between"><dt className="text-umber">{t.delivery}</dt><dd>{d.common.free}</dd></div>
            <div className="flex justify-between border-t border-travertine pt-3 text-[15px]"><dt>{t.total}</dt><dd className="font-serif text-[20px]">{money(order.total_cents)}</dd></div>
          </dl>
        </section>
      </div>

      {canCancel && <CancelOrderButton orderId={order.id} />}
    </div>
  );
}
