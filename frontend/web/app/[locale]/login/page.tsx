import type { Metadata } from "next";
import { Link } from "@/components/ui/link";
import { AuthShell } from "@/components/account/auth-shell";
import { LoginForm } from "@/components/account/login-form";
import { getI18n } from "@/lib/i18n/server";

export async function generateMetadata({ params }: PageProps<"/[locale]/login">): Promise<Metadata> {
  const { d } = await getI18n(params);
  return { title: d.auth.login.pageTitle };
}

export default async function LoginPage({ params }: PageProps<"/[locale]/login">) {
  const { d } = await getI18n(params);
  const t = d.auth.login;
  return (
    <AuthShell d={d} title={t.title} subtitle={t.subtitle} footer={<>{t.noAccount} <Link href="/register" className="text-ink underline underline-offset-4">{t.signUp}</Link></>}>
      <LoginForm />
    </AuthShell>
  );
}
