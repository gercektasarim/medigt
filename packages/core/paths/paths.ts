// Type-safe URL builder.
// All tenant-scoped URLs live under /h/:orgSlug/:branchSlug/...
// Pre-auth and global routes are single nouns or noun/verb pairs.

function enc(segment: string): string {
  return encodeURIComponent(segment);
}

export const paths = {
  // Pre-auth
  login: () => "/login",
  passwordReset: () => "/sifre-sifirla",
  invite: (token: string) => `/davet/${enc(token)}`,
  onboarding: () => "/onboarding",
  newHospital: () => "/hastaneler/yeni",
  authCallback: () => "/auth/callback",

  // Tenant-scoped builder
  hospital(orgSlug: string) {
    const orgBase = `/h/${enc(orgSlug)}`;
    return {
      root: () => orgBase,
      settings: () => `${orgBase}/ayarlar`,

      branch(branchSlug: string) {
        const base = `${orgBase}/${enc(branchSlug)}`;
        return {
          root: () => `${base}/baslangic`,
          baslangic: () => `${base}/baslangic`,
          inbox: () => `${base}/inbox`,
          takvim: () => `${base}/takvim`,

          // Klinik
          hasta: {
            list: () => `${base}/hasta`,
            yeni: () => `${base}/hasta/yeni`,
            detail: (id: string) => `${base}/hasta/${enc(id)}`,
            dosya: (id: string) => `${base}/hasta/${enc(id)}/dosya`,
            anamnez: (id: string) => `${base}/hasta/${enc(id)}/anamnez`,
            recete: (id: string) => `${base}/hasta/${enc(id)}/recete`,
            laboratuvar: (id: string) => `${base}/hasta/${enc(id)}/laboratuvar`,
            radyoloji: (id: string) => `${base}/hasta/${enc(id)}/radyoloji`,
            fatura: (id: string) => `${base}/hasta/${enc(id)}/fatura`,
          },
          randevu: {
            list: () => `${base}/randevu`,
            takvim: () => `${base}/randevu`,
            yeni: () => `${base}/randevu/yeni`,
            detail: (id: string) => `${base}/randevu/${enc(id)}`,
          },
          poliklinik: {
            queue: () => `${base}/poliklinik`,
            visit: (id: string) => `${base}/poliklinik/${enc(id)}`,
          },
          yatis: {
            board: () => `${base}/yatis`,
            detail: (id: string) => `${base}/yatis/${enc(id)}`,
            oda: () => `${base}/yatis/oda-yonetimi`,
          },
          laboratuvar: {
            queue: () => `${base}/laboratuvar`,
            order: (id: string) => `${base}/laboratuvar/${enc(id)}`,
            panel: () => `${base}/laboratuvar/test-paneli`,
          },
          radyoloji: {
            queue: () => `${base}/radyoloji`,
            exam: (id: string) => `${base}/radyoloji/${enc(id)}`,
          },
          ameliyat: {
            plan: () => `${base}/ameliyat`,
            detail: (id: string) => `${base}/ameliyat/${enc(id)}`,
          },
          diyaliz: {
            list: () => `${base}/diyaliz`,
            detail: (id: string) => `${base}/diyaliz/${enc(id)}`,
          },
          dis: () => `${base}/dis`,
          hemsire: {
            board: () => `${base}/hemsire`,
            ilacSaatleri: () => `${base}/hemsire/ilac-saatleri`,
            vitaller: () => `${base}/hemsire/vitaller`,
          },

          // Eczane / Stok
          ecza: {
            root: () => `${base}/ecza`,
            recete: (id: string) => `${base}/ecza/recete/${enc(id)}`,
          },
          ilac: () => `${base}/ilac`,
          depo: {
            root: () => `${base}/depo`,
            hareket: () => `${base}/depo/hareket`,
            sayim: () => `${base}/depo/sayim`,
          },

          // Mali
          vezne: {
            root: () => `${base}/vezne`,
            kasa: () => `${base}/vezne/kasa`,
            zRapor: (id: string) => `${base}/vezne/z-rapor/${enc(id)}`,
            odeme: (id: string) => `${base}/vezne/odeme/${enc(id)}`,
            borcAlacak: () => `${base}/vezne/borc-alacak`,
            fiyatGuncelleme: () => `${base}/vezne/fiyat-guncelleme`,
          },
          fatura: {
            list: () => `${base}/fatura`,
            detail: (id: string) => `${base}/fatura/${enc(id)}`,
          },
          hakedis: {
            list: () => `${base}/hakedis`,
            doktor: (doktorId: string) => `${base}/hakedis/${enc(doktorId)}`,
          },
          kasaRapor: {
            list: () => `${base}/kasa-rapor`,
            report: (slug: string) => `${base}/kasa-rapor/${enc(slug)}`,
          },
          rapor: {
            list: () => `${base}/rapor`,
            report: (slug: string) => `${base}/rapor/${enc(slug)}`,
          },

          // Entegrasyon
          medula: {
            root: () => `${base}/medula`,
            provizyon: () => `${base}/medula/provizyon`,
            fatura: () => `${base}/medula/fatura`,
            erapor: () => `${base}/medula/eraporlar`,
          },
          mernis: () => `${base}/mernis`,

          // Yönetim
          hizmet: {
            list: () => `${base}/hizmet`,
            detail: (id: string) => `${base}/hizmet/${enc(id)}`,
          },
          icd10: () => `${base}/icd10`,
          personel: {
            list: () => `${base}/personel`,
            detail: (id: string) => `${base}/personel/${enc(id)}`,
          },
          doktor: {
            list: () => `${base}/doktor`,
            detail: (id: string) => `${base}/doktor/${enc(id)}`,
          },
          brans: () => `${base}/brans`,
          kurum: {
            list: () => `${base}/kurum`,
            detail: (id: string) => `${base}/kurum/${enc(id)}`,
          },
          ayarlar: () => `${base}/ayarlar`,
          yetki: () => `${base}/yetki`,
          audit: () => `${base}/denetim`,
          asistan: () => `${base}/asistan`,
        };
      },
    };
  },
} as const;

export type HospitalPaths = ReturnType<typeof paths.hospital>;
export type BranchPaths = ReturnType<HospitalPaths["branch"]>;
