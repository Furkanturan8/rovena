import { StatusBadge } from "@/components/account/status-badge";
import { Link } from "@/components/ui/link";
import type { ApiOrderStatus } from "@/lib/api/types";
import { requireAdmin } from "@/lib/auth/session";
import { getAdminOrders } from "@/lib/data/admin-orders";
import { getI18n } from "@/lib/i18n/server";
import { displayStatus } from "@/lib/order-status";
import { cn, formatPrice } from "@/lib/utils";

const PAGE_SIZE = 20;
const STATUSES: ApiOrderStatus[] = ["pending", "awaiting_payment", "paid", "confirmed", "shipped", "delivered", "cancelled"];

type Props = {
  params: Promise<{ locale: string }>;
  searchParams: Promise<{ status?: string; page?: string }>;
};

function href(status: ApiOrderStatus | undefined, page?: number) {
  const qs = new URLSearchParams();
  if (status) qs.set("status", status);
  if (page && page > 1) qs.set("page", String(page));
  const s = qs.toString();
  return `/admin/orders${s ? `?${s}` : ""}`;
}

export default async function AdminOrdersPage({ params, searchParams }: Props) {
  const { locale, d } = await getI18n(params);
  const { token } = await requireAdmin(locale, `/${locale}/admin/orders`);
  const sp = await searchParams;
  const status = STATUSES.find((s) => s === sp.status);
  const page = Math.max(1, Math.floor(Number(sp.page)) || 1);
  const o = d.admin.orders;

  // Failures bubble up to the route's error boundary (error.tsx).
  const result = await getAdminOrders(token, { status, page, pageSize: PAGE_SIZE });
  const pages = Math.max(1, Math.ceil(result.total / PAGE_SIZE));
  const dateFmt = new Intl.DateTimeFormat(locale === "tr" ? "tr-TR" : "en-GB", { day: "numeric", month: "short", year: "numeric" });

  return (
    <div className="space-y-8">
      <div className="flex flex-wrap items-end justify-between gap-4">
        <h2 className="headline-md">{o.title}</h2>
        <p className="text-umber">{o.total(result.total)}</p>
      </div>

      <nav aria-label={o.filterLabel} className="no-scrollbar -mx-5 flex gap-1 overflow-x-auto px-5 md:mx-0 md:flex-wrap md:px-0">
        {[undefined, ...STATUSES].map((s) => {
          const active = s === status;
          return (
            <Link
              key={s ?? "all"}
              href={href(s)}
              aria-current={active ? "page" : undefined}
              className={cn(
                "label-caps shrink-0 border px-3 py-2 text-[10px] transition-colors",
                active ? "border-ink bg-ink text-canvas" : "border-travertine text-umber hover:text-ink",
              )}
            >
              {s ? d.status[s] : o.all}
            </Link>
          );
        })}
      </nav>

      {result.items.length === 0 ? (
        <p className="border border-travertine bg-paper p-6 text-umber">{o.empty}</p>
      ) : (
        <div className="overflow-x-auto border border-travertine">
          <table className="w-full min-w-[720px] text-left text-[14px]">
            <thead className="bg-paper">
              <tr className="label-caps text-[10px] text-umber">
                <th scope="col" className="px-4 py-3">{o.columns.order}</th>
                <th scope="col" className="px-4 py-3">{o.columns.date}</th>
                <th scope="col" className="px-4 py-3">{o.columns.items}</th>
                <th scope="col" className="px-4 py-3">{o.columns.total}</th>
                <th scope="col" className="px-4 py-3">{o.columns.status}</th>
                <th scope="col" className="px-4 py-3"><span className="sr-only">{o.view}</span></th>
              </tr>
            </thead>
            <tbody>
              {result.items.map((order) => (
                <tr key={order.id} className="border-t border-travertine">
                  <td className="px-4 py-3 font-mono text-[12px]">{order.id.slice(0, 8)}</td>
                  <td className="px-4 py-3">{dateFmt.format(new Date(order.created_at))}</td>
                  <td className="px-4 py-3">{o.itemCount(order.items.reduce((n, i) => n + i.quantity, 0))}</td>
                  <td className="px-4 py-3">{formatPrice(order.total_cents / 100, locale)}</td>
                  <td className="px-4 py-3"><StatusBadge status={displayStatus(order.status, order.refund)} /></td>
                  <td className="px-4 py-3 text-right">
                    <Link href={`/admin/orders/${order.id}`} className="label-caps text-[10px] text-bronze-deep underline-offset-4 hover:underline">{o.view}</Link>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {pages > 1 ? (
        <div className="flex items-center justify-between gap-4">
          {page > 1 ? <Link href={href(status, page - 1)} className="btn btn-secondary">{o.prev}</Link> : <span />}
          <span className="text-umber">{o.pageOf(page, pages)}</span>
          {page < pages ? <Link href={href(status, page + 1)} className="btn btn-secondary">{o.next}</Link> : <span />}
        </div>
      ) : null}
    </div>
  );
}
