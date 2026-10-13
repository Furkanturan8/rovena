"use client";

import { motion, type HTMLMotionProps } from "motion/react";
import { cn } from "@/lib/utils";

const ease = [0.2, 0.7, 0.2, 1] as const;

interface RevealProps extends Omit<HTMLMotionProps<"div">, "children"> {
  children: React.ReactNode;
  delay?: number;
  y?: number;
  once?: boolean;
}

export function Reveal({ children, delay = 0, y = 28, once = true, className, ...rest }: RevealProps) {
  return (
    <motion.div
      initial={{ opacity: 0, y }}
      whileInView={{ opacity: 1, y: 0 }}
      viewport={{ once, margin: "0px 0px -12% 0px" }}
      transition={{ duration: 0.9, delay, ease }}
      className={className}
      {...rest}
    >
      {children}
    </motion.div>
  );
}

export function Stagger({
  children,
  className,
  gap = 0.09,
  delay = 0,
}: {
  children: React.ReactNode;
  className?: string;
  gap?: number;
  delay?: number;
}) {
  return (
    <motion.div
      initial="hidden"
      whileInView="show"
      viewport={{ once: true, margin: "0px 0px -10% 0px" }}
      transition={{ staggerChildren: gap, delayChildren: delay }}
      className={className}
    >
      {children}
    </motion.div>
  );
}

export function StaggerItem({ children, className }: { children: React.ReactNode; className?: string }) {
  return (
    <motion.div
      variants={{ hidden: { opacity: 0, y: 32 }, show: { opacity: 1, y: 0, transition: { duration: 0.85, ease } } }}
      className={className}
    >
      {children}
    </motion.div>
  );
}

/** Görseller için perde açılışı: clip-path ile aşağıdan yukarı. */
export function ImageReveal({ children, className, delay = 0 }: { children: React.ReactNode; className?: string; delay?: number }) {
  return (
    <motion.div
      initial={{ clipPath: "inset(100% 0% 0% 0%)" }}
      whileInView={{ clipPath: "inset(0% 0% 0% 0%)" }}
      viewport={{ once: true, margin: "0px 0px -10% 0px" }}
      transition={{ duration: 1.2, delay, ease: [0.76, 0, 0.24, 1] }}
      className={cn(className)}
    >
      {children}
    </motion.div>
  );
}

/** Başlık kelimelerini tek tek açar. */
export function WordReveal({ text, className, delay = 0, as: Tag = "span" }: { text: string; className?: string; delay?: number; as?: "span" | "h1" | "h2" | "p" }) {
  const words = text.split(" ");
  const M = motion[Tag];
  return (
    <M
      initial="hidden"
      whileInView="show"
      viewport={{ once: true }}
      transition={{ staggerChildren: 0.07, delayChildren: delay }}
      className={className}
      aria-label={text}
    >
      {words.map((w, i) => (
        <span key={i} className="inline-block overflow-hidden align-bottom" aria-hidden>
          <motion.span
            className="inline-block"
            variants={{ hidden: { y: "110%" }, show: { y: "0%", transition: { duration: 0.9, ease } } }}
          >
            {w}
            {i < words.length - 1 ? " " : ""}
          </motion.span>
        </span>
      ))}
    </M>
  );
}
