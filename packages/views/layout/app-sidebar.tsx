"use client";

import { useMemo } from "react";
import {
  Inbox, Sunrise, CalendarDays, Users, CalendarClock, Stethoscope, BedDouble,
  FlaskConical, ScanLine, Scissors, Droplet, Smile, HeartPulse, Pill, Tablets,
  Warehouse, Wallet, ReceiptText, HandCoins, BarChart3, ShieldCheck, BadgeCheck,
  FileBarChart, UsersRound, GitBranch, ListTree, FileCode2, Building2, KeyRound,
  Settings, ClipboardList, Sparkles,
} from "lucide-react";
import { AppLink } from "@medigt/core/navigation";
import { paths } from "@medigt/core/paths";
import { useHospitalStore } from "@medigt/core/hospital";
import { useT } from "@medigt/core/i18n";
import { cn } from "@medigt/ui/lib/utils";

type NavItem = { key: string; label: string; href: string; icon: React.ComponentType<{ className?: string }> };

export function AppSidebar() {
  const organization = useHospitalStore((s) => s.organization);
  const branch = useHospitalStore((s) => s.branch);
  const t = useT();

  const groups = useMemo(() => {
    if (!organization || !branch) return [];
    const b = paths.hospital(organization.slug).branch(branch.slug);

    return [
      {
        label: "",
        items: [
          { key: "inbox", label: t.nav.inbox, href: b.inbox(), icon: Inbox },
          { key: "baslangic", label: t.nav.today, href: b.baslangic(), icon: Sunrise },
          { key: "takvim", label: t.nav.calendar, href: b.takvim(), icon: CalendarDays },
          { key: "asistan", label: "Hasta Kabul Asistanı", href: b.asistan(), icon: Sparkles },
        ],
      },
      {
        label: "Klinik",
        items: [
          { key: "hasta", label: t.modules.hasta, href: b.hasta.list(), icon: Users },
          { key: "randevu", label: t.modules.randevu, href: b.randevu.takvim(), icon: CalendarClock },
          { key: "poliklinik", label: t.modules.poliklinik, href: b.poliklinik.queue(), icon: Stethoscope },
          { key: "yatis", label: t.modules.yatis, href: b.yatis.board(), icon: BedDouble },
          { key: "laboratuvar", label: t.modules.laboratuvar, href: b.laboratuvar.queue(), icon: FlaskConical },
          { key: "radyoloji", label: t.modules.radyoloji, href: b.radyoloji.queue(), icon: ScanLine },
          { key: "ameliyat", label: t.modules.ameliyat, href: b.ameliyat.plan(), icon: Scissors },
          { key: "diyaliz", label: t.modules.diyaliz, href: b.diyaliz.list(), icon: Droplet },
          { key: "dis", label: t.modules.dis, href: b.dis(), icon: Smile },
          { key: "hemsire", label: t.modules.hemsire, href: b.hemsire.board(), icon: HeartPulse },
        ],
      },
      {
        label: "Eczane & Stok",
        items: [
          { key: "ecza", label: t.modules.ecza, href: b.ecza.root(), icon: Pill },
          { key: "ilac", label: t.modules.ilac, href: b.ilac(), icon: Tablets },
          { key: "depo", label: t.modules.depo, href: b.depo.root(), icon: Warehouse },
        ],
      },
      {
        label: "Mali",
        items: [
          { key: "vezne", label: t.modules.vezne, href: b.vezne.root(), icon: Wallet },
          { key: "fatura", label: t.modules.fatura, href: b.fatura.list(), icon: ReceiptText },
          { key: "hakedis", label: t.modules.hakedis, href: b.hakedis.list(), icon: HandCoins },
          { key: "kasaRapor", label: t.modules.kasaRapor, href: b.kasaRapor.list(), icon: BarChart3 },
        ],
      },
      {
        label: "Entegrasyon",
        items: [
          { key: "medula", label: t.modules.medula, href: b.medula.root(), icon: ShieldCheck },
          { key: "mernis", label: t.modules.mernis, href: b.mernis(), icon: BadgeCheck },
        ],
      },
      {
        label: "Raporlar",
        items: [
          { key: "rapor", label: t.modules.rapor, href: b.rapor.list(), icon: FileBarChart },
        ],
      },
      {
        label: "Yönetim",
        items: [
          { key: "personel", label: t.modules.personel, href: b.personel.list(), icon: UsersRound },
          { key: "doktor", label: t.modules.doktor, href: b.doktor.list(), icon: Stethoscope },
          { key: "brans", label: t.modules.brans, href: b.brans(), icon: GitBranch },
          { key: "hizmet", label: t.modules.hizmet, href: b.hizmet.list(), icon: ListTree },
          { key: "icd10", label: t.modules.icd10, href: b.icd10(), icon: FileCode2 },
          { key: "kurum", label: t.modules.kurum, href: b.kurum.list(), icon: Building2 },
          { key: "yetki", label: t.modules.yetki, href: b.yetki(), icon: KeyRound },
          { key: "audit", label: "Denetim Kayıtları", href: b.audit(), icon: ClipboardList },
          { key: "ayarlar", label: t.modules.ayarlar, href: b.ayarlar(), icon: Settings },
        ],
      },
    ];
  }, [organization, branch, t]);

  if (groups.length === 0) {
    return (
      <aside className="flex w-60 flex-col border-r border-sidebar-border bg-sidebar text-sidebar-foreground">
        <div className="flex items-center gap-2 border-b border-sidebar-border p-4">
          <span className="text-base font-semibold">MediGt</span>
        </div>
        <div className="p-4 text-xs text-muted-foreground">Hastane seçilmedi</div>
      </aside>
    );
  }

  return (
    <aside className="flex w-60 flex-col border-r border-sidebar-border bg-sidebar text-sidebar-foreground">
      <div className="flex flex-col gap-0.5 border-b border-sidebar-border p-4">
        <span className="text-base font-semibold">{organization?.name}</span>
        <span className="text-xs text-muted-foreground">{branch?.name}</span>
      </div>
      <nav className="flex-1 overflow-y-auto p-2">
        {groups.map((group, gi) => (
          <div key={gi} className="mb-3">
            {group.label && <div className="eyebrow-label px-2 pb-1.5">{group.label}</div>}
            <ul className="space-y-0.5">
              {group.items.map((item: NavItem) => (
                <li key={item.key}>
                  <AppLink
                    to={item.href}
                    className={cn(
                      "flex items-center gap-2 rounded-md px-2 py-1.5 text-sm",
                      "hover:bg-sidebar-accent hover:text-sidebar-accent-foreground",
                    )}
                  >
                    <item.icon className="h-4 w-4" />
                    <span>{item.label}</span>
                  </AppLink>
                </li>
              ))}
            </ul>
          </div>
        ))}
      </nav>

      {/* License attribution. MediGt Attribution License v1.0 requires this
       *  banner to remain visible to end users; remove only if you replace
       *  it with equivalent attribution in another visible location
       *  (Hakkında ekranı, footer, vb.). */}
      <div className="border-t border-sidebar-border p-3 text-[10px] leading-tight text-muted-foreground">
        <a
          href="https://github.com/gercektasarim/medigt"
          target="_blank"
          rel="noopener noreferrer"
          className="block transition hover:text-foreground"
        >
          <div>MediGt teknolojisini temel alır.</div>
          <div className="opacity-70">© Türker Aktaş</div>
        </a>
        <div className="mt-1 flex flex-wrap gap-x-2 opacity-60">
          <a href="mailto:turker.aktas81@gmail.com" className="hover:underline">
            turker.aktas81@gmail.com
          </a>
          <a href="tel:+905302889860" className="hover:underline">
            +90 530 288 98 60
          </a>
        </div>
      </div>
    </aside>
  );
}
