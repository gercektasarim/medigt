-- 005_icd10: ICD-10 (International Classification of Diseases, 10th rev.)
-- diagnosis catalog. System-wide reference data — organization_id NULL means
-- the standard catalog. Hospitals may add custom (sub)codes per org.
--
-- Seeded with ~150 of the most commonly used Turkish primary-care codes;
-- the full ICD-10 (~14k entries) is imported separately via scripts/icd10-import.

BEGIN;

CREATE TABLE icd10_code (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID REFERENCES organization(id) ON DELETE CASCADE,
    code            TEXT NOT NULL,             -- e.g. "I10", "E11.9", "J18.9"
    title_tr        TEXT NOT NULL,
    title_en        TEXT,
    parent_code     TEXT,                      -- parent in the ICD-10 tree
    chapter         TEXT,                      -- e.g. "IX. Dolaşım Sistemi Hastalıkları"
    is_active       BOOLEAN NOT NULL DEFAULT TRUE,
    is_system       BOOLEAN NOT NULL DEFAULT FALSE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE NULLS NOT DISTINCT (organization_id, code)
);

CREATE INDEX idx_icd10_code ON icd10_code(code);
CREATE INDEX idx_icd10_org ON icd10_code(organization_id);
CREATE INDEX idx_icd10_parent ON icd10_code(parent_code) WHERE parent_code IS NOT NULL;
-- Turkish full-text search on the title; backs the /api/icd10?q=... endpoint.
CREATE INDEX idx_icd10_title_fts ON icd10_code
    USING gin(to_tsvector('simple', code || ' ' || title_tr));

CREATE TRIGGER trg_icd10_updated_at BEFORE UPDATE ON icd10_code
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ---------- System seed: ~150 most-used codes ----------

