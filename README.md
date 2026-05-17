# MediGt — Hastane Bilgi Yönetim Sistemi

Modern bir hastane bilgi yönetim sistemi (HBYS). Go backend + Next.js frontend monorepo. Eski Medivizyon HBYS sisteminin yeniden inşası.

## Mimari

- **Backend:** Go 1.26 + Chi v5 + sqlc + PostgreSQL 17 (pgvector) + gorilla/websocket
- **Frontend:** Next.js 16 (App Router) + React 19 + TypeScript 5.9 + Tailwind 4 + shadcn/ui
- **Monorepo:** pnpm 10 workspaces + Turborepo 2
- **State:** TanStack Query v5 (server) + Zustand v5 (client)
- **Realtime:** WebSocket + Redis relay
- **Deploy:** Docker Compose (dev) + Helm/OpenShift (prod)

## Çoklu kiracı (multi-tenancy)

İki seviye: `organization` (hastane grubu) + `branch` (şube/poliklinik).

URL paterni: `/h/:org_slug/:branch_slug/...`

## Hızlı başlangıç

```bash
make dev          # .env oluştur + bağımlılıkları yükle + DB ayağa kaldır + migrate + start
```

Veya adım adım:

```bash
cp .env.example .env
pnpm install
make db-up
cd server && go run ./cmd/migrate up
make start
```

- Backend: <http://localhost:8088>
- Frontend: <http://localhost:3008>

## Modüller

**Klinik:** Hasta Kabul, Randevu, Poliklinik, Yatış, Laboratuvar, Radyoloji, Ameliyat, Diyaliz, Diş, Hemşire
**Eczane/Stok:** Eczane, İlaç katalogu, Depo
**Mali:** Vezne, Fatura, Hakediş, Kasa Raporları
**Entegrasyon:** Medula SGK, MERNIS (TC kimlik), PACS, e-Reçete, e-Nabız
**Yönetim:** Personel, Doktor, Branş, Hizmet Katalogu, ICD-10, Kurumlar, Roller & İzinler, Ayarlar

## Geliştirme

Bütün dökümantasyon ve mimari kuralları için `CLAUDE.md` dosyasına bakınız.

## E2E testleri

Playwright tabanlı uçtan uca testler `e2e/tests/` altında:

```bash
make e2e-install   # tek seferlik — Chromium indirir
make start         # backend + frontend ayağa kaldır (ayrı terminalde)
make e2e           # tüm spec'leri çalıştır
make e2e-ui        # interaktif arayüz
```

Test setleri:

- **auth-and-onboarding** — yeni kullanıcı kaydı + ilk hastane oluşturma
- **master-data-people** — branş, personel, doktor, kurum CRUD
- **master-data-services** — hizmet katalogu + kurum bazlı fiyat + ICD-10 arama
- **patient** — TC checksum, duplicate guard, debounced search
- **appointment** — randevu oluştur + state machine
- **clinical-flow** — randevu → muayene → vital → tanı → reçete → imzala → tamamla

Her test kendi org+branch'ini API üzerinden açar (`createTestApi`), browser'a token'ları enjekte eder, sonra UI'yı sürer. Backend dev master kodu `888888` test sürecinin temelidir — `APP_ENV=production` ile çalıştırırsanız testler kırılır.

## Lisans

**MediGt Attribution License (MAL) v1.0** — atıf zorunlu kullanım izni.

Yazılımı kullanma, değiştirme, dağıtma, satma ve hizmet sunma haklarınız
vardır; tek koşul **kaynağı açıkça belirtmenizdir**: end-user'a sunulan
uygulamada (Hakkında ekranı, footer, satış sözleşmesi vb.) en az birinde
"MediGt teknolojisini temel alır · © Türker Aktaş —
github.com/gercektasarim/medigt" notu görünür olmalıdır.

White-label kullanım, ticari satış ve SaaS operasyonu serbest — atıf
saklı kaldığı sürece. Detaylar için `LICENSE` dosyasına bakınız.

## İletişim

**Türker Aktaş**

- E-posta: <turker.aktas81@gmail.com>
- Telefon: [+90 530 288 98 60](tel:+905302889860)
- Repo: <https://github.com/gercektasarim/medigt>
