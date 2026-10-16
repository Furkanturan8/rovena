import { ProductForm, type ProductFormValues } from "@/components/admin/product-form";
import { VariantManager } from "@/components/admin/variant-manager";
import { Link } from "@/components/ui/link";
import { updateProduct } from "@/lib/admin/product-actions";
import { requireAdmin } from "@/lib/auth/session";
import { getAdminProduct, getCategoryOptions } from "@/lib/data/admin-products";
import { getI18n } from "@/lib/i18n/server";

type Props = { params: Promise<{ locale: string; id: string }> };

export default async function AdminEditProductPage({ params }: Props) {
  const { locale, d } = await getI18n(params);
  const { id } = await params;
  const { token } = await requireAdmin(locale, `/${locale}/admin/products/${id}`);
  const f = d.admin.productForm;

  // Other failures bubble up to the route's error boundary (error.tsx).
  const [product, categories] = await Promise.all([getAdminProduct(token, id), getCategoryOptions(locale)]);
  if (!product) {
    return (
      <div className="space-y-6">
        <Link href="/admin/products" className="label-caps text-[10px] text-bronze-deep">← {f.back}</Link>
        <div className="border border-travertine bg-paper p-8">
          <h2 className="headline-md">{f.notFoundTitle}</h2>
          <p className="mt-3 text-umber">{f.notFoundText}</p>
        </div>
      </div>
    );
  }

  // Only plain string/number attributes are editable rows; lists and objects are carried over untouched.
  const rows: ProductFormValues["attributes"] = [];
  const extraAttributes: Record<string, unknown> = {};
  for (const [key, value] of Object.entries(product.attributes ?? {})) {
    if (typeof value === "string" || typeof value === "number") rows.push({ key, value: String(value) });
    else extraAttributes[key] = value;
  }
  const en = product.translations?.en;

  const initial: Partial<ProductFormValues> = {
    name: product.name,
    brand: product.brand,
    categoryId: product.category_id,
    description: product.description,
    status: product.status,
    enName: en?.name ?? "",
    enDescription: en?.description ?? "",
    attributes: rows,
    images: (product.images ?? []).map((img) => ({ url: img.url, alt: img.alt })),
  };

  return (
    <div className="space-y-8">
      <div className="flex flex-wrap items-end justify-between gap-4">
        <h2 className="headline-md">{f.editTitle}</h2>
        <Link href="/admin/products" className="text-bronze-deep underline-offset-4 hover:underline">{f.back}</Link>
      </div>
      <p className="text-umber">{f.editVariantsHint}</p>
      <ProductForm
        action={updateProduct}
        categories={categories}
        mode="edit"
        initial={initial}
        slug={product.slug}
        hidden={{
          id: product.id,
          extraAttributes: JSON.stringify(extraAttributes),
          extraTranslations: JSON.stringify(product.translations ?? {}),
        }}
        cancelHref="/admin/products"
      />
      <VariantManager productId={product.id} variants={product.variants ?? []} />
    </div>
  );
}
