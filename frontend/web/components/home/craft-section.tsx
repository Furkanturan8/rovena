import { Photo } from "@/components/ui/photo";
import { VideoBackground } from "@/components/motion/video-bg";
import { CountUp } from "@/components/motion/count-up";
import { ImageReveal, Reveal, Stagger, StaggerItem, WordReveal } from "@/components/motion/reveal";
import { VIDEO } from "@/lib/media";
import type { Dictionary } from "@/lib/i18n/dictionaries/tr";

const images = [
  "/media/poster/atelier-sewing.jpg",
  "/media/products/simone-sweater-in-pewter/6.jpg",
  "/media/products/marlot-coat-in-dark-olive/3.jpg",
];

export function CraftSection({ d }: { d: Dictionary }) {
  const m = d.home.manifesto;
  return (
    <>
      <section className="relative isolate overflow-hidden bg-ink py-28 text-canvas md:py-40">
        <div className="absolute inset-0 -z-10 opacity-60">
          <VideoBackground video={VIDEO.wool} />
        </div>
        <div className="absolute inset-0 -z-10 bg-ink/55" />
        <div className="container-page">
          <div className="mx-auto max-w-4xl text-center">
            <Reveal><p className="label-caps mb-8 text-bronze">{m.eyebrow}</p></Reveal>
            <blockquote className="headline-xl md:!text-[56px] md:!leading-[64px]">
              <WordReveal text={m.quote} />
            </blockquote>
            <Reveal delay={0.4}><p className="label-caps mt-10 text-[10px] text-canvas/70">{m.cite}</p></Reveal>
          </div>
        </div>
      </section>

      <section className="py-24 md:py-32">
        <div className="container-page">
          <div className="grid gap-8 md:grid-cols-3">
            {m.items.map((it, i) => (
              <article key={it.n}>
                <ImageReveal delay={i * 0.12}>
                  <Photo src={images[i]} alt={it.title} ratio="4/5" sizes="(min-width:768px) 33vw, 100vw" />
                </ImageReveal>
                <Reveal delay={0.1 + i * 0.12}>
                  <p className="label-caps mt-5 text-[10px] text-bronze-deep">{it.n}</p>
                  <h3 className="headline-sm mt-1">{it.title}</h3>
                  <p className="mt-2 text-[13px] leading-6 text-umber">{it.text}</p>
                </Reveal>
              </article>
            ))}
          </div>

          <Stagger className="mt-20 grid grid-cols-1 gap-px border border-travertine bg-travertine md:grid-cols-3">
            {m.stats.map((s) => (
              <StaggerItem key={s.title} className="bg-canvas p-10 text-center">
                <p className="headline-xl"><CountUp to={s.to} prefix={"prefix" in s ? s.prefix : undefined} suffix={"suffix" in s ? s.suffix : undefined} /></p>
                <p className="label-caps mt-3 text-[10px]">{s.title}</p>
                <p className="mt-1 text-[12px] text-umber">{s.desc}</p>
              </StaggerItem>
            ))}
          </Stagger>
        </div>
      </section>
    </>
  );
}
