import { Link } from "@/components/ui/link";
import { requireAdmin } from "@/lib/auth/session";
import { getAdminStock, type AdminStockFilters } from "@/lib/data/admin-stock";
import { getI18n } from "@/lib/i18n/server";

const PAGE_SIZE = 20;
const LOW_STOCK = 5;
const SORTS: NonNullable<AdminStockFilters["sort"]>[] = ["newest", "sku", "available_asc"];

type Props = {
  params: Promise<{ locale: string }>;
  searchParams: Promise<{ sku?: string; below?: string; sort?: string; page?: string }>;
};

function href(sku: string, below: number | undefined, sort: string | undefined, page?: number) {
  const qs = new URLSearchParams();
  if (sku) qs.set("sku", sku);
  if (below) qs.set("below", String(below));
  if (sort) qs.set("sort", sort);
  if (page && page > 1) qs.set("page", String(page));
  const s = qs.toString();
  return `/admin/stock${s ? `?${s}` : ""}`;
}

export default async function AdminStockPage({ params, searchParams }: Props) {
  const { locale, d } = await getI18n(params);
  const { token } = await requireAdmin(locale, `/${locale}/admin/stock`);
  const sp = await searchParams;
  const sku = (sp.sku ?? "").trim().slice(0, 64);
  const belowNum = Math.floor(Number(sp.below));
  const below = belowNum > 0 && belowNum <= 100000 ? belowNum : undefined;
  const sort = SORTS.find((s) => s === sp.sort);
  const page = Math.max(1, Math.floor(Number(sp.page)) || 1);
  const s = d.admin.stock;

  // Failures bubble up to the route's error boundary (error.tsx).
  const result = await getAdminStock(token, { sku: sku || undefined, below, sort, page, pageSize: PAGE_SIZE });
  const pages = Math.max(1, Math.ceil(result.total / PAGE_SIZE));
  const dateFmt = new Intl.DateTimeFormat(locale === "tr" ? "tr-TR" : "en-GB", { day: "numeric", month: "short", year: "numeric" });

  return (
    <div className="space-y-8">
      <div className="flex flex-wrap items-end justify-between gap-4">
        <h2 className="headline-md">{s.title}</h2>
        <p className="text-umber">{s.total(result.total)}</p>
      </div>

      <form action={`/${locale}/admin/stock`} method="get" role="search" className="flex flex-wrap items-end gap-3">
        <div>
          <label className="field-label" htmlFor="stock-sku">{s.skuLabel}</label>
          <input id="stock-sku" name="sku" defaultValue={sku} maxLength={64} placeholder={s.skuPlaceholder} className="field" />
        </div>
        <div>
          <label className="field-label" htmlFor="stock-below">{s.belowLabel}</label>
          <input id="stock-below" name="below" type="number" min={1} step={1} defaultValue={below ?? ""} className="field w-32" />
        </div>
        <div>
          <label className="field-label" htmlFor="stock-sort">{s.sortLabel}</label>
          <select id="stock-sort" name="sort" defaultValue={sort ?? "newest"} className="field">
            {SORTS.map((o) => (
              <option key={o} value={o}>{s.sorts[o]}</option>
            ))}
          </select>
        </div>
        <button type="submit" className="btn btn-secondary">{s.filter}</button>
      </form>

      {result.items.length === 0 ? (
        <p className="border border-travertine bg-paper p-6 text-umber">{s.empty}</p>
      ) : (
        <div className="overflow-x-auto border border-travertine">
          <table className="w-full min-w-[640px] text-left text-[14px]">
            <thead className="bg-paper">
              <tr className="label-caps text-[10px] text-umber">
                <th scope="col" className="px-4 py-3">{s.columns.sku}</th>
                <th scope="col" className="px-4 py-3">{s.columns.onHand}</th>
                <th scope="col" className="px-4 py-3">{s.columns.reserved}</th>
                <th scope="col" className="px-4 py-3">{s.columns.available}</th>
                <th scope="col" className="px-4 py-3">{s.columns.updated}</th>
                <th scope="col" className="px-4 py-3"><span className="sr-only">{s.manage}</span></th>
              </tr>
            </thead>
            <tbody>
              {result.items.map((item) => (
                <tr key={item.variant_id} className="border-t border-travertine">
                  <td className="px-4 py-3 font-mono text-[12px]">{item.sku}</td>
                  <td className="px-4 py-3">{item.on_hand}</td>
                  <td className="px-4 py-3">{item.reserved}</td>
                  <td className="px-4 py-3">
                    {item.available}
                    {item.available < LOW_STOCK ? <span className="label-caps ml-2 text-[10px] text-red-700">{s.lowBadge}</span> : null}
                  </td>
                  <td className="px-4 py-3">{dateFmt.format(new Date(item.updated_at))}</td>
                  <td className="px-4 py-3 text-right">
                    <Link href={`/admin/stock/${item.variant_id}`} className="label-caps text-[10px] text-bronze-deep underline-offset-4 hover:underline">{s.manage}</Link>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {pages > 1 ? (
        <div className="flex items-center justify-between gap-4">
          {page > 1 ? <Link href={href(sku, below, sort, page - 1)} className="btn btn-secondary">{s.prev}</Link> : <span />}
          <span className="text-umber">{s.pageOf(page, pages)}</span>
          {page < pages ? <Link href={href(sku, below, sort, page + 1)} className="btn btn-secondary">{s.next}</Link> : <span />}
        </div>
      ) : null}
    </div>
  );
}
