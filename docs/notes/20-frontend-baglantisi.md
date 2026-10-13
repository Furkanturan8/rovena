# 20. Frontend'i backend'e bağlamak

Mağaza (`frontend/web`, Next.js) artık mock veri kullanmaz: ürünler, kategoriler, stok, üyelik, sepetten siparişe kadar her şey gateway üzerinden gerçek servislerden gelir. Bu bölüm bağlantının nasıl kurulduğunu ve backend'de neyin değiştiğini anlatır.

## Büyük resim

```
Tarayıcı ──► Next.js (frontend/web) ──► Gateway (nginx :8080) ──► identity / catalog / inventory / order / payment
```

- **Sunucu tarafı** (server component, server action, route handler) gateway'i `API_BASE_URL` ile çağırır.
- **Tarayıcı** yalnızca herkese açık okumaları (`/api/v1/products`, `categories`, `stock`) Next üzerinden yapar; Next bunları gateway'e yönlendirir (`next.config.ts` rewrites). Böylece CORS gerekmez.
- **Token'lar tarayıcı JavaScript'ine verilmez.** Access ve refresh token `httpOnly` çerezlerde durur; kimlik gerektiren çağrıları Next sunucusu o kullanıcı adına yapar.

## Katalog verisi: şema genişletmesi

Mağazanın ihtiyacı olan iki şey katalog tablolarında kolon olarak yoktu: **çok dilli metinler** ve **ürün özellikleri** (etiket, puan, işçilik süresi, renk örnekleri...). `migrations/00004_translations_attributes.sql` iki `jsonb` kolon ekler:

| Kolon | Tablo | İçerik |
| --- | --- | --- |
| `translations` | `categories`, `products` | `{"en": {"name": "...", "description": "...", "kind": "...", "material": "...", "composition": [...]}, "tr": {...}}`. `name` ve `description` kolonları varsayılan (Türkçe) metin olarak kalır |
| `attributes` | `products` | dilden bağımsız bilgiler: `sku_base`, `craft_hours`, `tag`, `rating`, `colors` ... |

Catalog bu JSON'un içini yorumlamaz, yalnızca JSON **nesnesi** olduğunu ve 32 KB'ı aşmadığını doğrular. Arama (`search_text`) İngilizce ad ve tür metinlerini de içerir, yani `trench` aramak `Manston Trençkot`'u bulur.

Diğer API değişiklikleri:

- Ürün ve kategori oluştururken isteğe bağlı `slug` verilebilir (kararlı adresler için); geçersizse 422, kullanımdaysa 409.
- **Listeleme cevabı** artık kartı çizmek için gereken her şeyi taşır: `description`, `images` (tümü), `variants` (yalnızca aktifler), `translations`, `attributes`. Mağaza ürün başına ayrı istek atmaz.

## Seed verisi

`make seed-catalog` artık `services/catalog/cmd/seed/store.json` dosyasını yükler (5 kategori, 19 ürün, 91 varyant; Türkçe ve İngilizce metinler). Görsel adresleri `.env` içindeki `MEDIA_BASE_URL` (Supabase Storage'ın herkese açık bucket'ı) altında mutlak URL olarak saklanır. `make seed-inventory` her varyanta `cmd/seed/stock.json` içindeki açılış stokunu verir (tükenmiş ve "son parçalar" bedenleri dahil).

```bash
make seed-catalog   SEED_FLAGS=-reset   # eski demo kataloğu silip yenisini yükler (yalnızca geliştirme)
make seed-inventory SEED_FLAGS=-reset
```

Fiyatlar kuruş cinsinden saklanır (`168000 ₺` → `16800000`).

## Frontend'in yaptıkları

