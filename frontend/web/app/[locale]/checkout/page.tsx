import type { Metadata } from "next";
import { Link } from "@/components/ui/link";
import { CheckoutForm } from "@/components/checkout/checkout-form";
import { getI18n } from "@/lib/i18n/server";

export async function generateMetadata({ params }: PageProps<"/[locale]/checkout">): Promise<Metadata> {
  const { d } = await getI18n(params);
  return { title: d.checkout.pageTitle };
}

export default async function CheckoutPage({ params }: PageProps<"/[locale]/checkout">) {
  const { d } = await getI18n(params);
  return (
    <div className="container-page py-12 md:py-16">
      <div className="mb-12 flex items-center justify-between">
        <div>
          <p className="label-caps mb-3 text-umber">{d.checkout.eyebrow}</p>
          <h1 className="headline-xl">{d.checkout.title}</h1>
        </div>
        <Link href="/cart" className="link-underline hidden sm:inline-block">{d.checkout.backToCart}</Link>
      </div>
      <CheckoutForm />
    </div>
  );
}
