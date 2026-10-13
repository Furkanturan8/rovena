import Image from "next/image";
import { Link } from "@/components/ui/link";
import { Plus } from "lucide-react";
import { SectionHeading } from "@/components/ui/section-heading";
import { Photo } from "@/components/ui/photo";
import { ImageReveal, Reveal } from "@/components/motion/reveal";
import { Parallax } from "@/components/motion/parallax";
import type { Dictionary } from "@/lib/i18n/dictionaries/tr";

function ParallaxPhoto({ src, alt, className, sizes }: { src: string; alt: string; className: string; sizes: string }) {
  return (
    <div className={`relative overflow-hidden bg-sand ${className}`}>
      <Parallax offset={7}>
        <Image src={src} alt={alt} fill sizes={sizes} className="object-cover object-top" />
      </Parallax>
    </div>
  );
}

export function CapsuleSection({ d }: { d: Dictionary }) {
  const c = d.home.capsule;
  return (
    <section className="py-24 md:py-32">
      <div className="container-page">
        <Reveal>
          <SectionHeading
            eyebrow={c.eyebrow}
            title={c.title}
            description={c.description}
            action={<Link href="/collection" className="link-underline self-start md:self-end">{c.allCollection}</Link>}
          />
        </Reveal>

        <div className="mt-14 grid gap-6 md:grid-cols-12">
          <Link href="/collection?category=coats" className="group relative md:col-span-7">
            <ImageReveal>
              <div className="relative">
                <ParallaxPhoto src="/media/products/sonti-coat-in-charcoal/2.jpg" alt={c.bigTitle} className="aspect-[4/5] md:aspect-[5/6]" sizes="(min-width:768px) 58vw, 100vw" />
                <span className="chip chip-solid absolute left-4 top-4">{c.badge}</span>
              </div>
            </ImageReveal>
            <Reveal delay={0.1} className="mt-4 flex items-end justify-between gap-6">
              <div>
                <p className="headline-md transition-colors group-hover:text-bronze-deep">{c.bigTitle}</p>
                <p className="mt-1 text-[13px] text-umber">{c.bigNote}</p>
              </div>
              <span className="label-caps text-umber transition-transform duration-500 group-hover:translate-x-1">{c.discover}</span>
            </Reveal>
          </Link>

          <div className="grid grid-cols-2 gap-6 md:col-span-5">
            <Link href="/collection?category=trousers" className="group col-span-2">
              <ImageReveal delay={0.1}>
                <div className="relative">
                  <Photo src="/media/products/august-pant-in-charcoal/3.jpg" alt={c.trousersTitle} ratio="16/9" sizes="(min-width:768px) 40vw, 100vw" imgClassName="object-[50%_55%] transition-transform duration-[1400ms] group-hover:scale-[1.05]" />
                  <span className="chip absolute left-3 top-3">{c.trousersChip}</span>
                  <span className="absolute bottom-3 right-3 flex h-9 w-9 items-center justify-center bg-canvas transition-transform duration-500 group-hover:rotate-90"><Plus size={16} /></span>
                </div>
              </ImageReveal>
              <Reveal delay={0.15}><p className="headline-sm mt-3">{c.trousersTitle}</p></Reveal>
            </Link>
            <Link href="/collection?category=accessories" className="group">
              <ImageReveal delay={0.2}>
                <Photo src="/media/products/donna-shoulder-bag-in-ebano/1.jpg" alt={c.bagsTitle} ratio="1/1" sizes="20vw" imgClassName="transition-transform duration-[1400ms] group-hover:scale-[1.06]" />
              </ImageReveal>
              <Reveal delay={0.25}>
                <p className="headline-sm mt-3">{c.bagsTitle}</p>
                <p className="text-[12px] text-umber">{c.bagsNote}</p>
              </Reveal>
            </Link>
            <Link href="/collection?category=knitwear" className="group">
              <ImageReveal delay={0.3}>
                <Photo src="/media/products/simone-sweater-in-pewter/3.jpg" alt={c.knitTitle} ratio="1/1" sizes="20vw" imgClassName="object-top transition-transform duration-[1400ms] group-hover:scale-[1.06]" />
              </ImageReveal>
              <Reveal delay={0.35}>
                <p className="headline-sm mt-3">{c.knitTitle}</p>
                <p className="text-[12px] text-umber">{c.knitNote}</p>
              </Reveal>
            </Link>
          </div>
        </div>
      </div>
    </section>
  );
}
