"use client";

import { useState } from "react";
import { ChevronDown } from "lucide-react";
import { cn } from "@/lib/utils";

export interface AccordionItem {
  title: string;
  content: React.ReactNode;
}

export function Accordion({ items, defaultOpen = 0 }: { items: AccordionItem[]; defaultOpen?: number }) {
  const [open, setOpen] = useState<number | null>(defaultOpen);
  return (
    <div className="divide-y divide-travertine border-y border-travertine">
      {items.map((it, i) => (
        <div key={it.title}>
          <button
            onClick={() => setOpen(open === i ? null : i)}
            aria-expanded={open === i}
            className="flex w-full items-center justify-between py-5 text-left"
          >
            <span className="headline-sm">{it.title}</span>
            <ChevronDown size={18} strokeWidth={1.4} className={cn("transition-transform", open === i && "rotate-180")} />
          </button>
          <div className={cn("grid transition-all duration-300", open === i ? "grid-rows-[1fr] pb-6" : "grid-rows-[0fr]")}>
            <div className="overflow-hidden text-[14px] leading-6 text-umber">{it.content}</div>
          </div>
        </div>
      ))}
    </div>
  );
}
