import { StockForms } from "@/components/admin/stock-forms";
import { Link } from "@/components/ui/link";
import { requireAdmin } from "@/lib/auth/session";
import { getAdminStockItem, getAdminStockMovements } from "@/lib/data/admin-stock";
import { getI18n } from "@/lib/i18n/server";

export default async function AdminStockDetailPage({ params }: { params: Promise<{ locale: string; variantId: string }> }) {
  const { locale, d } = await getI18n(params);
  const { variantId } = await params;
  const { token } = await requireAdmin(locale, `/${locale}/admin/stock/${variantId}`);
  const s = d.admin.stock;
  const t = s.detail;

  // Other failures bubble up to the route's error boundary.
  const item = await getAdminStockItem(token, variantId);
  if (!item) {
    return (
      <div className="space-y-6">
        <Link href="/admin/stock" className="label-caps text-[10px] text-bronze-deep">← {s.back}</Link>
        <div className="border border-travertine bg-paper p-8">
          <h2 className="headline-md">{s.notFoundTitle}</h2>
          <p className="mt-3 text-umber">{s.notFoundText}</p>
        </div>
      </div>
    );
  }

  // The ledger is secondary: show the rest of the page even if it cannot be loaded.
  const movements = await getAdminStockMovements(token, variantId).catch(() => "error" as const);
  const dateTime = new Intl.DateTimeFormat(locale === "tr" ? "tr-TR" : "en-GB", {
    day: "numeric",
    month: "short",
    year: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });

  return (
    <div className="space-y-10">
      <Link href="/admin/stock" className="label-caps text-[10px] text-bronze-deep">← {s.back}</Link>

      <header>
        <h2 className="headline-md font-mono">{item.sku}</h2>
        <p className="mt-2 text-umber">{t.updated}: {dateTime.format(new Date(item.updated_at))}</p>
      </header>

      <dl className="grid gap-px border border-travertine bg-travertine sm:grid-cols-3">
        <div className="bg-canvas p-6">
          <dt className="label-caps text-[10px] text-umber">{t.onHand}</dt>
          <dd className="headline-md mt-2">{item.on_hand}</dd>
        </div>
        <div className="bg-canvas p-6">
          <dt className="label-caps text-[10px] text-umber">{t.reserved}</dt>
          <dd className="headline-md mt-2">{item.reserved}</dd>
        </div>
        <div className="bg-canvas p-6">
          <dt className="label-caps text-[10px] text-umber">{t.available}</dt>
          <dd className="headline-md mt-2">{item.available}</dd>
        </div>
      </dl>

      <StockForms variantId={item.variant_id} />

      <section aria-labelledby="stock-movements">
        <h3 id="stock-movements" className="label-caps mb-3 text-[10px] text-umber">{t.movements}</h3>
        {movements === "error" || movements === null ? (
          <p role="alert" className="border border-travertine bg-paper p-6 text-umber">{t.movementsError}</p>
        ) : movements.items.length === 0 ? (
          <p className="border border-travertine bg-paper p-6 text-umber">{t.noMovements}</p>
        ) : (
          <div className="overflow-x-auto border border-travertine">
            <table className="w-full min-w-[560px] text-left text-[14px]">
              <thead className="bg-paper">
                <tr className="label-caps text-[10px] text-umber">
                  <th scope="col" className="px-4 py-3">{t.columns.date}</th>
                  <th scope="col" className="px-4 py-3">{t.columns.type}</th>
                  <th scope="col" className="px-4 py-3">{t.columns.delta}</th>
                  <th scope="col" className="px-4 py-3">{t.columns.reason}</th>
                </tr>
              </thead>
              <tbody>
                {movements.items.map((m) => (
                  <tr key={m.id} className="border-t border-travertine">
                    <td className="px-4 py-3">{dateTime.format(new Date(m.created_at))}</td>
                    <td className="px-4 py-3">{t.movementTypes[m.type]}</td>
                    <td className="px-4 py-3">{m.quantity_delta > 0 ? `+${m.quantity_delta}` : m.quantity_delta}</td>
                    <td className="px-4 py-3 text-umber">{m.reason || "—"}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </section>
    </div>
  );
}
