import type { Metadata } from "next";
import { WishlistGrid } from "@/components/account/wishlist-grid";
import { getI18n } from "@/lib/i18n/server";

export async function generateMetadata({ params }: PageProps<"/[locale]/account/wishlist">): Promise<Metadata> {
  const { d } = await getI18n(params);
  return { title: d.account.wishlist.pageTitle };
}

export default async function WishlistPage({ params }: PageProps<"/[locale]/account/wishlist">) {
  const { d } = await getI18n(params);
  return (
    <div>
      <h2 className="headline-md mb-8">{d.account.wishlist.pageTitle}</h2>
      <WishlistGrid />
    </div>
  );
}
