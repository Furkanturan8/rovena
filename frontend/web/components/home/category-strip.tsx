import { Link } from "@/components/ui/link";
import { Photo } from "@/components/ui/photo";
import { Stagger, StaggerItem } from "@/components/motion/reveal";
import type { Category } from "@/types";

export function CategoryStrip({ categories }: { categories: Category[] }) {
  return (
    <section id="categories" className="scroll-mt-20 border-b border-travertine bg-canvas py-16 md:py-24">
      <div className="container-page">
        <Stagger className="no-scrollbar -mx-5 flex gap-4 overflow-x-auto px-5 md:mx-0 md:grid md:grid-cols-5 md:gap-6 md:overflow-visible md:px-0">
          {categories.map((c) => (
            <StaggerItem key={c.slug} className="w-[62%] shrink-0 sm:w-[38%] md:w-auto">
              <Link href={`/collection?category=${c.slug}`} className="group block">
                <Photo src={c.image} alt={c.label} ratio="4/5" sizes="(min-width:768px) 20vw, 60vw" imgClassName="transition-transform duration-[1200ms] ease-out group-hover:scale-[1.06]" />
                <div className="mt-4 flex items-center justify-between border-b border-travertine pb-3 transition-colors group-hover:border-ink">
                  <span className="headline-sm">{c.label}</span>
                  <span aria-hidden className="transition-transform duration-500 group-hover:translate-x-1.5">→</span>
                </div>
              </Link>
            </StaggerItem>
          ))}
        </Stagger>
      </div>
    </section>
  );
}
