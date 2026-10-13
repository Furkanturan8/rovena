"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import { Search, X } from "lucide-react";
import { Link } from "@/components/ui/link";
import { getProducts } from "@/lib/mock/catalog";
import { Photo } from "@/components/ui/photo";
import { useDict, useLocale } from "@/lib/i18n/provider";
import { formatPrice } from "@/lib/utils";

export function SearchOverlay({ open, onClose }: { open: boolean; onClose: () => void }) {
  const d = useDict();
  const locale = useLocale();
  const [q, setQ] = useState("");
  const inputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    if (!open) return;
    inputRef.current?.focus();
    const onKey = (e: KeyboardEvent) => e.key === "Escape" && onClose();
    window.addEventListener("keydown", onKey);
    document.body.style.overflow = "hidden";
    return () => {
      window.removeEventListener("keydown", onKey);
      document.body.style.overflow = "";
    };
  }, [open, onClose]);

  const results = useMemo(() => {
    const term = q.trim().toLocaleLowerCase(locale);
    if (term.length < 2) return [];
    return getProducts(locale)
      .filter((p) => `${p.name} ${p.kind} ${p.material}`.toLocaleLowerCase(locale).includes(term))
      .slice(0, 4);
  }, [q, locale]);

  if (!open) return null;

  return (
    <div className="fixed inset-0 z-50 flex animate-fade-up flex-col bg-canvas text-ink" role="dialog" aria-modal="true" aria-label={d.search.title}>
      <div className="container-page flex h-[72px] items-center justify-between border-b border-travertine">
        <span className="label-caps text-umber">{d.search.title}</span>
        <button onClick={onClose} aria-label={d.common.close} className="p-2">
          <X size={22} strokeWidth={1.4} />
        </button>
      </div>
      <div className="container-page flex-1 overflow-y-auto py-10 md:py-16">
        <div className="mx-auto max-w-3xl">
          <div className="flex items-center gap-4 border-b border-ink pb-4">
            <Search size={22} strokeWidth={1.4} />
            <input
              ref={inputRef}
              value={q}
              onChange={(e) => setQ(e.target.value)}
              placeholder={d.search.placeholder}
              className="headline-md w-full bg-transparent placeholder:text-umber/50 focus:outline-none"
            />
          </div>
          {results.length === 0 ? (
            <div className="mt-10">
              <p className="label-caps mb-4 text-umber">{d.search.popular}</p>
              <div className="flex flex-wrap gap-2">
                {d.search.suggestions.map((s) => (
                  <button key={s} onClick={() => setQ(s)} className="chip hover:border-ink hover:text-ink">
                    {s}
                  </button>
                ))}
              </div>
              {q.trim().length >= 2 && <p className="mt-10 text-umber">{d.search.noResults(q)}</p>}
            </div>
          ) : (
            <div className="mt-10 grid grid-cols-2 gap-6 md:grid-cols-4">
              {results.map((p) => (
                <Link key={p.id} href={`/product/${p.slug}`} onClick={onClose} className="group">
                  <Photo src={p.images[0]} alt={p.name} sizes="25vw" imgClassName="object-top transition-transform duration-700 group-hover:scale-[1.04]" />
                  <p className="mt-3 text-[13px]">{p.name}</p>
                  <p className="label-mono text-umber">{formatPrice(p.price, locale)}</p>
                </Link>
              ))}
            </div>
          )}
          {results.length > 0 && (
            <Link href={`/collection?q=${encodeURIComponent(q)}`} onClick={onClose} className="link-underline mt-10">
              {d.search.seeAll}
            </Link>
          )}
        </div>
      </div>
    </div>
  );
}
