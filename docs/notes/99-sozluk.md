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
| **SKU** | Stok kodu; bir varyantı tekil olarak tanımlar (`TEE-BLK-M`) |
| **Varyant** | Bir ürünün satın alınabilir somut hali (beden + renk + fiyat) |
| **Slug** | URL'de kullanılan okunur ad (`deri-ceket`) |
| **Minor unit (kuruş)** | Paranın en küçük birimi; fiyat tamsayı olarak bunla saklanır |
| **Trigram (`pg_trgm`)** | Metni 3 harflik parçalara bölerek `LIKE '%x%'` aramasını indekslenebilir kılan PostgreSQL eklentisi |
| **Soft delete / arşivleme** | Kaydı silmek yerine görünmez duruma getirme |
| **Race condition (yarış durumu)** | Sonucun, iki işlemin hangi sırayla araya girdiğine bağlı olması |
| **Check-then-act** | "Kontrol et, sonra işlem yap" kalıbı; kontrol ile işlem arasında durum değişebildiği için kilitsiz güvensizdir |
| **Row lock (`FOR UPDATE`)** | Bir satırı transaction bitene kadar diğer işlemlere karşı kilitler; işlemler sırayla çalışır |
| **Partial index (kısmi indeks)** | Yalnızca bir koşulu sağlayan satırları kapsayan indeks (`WHERE active`) |
| **EXPLAIN ANALYZE** | PostgreSQL'in bir sorguyu nasıl çalıştırdığını ve ne kadar sürdüğünü gösteren komut |
| **Entegrasyon testi** | Sahte bağımlılık yerine gerçek veritabanı gibi gerçek parçalarla yapılan test |
| **Mutasyon testi** | Kodu bilerek bozup testlerin başarısız olup olmadığına bakarak testlerin kalitesini ölçme |
| **Test double (sahte)** | Gerçek bir bağımlılığın yerine geçen test nesnesi (bellek içi repository gibi) |
| **Build tag** | Bir dosyanın yalnızca belirli bir bayrakla (`-tags integration`) derlenmesini sağlayan Go işareti |
| **Fixture** | Bir testin ihtiyaç duyduğu, önceden kurulan örnek veri |
| **Advisory lock** | Bir satıra değil, uygulamanın seçtiği bir sayıya bağlı PostgreSQL kilidi; tabloyu ilgilendiren kuralları serileştirmek için kullanılır |
| **Mass assignment** | İstemcinin göndermemesi gereken alanları (`role`, `id`) gönderip sunucunun bunları körü körüne kaydetmesi |
| **User enumeration** | Giriş/kayıt cevaplarının farkından hangi e-postaların kayıtlı olduğunun anlaşılması |
| **OpenAPI** | REST API'nin yollarını, parametrelerini ve cevap şemalarını anlatan standart, makine okunur format |
| **Sözleşme testi (contract test)** | API'nin gerçek davranışının yazılı şemayla uyuştuğunu doğrulayan test |
| **Şema (schema)** | Bir JSON'un hangi alanlara, tiplere ve kısıtlara sahip olması gerektiğinin tarifi |
| **Overselling** | Elde olmayan malı satmak; stok servisinin engellemek için var olduğu asıl risk |
| **Rezervasyon** | Stoğun bir sipariş için geçici olarak ayrılması; onaylanır (satış) ya da bırakılır |
| **Koşullu güncelleme** | Kontrolü `UPDATE ... WHERE stok >= n` gibi güncellemenin kendi içine koymak; check-then-act yarışını ortadan kaldırır |
| **TTL (time to live)** | Bir kaydın geçerlilik süresi; rezervasyonda "bu kadar süre sonra stok geri verilir" |
| **Deadlock (kilitlenme)** | İki işlemin birbirinin kilidini beklemesi; sabit bir kilit sırası belirleyerek önlenir |
| **Ledger (hareket defteri)** | Değişiklikleri üzerine yazmak yerine art arda ekleyen kayıt; "sayı neden bu" sorusu sonradan cevaplanabilir |
| **Telafi işlemi (compensating action)** | Geri alınamayan bir adımın etkisini tersine çeviren işlem (rezervasyonu bırakmak gibi) |
| **Saga** | Servisler arası işlemi adım adım yapıp hata olursa telafi eden kalıp (ileride) |
| **Transactional Outbox** | Olayı iş verisiyle aynı transaction'da kaydedip sonra güvenle yayınlama kalıbı (ileride) |
| **Mesaj kuyruğu (RabbitMQ)** | Servisler arası asenkron mesaj taşıyan aracı (ileride) |
