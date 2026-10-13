import type { Metadata } from "next";
import { notFound } from "next/navigation";
import { Link } from "@/components/ui/link";
import { OrderProgress } from "@/components/checkout/order-progress";
import { getI18n } from "@/lib/i18n/server";
import { requireUser } from "@/lib/auth/session";
import { getMyOrder } from "@/lib/data/orders";

export async function generateMetadata({ params }: PageProps<"/[locale]/order-confirmation/[number]">): Promise<Metadata> {
  const { d } = await getI18n(params);
  return { title: d.confirmation.pageTitle };
}

// The route segment is the order id returned by the order service.
export default async function OrderConfirmationPage({ params }: PageProps<"/[locale]/order-confirmation/[number]">) {
  const { locale, d } = await getI18n(params);
  const { number: id } = await params;
  const { token } = await requireUser(locale, `/${locale}/order-confirmation/${id}`);
  const order = await getMyOrder(token, id);
  if (!order) notFound();
  const c = d.confirmation;

  return (
    <div className="container-page py-16 md:py-24">
      <div className="mx-auto max-w-2xl text-center">
        <OrderProgress orderId={order.id} initial={order} />

        <div className="mt-14 flex flex-col justify-center gap-3 sm:flex-row">
          <Link href="/account/orders" className="btn btn-primary">{c.viewOrders}</Link>
          <Link href="/collection" className="btn btn-secondary">{c.continueShopping}</Link>
        </div>
      </div>
    </div>
  );
}
