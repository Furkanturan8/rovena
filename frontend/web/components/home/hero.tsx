"use client";

import { useRef } from "react";
import { motion, useScroll, useTransform } from "motion/react";
import { ArrowDown } from "lucide-react";
import { VideoBackground } from "@/components/motion/video-bg";
import { WordReveal } from "@/components/motion/reveal";
import { Link } from "@/components/ui/link";
import { VIDEO } from "@/lib/media";
import { useDict } from "@/lib/i18n/provider";

export function Hero() {
  const d = useDict();
  const h = d.home.hero;
  const ref = useRef<HTMLElement>(null);
  const { scrollYProgress } = useScroll({ target: ref, offset: ["start start", "end start"] });
  const scale = useTransform(scrollYProgress, [0, 1], [1, 1.15]);
  const textY = useTransform(scrollYProgress, [0, 1], ["0%", "-25%"]);
  const fade = useTransform(scrollYProgress, [0, 0.7], [1, 0]);

  return (
    <section ref={ref} className="relative isolate flex h-[calc(100svh-36px)] min-h-[620px] items-end overflow-hidden bg-ink text-canvas">
      <motion.div style={{ scale }} className="absolute inset-0 -z-10">
        <VideoBackground video={VIDEO.hero} priority />
      </motion.div>
      <div className="absolute inset-0 -z-10 bg-gradient-to-t from-ink/85 via-ink/25 to-ink/20" />
      <div className="absolute inset-0 -z-10 bg-gradient-to-r from-ink/55 via-ink/10 to-transparent" />

      <motion.div style={{ y: textY, opacity: fade }} className="container-page pb-24 md:pb-32">
        <div className="max-w-2xl">
          <motion.span
            initial={{ opacity: 0, y: 12 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ duration: 0.8, delay: 0.2 }}
            className="label-caps inline-flex items-center gap-3 border border-canvas/40 px-3 py-1.5 text-[10px] backdrop-blur-sm"
          >
            <span className="h-1 w-1 bg-bronze" />
            {h.eyebrow}
          </motion.span>
          <h1 className="display-hero mt-7">
            <WordReveal text={h.line1} delay={0.35} />
            <br />
            <em className="font-serif italic">
              <WordReveal text={h.line2} delay={0.7} />
            </em>
          </h1>
          <motion.p
            initial={{ opacity: 0, y: 16 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ duration: 0.9, delay: 1.0 }}
            className="mt-6 max-w-lg text-[15px] leading-7 text-canvas/85"
          >
            {h.description}
          </motion.p>
          <motion.div
            initial={{ opacity: 0, y: 16 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ duration: 0.9, delay: 1.2 }}
            className="mt-9 flex flex-wrap gap-4"
          >
            <Link href="/collection" className="btn btn-light">{h.primary}</Link>
            <Link href="/gallery" className="btn border-canvas/60 text-canvas hover:bg-canvas hover:text-ink">{h.secondary}</Link>
          </motion.div>
        </div>
      </motion.div>

      <div className="absolute inset-x-0 bottom-0 border-t border-canvas/20">
        <div className="container-page flex h-14 items-center justify-between text-canvas/80">
          <span className="label-caps text-[10px]">{h.caption}</span>
          <Link href="#categories" className="label-caps flex items-center gap-3 text-[10px] hover:text-canvas">
            {h.scroll}
            <span className="relative flex h-8 w-px overflow-hidden bg-canvas/25">
              <motion.span
                className="absolute inset-x-0 top-0 h-3 bg-canvas"
                animate={{ y: ["-100%", "300%"] }}
                transition={{ duration: 2, repeat: Infinity, ease: "easeInOut" }}
              />
            </span>
            <ArrowDown size={12} className="-ml-1" />
          </Link>
        </div>
      </div>
    </section>
  );
}
