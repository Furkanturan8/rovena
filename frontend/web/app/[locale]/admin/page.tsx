import { Link } from "@/components/ui/link";
import { requireAdmin } from "@/lib/auth/session";
import { getAdminOrders } from "@/lib/data/admin-orders";
import { getAdminProducts } from "@/lib/data/admin-products";
import { getAdminStock } from "@/lib/data/admin-stock";
import { getAdminUsers } from "@/lib/data/admin-users";
import { getI18n } from "@/lib/i18n/server";

const LOW_STOCK_BELOW = 5;

export default async function AdminOverview({ params }: { params: Promise<{ locale: string }> }) {
  const { locale, d } = await getI18n(params);
  const { token } = await requireAdmin(locale, `/${locale}/admin`);
  const o = d.admin.overview;
  // Failures bubble up to the route's error boundary (error.tsx).
  const [orders, products, lowStock, users] = await Promise.all([
    getAdminOrders(token, { pageSize: 1 }),
    getAdminProducts(token, { pageSize: 1 }),
    getAdminStock(token, { below: LOW_STOCK_BELOW, pageSize: 1 }),
    getAdminUsers(token, 1, 1),
  ]);

  const cards = [
    { href: "/admin/orders", label: o.orders, value: orders.total },
    { href: "/admin/products", label: o.products, value: products.total },
    { href: "/admin/stock", label: o.lowStock, value: lowStock.total, hint: o.lowStockHint(LOW_STOCK_BELOW) },
    { href: "/admin/users", label: o.users, value: users.total },
  ];
  const isEmpty = cards.every((c) => c.value === 0);

  return (
    <div className="space-y-8">
      <p className="text-umber">{o.subtitle}</p>
      <div className="grid gap-px border border-travertine bg-travertine sm:grid-cols-2">
        {cards.map((c) => (
          <Link key={c.href} href={c.href} className="bg-canvas p-6 transition-colors hover:bg-paper">
            <p className="headline-md">{c.value}</p>
            <p className="label-caps mt-1 text-[10px] text-umber">{c.label}</p>
            {c.hint ? <p className="mt-2 text-[12px] text-umber">{c.hint}</p> : null}
          </Link>
        ))}
      </div>
      {isEmpty ? <p className="border border-travertine bg-paper p-6 text-umber">{o.empty}</p> : null}
    </div>
  );
}
