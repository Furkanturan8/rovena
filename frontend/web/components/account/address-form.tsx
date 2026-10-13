"use client";

import { useState, useTransition } from "react";
import { saveAddressAction } from "@/lib/account/actions";
import type { ApiAddress } from "@/lib/api/types";
import { useDict, useLocale } from "@/lib/i18n/provider";

interface Props {
  address?: ApiAddress;
  /** The first address is always the default, so the checkbox is only offered afterwards. */
  offerDefault: boolean;
  defaultName?: string;
  onSaved: (id: string) => void;
  onCancel?: () => void;
}

/** Not a <form>, so it can sit inside the checkout form without submitting it. */
export function AddressForm({ address, offerDefault, defaultName = "", onSaved, onCancel }: Props) {
  const d = useDict();
  const locale = useLocale();
  const t = d.account.addresses;
  const [pending, start] = useTransition();
  const [error, setError] = useState<string | null>(null);
  const [v, setV] = useState({
    title: address?.title ?? "",
    fullName: address?.full_name ?? defaultName,
    phone: address?.phone ?? "",
    line1: address?.line1 ?? "",
    line2: address?.line2 ?? "",
    district: address?.district ?? "",
    city: address?.city ?? "",
    postalCode: address?.postal_code ?? "",
    isDefault: address?.is_default ?? false,
  });
  const set = (k: keyof typeof v) => (e: React.ChangeEvent<HTMLInputElement>) => setV((o) => ({ ...o, [k]: e.target.value }));
  const complete = v.title.trim() && v.fullName.trim() && v.phone.trim().length >= 7 && v.line1.trim() && v.district.trim() && v.city.trim();

  function save() {
    setError(null);
    start(async () => {
      const res = await saveAddressAction(locale, address?.id ?? null, v);
      if (res.ok) onSaved(res.id);
      else setError(res.message);
    });
  }

  return (
    <div className="grid gap-4 sm:grid-cols-2">
      <div><label className="field-label">{t.titleLabel}</label><input value={v.title} onChange={set("title")} maxLength={40} placeholder={t.titlePlaceholder} className="field" /></div>
      <div><label className="field-label">{t.fullName}</label><input value={v.fullName} onChange={set("fullName")} maxLength={100} autoComplete="name" className="field" /></div>
      <div><label className="field-label">{t.phone}</label><input value={v.phone} onChange={set("phone")} maxLength={24} inputMode="tel" autoComplete="tel" className="field" /></div>
      <div><label className="field-label">{t.postalCode}</label><input value={v.postalCode} onChange={set("postalCode")} maxLength={16} autoComplete="postal-code" className="field" /></div>
      <div className="sm:col-span-2"><label className="field-label">{t.line1}</label><input value={v.line1} onChange={set("line1")} maxLength={200} autoComplete="address-line1" className="field" /></div>
      <div className="sm:col-span-2"><label className="field-label">{t.line2}</label><input value={v.line2} onChange={set("line2")} maxLength={200} autoComplete="address-line2" className="field" /></div>
      <div><label className="field-label">{t.district}</label><input value={v.district} onChange={set("district")} maxLength={80} className="field" /></div>
      <div><label className="field-label">{t.city}</label><input value={v.city} onChange={set("city")} maxLength={80} autoComplete="address-level1" className="field" /></div>
      {offerDefault && !address?.is_default && (
        <label className="flex cursor-pointer items-center gap-3 text-[13px] sm:col-span-2">
          <input type="checkbox" checked={v.isDefault} onChange={(e) => setV((o) => ({ ...o, isDefault: e.target.checked }))} className="h-4 w-4 appearance-none border border-ink checked:bg-ink" />
          {t.makeDefault}
        </label>
      )}
      {error && <p role="alert" className="text-[13px] text-danger sm:col-span-2">{error}</p>}
      <div className="flex gap-3 sm:col-span-2">
        <button type="button" onClick={save} disabled={pending || !complete} className="btn btn-primary">{t.save}</button>
        {onCancel && <button type="button" onClick={onCancel} className="btn btn-secondary">{t.cancel}</button>}
      </div>
    </div>
  );
}
