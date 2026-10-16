"use client";

import { useEffect, useRef, useState } from "react";
import { Link } from "@/components/ui/link";
import { logoutAction } from "@/lib/auth/actions";
import { useDict, useLocale } from "@/lib/i18n/provider";

export function AdminAccountMenu() {
  const m = useDict().admin.menu;
  const locale = useLocale();
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!open) return;
    const onPointer = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false);
    };
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") setOpen(false);
    };
    document.addEventListener("mousedown", onPointer);
    document.addEventListener("keydown", onKey);
    return () => {
      document.removeEventListener("mousedown", onPointer);
      document.removeEventListener("keydown", onKey);
    };
  }, [open]);

  return (
    <div ref={ref} className="relative">
      <button
        type="button"
        onClick={() => setOpen((v) => !v)}
        aria-expanded={open}
        aria-haspopup="menu"
        className="label-caps border border-travertine px-4 py-2.5 text-[10px] text-ink transition-colors hover:border-ink"
      >
        {m.account}
      </button>
      {open && (
        <div role="menu" className="absolute right-0 top-full z-50 mt-2 min-w-48 border border-travertine bg-canvas py-1">
          <Link
            href="/admin/account"
            role="menuitem"
            onClick={() => setOpen(false)}
            className="label-caps block px-4 py-3 text-[10px] text-umber transition-colors hover:text-ink"
          >
            {m.myAccount}
          </Link>
          <form action={logoutAction}>
            <input type="hidden" name="locale" value={locale} />
            <input type="hidden" name="to" value="login" />
            <button role="menuitem" className="label-caps block w-full px-4 py-3 text-left text-[10px] text-umber transition-colors hover:text-ink">
              {m.logout}
            </button>
          </form>
        </div>
      )}
    </div>
  );
}
