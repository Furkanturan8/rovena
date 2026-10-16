"use client";

import { useDict } from "@/lib/i18n/provider";

export default function AdminProductEditError({ reset }: { error: Error; reset: () => void }) {
  const t = useDict().admin;
  return (
    <div role="alert" className="border border-travertine bg-paper p-8">
      <h2 className="headline-md">{t.products.errorTitle}</h2>
      <p className="mt-3 max-w-md text-umber">{t.products.errorText}</p>
      <button onClick={reset} className="btn btn-primary mt-6">{t.overview.retry}</button>
    </div>
  );
}
