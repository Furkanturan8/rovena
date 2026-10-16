import { ProductForm } from "@/components/admin/product-form";
import { Link } from "@/components/ui/link";
import { createProductAction } from "@/lib/admin/product-actions";
import { requireAdmin } from "@/lib/auth/session";
import { getCategoryOptions } from "@/lib/data/admin-products";
import { getI18n } from "@/lib/i18n/server";

type Props = { params: Promise<{ locale: string }> };

export default async function AdminNewProductPage({ params }: Props) {
  const { locale, d } = await getI18n(params);
  await requireAdmin(locale, `/${locale}/admin/products/new`);
  const f = d.admin.productForm;

  // Failures bubble up to the route's error boundary (products/error.tsx).
  const categories = await getCategoryOptions(locale);

  return (
    <div className="space-y-8">
      <div className="flex flex-wrap items-end justify-between gap-4">
        <h2 className="headline-md">{f.newTitle}</h2>
        <Link href="/admin/products" className="text-bronze-deep underline-offset-4 hover:underline">{f.back}</Link>
      </div>
      <ProductForm action={createProductAction} categories={categories} cancelHref="/admin/products" />
    </div>
  );
}
