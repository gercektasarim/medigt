-- 002_people: Specialization catalog (tıbbi branşlar), staff_member (the HR
-- profile that wraps app_user), doctor + nurse role-specific profiles, and
-- a many-to-many between doctors and their specializations.

BEGIN;

-- ---------- Tıbbi branş kataloğu ----------

CREATE TABLE specialization (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID REFERENCES organization(id) ON DELETE CASCADE, -- NULL = system catalog
    code            TEXT NOT NULL,                                       -- SGK SUT code or short slug
    name            TEXT NOT NULL,
    parent_id       UUID REFERENCES specialization(id) ON DELETE SET NULL,
    is_system       BOOLEAN NOT NULL DEFAULT FALSE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE NULLS NOT DISTINCT (organization_id, code)
);

CREATE INDEX idx_specialization_org ON specialization(organization_id);
CREATE INDEX idx_specialization_parent ON specialization(parent_id);

CREATE TRIGGER trg_specialization_updated_at BEFORE UPDATE ON specialization
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ---------- Personel (HR profili) ----------

CREATE TABLE staff_member (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organization(id) ON DELETE CASCADE,
    user_id         UUID REFERENCES app_user(id) ON DELETE SET NULL,
    -- HR alanları
    employee_no     TEXT,
    first_name      TEXT NOT NULL,
    last_name       TEXT NOT NULL,
    title           TEXT,                          -- "Uzm. Dr.", "Hemşire", "Sekreter" vs
    employment_type TEXT NOT NULL DEFAULT 'full_time'
        CHECK (employment_type IN ('full_time', 'part_time', 'contract', 'consultant', 'intern')),
    hire_date       DATE,
    termination_date DATE,
    phone           TEXT,
    email           TEXT,
    notes           TEXT,
    is_active       BOOLEAN NOT NULL DEFAULT TRUE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (organization_id, employee_no)
);

CREATE INDEX idx_staff_member_org ON staff_member(organization_id);
CREATE INDEX idx_staff_member_user ON staff_member(user_id) WHERE user_id IS NOT NULL;
CREATE INDEX idx_staff_member_active ON staff_member(organization_id, is_active);

CREATE TRIGGER trg_staff_member_updated_at BEFORE UPDATE ON staff_member
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ---------- Doktor profili ----------

CREATE TABLE doctor (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    staff_member_id     UUID NOT NULL REFERENCES staff_member(id) ON DELETE CASCADE UNIQUE,
    diploma_no          TEXT,
    medula_doctor_code  TEXT,                       -- SGK Medula doktor kodu
    signature_image_path TEXT,
    license_expires_at  DATE,
    is_accepting_patients BOOLEAN NOT NULL DEFAULT TRUE,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_doctor_medula ON doctor(medula_doctor_code) WHERE medula_doctor_code IS NOT NULL;

CREATE TRIGGER trg_doctor_updated_at BEFORE UPDATE ON doctor
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE doctor_specialization (
    doctor_id          UUID NOT NULL REFERENCES doctor(id) ON DELETE CASCADE,
    specialization_id  UUID NOT NULL REFERENCES specialization(id) ON DELETE RESTRICT,
    is_primary         BOOLEAN NOT NULL DEFAULT FALSE,
    PRIMARY KEY (doctor_id, specialization_id)
);

-- ---------- Hemşire profili ----------

CREATE TABLE nurse (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    staff_member_id     UUID NOT NULL REFERENCES staff_member(id) ON DELETE CASCADE UNIQUE,
    license_no          TEXT,
    certification       JSONB NOT NULL DEFAULT '[]'::JSONB,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TRIGGER trg_nurse_updated_at BEFORE UPDATE ON nurse
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ---------- Seed: yaygın Türkçe tıbbi branşlar (system catalog) ----------
INSERT INTO specialization (organization_id, code, name, is_system) VALUES
    (NULL, 'IC_HASTALIKLARI',     'İç Hastalıkları',           TRUE),
    (NULL, 'KARDIYOLOJI',         'Kardiyoloji',               TRUE),
    (NULL, 'GENEL_CERRAHI',       'Genel Cerrahi',             TRUE),
    (NULL, 'KADIN_DOGUM',         'Kadın Hastalıkları ve Doğum', TRUE),
    (NULL, 'COCUK_SAGLIGI',       'Çocuk Sağlığı ve Hastalıkları', TRUE),
    (NULL, 'ORTOPEDI',            'Ortopedi ve Travmatoloji',  TRUE),
    (NULL, 'NORLOJI',             'Nöroloji',                  TRUE),
    (NULL, 'PSIKIYATRI',          'Psikiyatri',                TRUE),
    (NULL, 'DERMATOLOJI',         'Cildiye (Dermatoloji)',     TRUE),
    (NULL, 'GOZ',                 'Göz Hastalıkları',          TRUE),
    (NULL, 'KBB',                 'Kulak Burun Boğaz',         TRUE),
    (NULL, 'URLOJI',              'Üroloji',                   TRUE),
    (NULL, 'ANESTEZI',            'Anesteziyoloji ve Reanimasyon', TRUE),
    (NULL, 'RADYOLOJI',           'Radyoloji',                 TRUE),
    (NULL, 'BIYOKIMYA',           'Tıbbi Biyokimya',           TRUE),
    (NULL, 'MIKROBIYOLOJI',       'Tıbbi Mikrobiyoloji',       TRUE),
    (NULL, 'PATOLOJI',            'Tıbbi Patoloji',            TRUE),
    (NULL, 'AILE_HEKIMI',         'Aile Hekimliği',            TRUE),
    (NULL, 'ACIL_TIP',            'Acil Tıp',                  TRUE),
    (NULL, 'FTR',                 'Fiziksel Tıp ve Rehabilitasyon', TRUE),
    (NULL, 'GOGUS_HASTALIKLARI',  'Göğüs Hastalıkları',        TRUE),
    (NULL, 'ENDOKRINOLOJI',       'Endokrinoloji',             TRUE),
    (NULL, 'GASTROENTEROLOJI',    'Gastroenteroloji',          TRUE),
    (NULL, 'HEMATOLOJI',          'Hematoloji',                TRUE),
    (NULL, 'NEFROLOJI',           'Nefroloji',                 TRUE),
    (NULL, 'ROMATOLOJI',          'Romatoloji',                TRUE),
    (NULL, 'ONKOLOJI',            'Tıbbi Onkoloji',            TRUE),
    (NULL, 'ENFEKSIYON',          'Enfeksiyon Hastalıkları',   TRUE),
    (NULL, 'DIS_HEKIMLIGI',       'Diş Hekimliği',             TRUE),
    (NULL, 'AGIZ_DIS_CENE',       'Ağız Diş ve Çene Cerrahisi', TRUE);

COMMIT;
