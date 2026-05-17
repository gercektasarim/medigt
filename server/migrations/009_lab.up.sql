-- 009_lab: laboratory orders + results.
--
-- lab_test_catalog    — the catalog of tests we can order (system seeded
--                       + organization-specific)
-- lab_order           — a request placed by a doctor (usually from a visit)
-- lab_order_item      — the individual tests on that order, each with its
--                       own status + result fields. We keep result fields on
--                       the item row (rather than a separate lab_result
--                       table) so the LIS-style "order → result" path is
--                       a single update on a single row, which is much
--                       friendlier for hand-entry from the UI.
--
-- 20 of the most-ordered Turkish primary-care tests are seeded as a system
-- catalog. Hospitals can add their own (and override the unit + ranges).

BEGIN;

CREATE TYPE lab_order_status AS ENUM (
    'ordered',      -- istek girildi
    'sampled',      -- numune alındı, laboratuvara geçti
    'in_progress',  -- çalışılıyor
    'resulted',     -- sonuçlandı, doktor doğrulayabilir
    'verified',     -- doktor / sorumlu uzman onayladı
    'cancelled'
);

CREATE TYPE lab_order_priority AS ENUM ('routine', 'urgent', 'stat');

CREATE TYPE lab_result_flag AS ENUM (
    'normal',
    'low',
    'high',
    'critical_low',
    'critical_high',
    'abnormal'
);

CREATE TYPE lab_sample_type AS ENUM (
    'blood', 'urine', 'stool', 'sputum',
    'throat_swab', 'nasal_swab', 'csf', 'tissue', 'other'
);

-- Catalog -----------------------------------------------------------------

CREATE TABLE lab_test_catalog (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID REFERENCES organization(id) ON DELETE CASCADE, -- NULL = system
    code            TEXT NOT NULL,
    loinc_code      TEXT,
    sut_code        TEXT,
    name            TEXT NOT NULL,
    sample_type     lab_sample_type NOT NULL DEFAULT 'blood',
    unit            TEXT,
    reference_range TEXT,
    is_system       BOOLEAN NOT NULL DEFAULT FALSE,
    is_active       BOOLEAN NOT NULL DEFAULT TRUE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE NULLS NOT DISTINCT (organization_id, code)
);

CREATE INDEX idx_lab_test_catalog_org ON lab_test_catalog(organization_id);
CREATE INDEX idx_lab_test_catalog_search ON lab_test_catalog
    USING gin(to_tsvector('simple', code || ' ' || name));

CREATE TRIGGER trg_lab_test_catalog_updated_at BEFORE UPDATE ON lab_test_catalog
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Orders ------------------------------------------------------------------

CREATE SEQUENCE lab_order_no_seq START 100000 INCREMENT 1;

CREATE TABLE lab_order (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id     UUID NOT NULL REFERENCES organization(id) ON DELETE CASCADE,
    branch_id           UUID NOT NULL REFERENCES branch(id) ON DELETE CASCADE,
    visit_id            UUID REFERENCES visit(id) ON DELETE SET NULL,
    patient_id          UUID NOT NULL REFERENCES patient(id) ON DELETE RESTRICT,
    ordering_doctor_id  UUID REFERENCES doctor(id) ON DELETE SET NULL,

    order_no            TEXT NOT NULL,
    status              lab_order_status NOT NULL DEFAULT 'ordered',
    priority            lab_order_priority NOT NULL DEFAULT 'routine',

    clinical_indication TEXT,
    notes               TEXT,

    ordered_by_user_id  UUID REFERENCES app_user(id) ON DELETE SET NULL,
    ordered_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    sampled_at          TIMESTAMPTZ,
    sampled_by_user_id  UUID REFERENCES app_user(id) ON DELETE SET NULL,
    completed_at        TIMESTAMPTZ,
    cancelled_at        TIMESTAMPTZ,

    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (organization_id, order_no)
);

CREATE INDEX idx_lab_order_branch_status ON lab_order(branch_id, status);
CREATE INDEX idx_lab_order_visit ON lab_order(visit_id) WHERE visit_id IS NOT NULL;
CREATE INDEX idx_lab_order_patient ON lab_order(patient_id);
CREATE INDEX idx_lab_order_ordered_at ON lab_order(branch_id, ordered_at DESC);

