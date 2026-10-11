# 02 - Proje haritası

```text
rovena/
├── .claude/                  Claude Code ayarları (kurallar, izinler); uygulamanın parçası değil
├── CLAUDE.md                 Claude için proje özeti ve mimari kararlar
├── docs/                     Dokümantasyon
│   ├── readme.md             Proje tanıtımı
│   ├── architecture.md       Hedef mimari
│   └── notes/                Bu öğrenme notları
├── shared/                   Tüm servislerin ortak kullandığı Go kodu (ayrı Go modülü)
├── services/
│   ├── identity/             Kullanıcı ve kimlik servisi (ayrı Go modülü)
│   ├── catalog/              Ürün kataloğu servisi (ayrı Go modülü)
│   ├── inventory/            Stok ve rezervasyon servisi (ayrı Go modülü)
│   └── payment/              Ödeme servisi (ayrı Go modülü)
├── infrastructure/           Servis olmayan altyapı ayarları
│   ├── postgres/             Veritabanlarını oluşturan başlangıç betiği
│   └── nginx/                API Gateway ayarları
├── docker-compose.yml        Tüm sistemi tek komutla ayağa kaldıran tanım
├── Makefile                  Kısa komutlar (make up, make test)
├── .env.example              Gerekli ortam değişkenlerinin şablonu
├── .env                      Gerçek (gizli) değerler; git'e girmez
└── .gitignore
```

## Kök dosyalar

- **`docker-compose.yml`**: Hangi konteynerlerin (Postgres, RabbitMQ, Identity, Nginx...) nasıl çalışacağını tanımlar. Ayrıntı: [03](03-altyapi.md).
- **`Makefile`**: `make up`, `make down`, `make test`, `make seed-admin` gibi uzun komutlara kısayollar.
- **`.env` / `.env.example`**: Parolalar ve JWT secret gibi gizli değerler. `.env.example` şablondur ve git'e girer, `.env` gerçek değerleri taşır ve girmez. Gizli değerler kodun içine yazılmaz; konteynere ortam değişkeni olarak verilir.
- **`CLAUDE.md`**: Yapay zeka asistanına projeyi anlatan not. Diğer geliştiriciler için de kısa bir özet işlevi görür.

## Neden her servis ve shared ayrı `go.mod`?

Go'da `go.mod` bir "modül"ü tanımlar (bağımlılıkları ve sürümleriyle birlikte bir paket topluluğu). Her servis kendi modülü olduğu için:

- Servislerin bağımlılıkları birbirinden bağımsız yükseltilebilir.
- Bir servis tek başına derlenebilir ve kendi Docker imajına paketlenebilir.
- Servis sınırı kodda da görünür: bir servis, ötekinin `internal/` klasörünü import edemez.

`services/identity/go.mod` içinde şu satırlar `shared` modülünü bağlar:

```text
require github.com/Furkanturan8/rovena/shared v0.0.0
replace github.com/Furkanturan8/rovena/shared => ../../shared
```

`replace`, "bu modülü internetten değil, bilgisayardaki şu klasörden al" demektir. Böylece `shared`'de yaptığın değişiklik servislerde anında görünür.

## `internal/` klasörü neden var?

Go'da `internal/` altındaki paketler yalnızca onun bulunduğu modülün içinden import edilebilir. Yani Catalog servisi, Identity'nin `internal/domain` paketini kullanamaz. Bu, "servisler birbirinin içine uzanmasın" kuralını derleyiciye zorlatır.

## `cmd/` klasörü

Bir Go modülü birden fazla çalıştırılabilir program üretebilir. Her biri `cmd/<ad>/main.go` içinde yaşar. Her serviste üç tane var: `api` (web sunucusu), `migrate` (veritabanı tablolarını kuran program), `seed` (başlangıç verisi: Identity'de ilk admin, Catalog'da demo ürünler, Inventory'de bu ürünlerin açılış stoğu). Ayrıntı: [05](05-identity-katmanlar.md).
