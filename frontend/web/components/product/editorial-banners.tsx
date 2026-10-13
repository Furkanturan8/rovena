import { Link } from "@/components/ui/link";
import { Photo } from "@/components/ui/photo";
import { VideoBackground } from "@/components/motion/video-bg";
import { PlayButton } from "@/components/motion/video-modal";
import { Reveal } from "@/components/motion/reveal";
import { VIDEO } from "@/lib/media";
import { formatPrice } from "@/lib/utils";
import { getProduct } from "@/lib/mock/catalog";
import type { Dictionary } from "@/lib/i18n/dictionaries/tr";
import type { Locale } from "@/lib/i18n/config";

export function FilmBanner({ d, locale }: { d: Dictionary; locale: Locale }) {
  const f = d.banner.film;
  const hero = getProduct("sonti-coat-in-charcoal", locale)!;
  return (
    <Reveal>
      <section className="grid overflow-hidden bg-ink text-canvas md:grid-cols-2">
        <div className="relative min-h-[360px] overflow-hidden">
          <div className="absolute inset-0"><VideoBackground video={VIDEO.sewing} /></div>
          <div className="absolute inset-0 bg-ink/30" />
          <span className="chip absolute left-4 top-4 !border-canvas/40 !text-canvas">{f.chip}</span>
          <PlayButton video={VIDEO.sewing} label={f.play} className="absolute left-1/2 top-1/2 -translate-x-1/2 -translate-y-1/2" />
          <p className="label-caps absolute bottom-4 left-4 text-[10px] text-canvas/80">{f.caption}</p>
        </div>
        <div className="flex flex-col justify-center p-8 md:p-14">
          <p className="label-caps mb-5 text-bronze">{f.eyebrow}</p>
          <blockquote className="headline-lg">&ldquo;{f.quote}&rdquo;</blockquote>
          <p className="mt-5 max-w-md text-[14px] leading-6 text-canvas/70">{f.text}</p>
          <Link href={`/product/${hero.slug}`} className="mt-8 flex items-center gap-4 border border-canvas/20 p-4 hover:border-canvas/60">
            <Photo src={hero.images[0]} alt={hero.name} className="w-16 shrink-0" sizes="64px" />
            <div>
              <p className="label-caps text-[10px] text-bronze">{f.featured}</p>
              <p className="font-serif text-[17px]">{hero.name}</p>
              <p className="label-mono text-canvas/70">{formatPrice(hero.price, locale)} · {f.madeToOrder}</p>
            </div>
          </Link>
        </div>
      </section>
    </Reveal>
  );
}

export function CraftFeature({ d }: { d: Dictionary }) {
  const c = d.banner.craft;
  return (
    <section className="grid gap-0 border border-travertine bg-paper md:grid-cols-[minmax(0,5fr)_minmax(0,6fr)]">
      <div className="p-6 md:p-10">
        <div className="relative">
          <Photo src="/media/products/sonti-coat-in-charcoal/4.jpg" alt={c.title} ratio="3/4" sizes="(min-width:768px) 40vw, 100vw" />
          <span className="chip absolute right-3 top-3">{c.chip}</span>
        </div>
      </div>
      <div className="flex flex-col justify-center p-6 md:p-14">
        <p className="label-caps mb-4 text-bronze-deep">{c.eyebrow}</p>
        <h3 className="headline-lg">{c.title}</h3>
        <p className="mt-5 text-[14px] leading-6 text-umber">{c.text}</p>
        <dl className="mt-8 grid grid-cols-2 gap-6">
          {c.stats.map(([v, l]) => (
            <div key={v}>
              <dt className="headline-md">{v}</dt>
              <dd className="label-caps text-[10px] text-umber">{l}</dd>
            </div>
          ))}
        </dl>
        <div className="mt-10 flex flex-wrap gap-3">
          <Link href="/collection?category=coats" className="btn btn-primary">{c.viewCollection}</Link>
          <Link href="/appointment" className="btn btn-secondary">{c.requestSample}</Link>
        </div>
      </div>
    </section>
  );
}
