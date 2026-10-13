import type { Metadata } from "next";
import { ProfileForm } from "@/components/account/profile-form";
import { requireUser } from "@/lib/auth/session";
import { getI18n } from "@/lib/i18n/server";

export async function generateMetadata({ params }: PageProps<"/[locale]/account/profile">): Promise<Metadata> {
  const { d } = await getI18n(params);
  return { title: d.account.profile.pageTitle };
}

export default async function ProfilePage({ params }: PageProps<"/[locale]/account/profile">) {
  const { locale, d } = await getI18n(params);
  const { user } = await requireUser(locale, `/${locale}/account/profile`);
  const t = d.account.profile;
  return (
    <div className="max-w-2xl space-y-14">
      <ProfileForm user={{ firstName: user.first_name, lastName: user.last_name, email: user.email }} />
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
