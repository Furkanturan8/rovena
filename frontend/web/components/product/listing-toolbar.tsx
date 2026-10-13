"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { usePathname, useRouter, useSearchParams } from "next/navigation";
import { ChevronDown, SlidersHorizontal, X } from "lucide-react";
import { sortKeys, type SortKey } from "@/lib/listing";
import { useDict, useLocale } from "@/lib/i18n/provider";
import { cn, formatPrice } from "@/lib/utils";
import type { ColorOption } from "@/types";

function useQueryUpdater() {
  const router = useRouter();
  const pathname = usePathname();
  const params = useSearchParams();
  return useCallback(
    (patch: Record<string, string | null>) => {
      const next = new URLSearchParams(params.toString());
      for (const [k, v] of Object.entries(patch)) {
        if (v === null || v === "") next.delete(k);
        else next.set(k, v);
      }
      next.delete("page");
      const qs = next.toString();
      router.push(qs ? `${pathname}?${qs}` : pathname, { scroll: false });
    },
    [params, pathname, router],
  );
}

function Popover({ label, active, children }: { label: string; active?: boolean; children: React.ReactNode }) {
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);
  useEffect(() => {
    if (!open) return;
    const onDown = (e: MouseEvent) => !ref.current?.contains(e.target as Node) && setOpen(false);
    const onKey = (e: KeyboardEvent) => e.key === "Escape" && setOpen(false);
    document.addEventListener("mousedown", onDown);
    document.addEventListener("keydown", onKey);
    return () => {
      document.removeEventListener("mousedown", onDown);
      document.removeEventListener("keydown", onKey);
    };
  }, [open]);
  return (
    <div ref={ref} className="relative">
      <button
        onClick={() => setOpen((o) => !o)}
        aria-expanded={open}
        className={cn(
          "label-caps flex items-center gap-2 border px-4 py-2.5 text-[10px] transition-colors",
          active || open ? "border-ink" : "border-travertine hover:border-ink",
        )}
      >
        {label}
        <ChevronDown size={12} className={cn("transition-transform", open && "rotate-180")} />
      </button>
      {open && <div className="absolute left-0 top-full z-30 mt-2 min-w-[260px] border border-ink bg-canvas p-5">{children}</div>}
    </div>
  );
}

function toggleIn(list: string[], v: string) {
  return list.includes(v) ? list.filter((x) => x !== v) : [...list, v];
}

interface Props {
  total: number;
  shown: number;
  sizes: string[];
  colors: ColorOption[];
  priceRange: { min: number; max: number };
  selected: { size: string[]; color: string[]; min?: number; max?: number; sort: SortKey; q?: string };
}