CREATE TRIGGER trg_lab_order_updated_at BEFORE UPDATE ON lab_order
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE lab_order_item (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    lab_order_id        UUID NOT NULL REFERENCES lab_order(id) ON DELETE CASCADE,
    lab_test_catalog_id UUID NOT NULL REFERENCES lab_test_catalog(id) ON DELETE RESTRICT,

    -- Snapshot of the catalog row at order time so historical results stay
    -- readable even if the catalog later changes name/unit/range.
    test_code           TEXT NOT NULL,
    test_name           TEXT NOT NULL,
    sample_type         lab_sample_type NOT NULL,
    unit                TEXT,
    reference_range     TEXT,

    status              lab_order_status NOT NULL DEFAULT 'ordered',
    sort_order          INTEGER NOT NULL DEFAULT 0,

    -- Result fields — populated when status -> resulted.
    value_numeric       NUMERIC(15,4),
    value_text          TEXT,
    flag                lab_result_flag,
    resulted_at         TIMESTAMPTZ,
    resulted_by_user_id UUID REFERENCES app_user(id) ON DELETE SET NULL,
    verified_at         TIMESTAMPTZ,
    verified_by_user_id UUID REFERENCES app_user(id) ON DELETE SET NULL,
    notes               TEXT,

    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_lab_order_item_order ON lab_order_item(lab_order_id, sort_order);
CREATE INDEX idx_lab_order_item_status ON lab_order_item(lab_order_id, status);

CREATE TRIGGER trg_lab_order_item_updated_at BEFORE UPDATE ON lab_order_item
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ---------- System seed: 20 most-common Turkish primary-care tests ----------

INSERT INTO lab_test_catalog (organization_id, code, name, sample_type, unit, reference_range, is_system) VALUES
    -- Hemogram / hematology
    (NULL, 'CBC',       'Tam Kan Sayımı (Hemogram)',     'blood', NULL,         NULL,                  TRUE),
    (NULL, 'HGB',       'Hemoglobin',                    'blood', 'g/dL',       '12-16 (K) / 13-17 (E)', TRUE),
    (NULL, 'HCT',       'Hematokrit',                    'blood', '%',          '36-46 (K) / 40-54 (E)', TRUE),
    (NULL, 'WBC',       'Beyaz Küre Sayısı',             'blood', '10^3/uL',    '4.0-10.0',            TRUE),
    (NULL, 'PLT',       'Trombosit',                     'blood', '10^3/uL',    '150-400',             TRUE),
    -- Biochemistry
    (NULL, 'GLU',       'Glukoz (Açlık)',                'blood', 'mg/dL',      '70-100',              TRUE),
    (NULL, 'HBA1C',     'HbA1c',                         'blood', '%',          '< 5.7',               TRUE),
    (NULL, 'UREA',      'Üre',                           'blood', 'mg/dL',      '15-45',               TRUE),
    (NULL, 'CREA',      'Kreatinin',                     'blood', 'mg/dL',      '0.6-1.2',             TRUE),
    (NULL, 'AST',       'AST (SGOT)',                    'blood', 'U/L',        '< 40',                TRUE),
    (NULL, 'ALT',       'ALT (SGPT)',                    'blood', 'U/L',        '< 40',                TRUE),
    (NULL, 'CHOL',      'Total Kolesterol',              'blood', 'mg/dL',      '< 200',               TRUE),
    (NULL, 'LDL',       'LDL Kolesterol',                'blood', 'mg/dL',      '< 130',               TRUE),
    (NULL, 'HDL',       'HDL Kolesterol',                'blood', 'mg/dL',      '> 40',                TRUE),
    (NULL, 'TRIG',      'Trigliserid',                   'blood', 'mg/dL',      '< 150',               TRUE),
    -- Thyroid
    (NULL, 'TSH',       'TSH',                           'blood', 'uIU/mL',     '0.4-4.5',             TRUE),
    (NULL, 'FT4',       'Serbest T4',                    'blood', 'ng/dL',      '0.8-1.8',             TRUE),
    -- Inflammation / others
    (NULL, 'CRP',       'C-Reaktif Protein',             'blood', 'mg/L',       '< 5',                 TRUE),
    (NULL, 'ESR',       'Sedimantasyon',                 'blood', 'mm/saat',    '< 20 (K) / < 15 (E)', TRUE),
    -- Urine
    (NULL, 'URINALYSIS', 'Tam İdrar Tetkiki',            'urine', NULL,         NULL,                  TRUE);

COMMIT;
