# 01 - Mikroservis nedir?

## Monolit

Tüm özelliklerin (kullanıcılar, ürünler, siparişler, ödeme) tek bir programda ve genelde tek bir veritabanında olduğu yapıdır. Tek parça derlenir, tek parça yayınlanır.

Artıları: basit, hızlı geliştirilir, tek veritabanı transaction'ı her şeyi tutarlı tutar.
Eksileri: büyüdükçe karmaşıklaşır, bir parçadaki hata tüm sistemi düşürebilir, parçalar bağımsız ölçeklenemez.

## Mikroservis

Sistem, her biri **tek bir iş alanından sorumlu** küçük, bağımsız programlara bölünür. Her servis:

- Kendi kodunu taşır, ayrı derlenir ve ayrı çalışır.
- **Kendi veritabanına sahiptir.** Başka servisin tablosuna asla doğrudan dokunmaz.
- Diğer servislerle ağ üzerinden konuşur (HTTP ya da mesaj kuyruğu).

Artıları: bağımsız geliştirme/yayınlama, hata izolasyonu, servis bazında ölçekleme.
Eksileri: ağ hataları, dağıtık tutarlılık problemi, daha çok altyapı, hata ayıklaması zor.

Bu projede mikroservis seçilmesinin sebebi, bu zorlukları öğrenmektir. Gerçek bir proje için bu boyutta bir e-ticaret çoğu zaman monolit olarak da yeterlidir. `docs/architecture.md` bunu açıkça belirtir.

## Rovena'daki servisler

| Servis | Sorumluluğu | Durum |
| --- | --- | --- |
| Identity | Kullanıcı kaydı, giriş, roller | Yazıldı |
| Catalog | Ürünler, kategoriler, varyantlar | Yazıldı |
| Inventory | Stok, rezervasyon | Yazıldı |
| Order | Sepet, sipariş, checkout akışı | Planlı |
| Payment | Ödeme (sahte sağlayıcı) | Yazıldı |

## İki tür iletişim

**Senkron (REST/HTTP):** "Bunu yap ve cevabını bekliyorum." Örnek: tarayıcı `GET /api/v1/users/me` der ve cevabı bekler.

**Asenkron (RabbitMQ mesajları):** "Şu oldu, ilgilenen bakar." Örnek: Order servisi "OrderCreated" olayını yayınlar; Inventory servisi bunu alıp stok ayırır. Gönderen cevabı beklemez. Altyapısı hazır ([12](12-mesajlasma.md)); ilk kullanıcıları Payment ve Order servisleri olacak.

## En zor konu: dağıtık tutarlılık

Monolitte "sipariş oluştur + stok düş + ödeme al" tek veritabanı transaction'ıdır; ya hepsi olur ya hiçbiri. Mikroservislerde bu üç iş üç ayrı veritabanındadır, tek transaction yoktur. Çözüm kalıpları (ileride yazılacak):

- **Saga:** Adımlar sırayla yapılır, bir adım başarısız olursa önceki adımlar "telafi" edilir (örn. ödeme düşerse ayrılan stok geri bırakılır).
- **Transactional Outbox:** Olay, iş verisiyle aynı transaction'da bir tabloya yazılır; ayrı bir süreç onu kuyruğa gönderir. Böylece "veriyi kaydettim ama olayı göndermeyi unuttum" durumu olmaz.
- **Idempotency:** Aynı mesaj/istek iki kez gelse bile sonuç tek kez uygulanmış gibi olmalı.

Şu an yalnızca Identity yazıldığı için bu kalıplar henüz kodda yok.

## API Gateway

İstemci (tarayıcı) beş farklı servisin adresini bilmek zorunda kalmasın diye hepsinin önüne tek bir kapı konur. Rovena'da bu Nginx'tir (`localhost:8080`). Yola bakıp isteği doğru servise yönlendirir. Ayrıntı: [03 - Altyapı](03-altyapi.md).
