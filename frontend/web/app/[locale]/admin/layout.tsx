import type { Metadata } from "next";
import { AdminNav } from "@/components/admin/admin-nav";
import { requireAdmin } from "@/lib/auth/session";
import { getI18n } from "@/lib/i18n/server";

export async function generateMetadata({ params }: LayoutProps<"/[locale]/admin">): Promise<Metadata> {
  const { d } = await getI18n(params);
  return { title: d.admin.metaTitle, robots: { index: false, follow: false } };
}

export default async function AdminLayout({ children, params }: LayoutProps<"/[locale]/admin">) {
  const { locale, d } = await getI18n(params);
  await requireAdmin(locale, `/${locale}/admin`);
  return (
    <div className="container-page py-12 md:py-16">
      <header className="mb-12 border-b border-travertine pb-10">
        <p className="label-caps mb-3 text-bronze-deep">{d.admin.eyebrow}</p>
        <h1 className="headline-xl">{d.admin.title}</h1>
      </header>
      <div className="grid gap-10 lg:grid-cols-[240px_1fr] lg:gap-16">
        <AdminNav />
        <div className="min-w-0">{children}</div>
      </div>
    </div>
  );
}
