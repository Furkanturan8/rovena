import type { Metadata } from "next";
import { ChevronRight } from "lucide-react";
import { Link } from "@/components/ui/link";
import { ListingToolbar } from "@/components/product/listing-toolbar";
import { ProductCard } from "@/components/product/product-card";
import { CraftFeature, FilmBanner } from "@/components/product/editorial-banners";
import { Stagger, StaggerItem } from "@/components/motion/reveal";
import { categories, getCategory, productBases } from "@/lib/mock/catalog";
import { filterProducts, PAGE_SIZE, parseListingParams } from "@/lib/listing";
import { getI18n } from "@/lib/i18n/server";
import { cn } from "@/lib/utils";

export async function generateMetadata({ params }: PageProps<"/[locale]/collection">): Promise<Metadata> {
  const { d } = await getI18n(params);
  return { title: d.listing.title };
}

export default async function CollectionPage({ params, searchParams }: PageProps<"/[locale]/collection">) {
  const { locale, d } = await getI18n(params);
  const l = d.listing;
  const raw = await searchParams;
  const query = parseListingParams(raw);
  const filtered = filterProducts(query, locale);
  const pageCount = Math.max(1, Math.ceil(filtered.length / PAGE_SIZE));
  const page = Math.min(query.page, pageCount);
  const slice = filtered.slice((page - 1) * PAGE_SIZE, page * PAGE_SIZE);
  const category = query.category ? getCategory(query.category) : undefined;
  const countsBase = filterProducts(query, locale, "category");

  // Filtre değişince satırları yeniden bağla ki giriş animasyonu yeni ürünler için tekrar çalışsın.
  const listKey = JSON.stringify(raw);

  const href = (patch: Record<string, string | null>) => {
    const next = new URLSearchParams();
    for (const [k, v] of Object.entries(raw)) {
      const val = Array.isArray(v) ? v[0] : v;
      if (val) next.set(k, val);
    }
    for (const [k, v] of Object.entries(patch)) {
      if (v === null) next.delete(k);
      else next.set(k, v);
    }
    const qs = next.toString();
    return qs ? `/collection?${qs}` : "/collection";
  };

  const rows: React.ReactNode[] = [];
  for (let i = 0; i < slice.length; i += 3) {
    rows.push(
      <Stagger key={`${listKey}-row-${i}`} className="grid grid-cols-2 gap-x-4 gap-y-12 md:grid-cols-3 md:gap-x-6">
        {slice.slice(i, i + 3).map((p, j) => (
          <StaggerItem key={p.id}>
            <ProductCard product={p} showDescription sizes="(min-width:768px) 33vw, 50vw" priority={i + j < 3} />
          </StaggerItem>
        ))}
      </Stagger>,
    );
    if (page === 1 && i === 3 && slice.length > 6 && !query.q) rows.push(<FilmBanner key="film" d={d} locale={locale} />);
  }

  return (
    <div>
      <div className="container-page pt-8">
        <nav aria-label="Breadcrumb" className="label-caps flex items-center gap-2 text-[10px] text-umber">
          <Link href="/" className="hover:text-ink">{l.breadcrumb}</Link>
          <ChevronRight size={11} />
          <Link href="/collection" className={cn(!category && "text-ink")}>{l.title}</Link>
          {category && (
            <>
              <ChevronRight size={11} />
              <span className="text-ink">{d.categories[category.slug].label}</span>
            </>
          )}
        </nav>

        <header className="mt-8 grid gap-8 pb-12 md:grid-cols-12 md:items-end">
          <div className="md:col-span-8">
            <p className="label-caps mb-4 text-umber">{l.eyebrow}</p>
            <h1 className="headline-xl md:!text-[64px] md:!leading-[72px]">{category ? d.categories[category.slug].label : l.title}</h1>
          </div>
          <p className="max-w-md text-[14px] leading-6 text-umber md:col-span-4">{l.description(productBases.length)}</p>
        </header>

        <div className="no-scrollbar -mx-5 flex gap-2 overflow-x-auto px-5 pb-6 md:mx-0 md:px-0">
          <Link href={href({ category: null })} className={cn("label-caps shrink-0 border px-5 py-3 text-[10px]", !category ? "border-ink bg-ink text-canvas" : "border-travertine hover:border-ink")}>
            {l.all(countsBase.length)}
          </Link>
          {categories.map((c) => (
            <Link
              key={c.slug}
              href={href({ category: c.slug })}
              className={cn("label-caps shrink-0 border px-5 py-3 text-[10px]", query.category === c.slug ? "border-ink bg-ink text-canvas" : "border-travertine hover:border-ink")}
            >
              {d.categories[c.slug].label} ({countsBase.filter((p) => p.category === c.slug).length})
            </Link>
          ))}
        </div>

        <ListingToolbar
          total={filtered.length}
          shown={slice.length}
          selected={{ size: query.size, color: query.color, min: query.min, max: query.max, sort: query.sort, q: query.q }}
        />

        {slice.length === 0 ? (
          <div className="py-24 text-center">
            <p className="headline-md">{l.noResultsTitle}</p>
            <p className="mt-3 text-umber">{l.noResultsText}</p>
            <Link href="/collection" className="btn btn-primary mt-8">{l.allCollection}</Link>
          </div>
        ) : (
          <div className="space-y-16 pb-16 pt-4">{rows}</div>
        )}

        {pageCount > 1 && (
          <nav aria-label={l.pagination} className="flex items-center justify-between border-t border-travertine py-8">
            <p className="label-caps text-[10px] text-umber">{l.pageOf(page, pageCount, filtered.length)}</p>
            <div className="flex gap-2">
              {Array.from({ length: pageCount }, (_, i) => i + 1).map((n) => (
                <Link key={n} href={href({ page: n === 1 ? null : String(n) })} aria-current={n === page ? "page" : undefined} className={cn("flex h-10 w-10 items-center justify-center border text-[12px]", n === page ? "border-ink bg-ink text-canvas" : "border-travertine hover:border-ink")}>
                  {String(n).padStart(2, "0")}
                </Link>
              ))}
            </div>
          </nav>
        )}

        {page === pageCount && slice.length > 0 && !query.q && (
          <div className="pb-20">
            <CraftFeature d={d} />
          </div>
        )}
      </div>

      <section className="border-t border-travertine bg-paper/60 py-16">
        <div className="container-page flex flex-col items-start justify-between gap-8 md:flex-row md:items-center">
          <div>
            <p className="label-caps mb-3 text-umber">{l.ctaEyebrow}</p>
            <h2 className="headline-lg max-w-2xl">{l.ctaTitle}</h2>
          </div>
          <div className="flex w-full flex-col gap-3 sm:w-auto">
            <Link href="/appointment" className="btn btn-primary">{l.ctaBook}</Link>
            <Link href="/about" className="btn btn-secondary">{l.ctaCatalog}</Link>
          </div>
        </div>
      </section>
    </div>
  );
}
