import type { Metadata } from "next";
import { Link } from "@/components/ui/link";
import { StatusBadge } from "@/components/account/status-badge";
import { displayStatus } from "@/lib/order-status";
import { Photo } from "@/components/ui/photo";
import { getI18n } from "@/lib/i18n/server";
import { requireUser } from "@/lib/auth/session";
import { getMyOrders } from "@/lib/data/orders";
import { productsForOrders } from "@/lib/data/order-products";
import { formatPrice } from "@/lib/utils";

export async function generateMetadata({ params }: PageProps<"/[locale]/account/orders">): Promise<Metadata> {
  const { d } = await getI18n(params);
  return { title: d.account.orders.pageTitle };
}

export default async function OrdersPage({ params }: PageProps<"/[locale]/account/orders">) {
  const { locale, d } = await getI18n(params);
  const { token } = await requireUser(locale, `/${locale}/account/orders`);
  const t = d.account.orders;
  const orders = await getMyOrders(token);
  const products = await productsForOrders(orders, locale);
  const dateFmt = new Intl.DateTimeFormat(locale === "tr" ? "tr-TR" : "en-GB", { day: "numeric", month: "long", year: "numeric" });

  return (
    <div>
      <h2 className="headline-md mb-8">{t.pageTitle}</h2>
      {orders.length === 0 ? (
        <div className="border border-travertine py-20 text-center">
          <p className="headline-md">{t.empty}</p>
          <Link href="/collection" className="btn btn-primary mt-8">{t.emptyCta}</Link>
        </div>
      ) : (
        <ul className="space-y-6">
          {orders.map((o) => (
            <li key={o.id} className="border border-travertine bg-paper">
              <div className="flex flex-wrap items-center justify-between gap-4 border-b border-travertine px-6 py-4">
                <div>
                  <p className="label-mono">#{o.id.slice(-8).toUpperCase()}</p>
                  <p className="text-[12px] text-umber">{dateFmt.format(new Date(o.created_at))}</p>
                </div>
                <StatusBadge status={displayStatus(o.status, o.refund)} />
              </div>
              <div className="flex flex-col gap-6 p-6 sm:flex-row sm:items-center">
                <div className="flex gap-3">
                  {o.items.map((it) => {
                    const p = products.get(it.product_id);
                    return p ? <Photo key={it.variant_id} src={p.images[0]} alt={p.name} className="w-20" sizes="80px" imgClassName="object-top" /> : null;
                  })}
                </div>
                <div className="flex-1 text-[13px]">
                  {o.items.map((it) => (
                    <p key={it.variant_id}>{products.get(it.product_id)?.name ?? it.product_name} <span className="text-umber">· {d.sizes[it.size] ?? it.size} · x{it.quantity}</span></p>
                  ))}
                </div>
                <div className="flex items-center gap-6 sm:flex-col sm:items-end sm:gap-3">
                  <p className="font-serif text-[20px]">{formatPrice(o.total_cents / 100, locale)}</p>
                  <Link href={`/account/orders/${o.id}`} className="link-underline">{d.common.detail}</Link>
                </div>
              </div>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
