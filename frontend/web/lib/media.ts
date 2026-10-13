import { resolveMediaUrl } from "@/lib/images";

// Pexels (ücretsiz lisans) videoları; public/media altında yerel olarak servis edilir veya CDN üzerinden çözümlenir.
export type VideoKey = "hero" | "wool" | "sewing";

export interface VideoAsset {
  key: VideoKey;
  src: string;
  srcSm?: string;
  poster: string;
  credit: string;
}

export const VIDEO: Record<VideoKey, VideoAsset> = {
  hero: {
    key: "hero",
    src: resolveMediaUrl("/media/video/hero-loop.mp4"),
    srcSm: resolveMediaUrl("/media/video/hero-loop-sm.mp4"),
    poster: resolveMediaUrl("/media/poster/hero-coat.jpg"),
    credit: "Pexels · 19255239",
  },
  wool: {
    key: "wool",
    src: resolveMediaUrl("/media/video/wool-texture.mp4"),
    poster: resolveMediaUrl("/media/poster/wool-texture.jpg"),
    credit: "Pexels · 7792708",
  },
  sewing: {
    key: "sewing",
    src: resolveMediaUrl("/media/video/atelier-sewing.mp4"),
    poster: resolveMediaUrl("/media/poster/atelier-sewing.jpg"),
    credit: "Pexels · 6459999",
  },
};
