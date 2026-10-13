import type { Metadata } from "next";
import { CartView } from "@/components/cart/cart-view";
import { getI18n } from "@/lib/i18n/server";

export async function generateMetadata({ params }: PageProps<"/[locale]/cart">): Promise<Metadata> {
  const { d } = await getI18n(params);
  return { title: d.cart.title };
}

export default async function CartPage({ params }: PageProps<"/[locale]/cart">) {
  const { d } = await getI18n(params);
  return (
    <div className="container-page py-12 md:py-16">
      <p className="label-caps mb-3 text-umber">{d.cart.eyebrow}</p>
      <h1 className="headline-xl mb-12">{d.cart.title}</h1>
      <CartView />
    </div>
  );
}
