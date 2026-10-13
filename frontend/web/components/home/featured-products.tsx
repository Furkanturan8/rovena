"use client";

import { useState } from "react";
import { AnimatePresence, motion } from "motion/react";
import { Link } from "@/components/ui/link";
import { categories, getProducts } from "@/lib/mock/catalog";
import { useDict, useLocale } from "@/lib/i18n/provider";
import { ProductCard } from "@/components/product/product-card";
import { SectionHeading } from "@/components/ui/section-heading";
import { Reveal } from "@/components/motion/reveal";
import { cn } from "@/lib/utils";

export function FeaturedProducts() {
  const d = useDict();
  const locale = useLocale();
  const f = d.home.featured;
  const tabs = [{ slug: "all", label: f.all }, ...categories.map((c) => ({ slug: c.slug as string, label: d.categories[c.slug].short }))];
  const [tab, setTab] = useState<string>("all");
  const list = getProducts(locale).filter((p) => tab === "all" || p.category === tab).slice(0, 4);

  return (
    <section className="border-y border-travertine bg-paper/60 py-24 md:py-32">
      <div className="container-page">
        <Reveal>
          <SectionHeading
            eyebrow={f.eyebrow}
            title={f.title}
            action={
              <div role="tablist" aria-label={d.nav.collection} className="no-scrollbar -mx-5 flex gap-2 overflow-x-auto px-5 md:mx-0 md:px-0">
                {tabs.map((t) => (
                  <button
                    key={t.slug}
                    role="tab"
                    aria-selected={tab === t.slug}
                    onClick={() => setTab(t.slug)}
                    className={cn(
                      "label-caps relative shrink-0 border px-4 py-2 text-[10px] transition-colors duration-300",
                      tab === t.slug ? "border-ink text-canvas" : "border-travertine text-umber hover:border-ink hover:text-ink",
                    )}
                  >
                    {tab === t.slug && <motion.span layoutId="featured-tab" className="absolute inset-0 -z-0 bg-ink" transition={{ type: "spring", stiffness: 420, damping: 36 }} />}
                    <span className="relative z-10">{t.label}</span>
                  </button>
                ))}
              </div>
            }
          />
        </Reveal>

        <div className="mt-14 min-h-[480px]">
          <AnimatePresence mode="wait">
            <motion.div
              key={tab}
              initial={{ opacity: 0, y: 18 }}
              animate={{ opacity: 1, y: 0 }}
              exit={{ opacity: 0, y: -10 }}
              transition={{ duration: 0.4 }}
              className="grid grid-cols-2 gap-x-4 gap-y-10 md:grid-cols-4 md:gap-x-6"
            >
              {list.map((p) => (
                <ProductCard key={p.id} product={p} />
              ))}
            </motion.div>
          </AnimatePresence>
        </div>

        <Reveal className="mt-16 flex flex-col items-start justify-between gap-6 border border-travertine bg-canvas p-6 md:flex-row md:items-center md:p-8">
          <div>
            <p className="headline-sm">{f.b2bTitle}</p>
            <p className="mt-1 max-w-xl text-[13px] text-umber">{f.b2bText}</p>
          </div>
          <Link href="/appointment" className="btn btn-primary shrink-0">{f.b2bCta}</Link>
        </Reveal>
      </div>
    </section>
  );
}
