import type { ReportDescriptor } from "../types/report";

// The single source of truth for which reports show up in the hub. Each
// descriptor's `id` must match a backend handler in
// server/internal/handler/reports.go::registeredReports.

function monthStart(): string {
  const d = new Date();
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}-01`;
}

function todayISO(): string {
  const d = new Date();
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}-${String(d.getDate()).padStart(2, "0")}`;
}

const rangeParams = (defaultFrom = monthStart()): ReportDescriptor["params"] => [
  { key: "from", label: "Başlangıç", type: "date", default: defaultFrom },
  { key: "to", label: "Bitiş", type: "date", default: todayISO() },
];

export const REPORTS: ReportDescriptor[] = [
  // Finans
  {
    id: "daily-cash",
    slug: "kasa-ozeti",
    group: "Finans",
    title: "Kasa Özeti",
    description: "Tarih aralığında açılan tüm kasa oturumlarının nakit hareketleri, sayım farkı.",
    params: rangeParams(todayISO()),
  },
  {
    id: "payment-mix",
    slug: "odeme-yontemi",
    group: "Finans",
    title: "Ödeme Yöntemi Dağılımı",
    description: "Tahsilatların yöntem bazlı dağılımı (nakit / kart / havale / mobil / diğer).",
    params: rangeParams(),
  },
  {
    id: "unpaid-invoices",
    slug: "odenmemis-faturalar",
    group: "Finans",
    title: "Ödenmemiş Faturalar",
    description: "Açık bakiyesi olan onaylı faturalar (yaşlandırma günleriyle).",
    params: [],
  },
  {
    id: "institution-revenue",
    slug: "kurum-hasilat",
    group: "Finans",
    title: "Kurum Bazlı Hasılat",
    description: "Kuruma göre fatura adedi, toplam, tahsil, kalan.",
    params: rangeParams(),
  },

  // Klinik
  {
    id: "doctor-revenue",
    slug: "doktor-hasilat",
    group: "Klinik",
    title: "Doktor Bazlı Hasılat",
    description: "Doktor başına brüt, tahsil edilen, iptal ve bekleyen tutarlar.",
    params: rangeParams(),
  },
  {
    id: "outpatient-by-doctor",
    slug: "poliklinik-doktor",
    group: "Klinik",
    title: "Doktor Bazlı Poliklinik",
    description: "Tarih aralığında doktor başına muayene + benzersiz hasta + tamamlanan sayıları.",
    params: rangeParams(),
  },
  {
    id: "lab-volume",
    slug: "lab-hacim",
    group: "Klinik",
    title: "Laboratuvar Hacmi",
    description: "İstek durumlarına göre lab iş yükü dağılımı.",
    params: rangeParams(),
  },
  {
    id: "bed-occupancy",
    slug: "yatak-doluluk",
    group: "Klinik",
    title: "Yatak Doluluğu",
    description: "Servis bazında yatak durumu ve doluluk oranı.",
    params: [],
  },

  // Stok
  {
    id: "low-expiry-stock",
    slug: "skt-yaklasan",
    group: "Stok",
    title: "SKT'si Yaklaşan İlaçlar",
    description: "Belirtilen gün içinde son kullanma tarihi dolacak lotlar.",
    params: [
      {
        key: "days",
        label: "Gün",
        type: "select",
        default: "90",
        options: [
          { value: "30", label: "30 gün" },
          { value: "60", label: "60 gün" },
          { value: "90", label: "90 gün" },
          { value: "180", label: "180 gün" },
        ],
      },
    ],
  },
  {
    id: "stock-valuation",
    slug: "stok-degerleme",
    group: "Stok",
    title: "Stok Değerleme",
    description: "Aktif lotların son alış birim fiyatına göre yaklaşık değerlemesi.",
    params: [],
  },
  {
    id: "top-medications",
    slug: "en-cok-verilen-ilaclar",
    group: "Stok",
    title: "En Çok Verilen İlaçlar",
    description: "Belirtilen tarih aralığında dispense sayısı en yüksek ilaçlar (Top 30).",
    params: rangeParams(),
  },

  // Finans (ek)
  {
    id: "hourly-collection",
    slug: "saatlik-tahsilat",
    group: "Finans",
    title: "Saatlik Tahsilat",
    description: "Gün içinde saat saat tahsilat hacmi — vezne yoğunluğunun pikleri için.",
    params: rangeParams(todayISO()),
  },
  {
    id: "cashier-collection",
    slug: "kasiyer-tahsilat",
    group: "Finans",
    title: "Kasiyer Bazlı Tahsilat",
    description: "Kasiyer başına oturum sayısı + tahsilat / iade / net.",
    params: rangeParams(),
  },
  {
    id: "open-advances",
    slug: "acik-avanslar",
    group: "Finans",
    title: "Açık Avans Bakiyeleri",
    description: "Pozitif cari bakiyesi olan hastalar (avans alacağı).",
    params: [],
  },

  // Klinik (ek)
  {
    id: "diagnosis-distribution",
    slug: "tani-dagilimi",
    group: "Klinik",
    title: "Tanı Dağılımı (ICD-10)",
    description: "Tarih aralığında en sık konulan 50 tanı (poliklinik epidemiyolojisi).",
    params: rangeParams(),
  },
  {
    id: "polyclinic-by-hour",
    slug: "poliklinik-saatlik",
    group: "Klinik",
    title: "Poliklinik Saatlik Yoğunluk",
    description: "Saat saat randevu yükü + tamamlanan / iptal-gelmeyen oranı.",
    params: rangeParams(todayISO()),
  },
  {
    id: "lab-test-volume",
    slug: "lab-test-hacim",
    group: "Klinik",
    title: "Lab Test Hacmi",
    description: "Test kalemi bazlı istek + sonuçlanan + kritik flag sayısı (Top 50).",
    params: rangeParams(),
  },

  // Yatış / Operasyon
  {
    id: "ward-admission-stats",
    slug: "servis-yatis-istatistik",
    group: "Yatış",
    title: "Servis Bazlı Yatış İstatistikleri",
    description: "Yatış / taburcu sayıları + aktif yatak + ortalama yatış süresi (gün).",
    params: rangeParams(),
  },
  {
    id: "surgeon-performance",
    slug: "cerrah-performans",
    group: "Yatış",
    title: "Cerrah Performansı",
    description: "Ameliyat sayıları (toplam / tamamlanan / iptal) + ortalama süre.",
    params: rangeParams(),
  },

  // Entegrasyon
  {
    id: "medula-success-rate",
    slug: "medula-basari",
    group: "Entegrasyon",
    title: "Medula Provizyon Başarı Oranı",
    description: "SGK provizyon isteklerinin başarı / red / bekleyen dağılımı.",
    params: rangeParams(),
  },
];

export function findReport(slug: string): ReportDescriptor | undefined {
  return REPORTS.find((r) => r.slug === slug);
}

export function reportGroups(): { group: string; reports: ReportDescriptor[] }[] {
  const map = new Map<string, ReportDescriptor[]>();
  for (const r of REPORTS) {
    const arr = map.get(r.group) ?? [];
    arr.push(r);
    map.set(r.group, arr);
  }
  return [...map.entries()].map(([group, reports]) => ({ group, reports }));
}
