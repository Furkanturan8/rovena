import type { Locale } from "@/lib/i18n/config";

export const legalSlugs = ["privacy", "distance-sales", "kvkk", "cookies"] as const;
export type LegalSlug = (typeof legalSlugs)[number];

export interface LegalSection {
  heading: string;
  /** Plain paragraphs; an entry starting with "- " is rendered as a list item. */
  body: string[];
}

export interface LegalDoc {
  title: string;
  intro: string;
  sections: LegalSection[];
}

// Bracketed values are company details to be filled in before launch.
const tr: Record<LegalSlug, LegalDoc> = {
  privacy: {
    title: "Gizlilik Politikası",
    intro: "Rovena olarak kişisel bilgilerinizin gizliliğine önem veriyoruz. Bu politika, sitemizi kullanırken hangi bilgileri topladığımızı, nasıl kullandığımızı ve nasıl koruduğumuzu anlatır.",
    sections: [
      {
        heading: "Topladığımız bilgiler",
        body: [
          "- Üyelik bilgileri: ad, soyad, e-posta adresi ve şifreniz (şifre yalnızca geri döndürülemez biçimde şifrelenmiş olarak saklanır).",
          "- Adres bilgileri: teslimat için kaydettiğiniz ad, telefon ve adres bilgileri.",
          "- Sipariş bilgileri: satın aldığınız ürünler, tutar, sipariş durumu ve kargo bilgisi.",
          "- Ödeme bilgileri: kart numaranız Rovena sunucularına gönderilmez ve saklanmaz; ödeme, kartınızdan üretilen bir ödeme belirteci (token) ile yapılır.",
          "- Teknik veriler: sitenin çalışması için gereken oturum bilgileri ve güvenlik amaçlı istek kayıtları (IP adresi, tarih, istenen sayfa).",
        ],
      },
      {
        heading: "Bilgileri nasıl kullanıyoruz",
        body: [
          "Bilgilerinizi siparişinizi almak, ödemeyi tamamlamak, ürünü teslim etmek, iade ve değişim işlemlerini yürütmek, hesabınızı yönetmek ve yasal yükümlülüklerimizi yerine getirmek için kullanırız. Bülten aboneliği gibi ticari iletiler yalnızca onayınızla gönderilir ve her iletide abonelikten çıkabilirsiniz.",
        ],
      },
      {
        heading: "Üçüncü kişilerle paylaşım",
        body: [
          "Bilgilerinizi satmayız. Yalnızca hizmetin gerektirdiği ölçüde; kargo firmaları, ödeme hizmeti sağlayıcıları ve barındırma/altyapı sağlayıcıları ile ve yasal bir zorunluluk olduğunda yetkili kurumlarla paylaşırız.",
        ],
      },
      {
        heading: "Saklama süresi",
        body: [
          "Bilgileriniz, işlenme amacı için gerekli süre ve ilgili mevzuatta (vergi, ticaret ve tüketici hukuku) öngörülen saklama süreleri boyunca tutulur; süre dolunca silinir veya anonim hale getirilir. Hesabınızı kapatmak için bizimle iletişime geçebilirsiniz.",
        ],
      },
      {
        heading: "Güvenlik",
        body: [
          "Şifreler geri döndürülemez biçimde saklanır, oturum bilgileri tarayıcı komut dosyalarının erişemeyeceği (httpOnly) çerezlerde tutulur ve servisler arası iletişim yetkilendirme ile korunur. Hiçbir sistem tamamen risksiz değildir; bir ihlal durumunda mevzuattaki bildirim yükümlülüklerini yerine getiririz.",
        ],
      },
      {
        heading: "Haklarınız ve iletişim",
        body: [
          "Kişisel verilerinize erişme, düzeltme, silme ve işlenmesine itiraz etme haklarınızı KVKK Aydınlatma Metni'nde anlatılan yollarla kullanabilirsiniz.",
          "İletişim: Rovena, info@rovena.com",
        ],
      },
    ],
  },
  kvkk: {
    title: "KVKK Aydınlatma Metni",
    intro: "6698 sayılı Kişisel Verilerin Korunması Kanunu'nun 10. maddesi uyarınca, kişisel verilerinizin işlenmesi hakkında sizi bilgilendiririz.",
    sections: [
      {
        heading: "Veri sorumlusu",
        body: ["Rovena, info@rovena.com"],
      },
      {
        heading: "İşlenen kişisel veriler ve amaçlar",
        body: [
          "- Kimlik ve iletişim (ad, soyad, e-posta, telefon): üyelik oluşturma, siparişle ilgili iletişim.",
          "- Adres: siparişin teslimi, faturalama.",
          "- Müşteri işlemi (sipariş, iade, iptal kayıtları): sözleşmenin kurulması ve ifası, satış sonrası hizmetler.",
          "- İşlem güvenliği (IP adresi, istek kayıtları, oturum bilgisi): bilgi güvenliğinin sağlanması, hile ve kötüye kullanımın önlenmesi.",
          "- Pazarlama (e-posta adresi): yalnızca açık rızanız varsa kampanya ve yeni koleksiyon iletileri.",
        ],
      },
      {
        heading: "Toplama yöntemi ve hukuki sebepler",
        body: [
          "Verileriniz web sitesi üzerinden elektronik ortamda, siz formları doldurdukça ve siteyi kullandıkça toplanır. Hukuki sebepler: sözleşmenin kurulması ve ifası, hukuki yükümlülüğün yerine getirilmesi, veri sorumlusunun meşru menfaati ve pazarlama iletileri için açık rızanızdır.",
        ],
      },
      {
        heading: "Verilerin aktarılması",
        body: [
          "Verileriniz, işleme amaçlarıyla sınırlı olarak kargo ve lojistik firmalarına, ödeme hizmeti sağlayıcılarına, bilgi teknolojileri ve barındırma hizmeti sağlayıcılarına ve mevzuat gereği yetkili kamu kurum ve kuruluşlarına aktarılabilir. Yurt dışına aktarım gerekiyorsa Kanun'un 9. maddesindeki şartlara uyulur.",
        ],
      },
      {
        heading: "Kanun'un 11. maddesi kapsamındaki haklarınız",
        body: [
          "- Kişisel verilerinizin işlenip işlenmediğini öğrenme ve işlenmişse bilgi talep etme,",
          "- İşlenme amacını ve amacına uygun kullanılıp kullanılmadığını öğrenme,",
          "- Yurt içinde veya yurt dışında aktarıldığı üçüncü kişileri bilme,",
          "- Eksik veya yanlış işlenmişse düzeltilmesini isteme,",
          "- Kanun'da öngörülen şartlar çerçevesinde silinmesini veya yok edilmesini isteme ve bunun aktarıldığı üçüncü kişilere bildirilmesini isteme,",
          "- İşlenen verilerin münhasıran otomatik sistemlerle analiz edilmesi sonucu aleyhinize bir sonuç çıkmasına itiraz etme,",
          "- Kanuna aykırı işleme nedeniyle zarara uğramanız hâlinde zararın giderilmesini talep etme.",
        ],
      },
      {
        heading: "Başvuru",
        body: [
          "Haklarınızı kullanmak için taleplerinizi [KVKK başvuru e-posta adresi] adresine veya yukarıdaki adrese yazılı olarak iletebilirsiniz. Başvurunuz, niteliğine göre en geç otuz gün içinde sonuçlandırılır.",
        ],
      },
    ],
  },
  "distance-sales": {
    title: "Mesafeli Satış Sözleşmesi",
    intro: "Bu sözleşme, 6502 sayılı Tüketicinin Korunması Hakkında Kanun ve Mesafeli Sözleşmeler Yönetmeliği uyarınca, internet sitesi üzerinden yapılan satışlar için düzenlenmiştir. Siparişi onaylayarak bu sözleşmenin şartlarını kabul etmiş olursunuz.",
    sections: [
      {
        heading: "1. Taraflar",
        body: [
          "Satıcı: Rovena, info@rovena.com",
          "Alıcı: Sipariş sırasında ad, soyad, telefon ve teslimat adresini bildiren üye.",
        ],
      },
      {
        heading: "2. Sözleşmenin konusu",
        body: ["Alıcı'nın Rovena internet sitesinden elektronik ortamda sipariş verdiği, nitelikleri ve satış fiyatı sipariş özetinde belirtilen ürün(ler)in satışı ve teslimine ilişkin tarafların hak ve yükümlülükleridir."],
      },
      {
        heading: "3. Ürün, fiyat ve ödeme",
        body: [
          "Ürünün temel nitelikleri, beden ve rengi, KDV dahil satış fiyatı ve teslimat bedeli sipariş onay sayfasında ve sipariş özetinde yer alır. Fiyatlar Türk lirasıdır. Ödeme kredi/banka kartı ile yapılır; ödeme onaylanmadıkça sipariş tamamlanmaz. Ödeme alınamazsa sipariş iptal edilir ve ayrılan stok serbest bırakılır; tahsil edilmiş tutar varsa iade edilir.",
        ],
      },
      {
        heading: "4. Teslimat",
        body: [
          "Ürün, Alıcı'nın belirttiği teslimat adresine, sipariş sayfasında belirtilen süre içinde (stoktaki ürünlerde genellikle 2–4 iş günü, siparişe özel üretimlerde ürün sayfasındaki süre) kargo ile teslim edilir. Her halükârda yasal azami süre olan otuz günü aşmaz. Teslimat sırasında ambalajda hasar varsa tutanak tutturulmalı ve durum Satıcı'ya bildirilmelidir.",
          "Sipariş durumunuzu ve kargo takip numaranızı hesabınızdaki Siparişlerim bölümünden izleyebilirsiniz.",
        ],
      },
      {
        heading: "5. Cayma hakkı",
        body: [
          "Alıcı, ürünün tesliminden itibaren on dört gün içinde herhangi bir gerekçe göstermeksizin ve cezai şart ödemeksizin sözleşmeden cayma hakkına sahiptir. Rovena, bu yasal süreye ek olarak ürünün tesliminden itibaren 30 gün içinde ücretsiz iade ve değişim imkânı sunar.",
          "Cayma bildirimi [iletişim e-posta adresi] adresine yapılır. İade edilen ürün kullanılmamış, etiketleri ve orijinal ambalajı ile birlikte gönderilmelidir. Satıcı, cayma bildiriminin kendisine ulaşmasından itibaren en geç on dört gün içinde ürün bedelini, ödemede kullanılan yöntemle iade eder.",
        ],
      },
      {
        heading: "6. Cayma hakkının kullanılamayacağı haller",
        body: [
          "Mevzuat gereği; Alıcı'nın istek veya kişisel ihtiyaçları doğrultusunda hazırlanan (ör. monogram işlenmiş) ürünlerde, çabuk bozulabilen veya son kullanma tarihi geçebilecek ürünlerde, tesliminden sonra ambalajı açılmış hijyen ürünlerinde ve diğer yönetmelikte sayılan hallerde cayma hakkı kullanılamaz. Kişiye özel ürünler için Rovena'nın sunduğu ek iade imkânı, kusurlu ürün haklarınızı etkilemez.",
        ],
      },
      {
        heading: "7. Ayıplı ürün",
        body: ["Ürünün ayıplı çıkması hâlinde Alıcı, 6502 sayılı Kanun'un ilgili maddelerinde sayılan seçimlik haklarını (sözleşmeden dönme, bedel indirimi, ücretsiz onarım veya yenisiyle değiştirme) kullanabilir."],
      },
      {
        heading: "8. Uyuşmazlıkların çözümü",
        body: ["Bu sözleşmeden doğan uyuşmazlıklarda, Ticaret Bakanlığı'nca her yıl ilan edilen parasal sınırlar dahilinde Alıcı'nın yerleşim yerindeki veya sipariş verdiği yerdeki Tüketici Hakem Heyetleri, bu sınırları aşan durumlarda Tüketici Mahkemeleri yetkilidir."],
      },
      {
        heading: "9. Yürürlük",
        body: ["Alıcı, sipariş onayı ile bu sözleşmenin tüm şartlarını okuduğunu, anladığını ve elektronik ortamda kabul ettiğini beyan eder. Sözleşmenin bir örneği sipariş kaydınızla birlikte saklanır."],
      },
    ],
  },
  cookies: {
    title: "Çerez Politikası",
    intro: "Çerezler, bir siteyi ziyaret ettiğinizde tarayıcınıza kaydedilen küçük metin dosyalarıdır. Rovena yalnızca sitenin çalışması için gerekli çerezleri ve tarayıcı depolamasını kullanır; reklam veya izleme amaçlı çerez kullanmaz.",
    sections: [
      {
        heading: "Kullandığımız çerezler",
        body: [
          "- rovena_at ve rovena_rt (zorunlu): oturumunuzu açık tutar. Tarayıcı komut dosyalarının erişemeyeceği (httpOnly) çerezlerdir; giriş yaptığınızda oluşur, çıkış yaptığınızda silinir. rovena_at kısa ömürlüdür (15 dakika) ve rovena_rt ile yenilenir.",
          "- Dil tercihi (zorunlu/işlevsel): seçtiğiniz dili (TR/EN) hatırlar.",
        ],
      },
      {
        heading: "Tarayıcı depolaması",
        body: [
          "Sepetiniz ve favorileriniz, siz giriş yapmadan da korunsun diye tarayıcınızın yerel depolamasında (localStorage) tutulur. Bu veriler sunucumuza yalnızca ödeme sırasında, sepetinizdeki ürünler olarak iletilir.",
        ],
      },
      {
        heading: "Zorunlu olmayan çerezler",
        body: [
          "Şu anda analitik, reklam veya üçüncü taraf izleme çerezi kullanmıyoruz. İleride kullanmaya başlarsak bu sayfayı güncelleyecek ve zorunlu olmayan çerezler için önceden onayınızı isteyeceğiz.",
        ],
      },
      {
        heading: "Çerezleri yönetme",
        body: [
          "Çerezleri tarayıcı ayarlarınızdan silebilir veya engelleyebilirsiniz. Zorunlu çerezleri engellerseniz giriş yapamaz ve siparişinizi tamamlayamazsınız. Sepet ve favorileri silmek için tarayıcınızın site verilerini temizlemeniz yeterlidir.",
        ],
      },
    ],
  },
};

