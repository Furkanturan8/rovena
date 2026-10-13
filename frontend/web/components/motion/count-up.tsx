"use client";

import { useEffect, useRef, useState } from "react";
import { animate, useInView, useReducedMotion } from "motion/react";

export function CountUp({ to, prefix = "", suffix = "", duration = 1.8 }: { to: number; prefix?: string; suffix?: string; duration?: number }) {
  const ref = useRef<HTMLSpanElement>(null);
  const inView = useInView(ref, { once: true, margin: "0px 0px -10% 0px" });
  const reduce = useReducedMotion();
  const [val, setVal] = useState(reduce ? to : 0);

  useEffect(() => {
    if (!inView || reduce) return;
    const controls = animate(0, to, { duration, ease: [0.2, 0.7, 0.2, 1], onUpdate: (v) => setVal(Math.round(v)) });
    return () => controls.stop();
  }, [inView, reduce, to, duration]);

  return (
    <span ref={ref}>
      {prefix}
      {val}
      {suffix}
    </span>
  );
}
