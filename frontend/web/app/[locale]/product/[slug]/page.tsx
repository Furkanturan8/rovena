import type { Metadata } from "next";
import { notFound } from "next/navigation";
import { ChevronRight, Star } from "lucide-react";
import { Link } from "@/components/ui/link";
import { ProductGallery } from "@/components/product/product-gallery";
import { PurchasePanel } from "@/components/product/purchase-panel";
import { Accordion } from "@/components/product/accordion";
import { ProductCard } from "@/components/product/product-card";
import { SectionHeading } from "@/components/ui/section-heading";
import { Stagger, StaggerItem } from "@/components/motion/reveal";
import { getCategory, getProduct, getProducts, productBases } from "@/lib/mock/catalog";
import { getI18n } from "@/lib/i18n/server";
import { locales } from "@/lib/i18n/config";
import { VIDEO } from "@/lib/media";

export function generateStaticParams() {
  return locales.flatMap((locale) => productBases.map((p) => ({ locale, slug: p.slug })));
}

export async function generateMetadata({ params }: PageProps<"/[locale]/product/[slug]">): Promise<Metadata> {
  const { locale, slug } = await params;
  const p = getProduct(slug, locale === "en" ? "en" : "tr");
  return p ? { title: p.name, description: p.description } : {};
}

export default async function ProductPage({ params }: PageProps<"/[locale]/product/[slug]">) {
  const { locale, d } = await getI18n(params);
  const { slug } = await params;
  const p = getProduct(slug, locale);
  if (!p) notFound();

  const t = d.product;
  const category = getCategory(p.category);
  const all = getProducts(locale);
  const look = all.filter((x) => x.id !== p.id && x.category !== p.category).slice(0, 3);
  const similar = all.filter((x) => x.id !== p.id && x.category === p.category).slice(0, 4);
  const categoryLabel = category ? d.categories[category.slug].label : "";
  const rating = p.rating.toFixed(1);

  return (
    <div>
      <div className="container-page pt-6">
        <nav aria-label="Breadcrumb" className="label-caps flex flex-wrap items-center gap-2 text-[10px] text-umber">
          <Link href="/" className="hover:text-ink">{t.breadcrumbHome}</Link>
          <ChevronRight size={11} />
          <Link href="/collection" className="hover:text-ink">{d.common.collection}</Link>
          {category && (
            <>
              <ChevronRight size={11} />
              <Link href={`/collection?category=${category.slug}`} className="hover:text-ink">{categoryLabel}</Link>
            </>
          )}
          <ChevronRight size={11} />
          <span className="text-ink">{p.name}</span>
        </nav>

        <div className="mt-8 grid gap-10 lg:grid-cols-12 lg:gap-12">
          <div className="lg:col-span-7">
            <ProductGallery images={p.images} name={p.name} video={p.video ? VIDEO[p.video] : undefined} />
          </div>
          <div className="lg:col-span-5">
            <PurchasePanel product={p} />
          </div>
        </div>
      </div>

      <section className="mt-24 border-t border-travertine py-20 md:py-28">
        <div className="container-page">
          <SectionHeading align="center" eyebrow={t.craft.eyebrow} title={t.craft.title(p.craftHours)} description={t.craft.description} />
          <Stagger className="mt-14 grid gap-6 md:grid-cols-3">
            {t.craft.cards.map((x, i) => (
              <StaggerItem key={x.title} className="h-full">
                <article className="flex h-full flex-col border border-travertine bg-paper p-8 transition-colors duration-500 hover:border-ink">
                  <p className="label-caps text-[10px] text-bronze-deep">0{i + 1} / {x.sub}</p>
                  <h3 className="headline-sm mt-4">{x.title}</h3>
                  <p className="mt-3 flex-1 text-[13px] leading-6 text-umber">{x.text}</p>
                  <p className="label-caps mt-8 border-t border-travertine pt-4 text-[10px] text-umber">{t.craft.footers[i]}</p>
                </article>
              </StaggerItem>
            ))}
          </Stagger>
        </div>
      </section>

      <section className="pb-20 md:pb-28">
        <div className="container-page grid gap-12 lg:grid-cols-12">
          <div className="lg:col-span-7 lg:col-start-2">
            <Accordion
              items={[
                {
                  title: t.accordion.composition,
                  content: (
                    <ul className="space-y-2">
                      {p.composition.map((c) => <li key={c}>{c}</li>)}
                    </ul>
                  ),
                },
                { title: t.accordion.fit, content: <p>{t.accordion.fitText}</p> },
                { title: t.accordion.delivery, content: <p>{t.accordion.deliveryText}</p> },
              ]}
            />
            <div className="mt-10 flex items-center gap-6">
              <div className="flex items-center gap-1">
                {Array.from({ length: 5 }, (_, i) => (
                  <Star key={i} size={14} className={i < Math.round(p.rating) ? "fill-bronze text-bronze" : "text-travertine"} />
                ))}
              </div>
              <p className="text-[13px] text-umber">{t.reviews(rating, p.reviewCount)}</p>
            </div>
          </div>
        </div>
      </section>

      <section className="border-t border-travertine bg-paper/60 py-20 md:py-28">
        <div className="container-page">
          <SectionHeading
            eyebrow={t.completeLook.eyebrow}
            title={t.completeLook.title}
            action={<Link href="/collection" className="btn btn-primary self-start md:self-end">{t.completeLook.cta}</Link>}
          />
          <div className="mt-12 grid gap-6 md:grid-cols-3">
            {look.map((x) => (
              <ProductCard key={x.id} product={x} sizes="(min-width:768px) 33vw, 100vw" />
            ))}
          </div>
        </div>
      </section>

      {similar.length > 0 && (
        <section className="border-t border-travertine pb-24 pt-20">
          <div className="container-page">
            <SectionHeading eyebrow={t.similar.eyebrow} title={t.similar.title(categoryLabel)} />
            <div className="mt-12 grid grid-cols-2 gap-x-4 gap-y-10 md:grid-cols-4 md:gap-x-6">
              {similar.map((x) => <ProductCard key={x.id} product={x} />)}
            </div>
          </div>
        </section>
      )}

      <section className="border-t border-travertine bg-sand py-16">
        <div className="container-page flex flex-col items-start justify-between gap-6 md:flex-row md:items-center">
          <div>
            <p className="label-caps mb-3 text-umber">{t.advisor.eyebrow}</p>
            <h2 className="headline-lg">{t.advisor.title}</h2>
          </div>
          <div className="flex flex-col gap-3 sm:flex-row">
            <a href="https://wa.me/902120000000" className="btn btn-primary">{t.advisor.whatsapp}</a>
            <Link href="/appointment" className="btn btn-secondary">{t.advisor.fitting}</Link>
          </div>
        </div>
      </section>
    </div>
  );
}
