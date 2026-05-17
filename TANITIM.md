# MediGt — Türkiye HBYS'lerine Modern Şablon

> Eski VB.NET / Delphi tabanlı HBYS sistemlerini güncellemek isteyen
> hastaneler için **üretime hazır, açık mimarili bir HBYS şablonu**.
> Klinik akış + mali + Medula SGK entegrasyonu + KVKK uyumu — hepsi tek
> repoda, tek komutla ayağa kalkan.

---

## Neden bu proje var

Türkiye'deki orta ölçekli hastanelerin büyük çoğunluğu hâlâ **2010
öncesi mimaride** çalışan HBYS yazılımları kullanıyor — DevExpress
Windows Forms, Crystal Reports, WCF/SOAP, MS SQL Server, IIS. Bu
sistemler bugün üç ortak sorunla karşılaşıyor:

1. **Modernizasyon maliyeti** çok yüksek — sıfırdan başlamak yıllar
   alıyor; mevcut ekibin yeni stack'i öğrenmesi zor.
2. **Bulut + uzaktan erişim** desteklenmiyor — pandemiden sonra
   uzaktan poliklinik / hekim erişimi neredeyse zorunlu hâle geldi.
3. **KVKK 2024+ denetim talepleri** — eski sistemler kapsamlı audit
   log + alan-bazlı şifreleme + 10 yıl saklama gibi gereklilikleri
   karşılamıyor.

**MediGt bir HBYS şablonudur** — production-grade bir referans
mimari + tüm temel modülleri + entegrasyonları hazır halde sunar.
Hastaneler bunu **olduğu gibi konuşlandırabilir** veya kendi
markalarıyla fork edip özelleştirebilirler.

---

## Hedef kitle

- **Orta ölçek hastaneler** (200–2000 yatak): tam V1 doğrudan
  uyarlanabilir.
- **Tıp merkezleri / poliklinikler**: yatak yönetimini kapatıp
  outpatient odaklı çalıştırabilir.
- **Zincir hastaneler**: `organization` + `branch` iki seviyeli
  tenancy ile tek kurulumda 10+ şubeyi yönetebilir.
- **HBYS firmaları**: mevcut müşterilerine sunmak üzere
  white-label ürün geliştirmek isteyenler için sıfırdan yazmaktan
  çok daha hızlı bir başlangıç.

---

## Teknoloji yığınının gücü

### Go + Next.js + Electron birleşik stack

MediGt üç farklı dünyayı **tek bir monorepo** içinde birleştirir.
Her dünyayı tek bir kod tabanı yönetiyor.

| Katman             | Teknoloji                         | Neden bu seçim?                                                                                                                       |
| ------------------ | --------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------- |
| **Backend API**    | Go 1.26 + Chi + sqlc + pgx/v5     | Tek binary çıkıyor (~30 MB), Windows + Linux + Mac native, soğuk başlangıç <100 ms, 10 K eş zamanlı WebSocket bağlantısı tek pod'da. |
| **Veri katmanı**   | PostgreSQL 17 + pgvector          | ACID transaction; FOR UPDATE SKIP LOCKED ile horizontal outbox worker; JSONB ile esnek payload; pgvector ileride ML için hazır.       |
| **Realtime**       | gorilla/websocket + Redis pub-sub | App-level 30 sn heartbeat + client 60 sn timeout — half-open TCP'ye karşı 3 katmanlı koruma.                                          |
| **Web frontend**   | Next.js 16 + React 19 + TS strict | App Router + Server Components; Tailwind 4 + shadcn/ui = ortak design tokens; tree-shaking ile production bundle <500 KB başına route. |
| **Desktop app**    | Electron + electron-vite          | Aynı React component'leri masaüstüne çıkar — poliklinik, vezne kioskları için native pencere + tarayıcı API'leri (USB barkod, yazıcı). |
| **Mobile / PWA**   | Next.js PWA + native manifest     | Hemşire bedside, hekim tablet, hasta MRN sorgu cihazı — tarayıcı tabanlı, app store gerekmiyor.                                       |

### Üç dünyayı birleştiren mimari

