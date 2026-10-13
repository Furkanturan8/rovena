import type { Category, CategorySlug, ColorOption, Product, ProductBase, ProductText, SizeOption } from "@/types";
import type { Locale } from "@/lib/i18n/config";

// Kaynak klasörde bulunan görsel sayısı (varsayılan 6).
const IMAGE_COUNT: Record<string, number> = { "blake-crossbody-bag-in-midnight-navy": 5 };

const img = (handle: string, order: number[]) =>
  order.filter((n) => n <= (IMAGE_COUNT[handle] ?? 6)).map((n) => `/media/products/${handle}/${n}.jpg`);

export const categories: Category[] = [
  { slug: "coats", image: img("manston-coat-in-dark-brown", [2])[0] },
  { slug: "trousers", image: img("hunter-pant-in-desert-sand", [3])[0] },
  { slug: "jackets", image: img("hewit-jacket-in-dark-navy", [3])[0] },
  { slug: "knitwear", image: img("simone-sweater-in-pewter", [3])[0] },
  { slug: "accessories", image: img("cate-handbag-in-black-haircalf", [1])[0] },
];

// Ürün verisi ve fotoğrafları (kaynak: Khaite ürün kataloğu, sahibinin izniyle demo amaçlı).
// Fiyatlar USD'den TL'ye yaklaşık 35 kur ile çevrilmiştir.
const tl = (usd: number) => Math.round((usd * 35) / 500) * 500;

const COLOR_HEX: Record<string, string> = {
  "Dark Brown": "#3b2a22",
  Charcoal: "#44464a",
  "Dark Olive": "#6b5b2a",
  Bark: "#3a2a22",
  "Dark Navy": "#1f2a44",
  Black: "#16161a",
  "Desert Sand": "#c2a77e",
  Pewter: "#b9b9b6",
  Sterling: "#8a8a8a",
  Ivory: "#f0e9dc",
  Caramel: "#b9772d",
  "Midnight Navy": "#1b2340",
  Ebano: "#2d1f18",
};
const color = (name: string): ColorOption[] => [{ name, hex: COLOR_HEX[name] }];

const apparel = [3, 1, 2, 4, 5, 6];
// Bazı ürünlerde ilk model karesi yandan çekim olduğu için en iyi önden kare öne alınır.
const frontFirst = [2, 1, 3, 4, 5, 6];
const sontiOrder = [2, 4, 1, 3, 5, 6];
const natural = [1, 2, 3, 4, 5, 6];

const letterSizes = (stock: number[]): SizeOption[] =>
  ["XS", "S", "M", "L", "XL"].map((label, i) => ({ label, stock: stock[i] ?? 0 }));
const numSizes = (stock: number[]): SizeOption[] =>
  ["34", "36", "38", "40", "42", "44", "46"].map((label, i) => ({ label, stock: stock[i] ?? 0 }));
const shoeSizes = (stock: number[]): SizeOption[] =>
  ["36", "37", "38", "39", "40", "41"].map((label, i) => ({ label, stock: stock[i] ?? 0 }));
const oneSize = (n: number): SizeOption[] => [{ label: "OS", stock: n }];

interface ProductEntry {
  base: ProductBase;
  text: Record<Locale, ProductText>;
}