export function ListingToolbar({ total, shown, sizes, colors, priceRange, selected }: Props) {
  const d = useDict();
  const locale = useLocale();
  const l = d.listing;
  const update = useQueryUpdater();
  const [min, setMin] = useState(String(selected.min ?? ""));
  const [max, setMax] = useState(String(selected.max ?? ""));
  const [mobileOpen, setMobileOpen] = useState(false);
  const money = (v: number) => formatPrice(v, locale);
  const sizeName = (s: string) => d.sizes[s] ?? s;
  const colorName = (c: string) => d.colors[c] ?? c;

  const chips: { label: string; clear: () => void }[] = [
    ...selected.size.map((s) => ({ label: `${l.filterSize} ${sizeName(s)}`, clear: () => update({ size: selected.size.filter((x) => x !== s).join(",") || null }) })),
    ...selected.color.map((c) => ({ label: colorName(c), clear: () => update({ color: selected.color.filter((x) => x !== c).join(",") || null }) })),
    ...(selected.min !== undefined || selected.max !== undefined
      ? [{ label: `${money(selected.min ?? priceRange.min)} — ${money(selected.max ?? priceRange.max)}`, clear: () => update({ min: null, max: null }) }]
      : []),
    ...(selected.q ? [{ label: `“${selected.q}”`, clear: () => update({ q: null }) }] : []),
  ];

  const sizeGrid = (
    <div className="grid grid-cols-4 gap-2">
      {sizes.map((s) => (
        <button
          key={s}
          onClick={() => update({ size: toggleIn(selected.size, s).join(",") || null })}
          className={cn("border py-2 text-[12px]", selected.size.includes(s) ? "border-ink bg-ink text-canvas" : "border-travertine hover:border-ink")}
        >
          {s === "OS" ? "—" : s}
        </button>
      ))}
    </div>
  );

  const colorGrid = (big?: boolean) => (
    <div className="flex flex-wrap gap-3">
      {colors.map((c) => (
        <button
          key={c.name}
          title={colorName(c.name)}
          aria-label={colorName(c.name)}
          aria-pressed={selected.color.includes(c.name)}
          onClick={() => update({ color: toggleIn(selected.color, c.name).join(",") || null })}
          className="flex w-[68px] flex-col items-center gap-1.5 text-center text-[10px] leading-3 text-umber"
        >
          <span
            className={cn("rounded-full border border-ink/15 outline-offset-2", big ? "h-8 w-8" : "h-7 w-7", selected.color.includes(c.name) && "outline outline-1 outline-ink")}
            style={{ background: c.hex }}
          />
          {colorName(c.name)}
        </button>
      ))}
    </div>
  );

  const filterPanels = (
    <>
      <Popover label={l.filterSize} active={selected.size.length > 0}>
        {sizeGrid}
      </Popover>
      <Popover label={l.filterColor} active={selected.color.length > 0}>
        {colorGrid()}
      </Popover>
      <Popover label={l.filterPrice} active={selected.min !== undefined || selected.max !== undefined}>
        <p className="mb-3 text-[12px] text-umber">
          {money(priceRange.min)} — {money(priceRange.max)}
        </p>
        <div className="flex items-center gap-2">
          <input value={min} onChange={(e) => setMin(e.target.value)} inputMode="numeric" placeholder={l.min} className="field !py-2" />
          <span className="text-umber">—</span>
          <input value={max} onChange={(e) => setMax(e.target.value)} inputMode="numeric" placeholder={l.max} className="field !py-2" />
        </div>
        <button onClick={() => update({ min: min || null, max: max || null })} className="btn btn-primary mt-4 w-full !py-2.5">
          {d.common.apply}
        </button>
      </Popover>
    </>
  );

  return (
    <div>
      <div className="flex flex-wrap items-center justify-between gap-4 border-y border-travertine py-4">
        <div className="hidden flex-wrap items-center gap-2 md:flex">{filterPanels}</div>
        <button onClick={() => setMobileOpen(true)} className="label-caps flex items-center gap-2 border border-travertine px-4 py-2.5 text-[10px] md:hidden">
          <SlidersHorizontal size={13} /> {l.filter} {chips.length > 0 && `(${chips.length})`}
        </button>
        <label className="flex items-center gap-3">
          <span className="label-caps hidden text-[10px] text-umber sm:inline">{l.sort}</span>
          <select
            value={selected.sort}
            onChange={(e) => update({ sort: e.target.value === "featured" ? null : e.target.value })}
            className="label-caps cursor-pointer border border-travertine bg-transparent px-3 py-2.5 text-[10px] focus:border-ink focus:outline-none"
          >
            {sortKeys.map((k) => (
              <option key={k} value={k}>{l.sortOptions[k]}</option>
            ))}
          </select>
        </label>
      </div>

      <div className="flex min-h-10 flex-wrap items-center gap-2 py-4">
        {chips.map((c) => (
          <button key={c.label} onClick={c.clear} className="chip gap-2 hover:border-ink hover:text-ink">
            {c.label} <X size={11} />
          </button>
        ))}
        {chips.length > 0 && (
          <button onClick={() => update({ size: null, color: null, min: null, max: null, q: null })} className="label-caps text-[10px] text-umber underline underline-offset-4 hover:text-ink">
            {l.clearFilters}
          </button>
        )}
        <p className="label-caps ml-auto text-[10px] text-umber">{l.showing(shown, total)}</p>
      </div>

      {mobileOpen && (
        <div className="fixed inset-0 z-50 flex flex-col bg-canvas md:hidden">
          <div className="flex h-16 items-center justify-between border-b border-travertine px-5">
            <p className="headline-sm">{l.filter}</p>
            <button onClick={() => setMobileOpen(false)} aria-label={d.common.close}><X size={22} strokeWidth={1.4} /></button>
          </div>
          <div className="flex-1 space-y-8 overflow-y-auto p-5">
            <div>
              <p className="label-caps mb-3 text-umber">{l.filterSize}</p>
              {sizeGrid}
            </div>
            <div>
              <p className="label-caps mb-3 text-umber">{l.filterColor}</p>
              {colorGrid(true)}
            </div>
          </div>
          <div className="border-t border-travertine p-5">
            <button onClick={() => setMobileOpen(false)} className="btn btn-primary w-full">{l.showItems(shown)}</button>
          </div>
        </div>
      )}
    </div>
  );
}
