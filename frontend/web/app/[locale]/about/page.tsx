import type { Metadata } from "next";
import { PageHero } from "@/components/ui/page-hero";
import { Photo } from "@/components/ui/photo";
import { Accordion } from "@/components/product/accordion";
import { SectionHeading } from "@/components/ui/section-heading";
import { Reveal } from "@/components/motion/reveal";
import { VIDEO } from "@/lib/media";
import { getI18n } from "@/lib/i18n/server";

export async function generateMetadata({ params }: PageProps<"/[locale]/about">): Promise<Metadata> {
  const { d } = await getI18n(params);
  return { title: d.about.pageTitle };
}

const sizeRows = [
  ["34", "XS", "80 – 84", "62 – 66", "88 – 92"],
  ["36", "S", "84 – 88", "66 – 70", "92 – 96"],
  ["38", "M", "88 – 92", "70 – 74", "96 – 100"],
  ["40", "L", "92 – 96", "74 – 78", "100 – 104"],
  ["42", "XL", "96 – 100", "78 – 82", "104 – 108"],
  ["44", "XXL", "100 – 106", "82 – 88", "108 – 114"],
];

export default async function AboutPage({ params }: PageProps<"/[locale]/about">) {
  const { d } = await getI18n(params);
  const a = d.about;
  return (
    <>
      <PageHero eyebrow={a.eyebrow} title={a.title1} titleItalic={a.title2} description={a.description} video={VIDEO.sewing} />

      <section className="py-20 md:py-28">
        <div className="container-page grid gap-12 lg:grid-cols-2 lg:items-center">
          <Photo src="/media/poster/wool-texture.jpg" alt="" ratio="4/5" sizes="(min-width:1024px) 45vw, 100vw" />
          <Reveal>
            <SectionHeading eyebrow={a.mission.eyebrow} title={a.mission.title} description={a.mission.description} />
            <dl className="mt-10 grid grid-cols-2 gap-6">
              {a.mission.stats.map(([v, l]) => (
                <div key={l} className="border-t border-travertine pt-4"><dt className="headline-md">{v}</dt><dd className="label-caps text-[10px] text-umber">{l}</dd></div>
              ))}
            </dl>
          </Reveal>
        </div>
      </section>

      <section id="size-guide" className="scroll-mt-28 border-t border-travertine bg-paper/60 py-20 md:py-28">
        <div className="container-page">
          <SectionHeading eyebrow={a.sizeGuide.eyebrow} title={a.sizeGuide.title} description={a.sizeGuide.description} />
          <div className="mt-10 overflow-x-auto">
            <table className="w-full min-w-[520px] border border-travertine text-left text-[13px]">
              <thead className="bg-canvas">
                <tr>{a.sizeGuide.headers.map((h) => <th key={h} className="label-caps border-b border-travertine px-5 py-4 text-[10px] text-umber">{h}</th>)}</tr>
              </thead>
              <tbody>
                {sizeRows.map((r) => (
                  <tr key={r[0]} className="border-b border-travertine last:border-0">{r.map((c, i) => <td key={i} className={`px-5 py-4 ${i === 0 ? "font-medium" : "text-umber"}`}>{c}</td>)}</tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      </section>

      <section className="py-20 md:py-28">
        <div className="container-page grid gap-12 lg:grid-cols-12">
          <div id="care" className="scroll-mt-28 lg:col-span-5">
            <p className="label-caps mb-4 text-umber">{a.care.eyebrow}</p>
            <h2 className="headline-lg">{a.care.title}</h2>
          </div>
          <div className="lg:col-span-6 lg:col-start-7">
            <ul className="space-y-6 text-[14px] leading-6 text-umber">
              {a.care.items.map(([t, desc]) => (
                <li key={t} className="border-t border-travertine pt-4"><p className="headline-sm text-ink">{t}</p><p className="mt-1">{desc}</p></li>
              ))}
            </ul>
          </div>
        </div>
      </section>

      <section id="delivery" className="scroll-mt-28 border-t border-travertine bg-paper/60 py-20 md:py-28">
        <div className="container-page grid gap-12 lg:grid-cols-12">
          <div className="lg:col-span-5">
            <p className="label-caps mb-4 text-umber">{a.delivery.eyebrow}</p>
            <h2 className="headline-lg">{a.delivery.title}</h2>
          </div>
          <div className="lg:col-span-7">
            <Accordion items={a.delivery.items.map((it) => ({ title: it.title, content: <p>{it.text}</p> }))} />
          </div>
        </div>
      </section>
    </>
  );
}
