"use client";

import type { ReactNode } from "react";
import { usePathname } from "next/navigation";

type StoreShellProps = {
  locale: string;
  announcement: ReactNode;
  header: ReactNode;
  footer: ReactNode;
  cartDrawer: ReactNode;
  children: ReactNode;
};

export function StoreShell({
  locale,
  announcement,
  header,
  footer,
  cartDrawer,
  children,
}: StoreShellProps) {
  const pathname = usePathname();
  const adminRoot = `/${locale}/admin`;
  const isAdmin = pathname === adminRoot || pathname.startsWith(`${adminRoot}/`);

  if (isAdmin) return <>{children}</>;

  return (
    <>
      {announcement}
      {header}
      <main className="flex-1">{children}</main>
      {footer}
      {cartDrawer}
    </>
  );
}