INSERT INTO icd10_code (organization_id, code, title_tr, parent_code, chapter, is_system) VALUES
-- I. Enfeksiyon ve parazit hastalıkları (A00-B99)
(NULL, 'A09',   'Enfeksiyöz olduğu varsayılan diyare ve gastroenterit', NULL, 'I. Enfeksiyon Hastalıkları', TRUE),
(NULL, 'A41.9', 'Sepsis, tanımlanmamış',                     NULL, 'I. Enfeksiyon Hastalıkları', TRUE),
(NULL, 'B00.9', 'Herpes simpleks enfeksiyonu, tanımlanmamış', NULL, 'I. Enfeksiyon Hastalıkları', TRUE),
(NULL, 'B34.9', 'Viral enfeksiyon, tanımlanmamış',            NULL, 'I. Enfeksiyon Hastalıkları', TRUE),
(NULL, 'U07.1', 'COVID-19, virüs tespit edildi',             NULL, 'I. Enfeksiyon Hastalıkları', TRUE),
-- II. Neoplaziler (C00-D48)
(NULL, 'C50.9', 'Meme malign neoplazmı, tanımlanmamış',       NULL, 'II. Neoplaziler', TRUE),
(NULL, 'C61',   'Prostat malign neoplazmı',                   NULL, 'II. Neoplaziler', TRUE),
(NULL, 'C18.9', 'Kolon malign neoplazmı, tanımlanmamış',      NULL, 'II. Neoplaziler', TRUE),
(NULL, 'C34.9', 'Bronş ve akciğer malign neoplazmı',          NULL, 'II. Neoplaziler', TRUE),
(NULL, 'D17.9', 'Lipom, tanımlanmamış',                       NULL, 'II. Neoplaziler', TRUE),
(NULL, 'D25.9', 'Uterus leiomyomu, tanımlanmamış',            NULL, 'II. Neoplaziler', TRUE),
-- III. Kan hastalıkları (D50-D89)
(NULL, 'D50.9', 'Demir eksikliği anemisi, tanımlanmamış',     NULL, 'III. Kan Hastalıkları', TRUE),
(NULL, 'D64.9', 'Anemi, tanımlanmamış',                       NULL, 'III. Kan Hastalıkları', TRUE),
-- IV. Endokrin (E00-E89)
(NULL, 'E03.9', 'Hipotiroidizm, tanımlanmamış',               NULL, 'IV. Endokrin Hastalıkları', TRUE),
(NULL, 'E04.9', 'Guatr, tanımlanmamış',                       NULL, 'IV. Endokrin Hastalıkları', TRUE),
(NULL, 'E05.9', 'Tirotoksikoz, tanımlanmamış',                NULL, 'IV. Endokrin Hastalıkları', TRUE),
(NULL, 'E10',   'Tip 1 diabetes mellitus',                    NULL, 'IV. Endokrin Hastalıkları', TRUE),
(NULL, 'E11',   'Tip 2 diabetes mellitus',                    NULL, 'IV. Endokrin Hastalıkları', TRUE),
(NULL, 'E11.9', 'Tip 2 diabetes mellitus, komplikasyonsuz',   'E11', 'IV. Endokrin Hastalıkları', TRUE),
(NULL, 'E55.9', 'D vitamini eksikliği, tanımlanmamış',        NULL, 'IV. Endokrin Hastalıkları', TRUE),
(NULL, 'E66.9', 'Obezite, tanımlanmamış',                     NULL, 'IV. Endokrin Hastalıkları', TRUE),
(NULL, 'E78.0', 'Saf hiperkolesterolemi',                     NULL, 'IV. Endokrin Hastalıkları', TRUE),
(NULL, 'E78.5', 'Hiperlipidemi, tanımlanmamış',               NULL, 'IV. Endokrin Hastalıkları', TRUE),
(NULL, 'E86',   'Sıvı kaybı (dehidratasyon)',                 NULL, 'IV. Endokrin Hastalıkları', TRUE),
-- V. Ruh ve davranış (F00-F99)
(NULL, 'F32.9', 'Depresif atak, tanımlanmamış',               NULL, 'V. Ruhsal Hastalıklar', TRUE),
(NULL, 'F33.9', 'Tekrarlayan depresif bozukluk',              NULL, 'V. Ruhsal Hastalıklar', TRUE),
(NULL, 'F41.0', 'Panik bozukluk',                             NULL, 'V. Ruhsal Hastalıklar', TRUE),
(NULL, 'F41.1', 'Yaygın anksiyete bozukluğu',                 NULL, 'V. Ruhsal Hastalıklar', TRUE),
(NULL, 'F41.9', 'Anksiyete bozukluğu, tanımlanmamış',         NULL, 'V. Ruhsal Hastalıklar', TRUE),
(NULL, 'F51.0', 'İnsomni, organik olmayan',                   NULL, 'V. Ruhsal Hastalıklar', TRUE),
-- VI. Sinir sistemi (G00-G99)
(NULL, 'G40.9', 'Epilepsi, tanımlanmamış',                    NULL, 'VI. Sinir Sistemi', TRUE),
(NULL, 'G43.9', 'Migren, tanımlanmamış',                      NULL, 'VI. Sinir Sistemi', TRUE),
(NULL, 'G44.2', 'Gerilim tipi baş ağrısı',                    NULL, 'VI. Sinir Sistemi', TRUE),
(NULL, 'G47.0', 'Uykuya başlama ve uyku sürdürme bozuklukları', NULL, 'VI. Sinir Sistemi', TRUE),
(NULL, 'G56.0', 'Karpal tünel sendromu',                      NULL, 'VI. Sinir Sistemi', TRUE),
-- VII. Göz hastalıkları (H00-H59)
(NULL, 'H10.9', 'Konjonktivit, tanımlanmamış',                NULL, 'VII. Göz Hastalıkları', TRUE),
(NULL, 'H52.0', 'Hipermetropi',                               NULL, 'VII. Göz Hastalıkları', TRUE),
(NULL, 'H52.1', 'Miyopi',                                     NULL, 'VII. Göz Hastalıkları', TRUE),
(NULL, 'H52.2', 'Astigmatizma',                               NULL, 'VII. Göz Hastalıkları', TRUE),
(NULL, 'H53.9', 'Görme bozukluğu, tanımlanmamış',             NULL, 'VII. Göz Hastalıkları', TRUE),
-- VIII. Kulak hastalıkları (H60-H95)
(NULL, 'H60.9', 'Otitis externa, tanımlanmamış',              NULL, 'VIII. Kulak Hastalıkları', TRUE),
(NULL, 'H66.9', 'Orta kulak iltihabı (otitis media)',         NULL, 'VIII. Kulak Hastalıkları', TRUE),
(NULL, 'H81.0', 'Meniere hastalığı',                          NULL, 'VIII. Kulak Hastalıkları', TRUE),
(NULL, 'H81.4', 'Santral kökenli vertigo',                    NULL, 'VIII. Kulak Hastalıkları', TRUE),
(NULL, 'H93.1', 'Tinnitus',                                   NULL, 'VIII. Kulak Hastalıkları', TRUE),
-- IX. Dolaşım sistemi (I00-I99)
(NULL, 'I10',   'Esansiyel (primer) hipertansiyon',           NULL, 'IX. Dolaşım Sistemi', TRUE),
(NULL, 'I11.9', 'Hipertansif kalp hastalığı',                 NULL, 'IX. Dolaşım Sistemi', TRUE),
(NULL, 'I20.0', 'Anstabil angina',                            NULL, 'IX. Dolaşım Sistemi', TRUE),
(NULL, 'I20.9', 'Angina pektoris, tanımlanmamış',             NULL, 'IX. Dolaşım Sistemi', TRUE),
(NULL, 'I21.9', 'Akut miyokard enfarktüsü',                   NULL, 'IX. Dolaşım Sistemi', TRUE),
(NULL, 'I25.1', 'Aterosklerotik kalp hastalığı',              NULL, 'IX. Dolaşım Sistemi', TRUE),
(NULL, 'I48',   'Atrial fibrilasyon ve flutter',              NULL, 'IX. Dolaşım Sistemi', TRUE),
(NULL, 'I50.9', 'Kalp yetmezliği, tanımlanmamış',             NULL, 'IX. Dolaşım Sistemi', TRUE),
(NULL, 'I63.9', 'Serebral enfarktüs, tanımlanmamış',          NULL, 'IX. Dolaşım Sistemi', TRUE),
(NULL, 'I83.9', 'Alt ekstremite varisi',                      NULL, 'IX. Dolaşım Sistemi', TRUE),
(NULL, 'I84.9', 'Hemoroidler, tanımlanmamış',                 NULL, 'IX. Dolaşım Sistemi', TRUE),
-- X. Solunum (J00-J99)
(NULL, 'J00',   'Akut nazofarenjit (soğuk algınlığı)',        NULL, 'X. Solunum Sistemi', TRUE),
(NULL, 'J02.9', 'Akut farenjit, tanımlanmamış',               NULL, 'X. Solunum Sistemi', TRUE),
(NULL, 'J03.9', 'Akut tonsillit, tanımlanmamış',              NULL, 'X. Solunum Sistemi', TRUE),
(NULL, 'J06.9', 'Akut üst solunum yolu enfeksiyonu',          NULL, 'X. Solunum Sistemi', TRUE),
(NULL, 'J11.1', 'Grip, başka tanımlı belirtilerle',           NULL, 'X. Solunum Sistemi', TRUE),
(NULL, 'J18.9', 'Pnömoni, tanımlanmamış',                     NULL, 'X. Solunum Sistemi', TRUE),
(NULL, 'J20.9', 'Akut bronşit, tanımlanmamış',                NULL, 'X. Solunum Sistemi', TRUE),
(NULL, 'J30.4', 'Allerjik rinit, tanımlanmamış',              NULL, 'X. Solunum Sistemi', TRUE),
(NULL, 'J32.9', 'Kronik sinüzit, tanımlanmamış',              NULL, 'X. Solunum Sistemi', TRUE),
(NULL, 'J44.9', 'KOAH, tanımlanmamış',                        NULL, 'X. Solunum Sistemi', TRUE),
(NULL, 'J45.9', 'Astım, tanımlanmamış',                       NULL, 'X. Solunum Sistemi', TRUE),
-- XI. Sindirim (K00-K95)
(NULL, 'K02.9', 'Diş çürüğü, tanımlanmamış',                  NULL, 'XI. Sindirim Sistemi', TRUE),
(NULL, 'K21.9', 'Gastroözofageal reflü, özofajitsiz',         NULL, 'XI. Sindirim Sistemi', TRUE),
(NULL, 'K25.9', 'Mide ülseri, tanımlanmamış',                 NULL, 'XI. Sindirim Sistemi', TRUE),
(NULL, 'K29.7', 'Gastrit, tanımlanmamış',                     NULL, 'XI. Sindirim Sistemi', TRUE),
(NULL, 'K30',   'Fonksiyonel dispepsi',                       NULL, 'XI. Sindirim Sistemi', TRUE),
(NULL, 'K40.9', 'İnguinal herni, tanımlanmamış',              NULL, 'XI. Sindirim Sistemi', TRUE),
(NULL, 'K52.9', 'Gastroenterit, enfeksiyöz olmayan',          NULL, 'XI. Sindirim Sistemi', TRUE),
(NULL, 'K58.9', 'İrritabl bağırsak sendromu',                 NULL, 'XI. Sindirim Sistemi', TRUE),
(NULL, 'K59.0', 'Kabızlık',                                   NULL, 'XI. Sindirim Sistemi', TRUE),
(NULL, 'K80.2', 'Safra taşı (kolesistitsiz)',                 NULL, 'XI. Sindirim Sistemi', TRUE),
(NULL, 'K92.2', 'Gastrointestinal kanama, tanımlanmamış',     NULL, 'XI. Sindirim Sistemi', TRUE),
-- XII. Cilt (L00-L99)
(NULL, 'L08.9', 'Cilt enfeksiyonu, tanımlanmamış',            NULL, 'XII. Cilt Hastalıkları', TRUE),
(NULL, 'L20.9', 'Atopik dermatit, tanımlanmamış',             NULL, 'XII. Cilt Hastalıkları', TRUE),
(NULL, 'L23.9', 'Allerjik kontakt dermatit',                  NULL, 'XII. Cilt Hastalıkları', TRUE),
(NULL, 'L29.9', 'Kaşıntı, tanımlanmamış',                     NULL, 'XII. Cilt Hastalıkları', TRUE),
(NULL, 'L40.9', 'Psoriazis, tanımlanmamış',                   NULL, 'XII. Cilt Hastalıkları', TRUE),
(NULL, 'L50.9', 'Ürtiker, tanımlanmamış',                     NULL, 'XII. Cilt Hastalıkları', TRUE),
(NULL, 'L70.0', 'Akne vulgaris',                              NULL, 'XII. Cilt Hastalıkları', TRUE),
-- XIII. Kas-iskelet (M00-M99)
(NULL, 'M06.9', 'Romatoid artrit, tanımlanmamış',             NULL, 'XIII. Kas-İskelet Sistemi', TRUE),
(NULL, 'M15.9', 'Polyosteoartroz, tanımlanmamış',             NULL, 'XIII. Kas-İskelet Sistemi', TRUE),
(NULL, 'M17.9', 'Diz osteoartriti, tanımlanmamış',            NULL, 'XIII. Kas-İskelet Sistemi', TRUE),
(NULL, 'M25.5', 'Eklem ağrısı',                               NULL, 'XIII. Kas-İskelet Sistemi', TRUE),
(NULL, 'M51.1', 'Lomber disk hernisi (radikülopatili)',       NULL, 'XIII. Kas-İskelet Sistemi', TRUE),
(NULL, 'M54.2', 'Servikalji (boyun ağrısı)',                  NULL, 'XIII. Kas-İskelet Sistemi', TRUE),
(NULL, 'M54.5', 'Bel ağrısı',                                 NULL, 'XIII. Kas-İskelet Sistemi', TRUE),
(NULL, 'M62.9', 'Kas bozukluğu, tanımlanmamış',               NULL, 'XIII. Kas-İskelet Sistemi', TRUE),
(NULL, 'M77.9', 'Entezopati, tanımlanmamış',                  NULL, 'XIII. Kas-İskelet Sistemi', TRUE),
(NULL, 'M79.6', 'Ekstremite ağrısı',                          NULL, 'XIII. Kas-İskelet Sistemi', TRUE),
(NULL, 'M81.9', 'Osteoporoz, tanımlanmamış',                  NULL, 'XIII. Kas-İskelet Sistemi', TRUE),
-- XIV. Üriner (N00-N99)
(NULL, 'N18.9', 'Kronik böbrek hastalığı, tanımlanmamış',     NULL, 'XIV. Üriner Sistem', TRUE),
(NULL, 'N20.0', 'Böbrek taşı',                                NULL, 'XIV. Üriner Sistem', TRUE),
(NULL, 'N30.9', 'Sistit, tanımlanmamış',                      NULL, 'XIV. Üriner Sistem', TRUE),
(NULL, 'N39.0', 'İdrar yolu enfeksiyonu, lokalize olmamış',   NULL, 'XIV. Üriner Sistem', TRUE),
(NULL, 'N40',   'Benign prostat hiperplazisi',                NULL, 'XIV. Üriner Sistem', TRUE),
(NULL, 'N73.9', 'Pelvik inflamatuar hastalık',                NULL, 'XIV. Üriner Sistem', TRUE),
(NULL, 'N76.0', 'Akut vajinit',                               NULL, 'XIV. Üriner Sistem', TRUE),
(NULL, 'N92.0', 'Düzenli sikluslu çok ve sık menstrüasyon',   NULL, 'XIV. Üriner Sistem', TRUE),
(NULL, 'N94.6', 'Dismenore, tanımlanmamış',                   NULL, 'XIV. Üriner Sistem', TRUE),
-- XV. Gebelik (O00-O99)
(NULL, 'O80',   'Tek normal doğum',                           NULL, 'XV. Gebelik ve Doğum', TRUE),
(NULL, 'O82',   'Sezaryen ile tek doğum',                     NULL, 'XV. Gebelik ve Doğum', TRUE),
(NULL, 'Z34.9', 'Normal gebelik takibi, tanımlanmamış',       NULL, 'XV. Gebelik ve Doğum', TRUE),
-- XVI. Perinatal (P00-P96)
(NULL, 'P59.9', 'Yenidoğan sarılığı, tanımlanmamış',          NULL, 'XVI. Perinatal', TRUE),
-- XVII. Konjenital (Q00-Q99)
(NULL, 'Q65.9', 'Konjenital kalça displazisi',                NULL, 'XVII. Konjenital Anomaliler', TRUE),
-- XVIII. Belirti ve bulgular (R00-R99)
(NULL, 'R05',   'Öksürük',                                    NULL, 'XVIII. Belirti ve Bulgular', TRUE),
(NULL, 'R07.4', 'Göğüs ağrısı, tanımlanmamış',                NULL, 'XVIII. Belirti ve Bulgular', TRUE),
(NULL, 'R10.4', 'Karın ağrısı, lokalize olmamış',             NULL, 'XVIII. Belirti ve Bulgular', TRUE),
(NULL, 'R11',   'Bulantı ve kusma',                           NULL, 'XVIII. Belirti ve Bulgular', TRUE),
(NULL, 'R19.5', 'Anormal feçes bulguları',                    NULL, 'XVIII. Belirti ve Bulgular', TRUE),
(NULL, 'R21',   'Döküntü, tanımlanmamış',                     NULL, 'XVIII. Belirti ve Bulgular', TRUE),
(NULL, 'R42',   'Baş dönmesi ve sersemleme',                  NULL, 'XVIII. Belirti ve Bulgular', TRUE),
(NULL, 'R50.9', 'Ateş, tanımlanmamış',                        NULL, 'XVIII. Belirti ve Bulgular', TRUE),
(NULL, 'R51',   'Baş ağrısı',                                 NULL, 'XVIII. Belirti ve Bulgular', TRUE),
(NULL, 'R53',   'Halsizlik ve yorgunluk',                     NULL, 'XVIII. Belirti ve Bulgular', TRUE),
(NULL, 'R60.0', 'Lokalize ödem',                              NULL, 'XVIII. Belirti ve Bulgular', TRUE),
-- XIX. Yaralanma (S00-T98)
(NULL, 'S00.0', 'Saçlı deri yüzeyel travması',                NULL, 'XIX. Yaralanma ve Zehirlenme', TRUE),
(NULL, 'S52.5', 'Distal radius kırığı',                       NULL, 'XIX. Yaralanma ve Zehirlenme', TRUE),
(NULL, 'S72.0', 'Femur boyun kırığı',                         NULL, 'XIX. Yaralanma ve Zehirlenme', TRUE),
(NULL, 'S93.4', 'Ayak bileği burkulması',                     NULL, 'XIX. Yaralanma ve Zehirlenme', TRUE),
(NULL, 'T78.4', 'Allerji, tanımlanmamış',                     NULL, 'XIX. Yaralanma ve Zehirlenme', TRUE),
-- XXI. Sağlık hizmeti kullanım nedenleri (Z00-Z99) — sık kullanılır
(NULL, 'Z00.0', 'Genel tıbbi muayene',                        NULL, 'XXI. Sağlık Hizmeti Kullanım Nedenleri', TRUE),
(NULL, 'Z00.1', 'Çocuk rutin sağlık kontrolü',                NULL, 'XXI. Sağlık Hizmeti Kullanım Nedenleri', TRUE),
(NULL, 'Z01.4', 'Jinekolojik muayene (genel rutin)',          NULL, 'XXI. Sağlık Hizmeti Kullanım Nedenleri', TRUE),
(NULL, 'Z02.0', 'Eğitim kurumu kabul muayenesi',              NULL, 'XXI. Sağlık Hizmeti Kullanım Nedenleri', TRUE),
(NULL, 'Z02.7', 'Tıbbi sertifika için muayene',               NULL, 'XXI. Sağlık Hizmeti Kullanım Nedenleri', TRUE),
(NULL, 'Z03.9', 'Şüpheli durum gözlem, tanımlanmamış',        NULL, 'XXI. Sağlık Hizmeti Kullanım Nedenleri', TRUE),
(NULL, 'Z23.9', 'Aşılama, tek bir hastalığa karşı',           NULL, 'XXI. Sağlık Hizmeti Kullanım Nedenleri', TRUE),
(NULL, 'Z71.3', 'Diyet danışmanlığı ve izlenmesi',            NULL, 'XXI. Sağlık Hizmeti Kullanım Nedenleri', TRUE),
(NULL, 'Z76.0', 'Reçete yenileme',                            NULL, 'XXI. Sağlık Hizmeti Kullanım Nedenleri', TRUE);

COMMIT;