```
                   ┌─────────────────────────┐
                   │   packages/core         │ ← headless TS
                   │   (queries, mutations,  │   • zero react-dom
                   │    stores, types, i18n) │   • zero localStorage
                   └────────┬───────┬────────┘
                            │       │
              ┌─────────────┘       └─────────────┐
              ▼                                   ▼
      ┌──────────────┐                  ┌──────────────────┐
      │ packages/ui  │                  │ packages/views   │
      │ (atomic, no  │ ← shadcn         │ (business pages, │
      │   business)  │                  │  modal-driven)   │
      └──────┬───────┘                  └────────┬─────────┘
             │                                   │
             └─────────────┬─────────────────────┘
                           ▼
        ┌─────────────────────────────────────┐
        │  apps/web (Next.js)                 │
        │  apps/desktop (Electron)            │   ← target-specific
        │  apps/mobile (PWA / future Capacitor) │     shells only
        └─────────────────────────────────────┘
```

**Pay-off:** Yeni bir feature yazıldığında — örn. "kasa kapatma sayım
yardımcısı" — aynı kod web, desktop ve mobile'a tek seferde düşer.
Üç ekran üç defa yazılmaz.

---

## PostgreSQL'in tam gücü

MediGt **MS SQL Server'dan PostgreSQL'e geçiş** yapan firmalara büyük
kazanç sağlar:

- **Sıfır lisans maliyeti** — tek sunucu lisansı bile yıllık on
  binlerce dolar olabilir; PostgreSQL açık ve özgür.
- **JSONB esnekliği** — Medula SOAP cevapları, audit log payload'ları,
  HL7 ack'leri yapısal kolonlarla birlikte JSONB olarak saklanır;
  rapor için hem JSONB indexli sorgu hem de klasik ilişkisel join
  birlikte çalışır.
- **`FOR UPDATE SKIP LOCKED`** — outbox worker pattern'ini gerçekten
  horizontal scale eder: aynı worker'ı 5 pod'da çalıştırsanız aynı
  satırı çekmezler, race condition yok.
- **CHECK constraint + ENUM** — admission_status, prescription_status
  gibi durum makineleri DB seviyesinde garantili; bug'lı uygulama
  kodu yanlış değer yazamaz.
- **pgvector** — V1.5'te embeddings + similarity search (örn. benzer
  semptomlu hasta önerisi) için altyapı zaten hazır.
- **Logical replication** — şube ofislerinin yedek replikası
  saatlik senkron; ana site düşse yedekten read-only sürdürülür.
- **`pg_dump → S3`** — 10 yıllık KVKK retention için gece cron;
  point-in-time recovery WAL ile.

---

## Host esnekliği

MediGt **istediğiniz işletim sisteminde** çalışır. Go binary tek
dosya, dış bağımlılığı olmayan bir runtime'dır.

| Hedef                 | Nasıl                                                                                                                  |
| --------------------- | ---------------------------------------------------------------------------------------------------------------------- |
| **Linux on-prem**     | `make build` → `bin/server` binary'i sysv/systemd service'i olarak çalışır. Postgres + Redis aynı sunucuda olabilir.   |
| **Windows Server**    | `GOOS=windows go build` ile `.exe` çıkar; Windows Service olarak kurulur (nssm / sc.exe). Tıkır tıkır çalışır.        |
| **macOS**             | Geliştirme + Apple Silicon native — `arm64` derleme tek bayrak meselesi. Bazı küçük hastaneler Mac mini'lerle çalışır. |
| **Docker / Compose**  | `docker compose up` ile dev ortamı: Postgres + Redis + MinIO + Mailhog + backend + frontend, hepsi tek komut.         |
| **Kubernetes / OCP**  | Helm chart hazır — aşağıda detay.                                                                                      |
| **Bulut SaaS-style**  | AWS / Azure / GCP üzerinde ECS Fargate / Cloud Run / Container Apps — tek binary olduğu için herhangi bir runtime'a.   |

**KVKK kritik notu:** Türk hastalarının verileri **Türkiye coğrafi
sınırları içinde** kalmalı. MediGt'nin Helm chart'ında varsayılan
`storage.s3Region: eu-central-1`'tir — Frankfurt; Türkiye'ye en yakın
AB içi bölge. On-prem ya da TR-resident bulut tercih edenler için
S3-compatible MinIO + lokal Postgres seçeneği bir bayrakla açılır.

---

## Deploy mekanizmaları

### Docker Compose (dev / küçük klinikler)

```bash
cp .env.example .env
docker compose up
```

