import type { Metadata } from "next";
import { AdminAccountMenu } from "@/components/admin/admin-account-menu";
import { AdminNav } from "@/components/admin/admin-nav";
import { LanguageSwitcher } from "@/components/layout/language-switcher";
import { requireAdmin } from "@/lib/auth/session";
import { getI18n } from "@/lib/i18n/server";

type AdminLayoutProps = { children: React.ReactNode; params: Promise<{ locale: string }> };

export async function generateMetadata({ params }: Pick<AdminLayoutProps, "params">): Promise<Metadata> {
  const { d } = await getI18n(params);
  return { title: d.admin.metaTitle, robots: { index: false, follow: false } };
}

export default async function AdminLayout({ children, params }: AdminLayoutProps) {
  const { locale, d } = await getI18n(params);
  await requireAdmin(locale, `/${locale}/admin`);
  return (
    <div className="flex min-h-screen flex-col bg-canvas">
      <header className="border-b border-travertine bg-canvas">
        <div className="container-page flex items-center justify-between gap-4 py-4">
          <p className="flex items-baseline gap-3">
            <span className="font-serif text-xl tracking-wide text-ink">Rovena</span>
            <span className="label-caps text-[10px] text-bronze-deep">{d.admin.title}</span>
          </p>
          <div className="flex items-center gap-5">
            <LanguageSwitcher />
            <AdminAccountMenu />
          </div>
        </div>
      </header>
      <div className="container-page grid flex-1 gap-8 py-8 lg:grid-cols-[220px_1fr] lg:gap-14 lg:py-12">
        <aside>
          <AdminNav />
        </aside>
        <main className="min-w-0">{children}</main>
      </div>
    </div>
  );
}
