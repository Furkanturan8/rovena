"use client";

import { useState, useTransition } from "react";
import { useRouter } from "next/navigation";
import { Plus } from "lucide-react";
import type { ApiAddress } from "@/lib/api/types";
import { deleteAddressAction, setDefaultAddressAction } from "@/lib/account/actions";
import { useDict, useLocale } from "@/lib/i18n/provider";
import { AddressForm } from "./address-form";

export function AddressBook({ addresses, fullName }: { addresses: ApiAddress[]; fullName: string }) {
  const d = useDict();
  const locale = useLocale();
  const t = d.account.addresses;
  const router = useRouter();
  const [, start] = useTransition();
  // "new" opens the empty form; an id opens that address for editing.
  const [editing, setEditing] = useState<string | "new" | null>(null);

  const done = () => {
    setEditing(null);
    router.refresh();
  };
  const run = (fn: () => Promise<{ ok: boolean }>) => start(async () => { await fn(); router.refresh(); });

  return (
    <div>
      <h2 className="headline-md mb-8">{t.pageTitle}</h2>
      {addresses.length === 0 && editing !== "new" && <p className="mb-6 text-umber">{t.empty}</p>}
      <div className="grid gap-4 sm:grid-cols-2">
        {addresses.map((a) =>
          editing === a.id ? (
            <article key={a.id} className="border border-ink bg-paper p-6 sm:col-span-2">
              <p className="label-caps mb-5 text-[10px]">{t.editTitle}</p>
              <AddressForm address={a} offerDefault onSaved={done} onCancel={() => setEditing(null)} />
            </article>
          ) : (
            <article key={a.id} className="flex flex-col border border-travertine bg-paper p-6">
              <div className="flex items-center justify-between">
                <p className="label-caps text-[10px]">{a.title}</p>
                {a.is_default && <span className="chip chip-solid">{t.default}</span>}
              </div>
              <p className="mt-5 text-[14px]">{a.full_name}</p>
              <p className="text-[13px] text-umber">{a.line1}</p>
              {a.line2 && <p className="text-[13px] text-umber">{a.line2}</p>}
              <p className="text-[13px] text-umber">{a.district} / {a.city} {a.postal_code}</p>
              <p className="mt-2 text-[13px] text-umber">{a.phone}</p>
              <div className="mt-6 flex flex-wrap gap-5 border-t border-travertine pt-4">
                <button onClick={() => setEditing(a.id)} className="text-[12px] underline underline-offset-4 hover:text-bronze-deep">{d.common.edit}</button>
                {!a.is_default && (
                  <button onClick={() => run(() => setDefaultAddressAction(locale, a.id))} className="text-[12px] text-umber underline underline-offset-4 hover:text-ink">{t.setDefault}</button>
                )}
                <button
                  onClick={() => window.confirm(t.confirmDelete) && run(() => deleteAddressAction(locale, a.id))}
                  className="text-[12px] text-umber underline underline-offset-4 hover:text-ink"
                >
                  {d.common.delete}
                </button>
              </div>
            </article>
          ),
        )}
        {editing === "new" ? (
          <article className="border border-ink bg-paper p-6 sm:col-span-2">
            <p className="label-caps mb-5 text-[10px]">{t.newTitle}</p>
            <AddressForm offerDefault={addresses.length > 0} defaultName={fullName} onSaved={done} onCancel={() => setEditing(null)} />
          </article>
        ) : (
          <button
            onClick={() => setEditing("new")}
            className="flex min-h-48 flex-col items-center justify-center gap-3 border border-dashed border-travertine text-umber transition-colors hover:border-ink hover:text-ink"
          >
            <Plus size={20} strokeWidth={1.4} />
            <span className="label-caps text-[10px]">{t.add}</span>
          </button>
        )}
      </div>
    </div>
  );
}
