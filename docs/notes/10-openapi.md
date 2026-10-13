# 10 - API sözleşmesi: OpenAPI

## OpenAPI nedir?

Bir API'nin makine tarafından okunabilir tarifidir: hangi yollar var, hangi parametreleri alır, hangi durum kodlarını ve hangi JSON şeklini döner. Rovena'da servis başına bir dosya var: `docs/openapi/identity.yaml`, `catalog.yaml` ve `inventory.yaml` (ayrıntı ve kullanım: `docs/openapi/README.md`).

Bu dosyalar üç işe yarar:

1. **İnsan için dokümantasyon:** Frontend yazan biri koda bakmadan hangi uca ne gönderip ne alacağını görür.
2. **Elle test aracı:** Postman, Insomnia, Bruno gibi istemciler dosyayı içe aktarır; tüm uçlar, örnek gövdeler ve açıklamalarla hazır gelir.
3. **Sözleşme:** Mikroservislerde servisler ve istemciler birbirinden bağımsız değişir. API'nin şekli bir "sözleşme"dir; yazılı olmazsa bir tarafın değişikliği diğerini sessizce bozar. İleride frontend bu dosyadan tip üretebilir.

## Bir dosya nasıl okunur?

```text
openapi / info / servers      Sürüm, açıklama ve adres (gateway: http://localhost:18080)
paths                         Her yol ve HTTP metodu: parametreler, istek gövdesi, olası cevaplar
components
  ├─ schemas                  Yeniden kullanılan veri şekilleri (User, Product, Error...)
  ├─ parameters               Ortak sorgu/yol parametreleri (page, page_size...)
  ├─ responses                Ortak hata cevapları (401, 403, 404, 422...)
  └─ securitySchemes          Kimlik doğrulama yöntemi (Bearer JWT)
```

`$ref: '#/components/schemas/User'` bir şemaya **başvurudur**; aynı tanım onlarca yerde tekrar yazılmaz. Bu, koddaki "tekrar etme" kuralının aynısıdır.

Rovena'ya özgü kalıplar:
- Her başarılı cevap bir **zarfa** sarılıdır (`AuthEnvelope`, `ProductPageEnvelope`...): `{"success": true, "data": ...}`. Hata cevapları hep `Error` şemasıdır.
- `nullable: true` alan boş olabilir (`price_from_cents`: aktif varyantı olmayan ürün).
- Zorunlu alanlar `required`, uzunluk ve aralıklar `minLength`/`maximum` gibi kısıtlarla yazılır; bunlar koddaki `validate:"..."` etiketlerinin karşılığıdır.

## Üç farklı erişim türü, tek şemada

Inventory şeması aynı serviste üç tür uç barındırır ve bunları şemada ayrı tutar:

| Uç grubu | Güvenlik şeması | Sunucu |
| --- | --- | --- |
| Herkese açık (`/stock`) | yok | gateway (8080) |
| Admin (`/admin/stock/...`) | `bearerAuth` (JWT, admin rolü) | gateway (8080) |
| Dahili (`/internal/reservations/...`) | `internalKey` (`X-Internal-Key` başlığı) | **servis portu (8083)**, gateway'den geçmez |

OpenAPI bunu iki özellikle ifade eder: `securitySchemes` (kimlik doğrulama yöntemi: `http bearer` veya `apiKey in header`) ve **yol düzeyinde `servers`** (bu yolun farklı bir adreste olduğunu söyler). Dahili uçlar gateway'de yönlendirilmediği için istemci bunu şemadan öğrenir.

Rezervasyon uçlarında **idempotentlik** de şemada yazılıdır: aynı sipariş için ikinci `POST` yeni kayıt yerine mevcut rezervasyonu `200` ile döner (yenisi `201`). İstemci, yani Order servisi, zaman aşımına uğrayan bir çağrıyı güvenle yeniden deneyebilir. Bu tür davranışlar yalnızca durum kodlarına bakınca anlaşılmaz; şemanın açıklamasında belirtmek önemlidir.

## Şema ile kod ayrışırsa ne olur?

Elle yazılmış bir şema zamanla kodla çelişir: alan eklenir ama dosya unutulur, yeni bir hata kodu döner ama belgelenmez. Bunun için `docs/openapi/check_contract.py` yazıldı. Bu bir **sözleşme testi**dir:

- Çalışan yığına gerçek istekler atar (kayıt, giriş, ürün oluşturma, hatalı girdiler...).
- Her cevap için üç şeye bakar: durum kodu şemada **belgelenmiş mi**, gövde şemaya **uyuyor mu**, belgelenmemiş **fazladan alan** var mı.
- Her korumalı uca tokensız (401) ve customer token'ıyla (403) istek atar.
- Şemadaki tüm örnek gövdelerin kendi şemalarına uyduğunu doğrular.
- Oluşturduğu test kayıtlarını sonda siler (betik çökse bile; `atexit`).

Testin işe yaradığı da denendi: şemadan bir alan silindiğinde ("undocumented field"), gerçekte dönen bir durum kodu silindiğinde ("status not documented") ve bir alanın tipi yanlış yazıldığında ("is not of type") üçünü de yakaladı.

İlk gerçek çalıştırmada API ile şema arasında uyumsuzluk çıkmadı, ama bir **altyapı bulgusu** çıktı: betik çok hızlı istek atınca Nginx `limit_req` 429 yerine **503 ve HTML** döndürdü (JSON zarfı değil). Nginx'in varsayılan davranışı budur ve bir kesinti gibi okunur. `limit_req_status 429;` ile düzeltildi: gateway artık JSON zarflı 429 ve `Retry-After` döner (`nginx.conf` içindeki `@rate_limited` konumu). Şemalar bu davranışı belgeliyor.

## Sınırlar

- Şema elle yazılıyor. Koddan otomatik üretmek (swag gibi) mümkün ama Go'da DTO etiketlerinden üretilen şemalar bu kadar ayrıntılı hata/örnek içermiyor; elle yazım + sözleşme testi bu projede daha öğretici bir denge.
- Sözleşme testi bütün durumları denemez: 429, `readyz` 503 ve birkaç tekrarlayan 400 türü canlıda denenmez (birim ve HTTP testlerinde vardır).
- Sözleşme testi çalışan yığın gerektirir, bu yüzden `make test` içinde değildir.
- Dahili Inventory uçları için betiğin, çalışan konteynerin kullandığı `INTERNAL_API_KEY` değerini bilmesi gerekir. İlk denemede `backend/.env` ile konteyner farklı anahtarlar kullandığı için 401 aldı; bu da yapılandırmanın tek kaynaktan (`backend/.env`) okunmasının neden önemli olduğunu gösterdi.
