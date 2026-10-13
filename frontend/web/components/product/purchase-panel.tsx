"use client";

import { useEffect, useRef, useState } from "react";
import { AnimatePresence, motion } from "motion/react";
import { Bookmark, Check, Gift, MessageCircle, Ruler, ShieldCheck, Truck } from "lucide-react";
import { Link } from "@/components/ui/link";
import type { Product } from "@/types";
import { useStore } from "@/components/cart/store";
import { useDict, useLocale } from "@/lib/i18n/provider";
import { cn, formatPrice } from "@/lib/utils";

export function PurchasePanel({ product: p }: { product: Product }) {
  const d = useDict();
  const locale = useLocale();
  const t = d.product;
  const { add, wishlist, toggleWish, hydrated } = useStore();
  const [color, setColor] = useState(p.colors[0].name);
  const [size, setSize] = useState<string | null>(p.sizes.length === 1 && p.sizes[0].stock > 0 ? p.sizes[0].label : null);
  const [monogram, setMonogram] = useState(false);
  const [error, setError] = useState(false);
  const [added, setAdded] = useState(false);

  // Stock is tracked per variant (size x color), so availability follows the selected color.
  const variantFor = (label: string, colorName = color) => p.variants.find((v) => v.size === label && v.color === colorName);
  const stockFor = (label: string) => p.stock[variantFor(label)?.id ?? ""] ?? 0;
  const selectedStock = size ? stockFor(size) : 0;
  const soldOut = p.variants.every((v) => (p.stock[v.id] ?? 0) === 0);
  const wished = hydrated && wishlist.includes(p.id);
  const installment = Math.round(p.price / 6);
  const ctaRef = useRef<HTMLButtonElement>(null);
  const panelRef = useRef<HTMLDivElement>(null);
  const [ctaVisible, setCtaVisible] = useState(true);
  const money = (v: number) => formatPrice(v, locale);
  const sizeName = (l: string) => d.sizes[l] ?? l;
  const lead = p.madeToOrderWeeks ? t.madeToOrder(p.madeToOrderWeeks) : t.leadTime(p.leadTimeDays?.[0] ?? 2, p.leadTimeDays?.[1] ?? 4);

  useEffect(() => {
    const el = ctaRef.current;
    if (!el) return;
    const io = new IntersectionObserver(([e]) => setCtaVisible(e.isIntersecting), { threshold: 0 });
    io.observe(el);
    return () => io.disconnect();
  }, []);

  function addToBag() {
    if (!size) {
      setError(true);
      panelRef.current?.scrollIntoView({ behavior: "smooth", block: "center" });
      return;
    }
    const variant = variantFor(size);
    if (!variant) return;
    add({ productId: p.id, variantId: variant.id, size, color, qty: 1 });
    setAdded(true);
    setTimeout(() => setAdded(false), 2200);
  }

  return (
    <div ref={panelRef} className="lg:sticky lg:top-24">
      <div className="border border-travertine bg-paper p-6 md:p-8">
        <div className="flex flex-wrap items-center gap-2">
          {p.tag && <span className={cn("chip", p.tag === "limited" && "chip-solid")}>{d.tags[p.tag]}</span>}
          {p.edition && <span className="label-caps text-[10px] text-umber">{t.edition(p.edition)}</span>}
          <span className="label-caps ml-auto text-[10px] text-umber">{t.refPrefix} {p.sku}</span>
        </div>

        <p className="label-caps mt-6 text-[10px] text-umber">{p.kind}</p>
        <h1 className="headline-lg mt-1">{p.name}</h1>

        <div className="mt-6 flex items-baseline gap-4">
          <p className="font-serif text-[30px] leading-none">{money(p.price)}</p>
          <p className="text-[12px] text-umber">{t.vat}</p>
        </div>
        <p className="mt-2 text-[12px] text-umber">{t.installment(money(installment))}</p>

        <p className="mt-6 text-[14px] leading-6 text-umber">{p.description}</p>

        <div className="mt-8">
          <div className="flex items-center justify-between">
            <p className="label-caps text-[10px]">{t.color}</p>
            <p className="text-[12px] text-umber">{d.colors[color] ?? color}</p>
          </div>
          <div className="mt-3 flex gap-3">
            {p.colors.map((c) => (
              <button
                key={c.name}
                aria-label={d.colors[c.name] ?? c.name}
                aria-pressed={color === c.name}
                onClick={() => {
                  setColor(c.name);
                  if (size && (p.stock[variantFor(size, c.name)?.id ?? ""] ?? 0) === 0) setSize(null);
                }}
                className={cn("h-9 w-9 rounded-full border border-ink/15 outline-offset-[3px] transition-all", color === c.name && "outline outline-1 outline-ink")}
                style={{ background: c.hex }}
              />
            ))}
          </div>
        </div>

        <div className="mt-8">
          <div className="flex items-center justify-between">
            <p className={cn("label-caps text-[10px]", error && "text-danger")}>{error ? t.pleaseSelectSize : t.size}</p>
            <Link href="/about#size-guide" className="flex items-center gap-1.5 text-[12px] text-umber underline underline-offset-4 hover:text-ink">
              <Ruler size={12} /> {t.sizeGuide}
            </Link>
          </div>
          <div className={cn("mt-3 grid gap-2", p.sizes.length > 4 ? "grid-cols-6" : "grid-cols-4")}>
            {p.sizes.map((s) => (
              <button
                key={s.label}
                disabled={stockFor(s.label) === 0}
                aria-pressed={size === s.label}
                onClick={() => {
                  setSize(s.label);
                  setError(false);
                }}
                className={cn(
                  "relative border py-3 text-[13px] transition-colors",
                  p.sizes.length === 1 && "col-span-2",
                  size === s.label ? "border-ink bg-ink text-canvas" : "border-travertine hover:border-ink",
                  stockFor(s.label) === 0 && "cursor-not-allowed text-umber/40 line-through hover:border-travertine",
                )}
              >
                {sizeName(s.label)}
              </button>
            ))}
          </div>
          {size && selectedStock > 0 && selectedStock <= 2 && <p className="mt-3 text-[12px] text-bronze-deep">{t.onlyLeft(selectedStock)}</p>}
        </div>

        <label className="mt-8 flex cursor-pointer gap-3 border border-travertine p-4 hover:border-ink">
          <input
            type="checkbox"
            checked={monogram}
            onChange={(e) => setMonogram(e.target.checked)}
            className="mt-0.5 h-4 w-4 shrink-0 appearance-none border border-ink checked:bg-ink"
          />
          <span>
            <span className="block text-[13px] font-medium">{t.monogramTitle}</span>
            <span className="mt-0.5 block text-[12px] leading-5 text-umber">{t.monogramText}</span>
          </span>
        </label>

        <div className="mt-6 space-y-3">
          <button ref={ctaRef} onClick={addToBag} disabled={soldOut} className="btn btn-primary w-full">
            {soldOut ? t.soldOut : added ? (<><Check size={14} /> {t.addedToCart}</>) : t.addToCart}
          </button>
          <div className="grid grid-cols-[1fr_auto] gap-3">
            <Link href="/appointment" className="btn btn-secondary">{t.fittingAppointment}</Link>
            <button
              onClick={() => toggleWish(p.id)}
              aria-pressed={wished}
              aria-label={wished ? d.listing.removeFromWishlist : d.listing.addToWishlist}
              className="flex w-12 items-center justify-center border border-ink hover:bg-ink hover:text-canvas"
            >
              <Bookmark size={16} strokeWidth={1.5} className={wished ? "fill-current" : ""} />
            </button>
          </div>
        </div>
        <a href="https://wa.me/902120000000" className="mt-4 flex items-center justify-center gap-2 text-[12px] text-umber underline-offset-4 hover:text-ink hover:underline">
          <MessageCircle size={13} /> {t.whatsapp}
        </a>

        <ul className="mt-8 grid grid-cols-2 gap-x-4 gap-y-4 border-t border-travertine pt-6 text-[12px] text-umber">
          <li className="flex gap-2.5"><Truck size={15} strokeWidth={1.4} className="mt-0.5 shrink-0" /> {t.perks.delivery(lead)}</li>
          <li className="flex gap-2.5"><ShieldCheck size={15} strokeWidth={1.4} className="mt-0.5 shrink-0" /> {t.perks.certificate}</li>
          <li className="flex gap-2.5"><Gift size={15} strokeWidth={1.4} className="mt-0.5 shrink-0" /> {t.perks.gift}</li>
          <li className="flex gap-2.5"><Ruler size={15} strokeWidth={1.4} className="mt-0.5 shrink-0" /> {t.perks.alteration}</li>
        </ul>
      </div>

      <AnimatePresence>
        {!ctaVisible && !soldOut && (
          <motion.div
            initial={{ y: "100%" }}
            animate={{ y: 0 }}
            exit={{ y: "100%" }}
            transition={{ duration: 0.4, ease: [0.2, 0.7, 0.2, 1] }}
            className="fixed inset-x-0 bottom-0 z-30 flex items-center justify-between gap-4 border-t border-travertine bg-canvas/95 px-5 py-3 backdrop-blur-md lg:hidden"
          >
            <div className="min-w-0">
              <p className="truncate font-serif text-[15px]">{p.name}</p>
              <p className="label-mono text-umber">{money(p.price)}</p>
            </div>
            <button onClick={addToBag} className="btn btn-primary shrink-0 !px-6">
              {size ? t.addToCart : t.selectSize}
            </button>
          </motion.div>
        )}
      </AnimatePresence>
    </div>
  );
}
