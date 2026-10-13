import { Link } from "@/components/ui/link";
import { categories } from "@/lib/mock/catalog";
import { Photo } from "@/components/ui/photo";
import { useDict } from "@/lib/i18n/provider";
import { cn } from "@/lib/utils";

export function MegaMenu({ open, onNavigate }: { open: boolean; onNavigate: () => void }) {
  const d = useDict();
  const m = d.megaMenu;
  const features = [categories[0], categories[3]];
  return (
    <div
      className={cn(
        "absolute inset-x-0 top-full hidden border-b border-travertine bg-canvas text-ink transition-all duration-300 lg:block",
        open ? "visible translate-y-0 opacity-100" : "invisible -translate-y-2 opacity-0",
      )}
      aria-hidden={!open}
    >
      <div className="container-page grid grid-cols-12 gap-6 py-10">
        <div className="col-span-3">
          <p className="label-caps mb-5 text-umber">{m.categories}</p>
          <ul className="space-y-3">
            <li>
              <Link href="/collection" onClick={onNavigate} className="headline-sm hover:text-bronze-deep">
                {m.allCollection}
              </Link>
            </li>
            {categories.map((c) => (
              <li key={c.slug}>
                <Link href={`/collection?category=${c.slug}`} onClick={onNavigate} className="text-[15px] text-umber hover:text-ink">
                  {d.categories[c.slug].label}
                </Link>
              </li>
            ))}
          </ul>
        </div>
        <div className="col-span-3">
          <p className="label-caps mb-5 text-umber">{m.picks}</p>
          <ul className="space-y-3 text-[15px] text-umber">
            <li><Link href="/collection?sort=newest" onClick={onNavigate} className="hover:text-ink">{m.newArrivals}</Link></li>
            <li><Link href="/collection?tag=limited" onClick={onNavigate} className="hover:text-ink">{m.limited}</Link></li>
            <li><Link href="/collection?category=knitwear" onClick={onNavigate} className="hover:text-ink">{m.pureCashmere}</Link></li>
            <li><Link href="/gallery" onClick={onNavigate} className="hover:text-ink">{m.gallery}</Link></li>
            <li><Link href="/appointment" onClick={onNavigate} className="hover:text-ink">{m.appointment}</Link></li>
          </ul>
        </div>
        {features.map((c, i) => (
          <Link key={c.slug} href={`/collection?category=${c.slug}`} onClick={onNavigate} className="group col-span-3">
            <Photo src={c.image} alt={d.categories[c.slug].label} ratio="4/5" sizes="20vw" className="max-h-[230px]" imgClassName="object-top transition-transform duration-700 group-hover:scale-[1.04]" />
            <p className="label-caps mt-3 text-umber">{i === 0 ? m.featured : m.seasonTexture}</p>
            <p className="headline-sm">{d.categories[c.slug].label}</p>
          </Link>
        ))}
      </div>
    </div>
  );
}
