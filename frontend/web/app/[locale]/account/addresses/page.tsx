import type { Metadata } from "next";
import { AddressBook } from "@/components/account/address-book";
import { getI18n } from "@/lib/i18n/server";
import { requireUser } from "@/lib/auth/session";
import { getMyAddresses } from "@/lib/data/addresses";

export async function generateMetadata({ params }: PageProps<"/[locale]/account/addresses">): Promise<Metadata> {
  const { d } = await getI18n(params);
  return { title: d.account.addresses.pageTitle };
}

export default async function AddressesPage({ params }: PageProps<"/[locale]/account/addresses">) {
  const { locale } = await getI18n(params);
  const { user, token } = await requireUser(locale, `/${locale}/account/addresses`);
  const addresses = await getMyAddresses(token);
  return <AddressBook addresses={addresses} fullName={`${user.first_name} ${user.last_name}`.trim()} />;
}
