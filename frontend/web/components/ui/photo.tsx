import Image from "next/image";
import { cn } from "@/lib/utils";

interface PhotoProps {
  src: string;
  alt: string;
  className?: string;
  imgClassName?: string;
  sizes?: string;
  priority?: boolean;
  ratio?: "3/4" | "1/1" | "16/9" | "4/5" | "21/9" | "auto";
}

const ratios = {
  "3/4": "aspect-[3/4]",
  "1/1": "aspect-square",
  "16/9": "aspect-video",
  "4/5": "aspect-[4/5]",
  "21/9": "aspect-[21/9]",
  auto: "",
};

export function Photo({ src, alt, className, imgClassName, sizes = "(min-width:1024px) 33vw, 100vw", priority, ratio = "3/4" }: PhotoProps) {
  return (
    <div className={cn("relative overflow-hidden bg-sand", ratios[ratio], className)}>
      <Image
        src={src}
        alt={alt}
        fill
        sizes={sizes}
        priority={priority}
        className={cn("object-cover", imgClassName)}
      />
    </div>
  );
}
