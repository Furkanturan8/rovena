import type { Metadata } from "next";
import { Check, Loader2 } from "lucide-react";
import { Link } from "@/components/ui/link";
import { getI18n } from "@/lib/i18n/server";

export async function generateMetadata({ params }: PageProps<"/[locale]/order-confirmation/[number]">): Promise<Metadata> {
  const { d } = await getI18n(params);
  return { title: d.confirmation.pageTitle };
}

export default async function OrderConfirmationPage({ params }: PageProps<"/[locale]/order-confirmation/[number]">) {
  const { d } = await getI18n(params);
  const { number } = await params;
  const c = d.confirmation;
  return (
    <div className="container-page py-16 md:py-24">
      <div className="mx-auto max-w-2xl text-center">
        <span className="mx-auto flex h-14 w-14 items-center justify-center border border-ink"><Check size={22} strokeWidth={1.4} /></span>
        <p className="label-caps mt-8 text-umber">{c.eyebrow}</p>
        <h1 className="headline-xl mt-3">{c.title}</h1>
        <p className="mt-5 text-[15px] leading-7 text-umber">{c.text}</p>
        <p className="label-mono mt-8 inline-block border border-travertine bg-paper px-5 py-3">{c.orderNo(number)}</p>

        <ol className="mx-auto mt-12 max-w-md space-y-0 text-left">
          {c.steps.map((s, i) => (
            <li key={s.label} className="relative flex gap-5 pb-8 last:pb-0">
              {i < c.steps.length - 1 && <span aria-hidden className="absolute left-[15px] top-8 h-[calc(100%-2rem)] w-px bg-travertine" />}
              <span className={`flex h-8 w-8 shrink-0 items-center justify-center border ${s.done ? "border-ink bg-ink text-canvas" : "border-travertine bg-canvas"}`}>
                {s.done ? <Check size={14} /> : <Loader2 size={14} className="animate-spin text-umber" />}
              </span>
              <div>
                <p className="text-[14px] font-medium">{s.label}</p>
                <p className="text-[12px] text-umber">{s.note}</p>
              </div>
            </li>
          ))}
        </ol>

        <div className="mt-14 flex flex-col justify-center gap-3 sm:flex-row">
          <Link href="/account/orders" className="btn btn-primary">{c.viewOrders}</Link>
          <Link href="/collection" className="btn btn-secondary">{c.continueShopping}</Link>
        </div>
      </div>
    </div>
  );
}
