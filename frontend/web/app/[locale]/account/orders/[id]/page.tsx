import type { Metadata } from "next";
import { notFound } from "next/navigation";
import { Check, ChevronLeft } from "lucide-react";
import { Link } from "@/components/ui/link";
import { orders } from "@/lib/mock/account";
import { getProductById } from "@/lib/mock/catalog";
import { StatusBadge } from "@/components/account/status-badge";
import { Photo } from "@/components/ui/photo";
import { getI18n } from "@/lib/i18n/server";
import { locales } from "@/lib/i18n/config";
import { cn, formatPrice } from "@/lib/utils";

export function generateStaticParams() {
  return locales.flatMap((locale) => orders.map((o) => ({ locale, id: o.id })));
}

export async function generateMetadata({ params }: PageProps<"/[locale]/account/orders/[id]">): Promise<Metadata> {
  const { d } = await getI18n(params);
  return { title: d.account.orders.detailTitle };
}

export default async function OrderDetailPage({ params }: PageProps<"/[locale]/account/orders/[id]">) {
  const { locale, d } = await getI18n(params);
  const { id } = await params;
  const order = orders.find((o) => o.id === id);
  if (!order) notFound();

  const t = d.account.orders;
  const money = (v: number) => formatPrice(v, locale);
  const intl = locale === "tr" ? "tr-TR" : "en-GB";
  const dateTime = new Intl.DateTimeFormat(intl, { day: "numeric", month: "long", year: "numeric", hour: "2-digit", minute: "2-digit" });
  const dateOnly = new Intl.DateTimeFormat(intl, { day: "numeric", month: "long", year: "numeric" });
  const stepIndex = t.steps.findIndex((s) => s.key === order.status);
  const terminal = order.status === "cancelled" || order.status === "refunded";

  return (
    <div>
      <Link href="/account/orders" className="label-caps mb-6 inline-flex items-center gap-2 text-[10px] text-umber hover:text-ink">
        <ChevronLeft size={13} /> {t.back}
      </Link>
      <div className="flex flex-wrap items-end justify-between gap-4">
        <div>
          <h2 className="headline-md">{t.order(order.number)}</h2>
          <p className="mt-1 text-[13px] text-umber">{dateTime.format(new Date(order.placedAt))}</p>
        </div>
        <StatusBadge status={order.status} />
      </div>

      <section className="mt-10 border border-travertine bg-paper p-6 md:p-8">
        {terminal ? (
          <p className="text-[14px] text-umber">{order.status === "refunded" ? t.refunded(money(order.total)) : t.cancelled}</p>
        ) : (
          <ol className="grid gap-6 md:grid-cols-4">
            {t.steps.map((s, i) => {
              const done = i < stepIndex || (i === stepIndex && order.status === "delivered");
              const current = i === stepIndex && order.status !== "delivered";
              return (
                <li key={s.key} className="relative">
                  <div className="flex items-center gap-3 md:block">
                    <span className={cn("flex h-8 w-8 items-center justify-center border text-[12px]", done ? "border-ink bg-ink text-canvas" : current ? "border-ink" : "border-travertine text-umber")}>
                      {done ? <Check size={14} /> : i + 1}
                    </span>
                    <div className="md:mt-4">
                      <p className={cn("text-[14px]", !done && !current && "text-umber")}>{s.label}</p>
                      <p className="text-[12px] text-umber">{s.note}</p>
                    </div>
                  </div>
                </li>
              );
            })}
          </ol>
        )}
        {order.eta && <p className="mt-6 border-t border-travertine pt-4 text-[13px] text-umber">{t.eta} <span className="text-ink">{dateOnly.format(new Date(order.eta))}</span></p>}
        {order.trackingCode && <p className="mt-6 border-t border-travertine pt-4 text-[13px] text-umber">{t.tracking} <span className="label-mono text-ink">{order.trackingCode}</span></p>}
      </section>

      <section className="mt-10">
        <h3 className="headline-sm mb-4">{t.products}</h3>
        <ul className="divide-y divide-travertine border-y border-travertine">
          {order.lines.map((l) => {
            const p = getProductById(l.productId, locale)!;
            return (
              <li key={l.productId + l.size} className="flex gap-5 py-5">
                <Link href={`/product/${p.slug}`} className="w-24 shrink-0"><Photo src={p.images[0]} alt={p.name} sizes="96px" imgClassName="object-top" /></Link>
                <div className="flex-1">
                  <p className="label-caps text-[10px] text-umber">{p.kind}</p>
                  <p className="font-serif text-[18px]">{p.name}</p>
                  <p className="mt-1 text-[13px] text-umber">{d.colors[l.color] ?? l.color} · {d.cart.sizeLabel(d.sizes[l.size] ?? l.size)} · x{l.qty}</p>
                </div>
                <p className="label-mono">{money(l.unitPrice * l.qty)}</p>
              </li>
            );
          })}
        </ul>
      </section>

      <div className="mt-10 grid gap-8 md:grid-cols-2">
        <section>
          <h3 className="headline-sm mb-4">{t.deliveryPayment}</h3>
          <dl className="space-y-4 text-[13px]">
            <div><dt className="label-caps text-[10px] text-umber">{t.address}</dt><dd className="mt-1">{order.address}</dd></div>
            <div><dt className="label-caps text-[10px] text-umber">{t.payment}</dt><dd className="mt-1">{order.paymentLabel}</dd></div>
          </dl>
        </section>
        <section>
          <h3 className="headline-sm mb-4">{t.summary}</h3>
          <dl className="space-y-2 text-[13px]">
            <div className="flex justify-between"><dt className="text-umber">{t.subtotal}</dt><dd>{money(order.total - order.shipping)}</dd></div>
            <div className="flex justify-between"><dt className="text-umber">{t.delivery}</dt><dd>{order.shipping === 0 ? d.common.free : money(order.shipping)}</dd></div>
            <div className="flex justify-between border-t border-travertine pt-3 text-[15px]"><dt>{t.total}</dt><dd className="font-serif text-[20px]">{money(order.total)}</dd></div>
          </dl>
        </section>
      </div>

      {(order.status === "pending" || order.status === "confirmed" || order.status === "preparing") && (
        <div className="mt-10 flex flex-wrap gap-3">
          <button className="btn btn-secondary">{t.cancel}</button>
          <Link href="/appointment" className="btn btn-secondary">{t.contact}</Link>
        </div>
      )}
      {order.status === "delivered" && (
        <div className="mt-10 flex flex-wrap gap-3">
          <button className="btn btn-secondary">{t.returnRequest}</button>
          <button className="btn btn-secondary">{t.invoice}</button>
        </div>
      )}
    </div>
  );
}
