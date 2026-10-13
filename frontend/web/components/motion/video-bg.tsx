"use client";

import { useEffect, useRef } from "react";
import { useReducedMotion } from "motion/react";
import type { VideoAsset } from "@/lib/media";
import { cn } from "@/lib/utils";

/** Sessiz, döngülü arka plan videosu. Ekrandan çıkınca durur, hareket azaltma tercihinde sadece poster gösterilir. */
export function VideoBackground({ video, className, priority = false }: { video: VideoAsset; className?: string; priority?: boolean }) {
  const ref = useRef<HTMLVideoElement>(null);
  const reduce = useReducedMotion();

  useEffect(() => {
    const el = ref.current;
    if (!el || reduce) return;
    const io = new IntersectionObserver(
      ([entry]) => {
        if (entry.isIntersecting) el.play().catch(() => {});
        else el.pause();
      },
      { threshold: 0.05 },
    );
    io.observe(el);
    return () => io.disconnect();
  }, [reduce]);

  return (
    <video
      ref={ref}
      className={cn("h-full w-full object-cover", className)}
      poster={video.poster}
      muted
      loop
      playsInline
      autoPlay={!reduce}
      preload={priority ? "auto" : "metadata"}
      aria-hidden
      tabIndex={-1}
    >
      {video.srcSm && <source src={video.srcSm} type="video/mp4" media="(max-width: 767px)" />}
      <source src={video.src} type="video/mp4" />
    </video>
  );
}
