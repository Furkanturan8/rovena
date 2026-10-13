import type { Metadata } from "next";
import { Link } from "@/components/ui/link";
import { AuthShell } from "@/components/account/auth-shell";
import { RegisterForm } from "@/components/account/register-form";
import { getI18n } from "@/lib/i18n/server";

export async function generateMetadata({ params }: PageProps<"/[locale]/register">): Promise<Metadata> {
  const { d } = await getI18n(params);
  return { title: d.auth.register.pageTitle };
}

export default async function RegisterPage({ params, searchParams }: PageProps<"/[locale]/register">) {
  const { d } = await getI18n(params);
  const rawNext = (await searchParams).next;
  const next = typeof rawNext === "string" && rawNext.startsWith("/") && !rawNext.startsWith("//") ? rawNext : undefined;
  const suffix = next ? `?next=${encodeURIComponent(next)}` : "";
  const t = d.auth.register;
  return (
    <AuthShell d={d} title={t.title} subtitle={t.subtitle} footer={<>{t.haveAccount} <Link href={`/login${suffix}`} className="text-ink underline underline-offset-4">{t.signIn}</Link></>}>
      <RegisterForm next={next} />
    </AuthShell>
  );
}
