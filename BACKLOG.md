# MediGt Backlog

Önceden başlanmış olup ilerleyen sprintlere ertelenen işler. Her başlık
"hangi commit'lerle başlandı, kalan ne" özetini taşır.

## Kapsam dışı tutulanlar (mevcut iş bloğu)

### 1. Property test — yatak doluluğu + stok lot FIFO

**Durum:** EPIC-7 (vezne) için property test mevcut
(`e2e/tests/vezne-invariant.property.spec.ts`). Aynı paterni iki
domain'e daha taşımak gerekiyor.

**Tanımlanan invariantlar**

*Yatak doluluğu*
- Aktif admission → `bed_id IS NOT NULL` ve `bed.status='occupied'`
- Hiçbir yatak iki active admission'a sahip olamaz
  (unique partial index zaten korumakta; test concurrent admit deneyebilir)
- Discharge sonrası `bed.status='free'` döner
- Transfer: eski yatak `free`, yeni yatak `occupied`, audit row var

*Stok lot FIFO*
- Her lot için `dispense_total ≤ received_total` (asla negatif kalmaz)
- `medication_stock.quantity ≥ 0` (NULL değil)
- FIFO seçimi: en kısa SKT'li lot önce kullanılır
- Lot SKT geçtikten sonra dispense reddedilir

**Yapılacak:** `e2e/tests/yatis-invariant.property.spec.ts` +
`stock-fifo.property.spec.ts` — fast-check ile random admission/transfer/
discharge + receive/dispense sekansları.

---

### 2. Real PACS Orthanc entegrasyonu

**Durum:** Mock client + image_reference tablosu hazır
(`server/internal/integration/pacs/client.go`, migration 025). OHIF
Viewer iframe URL'si fake.

**Yapılacak**
- `pacs/client_http.go` — Orthanc REST API ile DICOM upload
  - `POST /instances` (binary DICOM body)
  - `GET /studies/{id}` (study metadata)
  - `GET /studies/{id}/preview` (thumbnail PNG)
- `pacs/factory.go` — `NewFromConfig(baseURL, user, pass)`
  - Empty → mock, dolu → real (HTTP Basic auth)
- Config: `PACS_BASE_URL`, `PACS_USERNAME`, `PACS_PASSWORD`
- Helm: `backend.pacs.{baseUrl, username, password}` Secret'a
- Test: httptest fake Orthanc — instance upload + study lookup +
  thumbnail bytes round-trip

---

### 3. MAR auto-discontinue scheduled job

**Durum:** `medication_order.ends_at` kolonu mevcut ama otomatik
status flip yok — bir order ends_at'i geçtiğinde elle 'expired' yapmak
gerekiyor.

**Yapılacak**
- Yeni cron worker `mar/auto_discontinue_worker.go`
  - 5 dakikada bir çalışır
  - `UPDATE medication_order SET status='expired' WHERE status='active'
    AND ends_at IS NOT NULL AND ends_at < NOW()`
  - Hangi rowlar etkilendi log düşer
  - Affected count metric (Prometheus serileri için)
- `main.go` worker'ı goroutine olarak başlat
- Audit: her bir expired flip için `mar.order.auto_expire` — toplu
  yerine per-row (hangi ilacın hangi hastanın hangi orderı düştüğü iz
  bırakır)
- Test: integration test — order ends_at'i geçmiş + status='active' →
  worker iterasyonu sonrası status='expired'

---

### 4. Frontend tutarlılık pass'i

**Durum:** Yeni eklenen sayfalar (yeni hastane wizard, fiyat
güncelleme wizard, audit log, asistan) için tasarım dili
oluştukça eski sayfalarla görsel tutarsızlık çıktı.

**Yapılacak**
- Wizardlar (yeni hastane, fiyat güncelleme, asistan) ortak
  `WizardLayout` + `StepIndicator` componenti çıkartılıp
  ortak DSL'e taşınmalı (`packages/views/common/wizard/`)
- PageHeader actions alanlarında button stilleri tutarsız —
  PrimaryButton/SecondaryButton/AppLink karışımı; AppLink wrapper'ı
  yapıp button stili veren bir `LinkButton` çıkarmalı
- Side-sheet drawer'ları (vezne kapatma, hakediş bulk, MAR doz ver)
  ortak header/footer slot'lu olmalı — şu an her birinin kendi
  Save/Cancel düzeni var
- Table empty-state mesajları farklı tonlarda — "Henüz X yok",
  "Filtreye uyan X yok", "Bu tarihte X yok" — i18n key'leri tek bir
  patternde toplanmalı
- Form Field hint stilleri tutarsız — bazısı altta italic gri,
  bazısı yanlışta kırmızı bordered. shadcn pattern'e hizalama gerekli

---

## Tarih: 2026-05-17
EOF