const entries: ProductEntry[] = [
  {
    base: {
      id: "p-001",
      slug: "manston-coat-in-dark-brown",
      category: "coats",
      price: tl(4800),
      craftHours: 48,
      edition: "No. 04/20",
      tag: "limited",
      colors: color("Dark Brown"),
      sizes: letterSizes([2, 4, 4, 2, 1]),
      images: img("manston-coat-in-dark-brown", frontFirst),
      video: "sewing",
      sku: "RV-8842",
      rating: 4.9,
      reviewCount: 18,
      releasedAt: "2025-09-12",
      leadTimeDays: [7, 10],
    },
    text: {
      tr: {
        name: "Manston Trençkot",
        kind: "Çift Yüzlü Yün Trençkot",
        description:
          "Klasik trençkotun yumuşak yapılı, çift yüzlü yünle yeniden yorumu. Dış dikişler elle atılır ve organza şeritlerle desteklenir; düşük kol oyuntusu rahat bir siluet verir. Bel kemeri ve yüzer sırt yokesi hacmi dengeler.",
        material: "Çift Yüzlü Yün",
        composition: ["%100 Yün, İtalya'da üretim", "Katmanlı omuz kapakları ve düğmeli apoletler", "Bakım: Yalnızca profesyonel kuru temizleme."],
      },
      en: {
        name: "Manston Trench Coat",
        kind: "Double-Faced Wool Trench",
        description:
          "A classic trench reimagined in softly structured, double-faced wool. Exterior seams are sewn by hand and reinforced with organza strips; low armholes give a relaxed silhouette. The belt and floating back yoke balance the volume.",
        material: "Double-Faced Wool",
        composition: ["100% wool, made in Italy", "Layered gun flaps and button-fastening epaulets", "Care: professional dry cleaning only."],
      },
    },
  },
  {
    base: {
      id: "p-002",
      slug: "sonti-coat-in-charcoal",
      category: "coats",
      price: tl(11500),
      craftHours: 64,
      edition: "No. 01/10",
      tag: "custom",
      colors: color("Charcoal"),
      sizes: letterSizes([0, 1, 2, 1, 0]),
      images: img("sonti-coat-in-charcoal", sontiOrder),
      sku: "RV-9001",
      rating: 5,
      reviewCount: 6,
      releasedAt: "2025-10-05",
      madeToOrderWeeks: 6,
    },
    text: {
      tr: {
        name: "Sonti Sarma Palto",
        kind: "Çift Yüzlü Kaşmir Palto",
        description:
          "İtalya'da, elle bitirilen çift yüzlü kaşmirden sarma palto. Cesur ölçekli yaka açık giyilebilir ya da kuşakla belden toplanarak yuvarlak bir hacim yaratır.",
        material: "Çift Yüzlü Kaşmir",
        composition: ["%100 Kaşmir, İtalya'da üretim", "Elle bitirilmiş kenarlar, sarma kuşak", "Bakım: Yalnızca profesyonel kuru temizleme."],
      },
      en: {
        name: "Sonti Wrap Coat",
        kind: "Double-Faced Cashmere Coat",
        description:
          "An enveloping wrap coat made in Italy of double-faced cashmere, finished by hand. The boldly scaled collar can be worn open or cinched with the sash belt for rolling volume.",
        material: "Double-Faced Cashmere",
        composition: ["100% cashmere, made in Italy", "Hand-finished edges, wrap sash", "Care: professional dry cleaning only."],
      },
    },
  },
  {
    base: {
      id: "p-003",
      slug: "marlot-coat-in-dark-olive",
      category: "coats",
      price: tl(8900),
      craftHours: 56,
      tag: "sealed",
      colors: color("Dark Olive"),
      sizes: letterSizes([1, 2, 3, 1, 0]),
      images: img("marlot-coat-in-dark-olive", apparel),
      sku: "RV-8231",
      rating: 4.8,
      reviewCount: 9,
      releasedAt: "2025-09-25",
      leadTimeDays: [7, 10],
    },
    text: {
      tr: {
        name: "Marlot Kruvaze Palto",
        kind: "Nappa Kuzu Derisi Palto",
        description:
          "Askeri bir klasiğin modern yorumu: tereyağı yumuşaklığında nappa kuzu derisinden kruvaze palto. Güçlü omuzlar, apoletler ve manşetlerde tekrar eden shearling kaplı yüksek yaka.",
        material: "Nappa Kuzu Derisi",
        composition: ["%100 Kuzu derisi, İtalya'da üretim", "Yaka ve manşetlerde shearling", "Bakım: Deri uzmanına danışınız."],
      },
      en: {
        name: "Marlot Double-Breasted Coat",
        kind: "Nappa Lambskin Coat",
        description:
          "A modern take on a military classic: a double-breasted coat in buttery nappa lambskin. Strong shoulders, epaulettes and a high shearling-lined collar that returns at the cuffs.",
        material: "Nappa Lambskin",
        composition: ["100% lambskin, made in Italy", "Shearling on collar and cuffs", "Care: consult a leather specialist."],
      },
    },
  },
  {
    base: {
      id: "p-004",
      slug: "simone-sweater-in-bark",
      category: "knitwear",
      price: tl(1880),
      craftHours: 18,
      tag: "new",
      colors: color("Bark"),
      sizes: letterSizes([3, 6, 5, 2, 1]),
      images: img("simone-sweater-in-bark", apparel),
      video: "wool",
      sku: "RV-5503",
      rating: 4.9,
      reviewCount: 42,
      releasedAt: "2025-10-08",
      leadTimeDays: [2, 4],
    },
    text: {
      tr: {
        name: "Simone Kazak",
        kind: "Saf Kaşmir Kazak",
        description:
          "Yoğun, kadifemsi dokusu zamanla yumuşayan saf kaşmirden dolgun bir kazak. Saddle kol yapısı yakada örme bantla sabitlenir; yarım balıkçı yaka ve ribanalı bitişler.",
        material: "Saf Kaşmir",
        composition: ["%100 Kaşmir", "Bakım: Elde, soğuk yıkama; düz kurutma."],
      },
      en: {
        name: "Simone Sweater",
        kind: "Pure Cashmere Sweater",
        description:
          "A robust, refined pullover in dense, velvety cashmere that softens over time. The saddle-sleeve silhouette is stabilized by a knitted tape at the neck, with a mock neck and ribbed trim.",
        material: "Pure Cashmere",
        composition: ["100% cashmere", "Care: hand wash cold; dry flat."],
      },
    },
  },
  {
    base: {
      id: "p-005",
      slug: "hewit-jacket-in-dark-navy",
      category: "jackets",
      price: tl(6700),
      craftHours: 40,
      tag: "new",
      colors: color("Dark Navy"),
      sizes: letterSizes([1, 3, 4, 2, 0]),
      images: img("hewit-jacket-in-dark-navy", apparel),
      sku: "RV-7021",
      rating: 4.8,
      reviewCount: 14,
      releasedAt: "2025-10-09",
      leadTimeDays: [3, 5],
    },
    text: {
      tr: {
        name: "Hewit Bomber Ceket",
        kind: "Plongé Deri Bomber",
        description:
          "Son derece yumuşak plongé deriden yeniden tasarlanmış bomber. Rahat ve yuvarlak siluet, temiz açılı yaka, düşük omuz dikişleri ve çift yönlü fermuar.",
        material: "Plongé Kuzu Derisi",
        composition: ["%100 Kuzu derisi", "Çift yönlü fermuar", "Bakım: Deri uzmanına danışınız."],
      },
      en: {
        name: "Hewit Bomber Jacket",
        kind: "Plongé Leather Bomber",
        description:
          "A reimagined bomber in exceptionally soft plongé leather. Relaxed, rounded silhouette with a clean angular neckline, dropped shoulder seams and a two-way zipper.",
        material: "Plongé Lambskin",
        composition: ["100% lambskin", "Two-way zipper", "Care: consult a leather specialist."],
      },
    },
  },
  {
    base: {
      id: "p-006",
      slug: "alder-jacket-in-charcoal",
      category: "jackets",
      price: tl(3200),
      craftHours: 30,
      colors: color("Charcoal"),
      sizes: letterSizes([0, 2, 4, 3, 1]),
      images: img("alder-jacket-in-charcoal", apparel),
      sku: "RV-7133",
      rating: 4.7,
      reviewCount: 11,
      releasedAt: "2025-09-20",
      leadTimeDays: [3, 5],
    },
    text: {
      tr: {
        name: "Alder Moto Ceket",
        kind: "Çift Yüzlü Yün Ceket",
        description:
          "Moto siluetinin mevsimsiz yünle yeniden yorumu. Çift yüzlü yapı, grafik detaylarla (zarif yaka, biyeli cepler, kemer kulpları) dökümlü, oversize bir gövde tanımlar.",
        material: "Çift Yüzlü Yün",
        composition: ["%100 Yün", "Bakım: Profesyonel kuru temizleme."],
      },
      en: {
        name: "Alder Moto Jacket",
        kind: "Double-Faced Wool Jacket",
        description:
          "A moto silhouette reimagined in seasonless wool. Double-faced construction defines a draped, oversized body with graphic details: a sleek collar, welt pockets and belt loops.",
        material: "Double-Faced Wool",
        composition: ["100% wool", "Care: professional dry cleaning."],
      },
    },
  },
  {
    base: {
      id: "p-007",
      slug: "meko-jacket-in-black",
      category: "jackets",
      price: tl(3600),
      craftHours: 32,
      tag: "sealed",
      colors: color("Black"),
      sizes: numSizes([1, 2, 3, 3, 2, 1, 0]),
      images: img("meko-jacket-in-black", apparel),
      sku: "RV-7140",
      rating: 4.6,
      reviewCount: 8,
      releasedAt: "2025-09-08",
      leadTimeDays: [3, 5],
    },
    text: {
      tr: {
        name: "Meko Kısa Ceket",
        kind: "Mimari Opera Ceketi",
        description:
          "Opera pelerinlerinden ilham alan mimari bir ceket. Yumuşak yapılı çift yüzlü yünden, kısa ve kruvaze; abartılı omuzlardaki iç destek 'yüzen' bir yaka çizgisi yaratır. Uzun kollar heykelsi sırtla dengelenir.",
        material: "Çift Yüzlü Yün",
        composition: ["%100 Yün, ABD'de üretim", "Bakım: Profesyonel kuru temizleme."],
      },
      en: {
        name: "Meko Cropped Jacket",
        kind: "Architectural Opera Jacket",
        description:
          "An architectural jacket inspired by opera capes. Cropped and double-breasted in softly structured double-faced wool; internal support at the exaggerated shoulders creates a floating neckline. Extended sleeves are balanced by a sculptural back.",
        material: "Double-Faced Wool",
        composition: ["100% wool, made in the USA", "Care: professional dry cleaning."],
      },
    },
  },
  {
    base: {
      id: "p-008",
      slug: "august-pant-in-charcoal",
      category: "trousers",
      price: tl(1580),
      craftHours: 16,
      colors: color("Charcoal"),
      sizes: numSizes([1, 3, 5, 4, 3, 1, 0]),
      images: img("august-pant-in-charcoal", apparel),
      sku: "RV-4410",
      rating: 4.7,
      reviewCount: 33,
      releasedAt: "2025-09-01",
      leadTimeDays: [2, 4],
    },
    text: {
      tr: {
        name: "August Pantolon",
        kind: "Pileli Yün-Kaşmir Pantolon",
        description:
          "Erkek terziliğinden ilhamla, hafif ve lüks İtalyan yün-kaşmir gömleklik kumaştan orta bel pantolon. Yakın yerleştirilmiş çift ters pileler temiz çizgiyi korur; önde hafifçe aşağı iner.",
        material: "Yün ve Kaşmir",
        composition: ["%90 Yün, %8 Kaşmir, %2 Elastan", "Kemer kulpları, yan ve arka biyeli cepler", "Bakım: Kuru temizleme."],
      },
      en: {
        name: "August Trousers",
        kind: "Pleated Wool-Cashmere Trousers",
        description:
          "A menswear-inspired, mid-rise trouser in lightweight, luxurious Italian wool-cashmere shirting. Paired reverse pleats sit close together to keep the lines clean; it dips slightly lower in front.",
        material: "Wool and Cashmere",
        composition: ["90% wool, 8% cashmere, 2% elastane", "Belt loops, on-seam side pockets and back welt pockets", "Care: dry clean."],
      },
    },
  },
  {
    base: {
      id: "p-009",
      slug: "hunter-pant-in-desert-sand",
      category: "trousers",
      price: tl(1380),
      craftHours: 14,
      tag: "new",
      colors: color("Desert Sand"),
      sizes: numSizes([2, 3, 4, 4, 2, 1, 0]),
      images: img("hunter-pant-in-desert-sand", apparel),
      sku: "RV-4422",
      rating: 4.6,
      reviewCount: 12,
      releasedAt: "2025-10-02",
      leadTimeDays: [2, 4],
    },
    text: {
      tr: {
        name: "Hunter Pantolon",
        kind: "Yün-Kaşmir Pantolon",
        description:
          "Dökümü için seçilmiş hafif ve lüks yün-kaşmir kumaştan tailored pantolon. Belde daha aşağıda oturan dar paça; bel düğmesi, iç cepler ve arkada tek kapaklı cep.",
        material: "Yün ve Kaşmir",
        composition: ["%90 Yün, %8 Kaşmir, %2 Elastan", "İtalya'da üretim", "Bakım: Kuru temizleme."],
      },
      en: {
        name: "Hunter Trousers",
        kind: "Wool-Cashmere Trousers",
        description:
          "A tailored trouser in lightweight, luxurious wool-cashmere shirting chosen for its drape. A sleek, narrow leg that sits lower on the waist, with a button closure, inset side pockets and a single flap pocket at the back.",
        material: "Wool and Cashmere",
        composition: ["90% wool, 8% cashmere, 2% elastane", "Made in Italy", "Care: dry clean."],
      },
    },
  },
  {
    base: {
      id: "p-010",
      slug: "clover-pant-in-dark-brown",
      category: "trousers",
      price: tl(1980),
      craftHours: 15,
      colors: color("Dark Brown"),
      sizes: letterSizes([2, 4, 4, 3, 1]),
      images: img("clover-pant-in-dark-brown", apparel),
      sku: "RV-4430",
      rating: 4.8,
      reviewCount: 19,
      releasedAt: "2025-08-18",
      leadTimeDays: [2, 4],
    },
    text: {
      tr: {
        name: "Clover Pantolon",
        kind: "Çift Yüzlü Yün Pantolon",
        description:
          "Çift yüzlü yünden beli lastikli pantolon. Kısa paça; uzatan çizgileri cesur kavislerle buluşturur. Temiz arka, yan cepler.",
        material: "Çift Yüzlü Yün",
        composition: ["%100 Yün", "Bakım: Profesyonel kuru temizleme."],
      },
      en: {
        name: "Clover Trousers",
        kind: "Double-Faced Wool Trousers",
        description:
          "A pull-on trouser in double-faced wool. The cropped silhouette combines elongating lines with bold curves. Clean back, elasticated waist, side pockets.",
        material: "Double-Faced Wool",
        composition: ["100% wool", "Care: professional dry cleaning."],
      },
    },
  },
  {
    base: {
      id: "p-011",
      slug: "cate-handbag-in-black-haircalf",
      category: "accessories",
      price: tl(5600),
      craftHours: 24,
      edition: "No. 12/20",
      tag: "limited",
      colors: color("Black"),
      sizes: oneSize(7),
      images: img("cate-handbag-in-black-haircalf", natural),
      sku: "RV-3308",
      rating: 4.9,
      reviewCount: 27,
      releasedAt: "2025-07-15",
      leadTimeDays: [2, 4],
    },
    text: {
      tr: {
        name: "Cate Çanta",
        kind: "Kısa Saplı Deri Çanta",
        description:
          "Kaplı bir çerçeveyle yapılandırılmış, hem ayrıntılı hem çevik bir kısa saplı siluet. 18 ayar altın kaplama özel kiss-lock kapama.",
        material: "Tüylü Dana Derisi",
        composition: ["%100 Dana derisi", "Donanım: 18 ayar altın kaplama", "Bakım: Deri uzmanına danışınız."],
      },
      en: {
        name: "Cate Handbag",
        kind: "Top-Handle Leather Bag",
        description:
          "Structured by a covered frame, this top-handled silhouette is at once intricate and nimble. Custom kiss-lock clasp plated in 18-karat gold.",
        material: "Haircalf",
        composition: ["100% calfskin", "Hardware: 18-karat gold plated", "Care: consult a leather specialist."],
      },
    },
  },
  {
    base: {
      id: "p-012",
      slug: "clive-ankle-boot-35-in-caramel",
      category: "accessories",
      price: tl(1680),
      craftHours: 16,
      colors: color("Caramel"),
      sizes: shoeSizes([2, 3, 3, 2, 1, 0]),
      images: img("clive-ankle-boot-35-in-caramel", natural),
      sku: "RV-3420",
      rating: 4.7,
      reviewCount: 15,
      releasedAt: "2025-09-28",
      leadTimeDays: [2, 4],
    },
    text: {
      tr: {
        name: "Clive Bilek Bot",
        kind: "Süet Western Bot",
        description:
          "Eskimiş Western klasiklerinden ilham alan, günlük kullanım için oranlanmış keskin ama rustik bir bilek botu. Sivri burun, kasıtlı eğimli konç; el işi split süet, dikişli welt ve patinalı yığma topuk.",
        material: "Split Süet Dana Derisi",
        composition: ["Dış: Split süet dana derisi", "Taban ve topuk: Patinalı deri", "Bakım: Süet fırçası ve koruyucu sprey."],
      },
      en: {
        name: "Clive Ankle Boot",
        kind: "Suede Western Boot",
        description:
          "A sharp yet rustic ankle boot inspired by well-worn Western classics and proportioned for daily wear. Snip-toe silhouette with a deliberately tilted shaft in hand-worked split-suede calfskin, stitched welt and stacked patinated-leather heel.",
        material: "Split Suede Calfskin",
        composition: ["Upper: split-suede calfskin", "Sole and heel: patinated leather", "Care: suede brush and protective spray."],
      },
    },
  },
  {
    base: {
      id: "p-013",
      slug: "blake-crossbody-bag-in-midnight-navy",
      category: "accessories",
      price: tl(4400),
      craftHours: 20,
      colors: color("Midnight Navy"),
      sizes: oneSize(5),
      images: img("blake-crossbody-bag-in-midnight-navy", natural),
      sku: "RV-3312",
      rating: 4.8,
      reviewCount: 19,
      releasedAt: "2025-08-18",
      leadTimeDays: [2, 4],
    },
    text: {
      tr: {
        name: "Blake Çapraz Çanta",
        kind: "Kapaklı Deri Çanta",
        description:
          "Kutu siluetini yeniden tanımlayan, çizgi ve kıvrımı birleştiren dinamik kapaklı çanta. Altın tonlu imza donanım, cepli iç bölme; çıkarılabilir ve ayarlanabilir askıyla omuzda ya da çapraz taşınır.",
        material: "Dana Derisi ve Süet",
        composition: ["%100 Dana derisi", "Altın tonlu donanım", "Ölçü: 24 × 16 × 6 cm"],
      },
      en: {
        name: "Blake Crossbody Bag",
        kind: "Flap Leather Bag",
        description:
          "A dynamic flap-front handbag that redefines the box silhouette, combining line with curves. Signature gold hardware and internal pocketing; the adjustable, removable strap can be worn on the shoulder or across the body.",
        material: "Calfskin and Suede",
        composition: ["100% calfskin", "Gold-tone hardware", "Size: 24 × 16 × 6 cm"],
      },
    },
  },
  {
    base: {
      id: "p-014",
      slug: "donna-shoulder-bag-in-ebano",
      category: "accessories",
      price: tl(2680),
      craftHours: 18,
      tag: "new",
      colors: color("Ebano"),
      sizes: oneSize(8),
      images: img("donna-shoulder-bag-in-ebano", natural),
      sku: "RV-3352",
      rating: 4.7,
      reviewCount: 12,
      releasedAt: "2025-10-04",
      leadTimeDays: [2, 4],
    },
    text: {
      tr: {
        name: "Donna Omuz Çantası",
        kind: "Süet Omuz Çantası",
        description:
          "Zengin rengi, mat derinliği ve kalıcı yumuşaklığı için seçilmiş ipeksi süet dana derisinden uzatılmış çanta. Doğu-batı siluetli, panelli ve körüklü; uzun tüp saplar omuzda ya da elde taşınır.",
        material: "Süet Dana Derisi",
        composition: ["%100 Dana derisi, İtalya'da üretim", "Çift yönlü fermuarlı üst", "Bakım: Süet fırçası."],
      },
      en: {
        name: "Donna Shoulder Bag",
        kind: "Suede Shoulder Bag",
        description:
          "An elongated handbag in silky calfskin suede chosen for its rich color, matte depth and enduring softness. East-west silhouette, paneled and gusseted, with long tubular handles to carry on the shoulder or by hand.",
        material: "Calfskin Suede",
        composition: ["100% calfskin, made in Italy", "Two-way zipper top", "Care: suede brush."],
      },
    },
  },
  {
    base: {
      id: "p-015",
      slug: "simone-sweater-in-pewter",
      category: "knitwear",
      price: tl(1880),
      craftHours: 18,
      colors: color("Pewter"),
      sizes: letterSizes([2, 5, 5, 3, 1]),
      images: img("simone-sweater-in-pewter", apparel),
      sku: "RV-5504",
      rating: 4.9,
      reviewCount: 37,
      releasedAt: "2025-09-14",
      leadTimeDays: [2, 4],
    },
    text: {
      tr: {
        name: "Simone Kazak",
        kind: "Saf Kaşmir Kazak",
        description:
          "Yoğun, kadifemsi dokusu zamanla yumuşayan saf kaşmirden dolgun bir kazak. Saddle kol yapısı yakada örme bantla sabitlenir; yarım balıkçı yaka ve ribanalı bitişler.",
        material: "Saf Kaşmir",
        composition: ["%100 Kaşmir", "Bakım: Elde, soğuk yıkama; düz kurutma."],
      },
      en: {
        name: "Simone Sweater",
        kind: "Pure Cashmere Sweater",
        description:
          "A robust, refined pullover in dense, velvety cashmere that softens over time. The saddle-sleeve silhouette is stabilized by a knitted tape at the neck, with a mock neck and ribbed trim.",
        material: "Pure Cashmere",
        composition: ["100% cashmere", "Care: hand wash cold; dry flat."],
      },
    },
  },
  {
    base: {
      id: "p-016",
      slug: "niels-cardigan-in-sterling",
      category: "knitwear",
      price: tl(2400),
      craftHours: 22,
      colors: color("Sterling"),
      sizes: letterSizes([1, 4, 4, 2, 0]),
      images: img("niels-cardigan-in-sterling", apparel),
      sku: "RV-5510",
      rating: 4.8,
      reviewCount: 25,
      releasedAt: "2025-09-30",
      leadTimeDays: [2, 4],
    },
    text: {
      tr: {
        name: "Niels Hırka",
        kind: "Kaşmir-İpek Hırka",
        description:
          "Melanj ipliğinin olağanüstü yumuşaklığını öne çıkaran hafif hırka: kaşmir ve ipek karışımı sarıcı sıcaklık ve rafine bir bitiş verir. Kısa, hafif büzük kesim; gevşek gerginlikli jarse örgü ve ribanalı bitişler.",
        material: "Kaşmir ve İpek",
        composition: ["%76 Kaşmir, %24 İpek", "Bakım: Elde yıkama."],
      },
      en: {
        name: "Niels Cardigan",
        kind: "Cashmere-Silk Cardigan",
        description:
          "A lightweight cardigan that spotlights the softness of its mélange yarn, a cashmere and silk blend offering enveloping warmth with a refined finish. Shrunken, gently cropped fit in a loose-tension jersey stitch with ribbed trim.",
        material: "Cashmere and Silk",
        composition: ["76% cashmere, 24% silk", "Care: hand wash."],
      },
    },
  },
  {
    base: {
      id: "p-017",
      slug: "elias-embroidery-sweater-in-ivory",
      category: "knitwear",
      price: tl(3400),
      craftHours: 36,
      tag: "sealed",
      colors: color("Ivory"),
      sizes: letterSizes([2, 3, 3, 1, 0]),
      images: img("elias-embroidery-sweater-in-ivory", apparel),
      sku: "RV-5522",
      rating: 4.9,
      reviewCount: 30,
      releasedAt: "2025-10-12",
      leadTimeDays: [3, 5],
    },
    text: {
      tr: {
        name: "Elias İşlemeli Kazak",
        kind: "Örgü Desenli Kazak",
        description:
          "Kaşmir ve moher karışımı, örgü desenli kazak üzerinde sanatsal nakışlar. Düşük omuzlarla rahatlatılmış siluet; yakada, manşetlerde ve etek ucunda 1x1 ribana.",
        material: "Kaşmir ve Moher",
        composition: ["%85 Kaşmir, %15 Moher", "El işi çiçek nakışı", "Bakım: Elde yıkama."],
      },
      en: {
        name: "Elias Embroidered Sweater",
        kind: "Cable-Knit Sweater",
        description:
          "An artfully embroidered pullover in a cable-knit blend of cashmere and mohair. Dropped shoulders ease the silhouette, with 1x1 ribbing at the neck, cuffs and hem.",
        material: "Cashmere and Mohair",
        composition: ["85% cashmere, 15% mohair", "Hand-embroidered floral motifs", "Care: hand wash."],
      },
    },
  },
  {
    base: {
      id: "p-018",
      slug: "wilson-coat-in-black",
      category: "coats",
      price: tl(6400),
      craftHours: 44,
      colors: color("Black"),
      sizes: letterSizes([1, 2, 3, 2, 1]),
      images: img("wilson-coat-in-black", apparel),
      sku: "RV-8330",
      rating: 4.8,
      reviewCount: 13,
      releasedAt: "2025-10-10",
      leadTimeDays: [7, 10],
    },
    text: {
      tr: {
        name: "Wilson Caban",
        kind: "Shearling Yakalı Deri Caban",
        description:
          "Dokusu ve dayanıklılığı için seçilmiş nappa kuzu derisinden rahat bir caban. Kıvırcık shearling yaka, klasik iş giysisinden ilham alan düğmeli ön, grafik dikişler; kapitone astar ve ördek tüyü dolgu.",
        material: "Nappa Kuzu Derisi",
        composition: ["%100 Kuzu derisi, İtalya'da üretim", "Astar: Kapitone, ördek tüyü dolgu", "Bakım: Deri uzmanına danışınız."],
      },
      en: {
        name: "Wilson Caban Coat",
        kind: "Shearling-Collar Leather Coat",
        description:
          "A relaxed caban in robust nappa lambskin chosen for its grained texture and durability. Curly shearling collar, a button front inspired by classic workwear and graphic stitching; quilted lining with duck-down insulation.",
        material: "Nappa Lambskin",
        composition: ["100% lambskin, made in Italy", "Lining: quilted, duck-down insulation", "Care: consult a leather specialist."],
      },
    },
  },
  {
    base: {
      id: "p-019",
      slug: "petra-knee-high-boot-in-black",
      category: "accessories",
      price: tl(2600),
      craftHours: 20,
      colors: color("Black"),
      sizes: shoeSizes([1, 3, 3, 2, 1, 0]),
      images: img("petra-knee-high-boot-in-black", natural),
      sku: "RV-3440",
      rating: 4.7,
      reviewCount: 16,
      releasedAt: "2025-09-05",
      leadTimeDays: [2, 4],
    },
    text: {
      tr: {
        name: "Petra Diz Altı Çizme",
        kind: "Deri Çizme",
        description:
          "Yapılandırılmış hassasiyette bir diz altı çizme: kare burun, düz konç, ince deri taban ve yığma blok topuk ile güçlü bir dikeylik. İpeksi dana derisi anilin boyalıdır; deriyi doğal karakteriyle korur.",
        material: "Anilin Dana Derisi",
        composition: ["Dış ve taban: Dana derisi, İtalya'da üretim", "Topuk: 7 cm blok topuk", "Bakım: Deri bakım kremi."],
      },
      en: {
        name: "Petra Knee-High Boot",
        kind: "Leather Boot",
        description:
          "A knee-high boot of structured precision: a square toe, straight shaft, sleek leather sole and stacked block heel express strong verticality. Saddle-soft calfskin is aniline dyed to preserve the natural character of the leather.",
        material: "Aniline Calfskin",
        composition: ["Upper and sole: calfskin, made in Italy", "Heel: 7 cm block heel", "Care: leather conditioner."],
      },
    },
  },
];

export const productBases: ProductBase[] = entries.map((e) => e.base);

const localize = (e: ProductEntry, locale: Locale): Product => ({ ...e.base, ...e.text[locale] });

export const getProducts = (locale: Locale): Product[] => entries.map((e) => localize(e, locale));
export const getProduct = (slug: string, locale: Locale): Product | undefined => {
  const e = entries.find((x) => x.base.slug === slug);
  return e ? localize(e, locale) : undefined;
};
export const getProductById = (id: string, locale: Locale): Product | undefined => {
  const e = entries.find((x) => x.base.id === id);
  return e ? localize(e, locale) : undefined;
};
export const getCategory = (slug: string) => categories.find((c) => c.slug === slug);
export const isCategorySlug = (v: string | undefined): v is CategorySlug => !!getCategory(v ?? "");

export const priceRange = {
  min: Math.min(...productBases.map((p) => p.price)),
  max: Math.max(...productBases.map((p) => p.price)),
};
