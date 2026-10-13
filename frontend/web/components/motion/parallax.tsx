"use client";

import { useRef } from "react";
import { motion, useScroll, useTransform } from "motion/react";
import { cn } from "@/lib/utils";

/** İçeriği kaydırmaya bağlı olarak dikeyde hafifçe kaydırır; kapsayıcıda overflow-hidden olmalı. */
export function Parallax({ children, className, offset = 8, scale = 1.12 }: { children: React.ReactNode; className?: string; offset?: number; scale?: number }) {
  const ref = useRef<HTMLDivElement>(null);
  const { scrollYProgress } = useScroll({ target: ref, offset: ["start end", "end start"] });
  const y = useTransform(scrollYProgress, [0, 1], [`-${offset}%`, `${offset}%`]);
  return (
    <div ref={ref} className={cn("absolute inset-0 overflow-hidden", className)}>
      <motion.div style={{ y, scale }} className="absolute inset-0">
        {children}
      </motion.div>
    </div>
  );
}
