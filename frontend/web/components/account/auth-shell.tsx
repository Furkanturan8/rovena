import { Link } from "@/components/ui/link";
import { Photo } from "@/components/ui/photo";
import { Logo } from "@/components/layout/logo";
import type { Dictionary } from "@/lib/i18n/dictionaries/tr";

export function AuthShell({ d, title, subtitle, children, footer }: { d: Dictionary; title: string; subtitle: string; children: React.ReactNode; footer: React.ReactNode }) {
  return (
    <div className="grid min-h-[calc(100vh-108px)] lg:grid-cols-2">
      <div className="relative hidden lg:block">
        <Photo src="/media/products/manston-coat-in-dark-brown/2.jpg" alt="" ratio="auto" className="absolute inset-0" imgClassName="object-top" sizes="50vw" priority />
        <div className="absolute inset-0 bg-ink/25" />
        <div className="absolute inset-x-0 bottom-0 p-14 text-canvas">
          <p className="label-caps mb-4 text-bronze">{d.auth.sideEyebrow}</p>
          <p className="headline-lg max-w-md">{d.auth.sideText}</p>
        </div>
      </div>
      <div className="flex items-center justify-center px-5 py-16">
        <div className="w-full max-w-md">
          <div className="mb-10 flex justify-center lg:hidden"><Logo /></div>
          <p className="label-caps mb-3 text-umber">{subtitle}</p>
          <h1 className="headline-xl">{title}</h1>
          <div className="mt-10">{children}</div>
          <p className="mt-8 text-center text-[13px] text-umber">{footer}</p>
          <p className="mt-10 text-center text-[11px] text-umber">
            <Link href="/" className="underline underline-offset-4 hover:text-ink">{d.auth.backHome}</Link>
          </p>
        </div>
      </div>
    </div>
  );
}
