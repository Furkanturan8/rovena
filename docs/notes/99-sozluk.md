# Sözlük

| Terim | Anlamı |
| --- | --- |
| **Monolit** | Tüm özelliklerin tek programda olduğu yapı |
| **Mikroservis** | Tek bir iş alanından sorumlu, bağımsız çalışan küçük servis |
| **API** | Programların birbiriyle konuşma arayüzü |
| **REST** | HTTP üzerinden kaynak odaklı API stili (`GET /users/me`) |
| **API Gateway** | Tüm isteklerin girdiği tek kapı; doğru servise yönlendirir (burada Nginx) |
| **Konteyner / Docker** | Programı bağımlılıklarıyla izole çalıştıran paket |
| **Docker Compose** | Birden fazla konteyneri tek dosyayla yöneten araç |
| **Migration** | Veritabanı şemasındaki değişikliklerin sürümlü SQL dosyaları olarak uygulanması |
| **Seed** | Başlangıç verisini (ilk admin gibi) oluşturma |
| **ORM (Bun)** | Veritabanı satırlarını Go struct'larına bağlayan kütüphane |
| **Fiber** | Kullandığımız Go web framework'ü |
| **Middleware** | İstek handler'a ulaşmadan (ya da sonra) araya giren ortak işlem: log, yetki kontrolü, rate limit |
| **Handler** | Bir route'a gelen isteği karşılayan fonksiyon |
| **DTO** | API'nin dış veri şekli; iç modelden ayrı tutulur |
| **Domain** | Uygulamanın iş alanı: kavramlar ve kuralları (kullanıcı, rol, sipariş...) |
| **Repository** | Veri erişimini soyutlayan katman |
| **Interface (arayüz)** | "Bu metodlara sahip olan her şey" sözleşmesi; sahteleriyle değiştirmeyi mümkün kılar |
| **Dependency Injection** | Bir nesnenin ihtiyaç duyduğu parçaları kendisi yaratmayıp dışarıdan alması |
| **Composition root** | Tüm nesnelerin oluşturulup bağlandığı tek yer (`cmd/api/main.go`) |
| **JWT** | İmzalı, içinde bilgi taşıyan token |
| **Access token** | Kısa ömürlü, her istekte gönderilen JWT |
| **Refresh token** | Uzun ömürlü, yalnızca yeni access token almak için kullanılan, sunucuda iptal edilebilen token |
| **Token rotasyonu** | Her refresh'te eski refresh token'ın iptal edilip yenisinin verilmesi |
| **bcrypt** | Bilerek yavaş çalışan parola hash algoritması |
| **Hash** | Tek yönlü özet; geri çevrilemez |
| **Rate limit** | Belirli sürede izin verilen istek sayısı sınırı |
| **CORS** | Tarayıcının başka bir alan adından gelen isteklere izin verme kuralı |
| **Idempotent** | Aynı işlemin birden fazla uygulanmasının sonucu değiştirmemesi |
| **Graceful shutdown** | Kapanırken süren işleri bitirip düzgünce çıkma |
| **Liveness / Readiness** | "Süreç ayakta mı" / "bağımlılıklar hazır, trafik alabilir mi" kontrolleri |
| **Saga** | Servisler arası işlemi adım adım yapıp hata olursa telafi eden kalıp (ileride) |
| **Transactional Outbox** | Olayı iş verisiyle aynı transaction'da kaydedip sonra güvenle yayınlama kalıbı (ileride) |
| **Mesaj kuyruğu (RabbitMQ)** | Servisler arası asenkron mesaj taşıyan aracı (ileride) |
