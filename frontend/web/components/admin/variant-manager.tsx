"use client";

import { startTransition, useActionState, useRef } from "react";
import { createVariant, updateVariant, type AdminVariantActionState } from "@/lib/admin/product-actions";
import { centsToTl } from "@/lib/admin/product-form";
import type { ApiVariant } from "@/lib/api/types";
import { useDict, useLocale } from "@/lib/i18n/provider";
import { cn } from "@/lib/utils";

interface Props {
  productId: string;
  variants: ApiVariant[];
}

const idle: AdminVariantActionState = { status: "idle" };

function FieldError({ id, message }: { id: string; message?: string }) {
  if (!message) return null;
  return <p id={id} role="alert" className="mt-1 text-[12px] text-red-700">{message}</p>;
}

function Status({ state, pending }: { state: AdminVariantActionState; pending: boolean }) {
  const v = useDict().admin.productForm.variantAdmin;
  if (pending) return null;
  if (state.status === "error") return <p role="alert" className="text-[13px] text-red-700">{state.message}</p>;
  if (state.status === "success") return <p role="status" className="text-[13px] text-umber">{v.saved}</p>;
  return null;
}

/** Variants of an existing product: add, edit size/color/price, activate or deactivate. Variants are never deleted. */
export function VariantManager({ productId, variants }: Props) {
  const v = useDict().admin.productForm.variantAdmin;
  return (
    <section className="space-y-4 border border-travertine bg-paper p-6" aria-labelledby="pf-variants-title">
      <h3 id="pf-variants-title" className="label-caps text-[10px] text-umber">{v.title} ({variants.length})</h3>
      <p className="text-umber">{v.hint}</p>
      {variants.length === 0 ? (
        <p className="border border-dashed border-travertine p-4 text-umber">{v.empty}</p>
      ) : (
        <ul className="space-y-4">
          {variants.map((variant) => (
            <VariantRow key={variant.id} productId={productId} variant={variant} />
          ))}
        </ul>
      )}
      <NewVariant productId={productId} />
    </section>
  );
}

function VariantRow({ productId, variant }: { productId: string; variant: ApiVariant }) {
  const f = useDict().admin.productForm;
  const v = f.variantAdmin;
  const locale = useLocale();
  const [state, run, pending] = useActionState(updateVariant, idle);
  const fields = state.status === "error" && !pending ? (state.fields ?? {}) : {};
  const uid = variant.id;

  function send(form: FormData) {
    if (pending) return;
    form.set("locale", locale);
    form.set("productId", productId);
    form.set("variantId", variant.id);
    startTransition(() => run(form));
  }

  function onSave(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    send(new FormData(e.currentTarget));
  }

  function onToggle() {
    const form = new FormData();
    form.set("intent", "toggle");
    form.set("active", String(!variant.active));
    send(form);
  }

  return (
    <li className={cn("space-y-3 border border-travertine p-4", !variant.active && "bg-canvas")} aria-busy={pending}>
      <form onSubmit={onSave} noValidate className="space-y-3">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <p className="label-caps text-[10px]">
            <span className="font-mono">{variant.sku}</span> · {variant.active ? v.active : v.inactive}
          </p>
        </div>
        <div className="grid gap-3 md:grid-cols-4">
          <div>
            <label className="field-label" htmlFor={`vm-sku-${uid}`}>{f.sku}</label>
            <input id={`vm-sku-${uid}`} value={variant.sku} readOnly className="field font-mono text-umber" />
          </div>
          <div>
            <label className="field-label" htmlFor={`vm-size-${uid}`}>{f.size}</label>
            <input id={`vm-size-${uid}`} name="size" defaultValue={variant.size} maxLength={32} aria-invalid={!!fields.size} aria-describedby={fields.size ? `vm-size-${uid}-err` : undefined} className="field" />
            <FieldError id={`vm-size-${uid}-err`} message={fields.size} />
          </div>
          <div>
            <label className="field-label" htmlFor={`vm-color-${uid}`}>{f.color}</label>
            <input id={`vm-color-${uid}`} name="color" defaultValue={variant.color} maxLength={32} aria-invalid={!!fields.color} aria-describedby={fields.color ? `vm-color-${uid}-err` : undefined} className="field" />
            <FieldError id={`vm-color-${uid}-err`} message={fields.color} />
          </div>
          <div>
            <label className="field-label" htmlFor={`vm-price-${uid}`}>{f.price}</label>
            <input id={`vm-price-${uid}`} name="price" defaultValue={centsToTl(variant.price_cents)} inputMode="decimal" placeholder={f.pricePlaceholder} aria-invalid={!!fields.price_cents} aria-describedby={fields.price_cents ? `vm-price-${uid}-err` : undefined} className="field" />
            <FieldError id={`vm-price-${uid}-err`} message={fields.price_cents} />
          </div>
        </div>
        <div className="flex flex-wrap items-center gap-3">
          <button type="submit" disabled={pending} className="btn btn-primary">{pending ? v.saving : v.save}</button>
          <button type="button" disabled={pending} onClick={onToggle} className="btn btn-secondary">
            {pending ? v.working : variant.active ? v.deactivate : v.activate}
          </button>
          <Status state={state} pending={pending} />
        </div>
      </form>
    </li>
  );
}

