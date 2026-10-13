import { Link } from "@/components/ui/link";
import { Logo } from "./logo";
import { NewsletterForm } from "./newsletter-form";
import { LanguageSwitcher } from "./language-switcher";
import type { Dictionary } from "@/lib/i18n/dictionaries/tr";
import type { Category } from "@/types";

export function Footer({ d, categories }: { d: Dictionary; categories: Category[] }) {
  const f = d.footer;
  const cols = [
    {
      title: f.columns.collection,
      links: [
        { label: f.newArrivals, href: "/collection?sort=newest" },
        ...categories.map((c) => ({ label: c.label, href: `/collection?category=${c.slug}` })),
      ],
    },
    {
      title: f.columns.company,
      links: [
        { label: f.links.about, href: "/about" },
        { label: f.links.gallery, href: "/gallery" },
        { label: f.links.appointment, href: "/appointment" },
      ],
    },
    {
      title: f.columns.help,
      links: [
        { label: f.links.orders, href: "/account/orders" },
        { label: f.links.delivery, href: "/about#delivery" },
        { label: f.links.sizeGuide, href: "/about#size-guide" },
        { label: f.links.care, href: "/about#care" },
      ],
    },
  ];

  return (
    <footer className="border-t border-travertine bg-canvas">
      <section className="border-b border-travertine py-16 md:py-20">
        <div className="container-page flex flex-col items-center text-center">
          <p className="label-caps mb-4 text-umber">{f.newsletterEyebrow}</p>
          <h2 className="headline-lg max-w-2xl">{f.newsletterTitle}</h2>
          <div className="mt-8 flex w-full justify-center">
            <NewsletterForm />
          </div>
        </div>
      </section>

      <div className="container-page grid gap-12 py-16 md:grid-cols-12">
        <div className="md:col-span-4">
          <Logo className="items-start" />
          <p className="mt-6 max-w-xs text-umber">{f.about}</p>
        </div>
        {cols.map((c) => (
          <div key={c.title} className="md:col-span-2">
            <p className="label-caps mb-5 text-umber">{c.title}</p>
            <ul className="space-y-3">
              {c.links.map((l) => (
                <li key={l.label}>
                  <Link href={l.href} className="text-[13px] hover:text-bronze-deep">
                    {l.label}
                  </Link>
                </li>
              ))}
            </ul>
          </div>
        ))}
      </div>

      <div className="border-t border-travertine py-6">
        <div className="container-page flex flex-col gap-4 text-[11px] text-umber md:flex-row md:items-center md:justify-between">
          <p className="label-caps text-[10px]">{f.rights}</p>
          <div className="label-caps flex flex-wrap items-center gap-x-6 gap-y-2 text-[10px]">
            {f.legal.map((l) => (
              <Link key={l} href="#" className="hover:text-ink">
                {l}
              </Link>
            ))}
            <LanguageSwitcher />
          </div>
        </div>
      </div>
    </footer>
  );
}
