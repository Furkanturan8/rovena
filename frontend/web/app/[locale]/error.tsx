"use client";

import { useDict } from "@/lib/i18n/provider";

export default function ErrorPage({ reset }: { error: Error; reset: () => void }) {
  const d = useDict();
  return (
    <div className="container-page flex min-h-[60vh] flex-col items-center justify-center py-24 text-center">
      <h1 className="headline-xl">{d.error.title}</h1>
      <p className="mt-5 max-w-md text-umber">{d.error.text}</p>
      <button onClick={reset} className="btn btn-primary mt-10">{d.error.retry}</button>
    </div>
  );
}
