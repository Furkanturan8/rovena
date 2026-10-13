import { notFound } from "next/navigation";
import { Hero } from "@/components/home/hero";
import { CategoryStrip } from "@/components/home/category-strip";
import { CapsuleSection } from "@/components/home/capsule-section";
import { FeaturedProducts } from "@/components/home/featured-products";
import { FilmSection } from "@/components/home/film-section";
import { ValuesSection } from "@/components/home/values-section";
import { CraftSection } from "@/components/home/craft-section";
import { AppointmentSection } from "@/components/home/appointment-section";
import { isLocale } from "@/lib/i18n/config";
import { getDictionary } from "@/lib/i18n/get-dictionary";

export default async function HomePage({ params }: PageProps<"/[locale]">) {
  const { locale } = await params;
  if (!isLocale(locale)) notFound();
  const d = await getDictionary(locale);
  return (
    <>
      <Hero />
      <CategoryStrip d={d} />
      <CapsuleSection d={d} />
      <FeaturedProducts />
      <FilmSection d={d} />
      <ValuesSection d={d} />
      <CraftSection d={d} />
      <AppointmentSection />
    </>
  );
}
