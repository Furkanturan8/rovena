import type { Metadata } from "next";
import { user } from "@/lib/mock/account";
import { getI18n } from "@/lib/i18n/server";

export async function generateMetadata({ params }: PageProps<"/[locale]/account/profile">): Promise<Metadata> {
  const { d } = await getI18n(params);
  return { title: d.account.profile.pageTitle };
}

export default async function ProfilePage({ params }: PageProps<"/[locale]/account/profile">) {
  const { d } = await getI18n(params);
  const t = d.account.profile;
  return (
    <div className="max-w-2xl space-y-14">
      <form>
        <h2 className="headline-md mb-8">{t.personal}</h2>
        <div className="grid gap-5 sm:grid-cols-2">
          <div><label className="field-label" htmlFor="fn">{t.firstName}</label><input id="fn" defaultValue={user.firstName} className="field" /></div>
          <div><label className="field-label" htmlFor="ln">{t.lastName}</label><input id="ln" defaultValue={user.lastName} className="field" /></div>
          <div className="sm:col-span-2"><label className="field-label" htmlFor="em">{t.email}</label><input id="em" type="email" defaultValue={user.email} className="field" /></div>
          <div className="sm:col-span-2"><label className="field-label" htmlFor="ph">{t.phone}</label><input id="ph" defaultValue={user.phone} className="field" /></div>
        </div>
        <button type="button" className="btn btn-primary mt-8">{d.common.save}</button>
      </form>

      <form>
        <h2 className="headline-md mb-8">{t.changePassword}</h2>
        <div className="grid gap-5">
          <div><label className="field-label" htmlFor="cur">{t.currentPassword}</label><input id="cur" type="password" className="field" /></div>
          <div><label className="field-label" htmlFor="new">{t.newPassword}</label><input id="new" type="password" className="field" /></div>
        </div>
        <button type="button" className="btn btn-secondary mt-8">{t.updatePassword}</button>
      </form>

      <section>
        <h2 className="headline-md mb-6">{t.preferences}</h2>
        <div className="space-y-4">
          {t.prefs.map((l, i) => (
            <label key={l} className="flex cursor-pointer items-center gap-3 text-[14px]">
              <input type="checkbox" defaultChecked={i !== 2} disabled={i === 0} className="h-4 w-4 appearance-none border border-ink checked:bg-ink disabled:opacity-50" />
              {l}
            </label>
          ))}
        </div>
      </section>
    </div>
  );
}
