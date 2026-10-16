"use client";

import { useDict } from "@/lib/i18n/provider";

export default function AdminError({ reset }: { error: Error; reset: () => void }) {
  const o = useDict().admin.overview;
  return (
    <div role="alert" className="border border-travertine bg-paper p-8">
      <h2 className="headline-md">{o.errorTitle}</h2>
      <p className="mt-3 max-w-md text-umber">{o.errorText}</p>
      <button onClick={reset} className="btn btn-primary mt-6">{o.retry}</button>
    </div>
  );
}
