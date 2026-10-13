import type { Metadata } from "next";
import { Link } from "@/components/ui/link";
import { PageHero } from "@/components/ui/page-hero";
import { Photo } from "@/components/ui/photo";
import { Reveal } from "@/components/motion/reveal";
import { getI18n } from "@/lib/i18n/server";

export async function generateMetadata({ params }: PageProps<"/[locale]/gallery">): Promise<Metadata> {
  const { d } = await getI18n(params);
  return { title: d.gallery.pageTitle };
}

const m = (handle: string, n = 3) => `/media/products/${handle}/${n}.jpg`;

const looks: { slug: string; image: string; span: string }[] = [
  { slug: "sonti-coat-in-charcoal", image: m("sonti-coat-in-charcoal", 2), span: "md:col-span-5" },
  { slug: "manston-coat-in-dark-brown", image: m("manston-coat-in-dark-brown", 2), span: "md:col-span-4 md:mt-24" },
  { slug: "elias-embroidery-sweater-in-ivory", image: m("elias-embroidery-sweater-in-ivory"), span: "md:col-span-3" },
  { slug: "hunter-pant-in-desert-sand", image: m("hunter-pant-in-desert-sand"), span: "md:col-span-3 md:mt-10" },
  { slug: "marlot-coat-in-dark-olive", image: m("marlot-coat-in-dark-olive"), span: "md:col-span-5" },
  { slug: "hewit-jacket-in-dark-navy", image: m("hewit-jacket-in-dark-navy"), span: "md:col-span-4 md:mt-20" },
  { slug: "meko-jacket-in-black", image: m("meko-jacket-in-black"), span: "md:col-span-4" },
  { slug: "simone-sweater-in-pewter", image: m("simone-sweater-in-pewter"), span: "md:col-span-4 md:mt-16" },
  { slug: "wilson-coat-in-black", image: m("wilson-coat-in-black"), span: "md:col-span-4 md:mt-6" },
];

export default async function GalleryPage({ params }: PageProps<"/[locale]/gallery">) {
  const { d } = await getI18n(params);
  const g = d.gallery;
  return (
    <>
      <PageHero eyebrow={g.eyebrow} title={g.title1} titleItalic={g.title2} description={g.description} image="/media/products/manston-coat-in-dark-brown/2.jpg" />
      <section className="py-20 md:py-28">
        <div className="container-page grid gap-x-8 gap-y-14 md:grid-cols-12">
          {looks.map((l, i) => {
            const text = g.looks[l.slug];
            return (
              <Reveal key={l.slug} className={l.span}>
                <Link href={`/product/${l.slug}`} className="group block">
                  <Photo src={l.image} alt={text.title} ratio="3/4" sizes="(min-width:768px) 40vw, 100vw" imgClassName="transition-transform duration-[1400ms] group-hover:scale-[1.04]" />
                  <div className="mt-4 flex items-baseline justify-between gap-4">
                    <div>
                      <p className="label-caps text-[10px] text-bronze-deep">{g.look(String(i + 1).padStart(2, "0"))}</p>
                      <p className="headline-md">{text.title}</p>
                      <p className="text-[13px] text-umber">{text.note}</p>
                    </div>
                    <span className="label-caps shrink-0 text-[10px]">{g.viewItem}</span>
                  </div>
                </Link>
              </Reveal>
            );
          })}
        </div>
      </section>
    </>
  );
}
