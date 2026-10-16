import type { Metadata } from "next";
import { notFound } from "next/navigation";
import { Playfair_Display, Plus_Jakarta_Sans } from "next/font/google";
import "../globals.css";
import { I18nProvider } from "@/lib/i18n/provider";
import { isLocale, locales } from "@/lib/i18n/config";
import { getDictionary } from "@/lib/i18n/get-dictionary";
import { MotionProvider } from "@/components/motion/motion-provider";
import { StoreProvider } from "@/components/cart/store";
import { CartDrawer } from "@/components/cart/cart-drawer";
import { AnnouncementBar } from "@/components/layout/announcement-bar";
import { Header } from "@/components/layout/header";
import { Footer } from "@/components/layout/footer";
import { StoreShell } from "@/components/layout/store-shell";
import { getCategoriesSafe } from "@/lib/data/catalog";

const playfair = Playfair_Display({
  variable: "--font-playfair",
  subsets: ["latin", "latin-ext"],
  style: ["normal", "italic"],
});

const jakarta = Plus_Jakarta_Sans({
  variable: "--font-jakarta",
  subsets: ["latin", "latin-ext"],
});

export function generateStaticParams() {
  return locales.map((locale) => ({ locale }));
}

export async function generateMetadata({ params }: LayoutProps<"/[locale]">): Promise<Metadata> {
  const { locale } = await params;
  if (!isLocale(locale)) return {};
  const d = await getDictionary(locale);
  return {
    title: { default: d.meta.title, template: d.meta.titleTemplate },
    description: d.meta.description,
    alternates: { languages: { tr: "/tr", en: "/en" } },
  };
}

export default async function RootLayout({ children, params }: LayoutProps<"/[locale]">) {
  const { locale } = await params;
  if (!isLocale(locale)) notFound();
  const d = await getDictionary(locale);
  const categories = await getCategoriesSafe(locale);

  return (
    <html lang={locale} className={`${playfair.variable} ${jakarta.variable}`}>
      <body className="flex min-h-screen flex-col">
        <I18nProvider locale={locale}>
          <MotionProvider>
            <StoreProvider>
              <StoreShell
                locale={locale}
                announcement={<AnnouncementBar messages={d.announcement} />}
                header={<Header categories={categories} />}
                footer={<Footer d={d} categories={categories} />}
                cartDrawer={<CartDrawer />}
              >
                {children}
              </StoreShell>
            </StoreProvider>
          </MotionProvider>
        </I18nProvider>
      </body>
    </html>
  );
}
