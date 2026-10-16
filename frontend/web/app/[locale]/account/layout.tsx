import { AccountNav } from "@/components/account/account-nav";
import { requireUser } from "@/lib/auth/session";
import { getI18n } from "@/lib/i18n/server";

export default async function AccountLayout({ children, params }: LayoutProps<"/[locale]/account">) {
  const { locale, d } = await getI18n(params);
  const { user } = await requireUser(locale, `/${locale}/account`);
  const since = new Date(user.created_at).getFullYear().toString();
  return (
    <div className="container-page py-12 md:py-16">
      <header className="mb-12 border-b border-travertine pb-10">
        <p className="label-caps mb-3 text-bronze-deep">{d.account.memberSince(d.account.tier, since)}</p>
        <h1 className="headline-xl">{d.account.welcome(user.first_name || user.email)}</h1>
      </header>
      <div className="grid gap-10 lg:grid-cols-[240px_1fr] lg:gap-16">
        <AccountNav isAdmin={user.role === "admin"} />
        <div className="min-w-0">{children}</div>
      </div>
    </div>
  );
}
