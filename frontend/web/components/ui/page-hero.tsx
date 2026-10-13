import { Photo } from "@/components/ui/photo";
import { Parallax } from "@/components/motion/parallax";
import { VideoBackground } from "@/components/motion/video-bg";
import { WordReveal } from "@/components/motion/reveal";
import type { VideoAsset } from "@/lib/media";

interface Props {
  eyebrow: string;
  title: string;
  titleItalic?: string;
  description?: string;
  image?: string;
  video?: VideoAsset;
}

export function PageHero({ eyebrow, title, titleItalic, description, image, video }: Props) {
  return (
    <section className="relative isolate flex min-h-[460px] items-end overflow-hidden bg-ink text-canvas md:min-h-[600px]">
      {video ? (
        <div className="absolute inset-0 -z-10">
          <VideoBackground video={video} priority />
        </div>
      ) : (
        image && (
          <Parallax className="-z-10">
            <Photo src={image} alt="" ratio="auto" className="absolute inset-0" imgClassName="object-[50%_30%]" sizes="100vw" priority />
          </Parallax>
        )
      )}
      <div className="absolute inset-0 -z-10 bg-gradient-to-t from-ink/85 via-ink/25 to-ink/10" />
      <div className="absolute inset-x-0 top-0 -z-10 h-40 bg-gradient-to-b from-ink/55 to-transparent" />
      <div className="container-page pb-12 md:pb-20">
        <p className="label-caps mb-5 text-bronze animate-fade-up">{eyebrow}</p>
        <h1 className="display-hero max-w-3xl">
          <WordReveal text={title} delay={0.15} />
          {titleItalic && (
            <>
              {" "}
              <em className="font-serif italic"><WordReveal text={titleItalic} delay={0.35} /></em>
            </>
          )}
        </h1>
        {description && (
          <p className="mt-6 max-w-xl text-[15px] leading-7 text-canvas/80 animate-fade-up [animation-delay:0.5s]">{description}</p>
        )}
      </div>
    </section>
  );
}
