-- 010_radiology: imaging procedures + per-order report (findings +
-- impression + recommendations) + PACS link fields. Unlike lab where
-- a single order can request multiple analytes, a radiology order
-- carries exactly one procedure (chest X-ray, abdominal USG, brain
-- MR...), so we keep the structure flat — no separate item table.
--
-- The order row holds:
--   • request (priority, clinical indication, clinical question)
--   • acquisition (scheduled_at, acquired_at + technician)
--   • report (findings, impression, recommendations, reporting +
--     verifying radiologist)
--   • PACS link (study UID, accession no, thumbnail URL) — optional
--     so the system works without a configured PACS

BEGIN;

CREATE TYPE radiology_modality AS ENUM (
    'XR',        -- röntgen / radyografi
    'USG',       -- ultrason
    'CT',        -- bilgisayarlı tomografi
    'MR',        -- manyetik rezonans
    'MAMMO',     -- mamografi
    'NM',        -- nükleer tıp
    'DEXA',      -- kemik dansitometri
    'PET',       -- pozitron emisyon tomografisi
    'ANGIO',     -- anjiyo
    'FLUORO',    -- floroskopi
    'OTHER'
);

CREATE TYPE radiology_order_status AS ENUM (
    'ordered',     -- istek girildi
    'scheduled',   -- çekim için randevulandı
    'in_progress', -- çekim sırasında
    'acquired',    -- çekim tamamlandı, rapor bekleniyor
    'reported',    -- radyolog raporu yazdı
    'verified',    -- sorumlu uzman onayladı
    'cancelled'
);

CREATE TYPE radiology_order_priority AS ENUM ('routine', 'urgent', 'stat');

-- Procedure catalog ------------------------------------------------------

CREATE TABLE radiology_procedure (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id   UUID REFERENCES organization(id) ON DELETE CASCADE, -- NULL = system
    code              TEXT NOT NULL,
    name              TEXT NOT NULL,
    modality          radiology_modality NOT NULL,
    body_region       TEXT,           -- "Toraks", "Abdomen", "Kranium" ...
    sut_code          TEXT,
    estimated_minutes INTEGER CHECK (estimated_minutes IS NULL OR estimated_minutes > 0),
    preparation_notes TEXT,           -- "Aç gelinmeli", "Kontrast var" ...
    is_system         BOOLEAN NOT NULL DEFAULT FALSE,
    is_active         BOOLEAN NOT NULL DEFAULT TRUE,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE NULLS NOT DISTINCT (organization_id, code)
);

CREATE INDEX idx_radprocedure_org ON radiology_procedure(organization_id);
CREATE INDEX idx_radprocedure_modality ON radiology_procedure(modality);
CREATE INDEX idx_radprocedure_search ON radiology_procedure
    USING gin(to_tsvector('simple', code || ' ' || name || ' ' || COALESCE(body_region, '')));

CREATE TRIGGER trg_radprocedure_updated_at BEFORE UPDATE ON radiology_procedure
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Orders -----------------------------------------------------------------

CREATE SEQUENCE radiology_order_no_seq START 100000 INCREMENT 1;

CREATE TABLE radiology_order (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id     UUID NOT NULL REFERENCES organization(id) ON DELETE CASCADE,
    branch_id           UUID NOT NULL REFERENCES branch(id) ON DELETE CASCADE,
    visit_id            UUID REFERENCES visit(id) ON DELETE SET NULL,
    patient_id          UUID NOT NULL REFERENCES patient(id) ON DELETE RESTRICT,
    ordering_doctor_id  UUID REFERENCES doctor(id) ON DELETE SET NULL,

    order_no            TEXT NOT NULL,
    status              radiology_order_status NOT NULL DEFAULT 'ordered',
    priority            radiology_order_priority NOT NULL DEFAULT 'routine',

    -- Procedure snapshot — survives catalog edits.
    procedure_id        UUID NOT NULL REFERENCES radiology_procedure(id) ON DELETE RESTRICT,
    procedure_code      TEXT NOT NULL,
    procedure_name      TEXT NOT NULL,
    modality            radiology_modality NOT NULL,
    body_region         TEXT,

    clinical_indication TEXT,    -- doktor neden istedi
    clinical_question   TEXT,    -- spesifik soru ("MS lehine bulgu var mı?")
    notes               TEXT,

    scheduled_at        TIMESTAMPTZ,
    acquired_at         TIMESTAMPTZ,
    acquired_by_user_id UUID REFERENCES app_user(id) ON DELETE SET NULL,

    -- Report
    reporting_doctor_id UUID REFERENCES doctor(id) ON DELETE SET NULL,
    findings            TEXT,    -- "Akciğerler havalı; sinüs frenik açıklar serbest..."
    impression          TEXT,    -- "Normal akciğer grafisi"
    recommendations     TEXT,    -- "İleri tetkik gerekmez"
    reported_at         TIMESTAMPTZ,
    verified_at         TIMESTAMPTZ,
    verified_by_user_id UUID REFERENCES app_user(id) ON DELETE SET NULL,

    -- PACS hook (optional)
    pacs_study_uid       TEXT,
    pacs_accession_number TEXT,
    thumbnail_url        TEXT,

    ordered_by_user_id   UUID REFERENCES app_user(id) ON DELETE SET NULL,
    ordered_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    cancelled_at         TIMESTAMPTZ,
    cancellation_reason  TEXT,

    created_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (organization_id, order_no)
);

