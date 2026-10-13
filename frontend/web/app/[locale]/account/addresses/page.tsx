import type { Metadata } from "next";
import { Plus } from "lucide-react";
import { addresses } from "@/lib/mock/account";
import { getI18n } from "@/lib/i18n/server";

export async function generateMetadata({ params }: PageProps<"/[locale]/account/addresses">): Promise<Metadata> {
  const { d } = await getI18n(params);
  return { title: d.account.addresses.pageTitle };
}

export default async function AddressesPage({ params }: PageProps<"/[locale]/account/addresses">) {
  const { d } = await getI18n(params);
  const t = d.account.addresses;
  return (
    <div>
      <h2 className="headline-md mb-8">{t.pageTitle}</h2>
      <div className="grid gap-4 sm:grid-cols-2">
        {addresses.map((a) => (
          <article key={a.id} className="flex flex-col border border-travertine bg-paper p-6">
            <div className="flex items-center justify-between">
              <p className="label-caps text-[10px]">{t.titles[a.title]}</p>
              {a.isDefault && <span className="chip chip-solid">{t.default}</span>}
            </div>
            <p className="mt-5 text-[14px]">{a.name}</p>
            {a.lines.map((l) => <p key={l} className="text-[13px] text-umber">{l}</p>)}
            <p className="mt-2 text-[13px] text-umber">{a.phone}</p>
            <div className="mt-6 flex gap-5 border-t border-travertine pt-4">
              <button className="text-[12px] underline underline-offset-4 hover:text-bronze-deep">{d.common.edit}</button>
              <button className="text-[12px] text-umber underline underline-offset-4 hover:text-ink">{d.common.delete}</button>
            </div>
          </article>
        ))}
        <button className="flex min-h-48 flex-col items-center justify-center gap-3 border border-dashed border-travertine text-umber transition-colors hover:border-ink hover:text-ink">
          <Plus size={20} strokeWidth={1.4} />
          <span className="label-caps text-[10px]">{t.add}</span>
        </button>
      </div>
    </div>
  );
}
