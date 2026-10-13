"use client";

import Image from "next/image";
import { useCallback, useEffect, useState } from "react";
import { ChevronLeft, ChevronRight, Maximize2, X } from "lucide-react";
import { VideoBackground } from "@/components/motion/video-bg";
import { PlayButton } from "@/components/motion/video-modal";
import type { VideoAsset } from "@/lib/media";
import { useDict } from "@/lib/i18n/provider";
import { cn } from "@/lib/utils";

export function ProductGallery({ images, name, video }: { images: string[]; name: string; video?: VideoAsset }) {
  const d = useDict();
  const g = d.product.gallery;
  const [lightbox, setLightbox] = useState<number | null>(null);
  const [active, setActive] = useState(0);

  const step = useCallback(
    (d: number) => setLightbox((i) => (i === null ? i : (i + d + images.length) % images.length)),
    [images.length],
  );

  useEffect(() => {
    if (lightbox === null) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") setLightbox(null);
      if (e.key === "ArrowRight") step(1);
      if (e.key === "ArrowLeft") step(-1);
    };
    window.addEventListener("keydown", onKey);
    document.body.style.overflow = "hidden";
    return () => {
      window.removeEventListener("keydown", onKey);
      document.body.style.overflow = "";
    };
  }, [lightbox, step]);

  return (
    <>
      {/* Mobil: kaydırmalı galeri */}
      <div className="md:hidden">
        <div
          className="no-scrollbar -mx-5 flex snap-x snap-mandatory overflow-x-auto"
          onScroll={(e) => {
            const el = e.currentTarget;
            setActive(Math.round(el.scrollLeft / el.clientWidth));
          }}
        >
          {images.map((src, i) => (
            <button key={src} onClick={() => setLightbox(i)} className="relative aspect-[3/4] w-full shrink-0 snap-center bg-sand" aria-label={`${name} ${g.imageN(i + 1, images.length)}`}>
              <Image src={src} alt={`${name} — ${i + 1}`} fill priority={i === 0} sizes="100vw" className="object-cover" />
            </button>
          ))}
          {video && (
            <div className="relative aspect-[3/4] w-full shrink-0 snap-center overflow-hidden bg-sand">
              <VideoBackground video={video} />
              <PlayButton video={video} size="md" className="absolute bottom-4 right-4" />
            </div>
          )}
        </div>
        <div className="mt-3 flex justify-center gap-1.5">
          {[...images, ...(video ? ["video"] : [])].map((_, i) => (
            <span key={i} className={cn("h-px w-6 transition-colors", i === active ? "bg-ink" : "bg-travertine")} />
          ))}
        </div>
      </div>

      {/* Masaüstü: mozaik */}
      <div className="hidden grid-cols-2 gap-3 md:grid">
        {images.map((src, i) => (
          <button
            key={src}
            onClick={() => setLightbox(i)}
            aria-label={`${name} ${g.imageN(i + 1, images.length)} — ${g.zoom}`}
            className={cn("group relative cursor-zoom-in overflow-hidden bg-sand", i === 0 ? "col-span-2 aspect-[4/5]" : "aspect-[3/4]")}
          >
            <Image src={src} alt={`${name} — ${i + 1}`} fill priority={i === 0} sizes="(min-width:1024px) 45vw, 60vw" className="object-cover transition-transform duration-[1200ms] group-hover:scale-[1.03]" />
            <span className="absolute right-3 top-3 flex h-8 w-8 items-center justify-center bg-canvas/90 opacity-0 transition-opacity group-hover:opacity-100">
              <Maximize2 size={13} />
            </span>
          </button>
        ))}
        {video && (
          <div className="relative col-span-2 aspect-video overflow-hidden bg-ink">
            <VideoBackground video={video} />
            <div className="absolute inset-0 bg-ink/15" />
            <span className="chip absolute left-3 top-3 !border-canvas/40 !text-canvas">{d.video.titles[video.key]}</span>
            <PlayButton video={video} size="md" className="absolute bottom-4 right-4" label={g.enlargeVideo} />
          </div>
        )}
      </div>

      {lightbox !== null && (
        <div className="fixed inset-0 z-[70] flex flex-col bg-canvas" role="dialog" aria-modal="true" aria-label={g.lightbox}>
          <div className="flex h-16 items-center justify-between px-5">
            <span className="label-caps text-[10px] text-umber">{lightbox + 1} / {images.length}</span>
            <button onClick={() => setLightbox(null)} aria-label={d.common.close} className="p-2"><X size={22} strokeWidth={1.4} /></button>
          </div>
          <div className="relative flex-1">
            <Image src={images[lightbox]} alt={name} fill sizes="100vw" className="object-contain" />
            <button onClick={() => step(-1)} aria-label={g.prev} className="absolute left-4 top-1/2 -translate-y-1/2 border border-travertine bg-canvas p-3 hover:border-ink"><ChevronLeft size={18} /></button>
            <button onClick={() => step(1)} aria-label={g.next} className="absolute right-4 top-1/2 -translate-y-1/2 border border-travertine bg-canvas p-3 hover:border-ink"><ChevronRight size={18} /></button>
          </div>
        </div>
      )}
    </>
  );
}
