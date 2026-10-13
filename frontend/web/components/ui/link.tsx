"use client";

import NextLink from "next/link";
import { useLocale } from "@/lib/i18n/provider";
import { withLocale } from "@/lib/i18n/config";

type Props = Omit<React.ComponentProps<typeof NextLink>, "href"> & { href: string };

/** next/link sarmalayıcısı: iç bağlantılara aktif dil önekini ekler. */
export function Link({ href, ...rest }: Props) {
  const locale = useLocale();
  const isInternal = href.startsWith("/") && !href.startsWith("//");
  return <NextLink href={isInternal ? withLocale(locale, href) : href} {...rest} />;
}