CREATE INDEX idx_radorder_branch_status ON radiology_order(branch_id, status);
CREATE INDEX idx_radorder_visit ON radiology_order(visit_id) WHERE visit_id IS NOT NULL;
CREATE INDEX idx_radorder_patient ON radiology_order(patient_id);
CREATE INDEX idx_radorder_modality ON radiology_order(branch_id, modality);
CREATE INDEX idx_radorder_ordered_at ON radiology_order(branch_id, ordered_at DESC);

CREATE TRIGGER trg_radorder_updated_at BEFORE UPDATE ON radiology_order
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ---------- System seed: 25 commonly-ordered Turkish radiology procedures ----------

INSERT INTO radiology_procedure (organization_id, code, name, modality, body_region, estimated_minutes, preparation_notes, is_system) VALUES
    -- X-ray
    (NULL, 'XR_AC',         'Akciğer Grafisi (PA)',         'XR',  'Toraks',       5,  NULL,                    TRUE),
    (NULL, 'XR_AC_YAN',     'Akciğer Grafisi (Lateral)',    'XR',  'Toraks',       5,  NULL,                    TRUE),
    (NULL, 'XR_DARK',       'Direkt Batın Grafisi',         'XR',  'Abdomen',      5,  NULL,                    TRUE),
    (NULL, 'XR_SERV',       'Servikal Vertebra Grafisi',    'XR',  'Servikal',     5,  NULL,                    TRUE),
    (NULL, 'XR_DORS',       'Dorsal Vertebra Grafisi',      'XR',  'Dorsal',       5,  NULL,                    TRUE),
    (NULL, 'XR_LOMB',       'Lomber Vertebra Grafisi',      'XR',  'Lomber',       5,  NULL,                    TRUE),
    (NULL, 'XR_PELVIS',     'Pelvis Grafisi',               'XR',  'Pelvis',       5,  NULL,                    TRUE),
    (NULL, 'XR_DIZ',        'Diz Grafisi',                  'XR',  'Diz',          5,  NULL,                    TRUE),
    -- USG
    (NULL, 'USG_BATIN',     'Tüm Batın USG',                'USG', 'Abdomen',      15, '6 saat aç olmak gereklidir.', TRUE),
    (NULL, 'USG_UST_BATIN', 'Üst Batın USG',                'USG', 'Abdomen',      10, '6 saat aç olmak gereklidir.', TRUE),
    (NULL, 'USG_PELVIK',    'Pelvik USG',                   'USG', 'Pelvis',       10, 'Tetkik için idrar tutulmalıdır.', TRUE),
    (NULL, 'USG_TIROID',    'Tiroid USG',                   'USG', 'Boyun',        10, NULL,                    TRUE),
    (NULL, 'USG_MEME',      'Meme USG',                     'USG', 'Meme',         15, NULL,                    TRUE),
    (NULL, 'USG_KAROTIS',   'Karotis Doppler USG',          'USG', 'Boyun',        20, NULL,                    TRUE),
    -- CT
    (NULL, 'CT_KRAN',       'Kraniyal BT (kontrastsız)',    'CT',  'Kranium',      15, NULL,                    TRUE),
    (NULL, 'CT_TORAKS',     'Toraks BT (kontrastsız)',      'CT',  'Toraks',       20, NULL,                    TRUE),
    (NULL, 'CT_TORAKS_K',   'Toraks BT (kontrastlı)',       'CT',  'Toraks',       30, 'Kontrast verilecek. Böbrek fonksiyonu kontrol edilmeli.', TRUE),
    (NULL, 'CT_BATIN',      'Tüm Batın BT (kontrastlı)',    'CT',  'Abdomen',      30, '4 saat aç olmak ve kontrast hazırlığı gereklidir.', TRUE),
    -- MR
    (NULL, 'MR_KRAN',       'Kraniyal MR',                  'MR',  'Kranium',      30, NULL,                    TRUE),
    (NULL, 'MR_LOMB',       'Lomber MR',                    'MR',  'Lomber',       30, NULL,                    TRUE),
    (NULL, 'MR_SERV',       'Servikal MR',                  'MR',  'Servikal',     30, NULL,                    TRUE),
    (NULL, 'MR_DIZ',        'Diz MR',                       'MR',  'Diz',          30, NULL,                    TRUE),
    (NULL, 'MR_OMUZ',       'Omuz MR',                      'MR',  'Omuz',         30, NULL,                    TRUE),
    -- Mammography + DEXA
    (NULL, 'MAMMO_BIL',     'Mamografi (bilateral)',        'MAMMO', 'Meme',       15, NULL,                    TRUE),
    (NULL, 'DEXA',          'Kemik Dansitometri (DEXA)',    'DEXA',  'Lomber+Femur', 15, NULL,                  TRUE);

COMMIT;
