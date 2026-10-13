"use client";

import { useState } from "react";
import { useDict } from "@/lib/i18n/provider";
import { cn } from "@/lib/utils";

export function NewsletterForm({ tone = "light" }: { tone?: "light" | "dark" }) {
  const d = useDict();
  const [done, setDone] = useState(false);
  if (done) {
    return <p className="label-caps text-bronze-deep">{d.footer.thanks}</p>;
  }
  return (
    <form
      onSubmit={(e) => {
        e.preventDefault();
        setDone(true);
      }}
      className="flex w-full max-w-md flex-col gap-3 sm:flex-row"
    >
      <input
        type="email"
        required
        placeholder={d.footer.emailPlaceholder}
        aria-label={d.footer.emailPlaceholder}
        className={cn("field flex-1", tone === "dark" && "border-canvas/30 text-canvas placeholder:text-canvas/40 focus:border-canvas")}
      />
      <button className={cn("btn", tone === "dark" ? "btn-light" : "btn-primary")}>{d.footer.subscribe}</button>
    </form>
  );
}
