"use client";

import { useDict } from "@/lib/i18n/provider";

export default function AdminAccountError({ reset }: { error: Error; reset: () => void }) {
  const a = useDict().admin.account;
  return (
    <div role="alert" className="border border-travertine bg-paper p-8">
      <h2 className="headline-md">{a.errorTitle}</h2>
      <p className="mt-3 max-w-md text-umber">{a.errorText}</p>
      <button onClick={reset} className="btn btn-primary mt-6">{a.retry}</button>
    </div>
  );
}