| İş | Nasıl |
| --- | --- |
| Ürün listesi, ürün detayı, kategori | `lib/data/catalog.ts`: sunucuda `/products` + `/categories` + `/stock`; `lib/api/mappers.ts` seçilen dile göre metinleri çözer |
| Filtre, sıralama, sayfalama | Katalog küçük olduğu için Next'te yapılır (tüm liste tek seferde çekilir, 30 sn önbellek). Büyüdüğünde API'nin `q`, `category`, `size`, `color`, `min_price`, `sort` parametrelerine taşınmalı |
| Stok | Varyant bazlı `/stock?variant_ids=...`; beden düğmeleri seçili renge göre kapanır |
| Üyelik | Server action'lar: `register`, `login`, `logout`, profil güncelleme. Oturumu `proxy.ts` yeniler: access çerezi bittiyse refresh çerezini kullanır |
| Sepet | Tarayıcıda (localStorage), yalnızca `variantId` ve adet tutulur. Ad, fotoğraf ve fiyat her zaman katalogdan okunur |
| Ödeme | Giriş gerekir. Server action sepeti Order servisine yazar (`DELETE /cart`, `PUT /cart/items/:id`) ve `POST /checkout` çağırır (`Idempotency-Key` ile). Fiyatı Order servisi katalogdan kendisi hesaplar; tarayıcıdan fiyat gönderilmez |
| Sipariş takibi | Onay sayfası sipariş durumunu 2 sn'de bir yoklar (`/api/orders/:id`): `pending → awaiting_payment → paid → confirmed`, ya da `cancelled` |
| Siparişlerim | Gerçek siparişler; ödenmemiş sipariş iptal edilebilir (`POST /orders/:id/cancel`). Detayda teslimat adresi, kargo firması ve takip no görünür |
| Adresler | Identity'nin `/users/me/addresses` uçları: ekleme, düzenleme, silme, varsayılan yapma. Ödemede kayıtlı adres seçilir (yoksa orada eklenir); sunucu aksiyonu adresi Identity'den okuyup `shipping_address` olarak gönderir, sipariş adresin **kopyasını** saklar |
| Şifre / e-posta | `POST /users/me/password` (diğer cihazların oturumunu kapatır, yeni token çifti döner, çerezler yenilenir) ve `POST /users/me/email`; ikisi de mevcut şifre ister |
| Teslimat aşamaları | Order servisinde `confirmed → shipped → delivered`. Frontend'de buton yok; operatör `POST /admin/orders/:id/ship` (`carrier`, `tracking_code`) ve `/deliver` çağırır |

### Kart numarası ve test kartları

Tarayıcı kart numarasını **sunucuya göndermez**; yalnızca bir ödeme sağlayıcı token'ı gider (`tok_visa`, `tok_declined`, ...). Sahte sağlayıcı için eşleme `components/checkout/checkout-form.tsx` içindedir: `4242 4242 4242 4242` onay, `4000 0000 0000 0002` ret, `4000 0000 0000 9995` yetersiz bakiye. Sahte sağlayıcının tutar sınırı `FAKE_PROVIDER_MAX_AMOUNT_CENTS` ile ayarlanır (Compose'ta 50 milyon TL); varsayılan 100.000 TL, mağazanın ürünlerini reddederdi.

## Bilinen boşluklar

- **Kargo, kupon, havale:** Order servisi yalnızca ürün toplamını tahsil eder; kargo ücreti, indirim kodu ve havale seçeneği bu yüzden mağazada yoktur.
- **Favoriler:** tarayıcıda (localStorage) tutulur, backend karşılığı yok.
- **Kargo/teslim paneli:** durumu ilerleten yalnızca admin API'si var; yönetim arayüzü yok. E-posta/bildirim gönderimi de yok.
- **E-posta değişimi:** doğrulama e-postası yok; yeni adres hemen geçerli olur. Önceden verilmiş access token'lar süresi (15 dk) dolana kadar eski adresi taşır.
- **Dağıtım:** Vercel'deki frontend'in çalışması için gateway'in internetten erişilebilir bir adreste olması ve `API_BASE_URL`'in ona işaret etmesi gerekir. Şu an backend yalnızca yerelde (Docker Compose) çalışıyor.
