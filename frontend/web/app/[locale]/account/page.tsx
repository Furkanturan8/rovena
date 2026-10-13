import type { Metadata } from "next";
import { ArrowRight } from "lucide-react";
import { Link } from "@/components/ui/link";
import { addresses, orders, user } from "@/lib/mock/account";
import { getProductById } from "@/lib/mock/catalog";
import { StatusBadge } from "@/components/account/status-badge";
import { Photo } from "@/components/ui/photo";
import { getI18n } from "@/lib/i18n/server";
import { formatPrice } from "@/lib/utils";

export async function generateMetadata({ params }: PageProps<"/[locale]/account">): Promise<Metadata> {
  const { d } = await getI18n(params);
  return { title: d.account.layoutTitle };
}

export default async function AccountOverview({ params }: PageProps<"/[locale]/account">) {
  const { locale, d } = await getI18n(params);
  const o = d.account.overview;
  const active = orders.find((x) => x.status === "preparing" || x.status === "shipped");
  const dateFmt = new Intl.DateTimeFormat(locale === "tr" ? "tr-TR" : "en-GB", { day: "numeric", month: "long", year: "numeric" });
  const spent = orders.filter((x) => x.status === "delivered" || x.status === "preparing").reduce((n, x) => n + x.total, 0);

  return (
    <div className="space-y-12">
      <div className="grid gap-px border border-travertine bg-travertine sm:grid-cols-3">
        {[
          [String(orders.length), o.totalOrders],
          [formatPrice(spent, locale), o.totalSpent],
          [String(addresses.length), o.addresses],
        ].map(([v, l]) => (
          <div key={l} className="bg-canvas p-6">
            <p className="headline-md">{v}</p>
            <p className="label-caps mt-1 text-[10px] text-umber">{l}</p>
          </div>
        ))}
      </div>

      {active && (
        <section>
          <div className="mb-5 flex items-end justify-between">
            <h2 className="headline-md">{o.activeOrder}</h2>
            <Link href={`/account/orders/${active.id}`} className="link-underline">{o.track}</Link>
          </div>
          <div className="flex flex-col gap-6 border border-travertine bg-paper p-6 sm:flex-row sm:items-center">
            <div className="flex gap-3">
              {active.lines.slice(0, 3).map((l) => {
                const p = getProductById(l.productId, locale)!;
                return <Photo key={l.productId} src={p.images[0]} alt={p.name} className="w-20" sizes="80px" imgClassName="object-top" />;
              })}
            </div>
            <div className="flex-1">
              <p className="label-mono">{active.number}</p>
              <p className="mt-1 font-serif text-[18px]">{getProductById(active.lines[0].productId, locale)?.name}</p>
              {active.eta && <p className="mt-2 text-[13px] text-umber">{o.eta(dateFmt.format(new Date(active.eta)))}</p>}
            </div>
            <StatusBadge status={active.status} />
          </div>
        </section>
      )}

      <section>
        <div className="mb-5 flex items-end justify-between">
          <h2 className="headline-md">{o.info}</h2>
          <Link href="/account/profile" className="link-underline">{d.common.edit}</Link>
        </div>
        <dl className="grid gap-px border border-travertine bg-travertine sm:grid-cols-2">
          {[
            [o.fullName, `${user.firstName} ${user.lastName}`],
            [o.email, user.email],
            [o.phone, user.phone],
            [o.membership, d.account.tier],
          ].map(([k, v]) => (
            <div key={k} className="bg-canvas p-5">
              <dt className="label-caps text-[10px] text-umber">{k}</dt>
              <dd className="mt-1 text-[14px]">{v}</dd>
            </div>
          ))}
        </dl>
      </section>

      <section className="flex flex-col items-start justify-between gap-6 bg-ink p-8 text-canvas md:flex-row md:items-center">
        <div>
          <p className="label-caps mb-2 text-bronze">{o.ctaEyebrow}</p>
          <p className="headline-md">{o.ctaTitle}</p>
        </div>
        <Link href="/appointment" className="btn btn-light shrink-0">{o.ctaButton} <ArrowRight size={14} /></Link>
      </section>
    </div>
  );
}
