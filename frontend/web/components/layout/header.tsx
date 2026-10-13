"use client";

import { usePathname } from "next/navigation";
import { useEffect, useRef, useState } from "react";
import { AnimatePresence, motion } from "motion/react";
import { Heart, Menu, Search, ShoppingBag, User, X } from "lucide-react";
import { Link } from "@/components/ui/link";
import { Logo } from "./logo";
import { LanguageSwitcher } from "./language-switcher";
import { MegaMenu } from "./mega-menu";
import { SearchOverlay } from "./search-overlay";
import { useStore } from "@/components/cart/store";
import { splitLocale } from "@/lib/i18n/config";
import { useDict } from "@/lib/i18n/provider";
import { cn } from "@/lib/utils";

const heroRoutes = /^\/($|gallery|about)/;

export function Header() {
  const d = useDict();
  const { count, wishlist, openDrawer, hydrated } = useStore();
  const rawPath = usePathname();
  const pathname = splitLocale(rawPath).rest;
  const nav = [
    { label: d.nav.collection, href: "/collection", mega: true },
    { label: d.nav.gallery, href: "/gallery" },
    { label: d.nav.about, href: "/about" },
  ];
  const [mega, setMega] = useState(false);
  const [mobile, setMobile] = useState(false);
  const [search, setSearch] = useState(false);
  const [scrolled, setScrolled] = useState(false);
  const [hidden, setHidden] = useState(false);
  const lastY = useRef(0);

  const hasHero = heroRoutes.test(pathname);
  const overlay = hasHero && !scrolled && !mega && !search;

  useEffect(() => {
    const onScroll = () => {
      const y = window.scrollY;
      const dy = y - lastY.current;
      setScrolled(y > 8);
      if (y < 480) setHidden(false);
      else if (dy > 6) setHidden(true);
      else if (dy < -6) setHidden(false);
      lastY.current = y;
    };
    onScroll();
    window.addEventListener("scroll", onScroll, { passive: true });
    return () => window.removeEventListener("scroll", onScroll);
  }, []);

  useEffect(() => {
    document.body.style.overflow = mobile ? "hidden" : "";
    return () => {
      document.body.style.overflow = "";
    };
  }, [mobile]);

  return (
    <>
      <header
        className={cn(
          "sticky top-0 z-40 border-b transition-[transform,background-color,border-color,color] duration-500 ease-[cubic-bezier(0.2,0.7,0.2,1)]",
          hasHero && "-mb-[73px]",
          overlay ? "border-transparent bg-transparent text-canvas" : "border-travertine bg-canvas/95 text-ink backdrop-blur-md",
          !scrolled && !hasHero && "border-transparent",
          hidden && !mega && !search && "-translate-y-full",
        )}
        onMouseLeave={() => setMega(false)}
      >
        <div className="container-page grid h-[72px] grid-cols-[1fr_auto_1fr] items-center">
          <nav className="hidden items-center gap-9 lg:flex" aria-label={d.nav.mainMenu}>
            {nav.map((n) => (
              <Link
                key={n.href}
                href={n.href}
                onMouseEnter={() => setMega(!!n.mega)}
                onFocus={() => setMega(!!n.mega)}
                aria-current={pathname.startsWith(n.href) ? "page" : undefined}
                className="label-caps relative py-2 after:absolute after:inset-x-0 after:bottom-0 after:h-px after:origin-left after:scale-x-0 after:bg-current after:transition-transform after:duration-500 hover:after:scale-x-100 aria-[current=page]:after:scale-x-100"
              >
                {n.label}
              </Link>
            ))}
          </nav>
          <button className="-ml-2 p-2 lg:hidden" aria-label={d.header.openMenu} onClick={() => setMobile(true)}>
            <Menu size={22} strokeWidth={1.4} />
          </button>

          <Logo tone={overlay ? "light" : "dark"} />

          <div className="flex items-center justify-end gap-1 sm:gap-2">
            <LanguageSwitcher className="mr-2 hidden md:flex" />
            <button className="p-2 transition-opacity hover:opacity-60" aria-label={d.header.search} onClick={() => setSearch(true)}>
              <Search size={20} strokeWidth={1.4} />
            </button>
            <Link href="/account/wishlist" className="relative hidden p-2 transition-opacity hover:opacity-60 sm:block" aria-label={d.header.wishlist}>
              <Heart size={20} strokeWidth={1.4} />
              {hydrated && wishlist.length > 0 && <span className="absolute right-0.5 top-0.5 h-1.5 w-1.5 rounded-full bg-bronze" />}
            </Link>
            <Link href="/login" className="hidden p-2 transition-opacity hover:opacity-60 sm:block" aria-label={d.header.account}>
              <User size={20} strokeWidth={1.4} />
            </Link>
            <button className="relative p-2 transition-opacity hover:opacity-60" aria-label={d.header.cart(count)} onClick={openDrawer}>
              <ShoppingBag size={20} strokeWidth={1.4} />
              <AnimatePresence>
                {hydrated && count > 0 && (
                  <motion.span
                    key={count}
                    initial={{ scale: 0.4, opacity: 0 }}
                    animate={{ scale: 1, opacity: 1 }}
                    exit={{ scale: 0.4, opacity: 0 }}
                    transition={{ type: "spring", stiffness: 500, damping: 18 }}
                    className={cn(
                      "absolute -right-0.5 -top-0.5 flex h-4 min-w-4 items-center justify-center px-1 text-[9px] font-semibold",
                      overlay ? "bg-canvas text-ink" : "bg-ink text-canvas",
                    )}
                  >
                    {count}
                  </motion.span>
                )}
              </AnimatePresence>
            </button>
          </div>
        </div>
        <MegaMenu open={mega} onNavigate={() => setMega(false)} />
      </header>

      <SearchOverlay open={search} onClose={() => setSearch(false)} />

      <AnimatePresence>
        {mobile && (
          <motion.div className="fixed inset-0 z-50 flex" initial={{ opacity: 0 }} animate={{ opacity: 1 }} exit={{ opacity: 0 }}>
            <motion.div
              initial={{ x: "-100%" }}
              animate={{ x: 0 }}
              exit={{ x: "-100%" }}
              transition={{ duration: 0.5, ease: [0.2, 0.7, 0.2, 1] }}
              className="flex h-full w-[86%] max-w-sm flex-col bg-canvas text-ink"
            >
              <div className="flex h-[72px] items-center justify-between border-b border-travertine px-5">
                <Logo />
                <button aria-label={d.common.close} onClick={() => setMobile(false)} className="p-2">
                  <X size={22} strokeWidth={1.4} />
                </button>
              </div>
              <nav className="flex flex-1 flex-col overflow-y-auto px-5 py-6">
                {nav.map((n, i) => (
                  <motion.div key={n.href} initial={{ opacity: 0, x: -16 }} animate={{ opacity: 1, x: 0 }} transition={{ delay: 0.12 + i * 0.06 }}>
                    <Link href={n.href} onClick={() => setMobile(false)} className="headline-md block border-b border-travertine py-4">
                      {n.label}
                    </Link>
                  </motion.div>
                ))}
                <div className="mt-8 flex flex-col gap-4">
                  <Link href="/account" onClick={() => setMobile(false)} className="label-caps">{d.header.myAccount}</Link>
                  <Link href="/account/wishlist" onClick={() => setMobile(false)} className="label-caps">{d.header.myWishlist}</Link>
                  <Link href="/account/orders" onClick={() => setMobile(false)} className="label-caps">{d.header.myOrders}</Link>
                  <Link href="/appointment" onClick={() => setMobile(false)} className="label-caps text-bronze-deep">{d.nav.appointment}</Link>
                  <LanguageSwitcher className="pt-2" />
                </div>
              </nav>
            </motion.div>
            <button aria-label={d.common.close} className="flex-1 bg-ink/40" onClick={() => setMobile(false)} />
          </motion.div>
        )}
      </AnimatePresence>
    </>
  );
}
