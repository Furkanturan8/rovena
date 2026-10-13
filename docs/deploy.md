# Yayına alma (VPS + Vercel)

Mimari: **backend** tek bir VPS'te Docker Compose ile çalışır (5 servis, Postgres, RabbitMQ, nginx gateway). HTTPS'i sunucudaki mevcut nginx sağlar. **Frontend** Vercel'de çalışır ve backend'i yalnızca Next'in sunucu tarafından çağırır (server action'lar, `rewrites`, `/api/orders/[id]`); tarayıcı backend'e doğrudan istek atmaz, bu yüzden CORS ayarı gerekmez.

```
Tarayıcı ──► Vercel (Next.js) ──HTTPS──► VPS: nginx :443 ──► gateway :18080 ──► servisler
                                                                   Postgres, RabbitMQ (dışarıya kapalı)
```

## 1. Sunucuyu hazırla

Ubuntu/Debian varsayılıyor. En az 2 GB RAM önerilir (Grafana/Prometheus/Jaeger dahil tüm yığın için).

```bash
# Docker (resmi betik)
curl -fsSL https://get.docker.com | sh
sudo usermod -aG docker $USER   # sonra çıkış yapıp tekrar gir

# Güvenlik duvarı: yalnızca SSH ve web
sudo ufw allow 22/tcp && sudo ufw allow 80/tcp && sudo ufw allow 443/tcp && sudo ufw enable
```

Compose varsayılan olarak hiçbir portu dışarı açmaz (gateway dahil `127.0.0.1`'e bağlıdır). Postgres, RabbitMQ, Grafana, Jaeger, Prometheus ve servis portları `127.0.0.1`'e bağlıdır.

## 2. Alan adı ve HTTPS

HTTPS gerekli (şifreler ve token'lar Vercel ile sunucu arasında dolaşır). Önce bir ad belirle:

- **Kendi alan adın varsa:** `api.alanadin.com` için sunucu IP'sine bir `A` kaydı ekle.
- **Alan adı yoksa:** `sslip.io` IP'yi alt alan adına çevirir; IP `203.0.113.10` ise `203-0-113-10.sslip.io`.

HTTPS'i sunucudaki nginx sağlar. Gateway `127.0.0.1:18080`'de dinliyor (8080 ve 8081 başka işler tarafından sık kullanıldığı için bilerek 18080/18081 seçildi); nginx'e yeni bir site ekle:

```nginx
# /etc/nginx/sites-available/rovena-api  (sites-enabled'a link ver)
server {
    server_name api.alanadin.com;          # ya da 203-0-113-10.sslip.io
    location / {
        proxy_pass http://127.0.0.1:18080;
        proxy_set_header Host $host;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_read_timeout 30s;
    }
}
```

```bash
sudo ln -s /etc/nginx/sites-available/rovena-api /etc/nginx/sites-enabled/
sudo nginx -t && sudo systemctl reload nginx
sudo certbot --nginx -d api.alanadin.com   # sertifikayı alır ve 443'ü ayarlar, kendiliğinden yeniler
```

Diğer sitelerine dokunmaz: nginx isteği `server_name`'e göre ayırır.

## 3. Kodu ve ayarları getir

```bash
git clone <repo-adresi> rovena && cd rovena/backend
cp .env.example .env
```

`.env` içinde **her değeri değiştir** (repoya girmez):

```bash
openssl rand -base64 24    # her *_PASSWORD için
openssl rand -base64 48    # JWT_SECRET ve INTERNAL_API_KEY için (iki ayrı değer)
```

Ayrıca: `ADMIN_EMAIL`, `ADMIN_PASSWORD` (ilk admin) ve `MEDIA_BASE_URL` (Supabase Storage'daki `media` kovası, `.../storage/v1/object/public/media`).

## 4. Başlat

```bash
make prod-up        # build + migrate + servisler
docker compose ps   # hepsi Up / migrate'ler Exited (0) olmalı
curl localhost:18080/healthz          # ok (sunucu içinden)
curl https://<api-adın>/healthz      # ok (HTTPS kuruluysa, dışarıdan)
```

İlk çalıştırmada imajlar sunucuda derlenir (birkaç dakika). 

Verileri yükle (bir kez):

```bash
make seed-admin
make seed-catalog
make seed-inventory
```

## 5. Frontend'i Vercel'e bağla

Vercel'de projeyi GitHub'dan içe aktar:

- **Root Directory:** `frontend/web`
- **Environment Variables:**
  - `API_BASE_URL` = `https://<api-adın>` (sonda `/` olmadan)
  - `NEXT_PUBLIC_MEDIA_URL` = `https://<proje>.supabase.co/storage/v1/object/public/media`
- Deploy et. İsteğe bağlı: Functions bölgesini sunucuya yakın seç (Settings → Functions → Region, ör. Frankfurt).

Kontrol: mağaza açılıyor mu, ürünler geliyor mu, üye ol → giriş → sepet → test kartı `4242 4242 4242 4242` ile sipariş → onay sayfası `confirmed`.

## 6. Güncelleme

```bash
cd rovena && git pull && cd backend && make prod-up
```

Migration'lar otomatik çalışır. Frontend, Vercel'de `main`'e push ile kendiliğinden yenilenir.

## 7. İşletme notları

- **Gözlem araçları** dışarıya kapalı; SSH tüneli ile aç: `ssh -L 3000:localhost:3000 -L 16686:localhost:16686 kullanici@sunucu` (Grafana, Jaeger). RabbitMQ yönetimi için `15672`, Prometheus için `9090`.
- **Loglar:** `docker compose logs -f --tail 100 order` (servis adı: identity, catalog, inventory, order, payment, gateway).
- **Yedek:** `docker compose exec -T postgres pg_dumpall -U postgres | gzip > rovena-$(date +%F).sql.gz` (cron ile günlük çalıştırılabilir). Volume'lar: `rovena_postgres-data`, `rovena_rabbitmq-data`. `docker compose down -v` bunları siler, kullanma.
- **Sahte ödeme:** ödeme sağlayıcısı demo amaçlıdır (`tok_visa` vb.); gerçek kart işlemi yapılmaz.
- **İstek sınırları:** Vercel tüm istekleri sunucu tarafından yaptığı için gateway ve servislerin IP başına sınırları (giriş: dakikada 20, genel: saniyede 30) tüm ziyaretçiler için ortak sayılır. Demo için yeterli; trafik artarsa Next'ten gerçek istemci IP'sini iletmek gerekir.
- **Sırlar** (`.env`) sunucuda kalır; `JWT_SECRET` değişirse tüm oturumlar düşer.