const en: Record<LegalSlug, LegalDoc> = {
  privacy: {
    title: "Privacy Policy",
    intro: "At Rovena we care about the privacy of your personal information. This policy explains what we collect when you use our site, how we use it and how we protect it.",
    sections: [
      {
        heading: "Information we collect",
        body: [
          "- Account details: first name, last name, email address and password (the password is stored only in an irreversibly hashed form).",
          "- Address details: the name, phone number and address you save for delivery.",
          "- Order details: the products you buy, the amount, the order status and shipment information.",
          "- Payment details: your card number is not sent to or stored on Rovena servers; payment is made with a payment token generated from your card.",
          "- Technical data: session information the site needs to work and security request logs (IP address, date, requested page).",
        ],
      },
      {
        heading: "How we use it",
        body: [
          "We use your information to receive your order, complete the payment, deliver the product, handle returns and exchanges, manage your account and meet our legal obligations. Marketing messages such as the newsletter are sent only with your consent, and you can unsubscribe in every message.",
        ],
      },
      {
        heading: "Sharing with third parties",
        body: [
          "We do not sell your information. We share it only as far as the service requires with carriers, payment service providers and hosting/infrastructure providers, and with authorities when the law requires it.",
        ],
      },
      {
        heading: "Retention",
        body: [
          "We keep your information for as long as needed for its purpose and for the periods required by tax, commercial and consumer law, then delete or anonymise it. Contact us to close your account.",
        ],
      },
      {
        heading: "Security",
        body: [
          "Passwords are stored irreversibly, session data lives in httpOnly cookies that browser scripts cannot read, and communication between our services is authorised. No system is entirely risk free; in the event of a breach we meet the notification duties set by law.",
        ],
      },
      {
        heading: "Your rights and contact",
        body: [
          "You can exercise your rights to access, correct, delete and object to the processing of your personal data as described in the KVKK Notice.",
          "Contact: Rovena, info@rovena.com",
        ],
      },
    ],
  },
  kvkk: {
    title: "KVKK Privacy Notice",
    intro: "Under Article 10 of the Turkish Personal Data Protection Law (No. 6698, KVKK) we inform you about how your personal data is processed.",
    sections: [
      {
        heading: "Data controller",
        body: ["Rovena, info@rovena.com"],
      },
      {
        heading: "Data processed and purposes",
        body: [
          "- Identity and contact (name, email, phone): creating your account and order-related communication.",
          "- Address: delivering the order and invoicing.",
          "- Customer transactions (orders, returns, cancellations): forming and performing the contract and after-sales service.",
          "- Transaction security (IP address, request logs, session data): information security and preventing fraud and misuse.",
          "- Marketing (email address): campaign and new-collection messages, only with your explicit consent.",
        ],
      },
      {
        heading: "Collection method and legal grounds",
        body: [
          "Your data is collected electronically through the website as you fill in forms and use the site. Legal grounds: forming and performing a contract, complying with a legal obligation, the controller's legitimate interest and, for marketing messages, your explicit consent.",
        ],
      },
      {
        heading: "Transfers",
        body: [
          "Your data may be transferred, limited to the purposes above, to carriers and logistics companies, payment service providers, IT and hosting providers and authorised public bodies as required by law. Any transfer abroad follows the conditions of Article 9 of the Law.",
        ],
      },
      {
        heading: "Your rights under Article 11",
        body: [
          "- To learn whether your personal data is processed and to request information about it,",
          "- To learn the purpose of processing and whether it is used accordingly,",
          "- To know the third parties it is transferred to, in Turkey or abroad,",
          "- To request correction if it is incomplete or inaccurate,",
          "- To request deletion or destruction under the conditions in the Law, and that this be notified to third parties it was transferred to,",
          "- To object to a result against you arising from analysis exclusively by automated systems,",
          "- To claim compensation for damage caused by unlawful processing.",
        ],
      },
      {
        heading: "How to apply",
        body: [
          "Send your requests to [KVKK request email] or in writing to the address above. Your request is answered within thirty days at the latest, depending on its nature.",
        ],
      },
    ],
  },
  "distance-sales": {
    title: "Distance Sales Agreement",
    intro: "This agreement is drawn up for sales made through the website under the Turkish Consumer Protection Law (No. 6502) and the Distance Contracts Regulation. By confirming your order you accept its terms.",
    sections: [
      {
        heading: "1. Parties",
        body: [
          "Seller: Rovena, info@rovena.com.",
          "Buyer: the member who gives their name, phone number and delivery address when ordering.",
        ],
      },
      {
        heading: "2. Subject",
        body: ["The rights and obligations of the parties for the sale and delivery of the product(s) the Buyer orders electronically on the Rovena website, whose features and sale price are shown in the order summary."],
      },
      {
        heading: "3. Product, price and payment",
        body: [
          "The main features, size and colour, the VAT-inclusive price and the delivery charge appear on the checkout page and in the order summary. Prices are in Turkish lira. Payment is by credit/debit card; the order is not complete until the payment is approved. If payment cannot be taken, the order is cancelled and the reserved stock is released; any amount already charged is refunded.",
        ],
      },
      {
        heading: "4. Delivery",
        body: [
          "The product is delivered by carrier to the delivery address given by the Buyer within the time shown on the order page (usually 2–4 business days for items in stock, the time on the product page for made-to-order items), and in any case within the legal maximum of thirty days. If the packaging is damaged on delivery, have a report drawn up and tell the Seller.",
          "You can follow your order status and tracking number under Orders in your account.",
        ],
      },
      {
        heading: "5. Right of withdrawal",
        body: [
          "The Buyer may withdraw from the contract within fourteen days of receiving the product, without giving a reason and without penalty. In addition to this legal period, Rovena offers free returns and exchanges within 30 days of delivery.",
          "Send the withdrawal notice to [contact email]. The returned product must be unused, with its tags and original packaging. The Seller refunds the price, by the payment method used, within fourteen days of receiving the withdrawal notice.",
        ],
      },
      {
        heading: "6. Cases without a right of withdrawal",
        body: [
          "By law, the right of withdrawal does not apply to products made to the Buyer's wishes or personal needs (for example monogrammed items), perishable goods, hygiene products whose packaging has been opened after delivery, and the other cases listed in the Regulation. Any extra return option Rovena offers for personalised items does not affect your rights for defective products.",
        ],
      },
      {
        heading: "7. Defective products",
        body: ["If the product is defective, the Buyer may use the options in the relevant articles of Law No. 6502: withdrawal, a price reduction, free repair or replacement."],
      },
      {
        heading: "8. Disputes",
        body: ["For disputes arising from this agreement, within the monetary limits announced each year by the Ministry of Trade, the Consumer Arbitration Committees at the Buyer's place of residence or place of order are competent; above these limits, the Consumer Courts."],
      },
      {
        heading: "9. Entry into force",
        body: ["By confirming the order the Buyer declares that they have read, understood and electronically accepted all terms of this agreement. A copy is kept with your order record."],
      },
    ],
  },
  cookies: {
    title: "Cookie Policy",
    intro: "Cookies are small text files stored in your browser when you visit a site. Rovena uses only the cookies and browser storage the site needs to work; we do not use advertising or tracking cookies.",
    sections: [
      {
        heading: "Cookies we use",
        body: [
          "- rovena_at and rovena_rt (essential): keep you signed in. They are httpOnly cookies that browser scripts cannot read; they are created when you sign in and deleted when you sign out. rovena_at is short-lived (15 minutes) and renewed with rovena_rt.",
          "- Language preference (essential/functional): remembers the language you chose (TR/EN).",
        ],
      },
      {
        heading: "Browser storage",
        body: [
          "Your cart and wishlist are kept in your browser's local storage so they survive without signing in. This data reaches our servers only at checkout, as the items in your cart.",
        ],
      },
      {
        heading: "Non-essential cookies",
        body: [
          "We currently use no analytics, advertising or third-party tracking cookies. If we start to, we will update this page and ask for your consent first for non-essential cookies.",
        ],
      },
      {
        heading: "Managing cookies",
        body: [
          "You can delete or block cookies in your browser settings. If you block the essential cookies you cannot sign in or complete an order. To remove the cart and wishlist, clear the site data in your browser.",
        ],
      },
    ],
  },
};

export const legalDocs: Record<Locale, Record<LegalSlug, LegalDoc>> = { tr, en };

export function isLegalSlug(v: string): v is LegalSlug {
  return (legalSlugs as readonly string[]).includes(v);
}
