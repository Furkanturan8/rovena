import { logoutAction } from "@/lib/auth/actions";
import { requireAdmin } from "@/lib/auth/session";
import { getI18n } from "@/lib/i18n/server";

type Props = { params: Promise<{ locale: string }> };

export default async function AdminAccountPage({ params }: Props) {
  const { locale, d } = await getI18n(params);
  const { user } = await requireAdmin(locale, `/${locale}/admin/account`);
  const a = d.admin.account;
  const name = `${user.first_name} ${user.last_name}`.trim();

  return (
    <div className="space-y-8">
      <div>
        <h2 className="headline-md">{a.title}</h2>
        <p className="mt-2 text-umber">{a.subtitle}</p>
      </div>

      <dl className="grid max-w-2xl divide-y divide-travertine border border-travertine">
        {[
          { label: a.name, value: name || "—" },
          { label: a.email, value: user.email },
          { label: a.role, value: a.roles[user.role] },
        ].map((row) => (
          <div key={row.label} className="grid gap-1 px-5 py-4 sm:grid-cols-[10rem_1fr] sm:gap-6">
            <dt className="label-caps text-[10px] text-umber">{row.label}</dt>
            <dd className="break-all text-ink">{row.value}</dd>
          </div>
        ))}
      </dl>

      <section className="max-w-2xl border border-travertine bg-paper p-6">
        <h3 className="font-display text-xl">{a.signOutTitle}</h3>
        <p className="mt-2 text-umber">{a.signOutText}</p>
        <form action={logoutAction} className="mt-6">
          <input type="hidden" name="locale" value={locale} />
          <input type="hidden" name="to" value="login" />
          <button className="btn btn-primary">{a.signOut}</button>
        </form>
      </section>
    </div>
  );
}
