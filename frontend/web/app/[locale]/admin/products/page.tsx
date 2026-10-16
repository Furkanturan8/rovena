import { Link } from "@/components/ui/link";
import type { ApiProductStatus } from "@/lib/api/types";
import { requireAdmin } from "@/lib/auth/session";
import { getAdminProducts } from "@/lib/data/admin-products";
import { getI18n } from "@/lib/i18n/server";
import { cn, formatPrice } from "@/lib/utils";

const PAGE_SIZE = 20;
const STATUSES: ApiProductStatus[] = ["active", "draft", "archived"];

type Props = {
  params: Promise<{ locale: string }>;
  searchParams: Promise<{ status?: string; q?: string; page?: string }>;
};

function href(status: ApiProductStatus | undefined, q: string, page?: number) {
  const qs = new URLSearchParams();
  if (status) qs.set("status", status);
  if (q) qs.set("q", q);
  if (page && page > 1) qs.set("page", String(page));
  const s = qs.toString();
  return `/admin/products${s ? `?${s}` : ""}`;
}

export default async function AdminProductsPage({ params, searchParams }: Props) {
  const { locale, d } = await getI18n(params);
  const { token } = await requireAdmin(locale, `/${locale}/admin/products`);
  const sp = await searchParams;
  const status = STATUSES.find((s) => s === sp.status);
  const q = (sp.q ?? "").trim().slice(0, 80);
  const page = Math.max(1, Math.floor(Number(sp.page)) || 1);
  const p = d.admin.products;

  // Failures bubble up to the route's error boundary (error.tsx).
  const result = await getAdminProducts(token, { status, q: q || undefined, page, pageSize: PAGE_SIZE });
  const pages = Math.max(1, Math.ceil(result.total / PAGE_SIZE));

  return (
    <div className="space-y-8">
      <div className="flex flex-wrap items-end justify-between gap-4">
        <h2 className="headline-md">{p.title}</h2>
        <div className="flex flex-wrap items-center gap-4">
          <p className="text-umber">{p.total(result.total)}</p>
          <Link href="/admin/products/new" className="btn btn-primary">{p.newProduct}</Link>
        </div>
      </div>

      <div className="flex flex-col gap-4 md:flex-row md:items-end md:justify-between">
        <nav aria-label={p.filterLabel} className="no-scrollbar -mx-5 flex gap-1 overflow-x-auto px-5 md:mx-0 md:flex-wrap md:px-0">
          {[undefined, ...STATUSES].map((s) => {
            const active = s === status;
            return (
              <Link
                key={s ?? "all"}
                href={href(s, q)}
                aria-current={active ? "page" : undefined}
                className={cn(
                  "label-caps shrink-0 border px-3 py-2 text-[10px] transition-colors",
                  active ? "border-ink bg-ink text-canvas" : "border-travertine text-umber hover:text-ink",
                )}
              >
                {s ? p.statuses[s] : p.all}
              </Link>
            );
          })}
        </nav>
        <form action={`/${locale}/admin/products`} method="get" role="search" className="flex items-end gap-2">
          {status ? <input type="hidden" name="status" value={status} /> : null}
          <div>
            <label className="field-label" htmlFor="product-q">{p.searchLabel}</label>
            <input id="product-q" name="q" defaultValue={q} maxLength={80} placeholder={p.searchPlaceholder} className="field" />
          </div>
          <button type="submit" className="btn btn-secondary">{p.search}</button>
        </form>
      </div>

      {result.items.length === 0 ? (
        <p className="border border-travertine bg-paper p-6 text-umber">{p.empty}</p>
      ) : (
        <div className="overflow-x-auto border border-travertine">
          <table className="w-full min-w-[720px] text-left text-[14px]">
            <thead className="bg-paper">
              <tr className="label-caps text-[10px] text-umber">
                <th scope="col" className="px-4 py-3">{p.columns.product}</th>
                <th scope="col" className="px-4 py-3">{p.columns.brand}</th>
                <th scope="col" className="px-4 py-3">{p.columns.status}</th>
                <th scope="col" className="px-4 py-3">{p.columns.price}</th>
                <th scope="col" className="px-4 py-3">{p.columns.variants}</th>
              </tr>
            </thead>
            <tbody>
              {result.items.map((product) => (
                <tr key={product.id} className="border-t border-travertine align-top">
                  <td className="px-4 py-3">
                    <Link href={`/admin/products/${product.id}`} className="text-bronze-deep underline-offset-4 hover:underline">
                      {product.name}
                    </Link>
                    <span className="block font-mono text-[12px] text-umber">{product.slug}</span>
                    <Link href={`/admin/products/${product.id}`} className="label-caps mt-1 inline-block text-[10px] text-bronze-deep">
                      {p.edit}
                    </Link>
                  </td>
                  <td className="px-4 py-3">{product.brand || "—"}</td>
                  <td className="px-4 py-3">{p.statuses[product.status]}</td>
                  <td className="px-4 py-3">
                    {product.price_from_cents == null ? "—" : formatPrice(product.price_from_cents / 100, locale)}
                  </td>
                  <td className="px-4 py-3">
                    {p.variantCount(product.variants.length)}
                    <ul className="mt-1 space-y-0.5">
                      {product.variants.map((v) => (
                        <li key={v.id} className="text-[12px]">
                          <Link href={`/admin/stock/${v.id}`} className="font-mono text-bronze-deep underline-offset-4 hover:underline">
                            {v.sku}
                          </Link>
                          <span className="text-umber"> · {[v.size, v.color].filter(Boolean).join(" · ")}</span>
                        </li>
                      ))}
                    </ul>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {pages > 1 ? (
        <div className="flex items-center justify-between gap-4">
          {page > 1 ? <Link href={href(status, q, page - 1)} className="btn btn-secondary">{p.prev}</Link> : <span />}
          <span className="text-umber">{p.pageOf(page, pages)}</span>
          {page < pages ? <Link href={href(status, q, page + 1)} className="btn btn-secondary">{p.next}</Link> : <span />}
        </div>
      ) : null}
    </div>
  );
}
