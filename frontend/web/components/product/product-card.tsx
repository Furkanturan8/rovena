"use client";

import { Bookmark } from "lucide-react";
import { Link } from "@/components/ui/link";
import type { Product } from "@/types";
import { Photo } from "@/components/ui/photo";
import { useStore } from "@/components/cart/store";
import { useDict, useLocale } from "@/lib/i18n/provider";
import { cn, formatPrice } from "@/lib/utils";

interface Props {
  product: Product;
  className?: string;
  sizes?: string;
  showDescription?: boolean;
  priority?: boolean;
}

export function ProductCard({ product: p, className, sizes, showDescription, priority }: Props) {
  const d = useDict();
  const locale = useLocale();
  const { wishlist, toggleWish, hydrated } = useStore();
  const wished = hydrated && wishlist.includes(p.id);
  const soldOut = p.sizes.every((s) => s.stock === 0);
  const lowStock = !soldOut && p.sizes.reduce((n, s) => n + s.stock, 0) <= 4;
  const hoverImg = p.images[1];
  const l = d.listing;

  return (
    <article className={cn("group relative", className)}>
      <div className="relative">
        <Link href={`/product/${p.slug}`} className="block" aria-label={p.name}>
          <div className="relative aspect-[3/4] overflow-hidden bg-sand">
            <Photo
              src={p.images[0]}
              alt={p.name}
              sizes={sizes ?? "(min-width:1024px) 25vw, 50vw"}
              priority={priority}
              className="absolute inset-0 aspect-auto"
              imgClassName="transition-opacity duration-700 group-hover:opacity-0"
            />
            {hoverImg && (
              <Photo
                src={hoverImg}
                alt=""
                sizes={sizes ?? "(min-width:1024px) 25vw, 50vw"}
                className="absolute inset-0 aspect-auto bg-transparent"
                imgClassName="opacity-0 transition-all duration-700 group-hover:opacity-100 group-hover:scale-[1.03]"
              />
            )}
            {p.tag && <span className={cn("chip absolute left-3 top-3", p.tag === "limited" && "chip-solid")}>{d.tags[p.tag]}</span>}
            {soldOut && (
              <div className="label-caps absolute inset-x-0 bottom-0 bg-canvas/90 py-2 text-center text-umber">{l.soldOut}</div>
            )}
            <span className="label-caps absolute inset-x-0 bottom-0 translate-y-full bg-canvas/95 py-3 text-center text-[10px] transition-transform duration-300 group-hover:translate-y-0">
              {l.quickView}
            </span>
          </div>
        </Link>
        <button
          onClick={() => toggleWish(p.id)}
          aria-label={wished ? l.removeFromWishlist : l.addToWishlist}
          aria-pressed={wished}
          className="absolute right-3 top-3 z-10 flex h-8 w-8 items-center justify-center border border-travertine bg-canvas/90 hover:border-ink"
        >
          <Bookmark size={14} strokeWidth={1.5} className={wished ? "fill-ink" : ""} />
        </button>
      </div>

      <div className="mt-4">
        <div className="flex items-center justify-between gap-3">
          <p className="label-caps text-[10px] text-umber">{p.kind}</p>
          {lowStock && <p className="label-caps text-[10px] text-bronze-deep">{l.lastPieces}</p>}
        </div>
        <div className="mt-1 flex items-start justify-between gap-4">
          <Link href={`/product/${p.slug}`} className="font-serif text-[18px] leading-6 hover:text-bronze-deep">
            {p.name}
          </Link>
          <p className="label-mono shrink-0 pt-1">{formatPrice(p.price, locale)}</p>
        </div>
        {showDescription && <p className="mt-2 line-clamp-2 text-[13px] leading-5 text-umber">{p.description}</p>}
        <div className="mt-3 flex items-center gap-1.5 text-[11px] text-umber">
          {p.colors.map((c) => (
            <span key={c.name} title={d.colors[c.name] ?? c.name} className="h-2.5 w-2.5 rounded-full border border-ink/15" style={{ background: c.hex }} />
          ))}
          <span className="ml-1">{p.colors.map((c) => d.colors[c.name] ?? c.name).join(", ")}</span>
        </div>
      </div>
    </article>
  );
}
