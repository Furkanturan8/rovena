import type { Metadata } from "next";
import { Link } from "@/components/ui/link";
import { orders } from "@/lib/mock/account";
import { getProductById } from "@/lib/mock/catalog";
import { StatusBadge } from "@/components/account/status-badge";
import { Photo } from "@/components/ui/photo";
import { getI18n } from "@/lib/i18n/server";
import { formatPrice } from "@/lib/utils";

export async function generateMetadata({ params }: PageProps<"/[locale]/account/orders">): Promise<Metadata> {
  const { d } = await getI18n(params);
  return { title: d.account.orders.pageTitle };
}

export default async function OrdersPage({ params }: PageProps<"/[locale]/account/orders">) {
  const { locale, d } = await getI18n(params);
  const dateFmt = new Intl.DateTimeFormat(locale === "tr" ? "tr-TR" : "en-GB", { day: "numeric", month: "long", year: "numeric" });
  return (
    <div>
      <h2 className="headline-md mb-8">{d.account.orders.pageTitle}</h2>
      <ul className="space-y-6">
        {orders.map((o) => (
          <li key={o.id} className="border border-travertine bg-paper">
            <div className="flex flex-wrap items-center justify-between gap-4 border-b border-travertine px-6 py-4">
              <div>
                <p className="label-mono">{o.number}</p>
                <p className="text-[12px] text-umber">{dateFmt.format(new Date(o.placedAt))}</p>
              </div>
              <StatusBadge status={o.status} />
            </div>
            <div className="flex flex-col gap-6 p-6 sm:flex-row sm:items-center">
              <div className="flex gap-3">
                {o.lines.map((l) => {
                  const p = getProductById(l.productId, locale)!;
                  return <Photo key={l.productId + l.size} src={p.images[0]} alt={p.name} className="w-20" sizes="80px" imgClassName="object-top" />;
                })}
              </div>
              <div className="flex-1 text-[13px]">
                {o.lines.map((l) => (
                  <p key={l.productId + l.size}>{getProductById(l.productId, locale)?.name} <span className="text-umber">· {d.sizes[l.size] ?? l.size} · x{l.qty}</span></p>
                ))}
              </div>
              <div className="flex items-center gap-6 sm:flex-col sm:items-end sm:gap-3">
                <p className="font-serif text-[20px]">{formatPrice(o.total, locale)}</p>
                <Link href={`/account/orders/${o.id}`} className="link-underline">{d.common.detail}</Link>
              </div>
            </div>
          </li>
        ))}
      </ul>
    </div>
  );
}