5 dakikada ayakta: Postgres + Redis + MinIO + Mailhog + backend
+ frontend. 50 kullanıcılık küçük poliklinikler için yeterli.

### Helm Chart (Kubernetes)

```bash
helm install medigt deploy/helm/medigt \
  -f deploy/helm/medigt/values-onprem.yaml
```

Chart tüm bileşenleri yönetir:

- **Backend Deployment** (replica: 2, HPA opsiyonel) + Service
- **Frontend Deployment** (replica: 2) + Service
- **Postgres StatefulSet** (PVC ile persistent; veya `enabled: false`
  yapıp managed Postgres'e bağlanın)
- **PVC** uploads için (PACS DICOM + dosya saklama)
- **Ingress** veya **OpenShift Route** (3 yol: `/`, `/api`, `/ws`)
- **NetworkPolicy** (multi-tenant cluster izolasyonu)
- **ResourceQuota** + **LimitRange** (tenant izolasyonu)
- **PodDisruptionBudget** (rolling restart sırasında en az 1
  replica ayakta kalır)
- **HorizontalPodAutoscaler** (CPU bazlı 2→10 replica)
- **ServiceMonitor** (Prometheus operatörü için)

### OpenShift

```bash
helm install medigt deploy/helm/medigt \
  -f deploy/helm/medigt/values-openshift.yaml
```

OCP-native: **Routes** (edge TLS), random non-root UID için
pre-create edilmiş `/app/data/uploads`, harici managed Postgres
varsayılan açık.

### Üretim için kontrol listesi

| Konu                    | Nereden                                                                                                                          |
| ----------------------- | -------------------------------------------------------------------------------------------------------------------------------- |
| **JWT secret**          | `helm install --set backend.jwtSecret=<32+ karakter>`                                                                            |
| **Field encryption**    | `--set backend.fieldEncryptionKey=<32+ karakter>` — TC, password, TOTP secret at-rest şifreleme                                  |
| **SGK Medula sertifikası** | Sprint 1'de başvurun (3-6 ay süreç). Cert gelince `--set backend.medula.*` değerleri set edilir; kod değişmez (factory swap).  |
| **e-Nabız Client ID**   | Sağlık Bakanlığı OAuth credential alındığında `--set backend.enabiz.*`                                                            |
| **TURKKEP e-İmza**      | TURKKEP cloud sertifikası alındığında `--set backend.turkkep.apiKey`                                                              |
| **Veri rezidansı**      | `--set backend.storage.s3Region=eu-central-1` veya tamamen yerel: `--set backend.storage.s3Bucket=""` ile PVC fallback           |

---

## Desktop ve mobile

### `apps/desktop` — Electron

Electron uygulaması aynı React component'lerini paketler. Sağladığı
ekstralar:

- **USB barkod tarayıcı** — bilekkartı + ilaç karekod (vezne, eczane,
  hemşire bedside)
- **POS yazarkasa** — fatura yazdırma + Z raporu çıktısı
- **Multi-monitor** — vezne için kasa ekranı + müşteri ekranı ayrı
  monitör (paralel ödeme detayı + fatura görüntüsü)
- **Offline-first cache** — şube ofislerinde geçici bağlantı kaybında
  son 24 saatlik veri lokal IndexedDB'de
- **Yerel printer** — termal yazıcı + lazer yazıcı seçimi sistem
  düzeyinde

```bash
pnpm --filter @medigt/desktop dev      # geliştirme
pnpm --filter @medigt/desktop build    # exe / dmg / AppImage
```

### `apps/web` — PWA + mobile

Web frontend zaten **responsive + PWA-uyumlu**:

- **Hemşire bedside** — tablet için büyük dokunmatik butonlar,
  MAR (ilaç verme) checklist'i + barkod input
- **Doktor tablet** — TipTap rich-text klinik notlar, tablet'te
  hızlı /şablon slash commands
- **Hasta MRN kiosk** — randevu kontrol + sıra numarası gösterim
- **Yönetici mobile** — uzaktan KPI dashboard, doluluk, kasa kapanış,
  Z raporu — telefonda da çalışır

App store dağıtımı gerekmez; tarayıcı **Add to Home Screen** ile
icon olur. Service worker offline-fallback sağlar.

---

## Modül kapsamı

Toplam **~30 modül** hazır — eski 880 formlu Medivizyon HBYS'nin
**~95'i** karşılanır.

### Klinik akış
Hasta Kabul · Randevu · Poliklinik · Yatış · Laboratuvar · Radyoloji
· Ameliyat · Diyaliz · Diş · Hemşire (MAR — 5 doğru kuralı)

### Mali
Vezne · Fatura · Hakediş · Kasa Raporları · Z Raporu · Fiyat
Güncelleme Sihirbazı · Cari Hesap (avans) · İade Yönetimi · Taksitli
Ödeme

### Eczane / Stok
Eczane · İlaç Katalogu · Depo · İrsaliye · FIFO/FEFO Dispense

### Entegrasyon
Medula SGK (provizyon + fatura + sevk + e-rapor — outbox pattern
ile garanti teslim) · MERNIS TC Kimlik · PACS (Orthanc / OHIF Viewer)
· e-Reçete · İTS (İlaç Takip Sistemi) · e-Nabız (Sağlık Bakanlığı PHR)
· TURKKEP e-İmza · HL7 v2 (ORU sonuç + ADT A01/A02/A03 yatış-transfer-taburcu)

### Yönetim
Personel · Doktor · Branş · Hizmet Katalogu · ICD-10 (~14 000 kod
seed) · Kurumlar · Roller & İzinler · Ayarlar

### KVKK
Audit Log Viewer (10 yıl retention, action/entity/tarih filter,
JSONB detay) · Veri Maskeleme (TC son 4 hane) · Alan-bazlı şifreleme
(TC, password, TOTP) · Field Encryption Key chart parametresi

### Asistan
Sesli intake asistanı — Web Speech API (tr-TR) STT + sunucu
slot-filler NLU. Hasta TC + ad + şikayet + branş söylendiğinde
otomatik hasta kabul + randevu açar, poliklinik kuyruğuna düşer.

---

## Güvenlik + uyum

- **KVKK 6698:** Audit log 10 yıl (env: `AUDIT_RETENTION_DAYS`),
  TC kimlik no audit detaylarında son 4 hane (util.MaskTC), at-rest
  alan-bazlı şifreleme (`FIELD_ENCRYPTION_KEY`)
- **TLS 1.3 zorunlu** — Helm `route.tls.termination: edge` veya
  reencrypt
- **OAuth2 + JWT (90 günlük)** — refresh token döngüsü, TOTP 2FA
- **RBAC 3 katmanlı:** sistem rolü + 13 klinik rolü + atomik
  permission (~120 izin)
- **Network policy** Kubernetes seviyesinde tenant izolasyonu
- **PostgreSQL row-level security** organization_id filter
- **CSP + CORS** Next.js platform-level
- **Property-based testing** (vezne para invariantı: gelir =
  tahsilat + alacak + iptal) — finansal hatalar daha CI'da yakalanır
- **Outbox pattern** + 30s→6h backoff — Medula / e-Nabız çağrıları
  asla "kaybolmuş gönderim" durumunda kalmaz

---

## Geliştirme deneyimi

- **Single command setup:** `make dev` her şeyi açar (.env oluştur,
  bağımlılıkları yükle, DB ayağa kaldır, migrate, start)
- **Hot reload:** Frontend Next.js Turbopack + backend `go run`
  veya `air` ile
- **Type-safe DB:** sqlc SQL→Go kod üretir; el yazımı SQL string
  yok
- **Type-safe API:** TypeScript types `packages/core/types/`
  altında elle yazılmış, backend JSON şemasını birebir yansıtır
  (CI'da eşleşme testleri var)
- **E2E coverage:** Playwright — happy path randevu→muayene→fatura
  →tahsilat tek senaryoda doğrulanır
- **8 kurumsal palet:** her tenant kendi rengini seçer
  (`<html data-palette="...">`), light + dark
- **Türkçe i18n:** sistemin tüm UI metinleri tek dosyada
  (`packages/core/i18n/locales/tr.ts`); İngilizce ve diğer diller
  pluggable

---

## Şu an "üretime hazır" demek için

MediGt **mock-ready** durumda — yani tüm dış entegrasyonların
test endpoint'leri çalışıyor, gerçek SGK / Bakanlık sertifikası
beklenirken pilot hastanede iç akış doğrulanabilir.

**Üretime almak için gereken kalan iş** (sertifikasyon süreçleri):

| İş                                | Süre               |
| --------------------------------- | ------------------ |
| Medula SGK sertifikası başvurusu  | 3-6 ay (Bakanlık)  |
| e-Nabız Client ID                 | 1-2 ay             |
| TURKKEP / TÜRKTRUST e-İmza sözleşme | 2-4 hafta        |
| PACS vendor seçimi (Orthanc / Carestream) | 1 ay        |
| UAT — 5 kullanıcı (doktor + hemşire + kabul + vezne + admin) | 2 hafta |
| Eski sistem → yeni sistem 1 ay paralel çalışma | 1 ay |

Sertifikasyon süreçleri **paralel başlatılabilir** — kod tarafında
hazırlık tamam, ops için aksiyon listesi `BACKLOG.md`'de.

---

## Lisans + kullanım

MediGt **MediGt Attribution License (MAL) v1.0** ile dağıtılır —
MIT temelli, **atıf zorunlu** bir lisans modeli.

**Serbest:** Kullanma, değiştirme, dağıtma, alt-lisanslama, satma,
SaaS olarak sunma, white-label etiketleme, başka bir HBYS ürününün
parçası olarak entegre etme — hepsi serbest, ücretsiz.

**Zorunlu olan tek şey:** Kaynağın açıkça belirtilmesi. Yani yazılımı
son kullanıcıya sunduğunuzda — web arayüzünde, masaüstü/mobil
uygulamada, satış sözleşmesinde veya kurulum dokümantasyonunda — en
az bir görünür yerde şu nota yer vermelisiniz:

> "MediGt teknolojisini temel alır.
> © Türker Aktaş — github.com/gercektasarim/medigt"

Bu yükümlülük white-label kullanım dahil — markayı değiştirseniz,
yeniden tasarlasanız, farklı bir isimle satsanız bile atıf saklı
kalır. Müşterilerinizin "yazılımı kimden aldıklarını" net bilmesi
gerekir; gizli white-label veya kaynağı belirsiz dağıtım kabul
edilmez.

Tam lisans metni (TR + EN) için repo kökündeki `LICENSE` dosyasına
bakınız.

**Ticari kullanım için ne yapılmalı?**

- Repo'yu fork edin veya doğrudan submodule olarak kullanın.
- "Hakkında" ekranınıza/footer'ınıza atıf bilgisini ekleyin.
- Müşterinize sözleşme imzalattığınızda yazılımın MediGt temeli
  açıkça yer alsın.
- Bu kadarı yeterli — başka bir sözleşme veya ücret yok.

**Ne yapamazsınız?**

- Atıf bildirimini gizlemek, silmek veya yanıltıcı şekilde göstermek
  → lisans otomatik fesholur.
- MediGt markası, logosu veya kurumsal kimliğini sahiplenmek.
- Yazılımın gerçek kaynağını bilerek kapatmak.

**İletişim — Türker Aktaş**

- E-posta: <turker.aktas81@gmail.com>
- Telefon: [+90 530 288 98 60](tel:+905302889860)
- Repo: <https://github.com/gercektasarim/medigt>

---

## Hızlı başlangıç

```bash
# 1. Repo'yu klonla
git clone <repo>
cd MediGt

# 2. Tek komutla aç
make dev
```

5 dakika içinde:

- Backend → <http://localhost:8088>
- Frontend → <http://localhost:3008>
- Mailhog → <http://localhost:8025> (login kodu burada görünür)
- Mailhog'taki kodu girip ilk admin user olun → "Yeni hastane oluştur"
  sihirbazı sizi 4 adımlık kuruluma götürür
- Başlangıç ekranında "Varsayılanları Yükle" tuşu — temel kurum
  (SGK + Cepten) + 10 hizmet + 1 eczane deposu otomatik eklenir
- Hasta kabul edip ilk randevu → poliklinik → muayene → fatura →
  tahsilat zincirini 10 dakikada deneyebilirsiniz

## Daha derin teknik bilgi için

- `README.md` — geliştirici quick reference
- `CLAUDE.md` — mimari kuralları + paket sınırları + state yönetimi
  disiplini
- `BACKLOG.md` — şu an ertelenmiş işler ve önceliklendirme
- `deploy/helm/medigt/README.md` — Helm chart override kılavuzu
