"use client";

import { Link } from "@/components/ui/link";
import { useDict } from "@/lib/i18n/provider";

export default function NotFound() {
  const d = useDict();
  return (
    <div className="container-page flex min-h-[60vh] flex-col items-center justify-center py-24 text-center">
      <p className="label-caps mb-4 text-umber">404</p>
      <h1 className="headline-xl">{d.notFound.title}</h1>
      <p className="mt-5 max-w-md text-umber">{d.notFound.text}</p>
      <div className="mt-10 flex flex-col gap-3 sm:flex-row">
        <Link href="/" className="btn btn-primary">{d.notFound.home}</Link>
        <Link href="/collection" className="btn btn-secondary">{d.notFound.collection}</Link>
      </div>
    </div>
  );
}
