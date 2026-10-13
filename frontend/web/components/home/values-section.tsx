import { Award, MessageCircle, RefreshCw, Truck } from "lucide-react";
import { SectionHeading } from "@/components/ui/section-heading";
import { Reveal, Stagger, StaggerItem } from "@/components/motion/reveal";
import type { Dictionary } from "@/lib/i18n/dictionaries/tr";

const icons = [Award, Truck, RefreshCw, MessageCircle];

export function ValuesSection({ d }: { d: Dictionary }) {
  const v = d.home.values;
  return (
    <section className="py-24 md:py-32">
      <div className="container-page">
        <Reveal>
          <SectionHeading eyebrow={v.eyebrow} title={v.title} description={v.description} align="center" />
        </Reveal>
        <Stagger className="mt-14 grid gap-px border border-travertine bg-travertine sm:grid-cols-2 lg:grid-cols-4">
          {v.items.map((it, i) => {
            const Icon = icons[i];
            return (
              <StaggerItem key={it.title} className="bg-canvas">
                <div className="h-full p-8 transition-colors duration-500 hover:bg-paper">
                  <Icon size={26} strokeWidth={1.1} className="text-bronze-deep" />
                  <p className="headline-sm mt-6">{it.title}</p>
                  <p className="mt-2 text-[13px] leading-6 text-umber">{it.text}</p>
                </div>
              </StaggerItem>
            );
          })}
        </Stagger>
      </div>
    </section>
  );
}
