import { Link } from "@/components/ui/link";
import { VideoBackground } from "@/components/motion/video-bg";
import { PlayButton } from "@/components/motion/video-modal";
import { Reveal, WordReveal } from "@/components/motion/reveal";
import { VIDEO } from "@/lib/media";
import type { Dictionary } from "@/lib/i18n/dictionaries/tr";

export function FilmSection({ d }: { d: Dictionary }) {
  const f = d.home.film;
  return (
    <section className="relative isolate overflow-hidden bg-ink text-canvas">
      <div className="absolute inset-0 -z-10 opacity-70">
        <VideoBackground video={VIDEO.sewing} />
      </div>
      <div className="absolute inset-0 -z-10 bg-gradient-to-r from-ink/90 via-ink/50 to-ink/20" />
      <div className="container-page relative grid min-h-[620px] items-center gap-12 py-24 md:grid-cols-2">
        <div>
          <Reveal>
            <p className="label-caps mb-5 text-bronze">{f.eyebrow}</p>
          </Reveal>
          <h2 className="headline-xl max-w-xl">
            <WordReveal text={f.title} />
          </h2>
          <Reveal delay={0.3}>
            <p className="mt-6 max-w-md text-[15px] leading-7 text-canvas/75">{f.text}</p>
            <Link href="/about" className="link-underline mt-8">{f.link}</Link>
          </Reveal>
        </div>
        <Reveal delay={0.2} className="flex items-center justify-start md:justify-center">
          <div className="relative flex flex-col items-center gap-5">
            <PlayButton video={VIDEO.sewing} label={f.play} />
            <p className="label-caps text-[10px] text-canvas/70">{f.caption}</p>
          </div>
        </Reveal>
      </div>
    </section>
  );
}
