"use client";

import { useState } from "react";
import { MapPin, Phone } from "lucide-react";
import { Reveal } from "@/components/motion/reveal";
import { useDict } from "@/lib/i18n/provider";

export function AppointmentSection() {
  const d = useDict();
  const a = d.appointment;
  const [sent, setSent] = useState(false);
  return (
    <section id="appointment" className="bg-ink py-20 text-canvas md:py-28">
      <div className="container-page grid gap-14 lg:grid-cols-2">
        <Reveal>
          <p className="label-caps mb-4 text-bronze">{a.eyebrow}</p>
          <h2 className="headline-xl">{a.title}</h2>
          <p className="mt-6 max-w-lg text-[15px] leading-7 text-canvas/70">{a.description}</p>
          <ul className="mt-10 space-y-5">
            {a.stores.map((s) => (
              <li key={s.city} className="flex gap-4 border-t border-canvas/15 pt-5">
                <MapPin size={18} strokeWidth={1.4} className="mt-0.5 shrink-0 text-bronze" />
                <div>
                  <p className="headline-sm">{s.city} · {s.place}</p>
                  <p className="text-[13px] text-canvas/60">{s.addr}</p>
                </div>
              </li>
            ))}
          </ul>
          <a href="tel:+902120000000" className="mt-10 inline-flex items-center gap-3 border border-canvas/25 px-5 py-4 hover:border-canvas">
            <Phone size={16} strokeWidth={1.4} className="text-bronze" />
            <span className="label-caps text-[10px]">{a.hotline}</span>
          </a>
        </Reveal>

        <form
          onSubmit={(e) => {
            e.preventDefault();
            setSent(true);
          }}
          className="bg-canvas p-6 text-ink md:p-10"
        >
          {sent ? (
            <div className="flex min-h-[420px] flex-col items-center justify-center text-center">
              <p className="headline-md">{a.doneTitle}</p>
              <p className="mt-3 max-w-xs text-umber">{a.doneText}</p>
            </div>
          ) : (
            <>
              <h3 className="headline-md">{a.formTitle}</h3>
              <p className="mt-2 text-[13px] text-umber">{a.formNote}</p>
              <div className="mt-8 grid gap-5 sm:grid-cols-2">
                <div className="sm:col-span-2">
                  <label className="field-label" htmlFor="ap-store">{a.store}</label>
                  <select id="ap-store" className="field">
                    {a.stores.map((s) => <option key={s.city}>{s.city} · {s.place}</option>)}
                  </select>
                </div>
                <div>
                  <label className="field-label" htmlFor="ap-name">{a.name}</label>
                  <input id="ap-name" required className="field" placeholder={a.namePlaceholder} />
                </div>
                <div>
                  <label className="field-label" htmlFor="ap-phone">{a.phone}</label>
                  <input id="ap-phone" required className="field" placeholder="+90" />
                </div>
                <div>
                  <label className="field-label" htmlFor="ap-email">{a.email}</label>
                  <input id="ap-email" type="email" required className="field" placeholder="name@example.com" />
                </div>
                <div>
                  <label className="field-label" htmlFor="ap-date">{a.date}</label>
                  <input id="ap-date" type="date" className="field" />
                </div>
                <div className="sm:col-span-2">
                  <label className="field-label" htmlFor="ap-note">{a.interest}</label>
                  <textarea id="ap-note" rows={3} className="field" placeholder={a.interestPlaceholder} />
                </div>
              </div>
              <button className="btn btn-primary mt-8 w-full">{a.submit}</button>
            </>
          )}
        </form>
      </div>
    </section>
  );
}
