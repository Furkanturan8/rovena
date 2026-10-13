import type { Metadata } from "next";
import { notFound } from "next/navigation";
import { locales } from "@/lib/i18n/config";
import { getI18n } from "@/lib/i18n/server";
import { isLegalSlug, legalDocs, legalSlugs } from "@/lib/legal/content";

export function generateStaticParams() {
  return locales.flatMap((locale) => legalSlugs.map((doc) => ({ locale, doc })));
}

export async function generateMetadata({ params }: PageProps<"/[locale]/legal/[doc]">): Promise<Metadata> {
  const { locale } = await getI18n(params);
  const { doc } = await params;
  if (!isLegalSlug(doc)) return {};
  return { title: legalDocs[locale][doc].title };
}

export default async function LegalPage({ params }: PageProps<"/[locale]/legal/[doc]">) {
  const { locale, d } = await getI18n(params);
  const { doc: slug } = await params;
  if (!isLegalSlug(slug)) notFound();
  const doc = legalDocs[locale][slug];

  return (
    <div className="container-page py-16 md:py-24">
      <article className="mx-auto max-w-3xl">
        <p className="label-caps mb-4 text-umber">{d.legal.eyebrow}</p>
        <h1 className="headline-xl">{doc.title}</h1>
        <p className="mt-6 text-[15px] leading-7 text-umber">{doc.intro}</p>
        {doc.sections.map((s) => {
          const items = s.body.filter((b) => b.startsWith("- "));
          const paras = s.body.filter((b) => !b.startsWith("- "));
          return (
            <section key={s.heading} className="mt-12 border-t border-travertine pt-8">
              <h2 className="headline-sm">{s.heading}</h2>
              {paras.map((p) => <p key={p} className="mt-4 text-[14px] leading-7 text-umber">{p}</p>)}
              {items.length > 0 && (
                <ul className="mt-4 list-disc space-y-2 pl-5 text-[14px] leading-7 text-umber marker:text-ink">
                  {items.map((i) => <li key={i}>{i.slice(2)}</li>)}
                </ul>
              )}
            </section>
          );
        })}
        <p className="mt-16 border-t border-travertine pt-6 text-[12px] text-umber">{d.legal.updated}</p>
      </article>
    </div>
  );
}
