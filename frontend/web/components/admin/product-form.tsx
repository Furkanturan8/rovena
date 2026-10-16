"use client";

import { startTransition, useActionState, useRef, useState } from "react";
import { Link } from "@/components/ui/link";
import type { AdminProductActionState } from "@/lib/admin/product-actions";
import {
  centsToTl,
  isHttpUrl,
  MAX_IMAGES,
  PRODUCT_STATUSES,
  tlToCents,
  validateProduct,
  type FieldErrors,
  type ProductPayload,
} from "@/lib/admin/product-form";
import type { ApiProductStatus } from "@/lib/api/types";
import type { CategoryOption } from "@/lib/data/admin-products";
import { useDict, useLocale } from "@/lib/i18n/provider";
import { cn } from "@/lib/utils";

export interface ProductFormValues {
  name: string;
  brand: string;
  categoryId: string;
  description: string;
  status: ApiProductStatus;
  enName: string;
  enDescription: string;
  attributes: { key: string; value: string }[];
  images: { url: string; alt: string }[];
  /** Create mode only; `priceCents` is shown in TL. */
  variants: { sku: string; size: string; color: string; priceCents: number }[];
}

type ProductFormAction = (prev: AdminProductActionState, form: FormData) => Promise<AdminProductActionState>;

interface Props {
  /** The server action that saves the form (create now; the edit page passes its own). */
  action: ProductFormAction;
  categories: CategoryOption[];
  mode?: "create" | "edit";
  initial?: Partial<ProductFormValues>;
  /** Extra hidden fields posted with the form (e.g. the product id in edit mode). */
  hidden?: Record<string, string>;
  cancelHref: string;
}

interface Row {
  id: number;
}
type AttrRow = Row & { key: string; value: string };
type ImageRow = Row & { url: string; alt: string };
type VariantRow = Row & { sku: string; size: string; color: string; price: string };

const idle: AdminProductActionState = { status: "idle" };
const KNOWN_FIELD = /^(name|brand|description|category_id|status|translations(\.en(\.(name|description))?)?|attributes(\[\d+\](\.(key|value))?)?|images(\[\d+\](\.(url|alt))?)?|variants(\[\d+\](\.(sku|size|color|price_cents))?)?)$/;

function move<T>(list: T[], from: number, to: number): T[] {
  if (to < 0 || to >= list.length) return list;
  const next = [...list];
  const [item] = next.splice(from, 1);
  next.splice(to, 0, item);
  return next;
}

function FieldError({ id, message }: { id: string; message?: string }) {
  if (!message) return null;
  return <p id={id} role="alert" className="mt-1 text-[12px] text-red-700">{message}</p>;
}

const iconBtn = "label-caps border border-travertine px-2 py-1 text-[10px] text-umber transition-colors hover:text-ink disabled:opacity-40 disabled:hover:text-umber";