function NewVariant({ productId }: { productId: string }) {
  const f = useDict().admin.productForm;
  const v = f.variantAdmin;
  const locale = useLocale();
  const formRef = useRef<HTMLFormElement>(null);
  const [state, run, pending] = useActionState(async (prev: AdminVariantActionState, form: FormData) => {
    const next = await createVariant(prev, form);
    if (next.status === "success") formRef.current?.reset();
    return next;
  }, idle);
  const fields = state.status === "error" && !pending ? (state.fields ?? {}) : {};

  function onSubmit(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    if (pending) return;
    const form = new FormData(e.currentTarget);
    form.set("locale", locale);
    form.set("productId", productId);
    startTransition(() => run(form));
  }

  return (
    <form ref={formRef} onSubmit={onSubmit} noValidate className="space-y-3 border border-travertine p-4" aria-busy={pending}>
      <p className="label-caps text-[10px]">{v.addTitle}</p>
      <div className="grid gap-3 md:grid-cols-4">
        <div>
          <label className="field-label" htmlFor="vm-new-sku">{f.sku}</label>
          <input id="vm-new-sku" name="sku" maxLength={64} required aria-invalid={!!fields.sku} aria-describedby={fields.sku ? "vm-new-sku-err" : undefined} className="field font-mono uppercase" />
          <FieldError id="vm-new-sku-err" message={fields.sku} />
        </div>
        <div>
          <label className="field-label" htmlFor="vm-new-size">{f.size}</label>
          <input id="vm-new-size" name="size" maxLength={32} aria-invalid={!!fields.size} aria-describedby={fields.size ? "vm-new-size-err" : undefined} className="field" />
          <FieldError id="vm-new-size-err" message={fields.size} />
        </div>
        <div>
          <label className="field-label" htmlFor="vm-new-color">{f.color}</label>
          <input id="vm-new-color" name="color" maxLength={32} aria-invalid={!!fields.color} aria-describedby={fields.color ? "vm-new-color-err" : undefined} className="field" />
          <FieldError id="vm-new-color-err" message={fields.color} />
        </div>
        <div>
          <label className="field-label" htmlFor="vm-new-price">{f.price}</label>
          <input id="vm-new-price" name="price" inputMode="decimal" placeholder={f.pricePlaceholder} required aria-invalid={!!fields.price_cents} aria-describedby={fields.price_cents ? "vm-new-price-err" : undefined} className="field" />
          <FieldError id="vm-new-price-err" message={fields.price_cents} />
        </div>
      </div>
      <div className="flex flex-wrap items-center gap-3">
        <button type="submit" disabled={pending} className="btn btn-secondary">{pending ? v.adding : v.add}</button>
        <Status state={state} pending={pending} />
      </div>
    </form>
  );
}
