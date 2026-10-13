"use client";

import { useEffect, useRef, useState } from "react";
import { AnimatePresence, motion } from "motion/react";
import { Play, X } from "lucide-react";
import type { VideoAsset } from "@/lib/media";
import { useDict } from "@/lib/i18n/provider";
import { cn } from "@/lib/utils";

export function PlayButton({ video, className, label, size = "lg" }: { video: VideoAsset; className?: string; label?: string; size?: "md" | "lg" }) {
  const d = useDict();
  const [open, setOpen] = useState(false);
  return (
    <>
      <button
        onClick={() => setOpen(true)}
        aria-label={label ?? d.video.play}
        className={cn(
          "group/play flex items-center justify-center border border-canvas/70 bg-ink/30 text-canvas backdrop-blur-sm transition-all duration-500 hover:scale-105 hover:bg-canvas hover:text-ink",
          size === "lg" ? "h-20 w-20" : "h-12 w-12",
          className,
        )}
      >
        <Play size={size === "lg" ? 22 : 16} fill="currentColor" className="translate-x-px" />
        <span className="absolute inset-0 -z-10 animate-ping border border-canvas/40 [animation-duration:2.6s]" aria-hidden />
      </button>
      <VideoModal video={video} open={open} onClose={() => setOpen(false)} />
    </>
  );
}

export function VideoModal({ video, open, onClose }: { video: VideoAsset; open: boolean; onClose: () => void }) {
  const d = useDict();
  const title = d.video.titles[video.key];
  const ref = useRef<HTMLVideoElement>(null);

  useEffect(() => {
    if (!open) return;
    const onKey = (e: KeyboardEvent) => e.key === "Escape" && onClose();
    window.addEventListener("keydown", onKey);
    document.body.style.overflow = "hidden";
    return () => {
      window.removeEventListener("keydown", onKey);
      document.body.style.overflow = "";
    };
  }, [open, onClose]);

  return (
    <AnimatePresence>
      {open && (
        <motion.div
          initial={{ opacity: 0 }}
          animate={{ opacity: 1 }}
          exit={{ opacity: 0 }}
          transition={{ duration: 0.3 }}
          className="fixed inset-0 z-[80] flex items-center justify-center bg-ink/95 p-4 md:p-12"
          role="dialog"
          aria-modal="true"
          aria-label={title}
          onClick={onClose}
        >
          <button onClick={onClose} aria-label={d.video.close} className="absolute right-5 top-5 p-2 text-canvas hover:opacity-70">
            <X size={26} strokeWidth={1.3} />
          </button>
          <motion.div
            initial={{ scale: 0.94, opacity: 0 }}
            animate={{ scale: 1, opacity: 1 }}
            exit={{ scale: 0.96, opacity: 0 }}
            transition={{ duration: 0.45, ease: [0.2, 0.7, 0.2, 1] }}
            className="w-full max-w-6xl"
            onClick={(e) => e.stopPropagation()}
          >
            <video ref={ref} src={video.src} poster={video.poster} controls autoPlay playsInline className="aspect-video w-full bg-ink" />
            <p className="label-caps mt-4 text-[10px] text-canvas/60">{title}</p>
          </motion.div>
        </motion.div>
      )}
    </AnimatePresence>
  );
}