export function ProductForm({ action, categories, mode = "create", initial, hidden, cancelHref }: Props) {
  const f = useDict().admin.productForm;
  const locale = useLocale();
  const [state, run, pending] = useActionState(action, idle);
  const nextId = useRef(1);
  const newId = () => nextId.current++;

  const [name, setName] = useState(initial?.name ?? "");
  const [brand, setBrand] = useState(initial?.brand ?? "");
  const [categoryId, setCategoryId] = useState(initial?.categoryId ?? "");
  const [description, setDescription] = useState(initial?.description ?? "");
  const [status, setStatus] = useState<ApiProductStatus>(initial?.status ?? "draft");
  const [enName, setEnName] = useState(initial?.enName ?? "");
  const [enDescription, setEnDescription] = useState(initial?.enDescription ?? "");
  const [attributes, setAttributes] = useState<AttrRow[]>(() => (initial?.attributes ?? []).map((a) => ({ ...a, id: newId() })));
  const [images, setImages] = useState<ImageRow[]>(() => (initial?.images ?? []).map((i) => ({ ...i, id: newId() })));
  const [variants, setVariants] = useState<VariantRow[]>(() =>
    mode === "create"
      ? (initial?.variants ?? [{ sku: "", size: "", color: "", priceCents: 0 }]).map((v) => ({
          id: newId(),
          sku: v.sku,
          size: v.size,
          color: v.color,
          price: v.priceCents > 0 ? centsToTl(v.priceCents) : "",
        }))
      : [],
  );
  const [clientErrors, setClientErrors] = useState<FieldErrors | null>(null);

  const serverFields = state.status === "error" && !pending ? (state.fields ?? {}) : {};
  const errors: FieldErrors = clientErrors ?? serverFields;
  const err = (key: string) => errors[key];
  const unmatched = Object.entries(errors).filter(([k]) => !KNOWN_FIELD.test(k));
  const banner = clientErrors
    ? Object.keys(clientErrors).length > 0
      ? f.messages.fixFields
      : null
    : state.status === "error" && !pending
      ? state.message
      : null;

  function update<T extends Row>(set: React.Dispatch<React.SetStateAction<T[]>>, id: number, patch: Partial<T>) {
    set((rows) => rows.map((r) => (r.id === id ? { ...r, ...patch } : r)));
  }

  function onSubmit(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    if (pending) return;
    const payload: ProductPayload = {
      name: name.trim(),
      brand: brand.trim(),
      categoryId,
      description: description.trim(),
      status,
      enName: enName.trim(),
      enDescription: enDescription.trim(),
      attributes: attributes.map((a) => ({ key: a.key.trim(), value: a.value.trim() })),
      images: images.map((i) => ({ url: i.url.trim(), alt: i.alt.trim() })),
      variants: variants.map((v) => ({
        sku: v.sku.trim(),
        size: v.size.trim(),
        color: v.color.trim(),
        priceCents: tlToCents(v.price) ?? 0,
      })),
    };
    const found = validateProduct(payload, f.errors);
    if (Object.keys(found).length > 0) {
      setClientErrors(found);
      return;
    }
    setClientErrors(null);
    const form = new FormData();
    form.set("locale", locale);
    form.set("payload", JSON.stringify(payload));
    for (const [k, v] of Object.entries(hidden ?? {})) form.set(k, v);
    startTransition(() => run(form));
  }

  return (
    <form onSubmit={onSubmit} noValidate className="space-y-8" aria-busy={pending}>
      {banner ? (
        <div role="alert" className="border border-red-700 bg-paper p-4 text-[14px] text-red-700">
          <p>{banner}</p>
          {unmatched.length > 0 ? (
            <ul className="mt-2 list-disc pl-5 text-[13px]">
              {unmatched.map(([k, v]) => (
                <li key={k}><span className="font-mono">{k}</span>: {v}</li>
              ))}
            </ul>
          ) : null}
        </div>
      ) : null}

      <section className="space-y-4 border border-travertine bg-paper p-6">
        <h3 className="label-caps text-[10px] text-umber">{f.sections.basics}</h3>
        <div className="grid gap-4 md:grid-cols-2">
          <div>
            <label className="field-label" htmlFor="pf-name">{f.name}</label>
            <input id="pf-name" value={name} onChange={(e) => setName(e.target.value)} maxLength={200} required aria-invalid={!!err("name")} aria-describedby={err("name") ? "pf-name-err" : undefined} className="field" />
            <FieldError id="pf-name-err" message={err("name")} />
          </div>
          <div>
            <label className="field-label" htmlFor="pf-brand">{f.brand}</label>
            <input id="pf-brand" value={brand} onChange={(e) => setBrand(e.target.value)} maxLength={100} aria-invalid={!!err("brand")} aria-describedby={err("brand") ? "pf-brand-err" : undefined} className="field" />
            <FieldError id="pf-brand-err" message={err("brand")} />
          </div>
          <div>
            <label className="field-label" htmlFor="pf-category">{f.category}</label>
            <select id="pf-category" value={categoryId} onChange={(e) => setCategoryId(e.target.value)} required aria-invalid={!!err("category_id")} aria-describedby={err("category_id") ? "pf-category-err" : undefined} className="field">
              <option value="">{f.categoryPlaceholder}</option>
              {categories.map((c) => (
                <option key={c.id} value={c.id}>{c.label}</option>
              ))}
            </select>
            <FieldError id="pf-category-err" message={err("category_id")} />
          </div>
          <div>
            <label className="field-label" htmlFor="pf-status">{f.status}</label>
            <select id="pf-status" value={status} onChange={(e) => setStatus(e.target.value as ApiProductStatus)} aria-invalid={!!err("status")} aria-describedby="pf-status-hint" className="field">
              {PRODUCT_STATUSES.map((s) => (
                <option key={s} value={s}>{f.statuses[s]}</option>
              ))}
            </select>
            <p id="pf-status-hint" className="mt-1 text-[12px] text-umber">{f.statusHint}</p>
            <FieldError id="pf-status-err" message={err("status")} />
          </div>
        </div>
      </section>

      <section className="space-y-4 border border-travertine bg-paper p-6">
        <h3 className="label-caps text-[10px] text-umber">{f.sections.translations}</h3>
        <p className="text-umber">{f.translationsHint}</p>
        <div className="grid gap-6 md:grid-cols-2">
          <div className="space-y-4">
            <p className="label-caps text-[10px]">{f.trTitle}</p>
            <p className="text-[13px] text-umber">{f.name}: {name || "—"}</p>
            <div>
              <label className="field-label" htmlFor="pf-description">{f.description}</label>
              <textarea id="pf-description" value={description} onChange={(e) => setDescription(e.target.value)} maxLength={10000} rows={6} aria-invalid={!!err("description")} aria-describedby={err("description") ? "pf-description-err" : undefined} className="field" />
              <FieldError id="pf-description-err" message={err("description")} />
            </div>
          </div>
          <div className="space-y-4">
            <p className="label-caps text-[10px]">{f.enTitle}</p>
            <div>
              <label className="field-label" htmlFor="pf-en-name">{f.enName}</label>
              <input id="pf-en-name" value={enName} onChange={(e) => setEnName(e.target.value)} maxLength={200} aria-invalid={!!err("translations.en.name")} aria-describedby={err("translations.en.name") ? "pf-en-name-err" : undefined} className="field" />
              <FieldError id="pf-en-name-err" message={err("translations.en.name") ?? err("translations.en") ?? err("translations")} />
            </div>
            <div>
              <label className="field-label" htmlFor="pf-en-description">{f.enDescription}</label>
              <textarea id="pf-en-description" value={enDescription} onChange={(e) => setEnDescription(e.target.value)} maxLength={10000} rows={6} aria-invalid={!!err("translations.en.description")} aria-describedby={err("translations.en.description") ? "pf-en-description-err" : undefined} className="field" />
              <FieldError id="pf-en-description-err" message={err("translations.en.description")} />
            </div>
          </div>
        </div>
      </section>

      <section className="space-y-4 border border-travertine bg-paper p-6">
        <h3 className="label-caps text-[10px] text-umber">{f.sections.attributes}</h3>
        <p className="text-umber">{f.attributesHint}</p>
        <FieldError id="pf-attributes-err" message={err("attributes")} />
        <ul className="space-y-3">
          {attributes.map((a, i) => (
            <li key={a.id} className="grid gap-2 md:grid-cols-[1fr_1fr_auto] md:items-start">
              <div>
                <label className="field-label" htmlFor={`pf-attr-key-${a.id}`}>{f.attributeKey}</label>
                <input id={`pf-attr-key-${a.id}`} value={a.key} onChange={(e) => update(setAttributes, a.id, { key: e.target.value })} maxLength={64} aria-invalid={!!err(`attributes[${i}].key`)} className="field" />
                <FieldError id={`pf-attr-key-${a.id}-err`} message={err(`attributes[${i}].key`)} />
              </div>
              <div>
                <label className="field-label" htmlFor={`pf-attr-value-${a.id}`}>{f.attributeValue}</label>
                <input id={`pf-attr-value-${a.id}`} value={a.value} onChange={(e) => update(setAttributes, a.id, { value: e.target.value })} maxLength={500} aria-invalid={!!err(`attributes[${i}].value`)} className="field" />
                <FieldError id={`pf-attr-value-${a.id}-err`} message={err(`attributes[${i}].value`)} />
              </div>
              <button type="button" onClick={() => setAttributes((rows) => rows.filter((r) => r.id !== a.id))} className={cn(iconBtn, "md:mt-6")}>{f.removeAttribute}</button>
            </li>
          ))}
        </ul>
        <button type="button" onClick={() => setAttributes((rows) => [...rows, { id: newId(), key: "", value: "" }])} className="btn btn-secondary">{f.addAttribute}</button>
      </section>

      <section className="space-y-4 border border-travertine bg-paper p-6">
        <h3 className="label-caps text-[10px] text-umber">{f.sections.images} ({images.length}/{MAX_IMAGES})</h3>
        <p className="text-umber">{f.imagesHint(MAX_IMAGES)}</p>
        <FieldError id="pf-images-err" message={err("images")} />
        <ul className="space-y-4">
          {images.map((img, i) => {
            const url = img.url.trim();
            return (
              <li key={img.id} className="grid gap-3 border border-travertine p-4 md:grid-cols-[72px_1fr_auto]">
                <div className="flex h-[90px] w-[72px] items-center justify-center overflow-hidden border border-travertine bg-canvas">
                  {isHttpUrl(url) ? (
                    // Arbitrary admin-entered hosts cannot go through next/image, hence a plain <img>.
                    // eslint-disable-next-line @next/next/no-img-element
                    <img src={url} alt={img.alt || f.previewAlt} className="h-full w-full object-cover" loading="lazy" />
                  ) : (
                    <span className="label-caps text-[9px] text-umber">{i + 1}</span>
                  )}
                </div>
                <div className="grid gap-3 md:grid-cols-2">
                  <div>
                    <label className="field-label" htmlFor={`pf-img-url-${img.id}`}>{f.imageN(i + 1)} · {f.imageUrl}</label>
                    <input id={`pf-img-url-${img.id}`} type="url" inputMode="url" value={img.url} onChange={(e) => update(setImages, img.id, { url: e.target.value })} maxLength={2048} placeholder="https://" aria-invalid={!!err(`images[${i}].url`)} aria-describedby={err(`images[${i}].url`) ? `pf-img-url-${img.id}-err` : undefined} className="field" />
                    <FieldError id={`pf-img-url-${img.id}-err`} message={err(`images[${i}].url`)} />
                  </div>
                  <div>
                    <label className="field-label" htmlFor={`pf-img-alt-${img.id}`}>{f.imageAlt}</label>
                    <input id={`pf-img-alt-${img.id}`} value={img.alt} onChange={(e) => update(setImages, img.id, { alt: e.target.value })} maxLength={200} aria-invalid={!!err(`images[${i}].alt`)} aria-describedby={err(`images[${i}].alt`) ? `pf-img-alt-${img.id}-err` : undefined} className="field" />
                    <FieldError id={`pf-img-alt-${img.id}-err`} message={err(`images[${i}].alt`)} />
                  </div>
                </div>
                <div className="flex flex-wrap gap-2 md:flex-col">
                  <button type="button" disabled={i === 0} onClick={() => setImages((rows) => move(rows, i, i - 1))} className={iconBtn}>{f.moveUp}</button>
                  <button type="button" disabled={i === images.length - 1} onClick={() => setImages((rows) => move(rows, i, i + 1))} className={iconBtn}>{f.moveDown}</button>
                  <button type="button" onClick={() => setImages((rows) => rows.filter((r) => r.id !== img.id))} className={iconBtn}>{f.removeImage}</button>
                </div>
              </li>
            );
          })}
        </ul>
        <button type="button" disabled={images.length >= MAX_IMAGES} onClick={() => setImages((rows) => [...rows, { id: newId(), url: "", alt: "" }])} className="btn btn-secondary">{f.addImage}</button>
      </section>

      {mode === "create" ? (
        <section className="space-y-4 border border-travertine bg-paper p-6">
          <h3 className="label-caps text-[10px] text-umber">{f.sections.variants}</h3>
          <p className="text-umber">{f.variantsHint}</p>
          <FieldError id="pf-variants-err" message={err("variants")} />
          <ul className="space-y-4">
            {variants.map((v, i) => (
              <li key={v.id} className="space-y-3 border border-travertine p-4">
                <div className="flex items-center justify-between gap-3">
                  <p className="label-caps text-[10px]">{f.variantN(i + 1)}</p>
                  <button type="button" disabled={variants.length <= 1} onClick={() => setVariants((rows) => rows.filter((r) => r.id !== v.id))} className={iconBtn}>{f.removeVariant}</button>
                </div>
                <div className="grid gap-3 md:grid-cols-4">
                  <div>
                    <label className="field-label" htmlFor={`pf-var-sku-${v.id}`}>{f.sku}</label>
                    <input id={`pf-var-sku-${v.id}`} value={v.sku} onChange={(e) => update(setVariants, v.id, { sku: e.target.value })} maxLength={64} required aria-invalid={!!err(`variants[${i}].sku`)} aria-describedby={err(`variants[${i}].sku`) ? `pf-var-sku-${v.id}-err` : undefined} className="field font-mono uppercase" />
                    <FieldError id={`pf-var-sku-${v.id}-err`} message={err(`variants[${i}].sku`)} />
                  </div>
                  <div>
                    <label className="field-label" htmlFor={`pf-var-size-${v.id}`}>{f.size}</label>
                    <input id={`pf-var-size-${v.id}`} value={v.size} onChange={(e) => update(setVariants, v.id, { size: e.target.value })} maxLength={32} aria-invalid={!!err(`variants[${i}].size`)} aria-describedby={err(`variants[${i}].size`) ? `pf-var-size-${v.id}-err` : undefined} className="field" />
                    <FieldError id={`pf-var-size-${v.id}-err`} message={err(`variants[${i}].size`)} />
                  </div>
                  <div>
                    <label className="field-label" htmlFor={`pf-var-color-${v.id}`}>{f.color}</label>
                    <input id={`pf-var-color-${v.id}`} value={v.color} onChange={(e) => update(setVariants, v.id, { color: e.target.value })} maxLength={32} aria-invalid={!!err(`variants[${i}].color`)} aria-describedby={err(`variants[${i}].color`) ? `pf-var-color-${v.id}-err` : undefined} className="field" />
                    <FieldError id={`pf-var-color-${v.id}-err`} message={err(`variants[${i}].color`)} />
                  </div>
                  <div>
                    <label className="field-label" htmlFor={`pf-var-price-${v.id}`}>{f.price}</label>
                    <input id={`pf-var-price-${v.id}`} value={v.price} onChange={(e) => update(setVariants, v.id, { price: e.target.value })} inputMode="decimal" placeholder={f.pricePlaceholder} required aria-invalid={!!err(`variants[${i}].price_cents`)} aria-describedby={err(`variants[${i}].price_cents`) ? `pf-var-price-${v.id}-err` : undefined} className="field" />
                    <FieldError id={`pf-var-price-${v.id}-err`} message={err(`variants[${i}].price_cents`)} />
                  </div>
                </div>
              </li>
            ))}
          </ul>
          <button type="button" onClick={() => setVariants((rows) => [...rows, { id: newId(), sku: "", size: "", color: "", price: "" }])} className="btn btn-secondary">{f.addVariant}</button>
        </section>
      ) : null}

      <div className="flex flex-wrap items-center gap-3">
        <button type="submit" disabled={pending} className="btn btn-primary">{pending ? f.submitting : f.submit}</button>
        <Link href={cancelHref} className="btn btn-secondary">{f.cancel}</Link>
      </div>
    </form>
  );
}
