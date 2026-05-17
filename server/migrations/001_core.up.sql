-- 001_core: Multi-tenant foundation — organization + branch + department,
-- user + membership + branch assignment, RBAC (role + permission), audit log.
-- All other domain tables FK to organization_id and (where applicable) branch_id.

BEGIN;

CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- ---------- Tenancy ----------

CREATE TABLE organization (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    slug            TEXT NOT NULL UNIQUE,
    name            TEXT NOT NULL,
    kind            TEXT NOT NULL CHECK (kind IN ('single_hospital', 'hospital_group', 'clinic', 'polyclinic')),
    tax_id          TEXT,
    sgk_employer_no TEXT,
    logo_url        TEXT,
    settings        JSONB NOT NULL DEFAULT '{}'::JSONB,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE branch (
    id                   UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id      UUID NOT NULL REFERENCES organization(id) ON DELETE CASCADE,
    slug                 TEXT NOT NULL,
    name                 TEXT NOT NULL,
    kind                 TEXT NOT NULL CHECK (kind IN ('hospital', 'polyclinic', 'lab', 'imaging_center', 'dialysis_center', 'dental_clinic')),
    address              TEXT,
    phone                TEXT,
    sgk_facility_code    TEXT,
    settings             JSONB NOT NULL DEFAULT '{}'::JSONB,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (organization_id, slug)
);

CREATE INDEX idx_branch_organization ON branch(organization_id);

CREATE TABLE department (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    branch_id   UUID NOT NULL REFERENCES branch(id) ON DELETE CASCADE,
    parent_id   UUID REFERENCES department(id) ON DELETE SET NULL,
    name        TEXT NOT NULL,
    kind        TEXT NOT NULL CHECK (kind IN (
        'outpatient', 'inpatient', 'emergency', 'lab', 'radiology',
        'pharmacy', 'cashier', 'surgery', 'dialysis', 'dental', 'administration'
    )),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_department_branch ON department(branch_id);

-- ---------- Users ----------

CREATE TABLE app_user (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email               TEXT NOT NULL UNIQUE,
    name                TEXT NOT NULL,
    phone               TEXT,
    -- national_id stored encrypted at app layer (FIELD_ENCRYPTION_KEY); only last 4 in clear
    national_id_enc     BYTEA,
    national_id_last4   TEXT,
    password_hash       TEXT,
    avatar_url          TEXT,
    totp_secret_enc     BYTEA,
    totp_enabled        BOOLEAN NOT NULL DEFAULT FALSE,
    is_active           BOOLEAN NOT NULL DEFAULT TRUE,
    last_login_at       TIMESTAMPTZ,
    failed_login_count  INT NOT NULL DEFAULT 0,
    locked_until        TIMESTAMPTZ,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_app_user_email ON app_user(lower(email));

CREATE TABLE user_session (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id             UUID NOT NULL REFERENCES app_user(id) ON DELETE CASCADE,
    refresh_token_hash  TEXT NOT NULL,
    user_agent          TEXT,
    ip_address          INET,
    expires_at          TIMESTAMPTZ NOT NULL,
    revoked_at          TIMESTAMPTZ,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_user_session_user ON user_session(user_id) WHERE revoked_at IS NULL;
CREATE INDEX idx_user_session_token ON user_session(refresh_token_hash);

-- ---------- Membership & RBAC ----------

CREATE TABLE org_membership (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organization(id) ON DELETE CASCADE,
    user_id         UUID NOT NULL REFERENCES app_user(id) ON DELETE CASCADE,
    system_role     TEXT NOT NULL CHECK (system_role IN ('platform_admin', 'org_owner', 'org_admin', 'org_member')),
    status          TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'invited', 'suspended')),
    invited_by      UUID REFERENCES app_user(id) ON DELETE SET NULL,
    invited_at      TIMESTAMPTZ,
    joined_at       TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (organization_id, user_id)
);

CREATE INDEX idx_org_membership_user ON org_membership(user_id);
CREATE INDEX idx_org_membership_org ON org_membership(organization_id);

CREATE TABLE permission (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    code        TEXT NOT NULL UNIQUE,
    module      TEXT NOT NULL,
    action      TEXT NOT NULL,
    description TEXT
);

CREATE INDEX idx_permission_module ON permission(module);

CREATE TABLE role (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID REFERENCES organization(id) ON DELETE CASCADE, -- NULL = system role
    code            TEXT NOT NULL,
    name            TEXT NOT NULL,
    description     TEXT,
    is_system       BOOLEAN NOT NULL DEFAULT FALSE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE NULLS NOT DISTINCT (organization_id, code)
);

CREATE TABLE role_permission (
    role_id       UUID NOT NULL REFERENCES role(id) ON DELETE CASCADE,
    permission_id UUID NOT NULL REFERENCES permission(id) ON DELETE CASCADE,
    PRIMARY KEY (role_id, permission_id)
);

CREATE TABLE branch_assignment (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID NOT NULL REFERENCES app_user(id) ON DELETE CASCADE,
    branch_id       UUID NOT NULL REFERENCES branch(id) ON DELETE CASCADE,
    role_id         UUID NOT NULL REFERENCES role(id) ON DELETE RESTRICT,
    primary_branch  BOOLEAN NOT NULL DEFAULT FALSE,
    valid_from      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    valid_until     TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (user_id, branch_id, role_id)
);

CREATE INDEX idx_branch_assignment_user ON branch_assignment(user_id);
CREATE INDEX idx_branch_assignment_branch ON branch_assignment(branch_id);

-- ---------- Audit log (KVKK-compliant) ----------

CREATE TABLE audit_log (
    id              BIGSERIAL PRIMARY KEY,
    organization_id UUID REFERENCES organization(id) ON DELETE SET NULL,
    branch_id       UUID REFERENCES branch(id) ON DELETE SET NULL,
    actor_user_id   UUID REFERENCES app_user(id) ON DELETE SET NULL,
    actor_session_id UUID REFERENCES user_session(id) ON DELETE SET NULL,
    action          TEXT NOT NULL,
    entity_type     TEXT NOT NULL,
    entity_id       TEXT,
    -- Plaintext personal data must NEVER land in details. Only IDs and structural metadata.
    details         JSONB NOT NULL DEFAULT '{}'::JSONB,
    ip_address      INET,
    user_agent      TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_audit_log_org_time ON audit_log(organization_id, created_at DESC);
CREATE INDEX idx_audit_log_branch_time ON audit_log(branch_id, created_at DESC);
CREATE INDEX idx_audit_log_actor_time ON audit_log(actor_user_id, created_at DESC);
CREATE INDEX idx_audit_log_entity ON audit_log(entity_type, entity_id);

-- ---------- updated_at trigger helper ----------

CREATE OR REPLACE FUNCTION set_updated_at() RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_organization_updated_at BEFORE UPDATE ON organization
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER trg_branch_updated_at BEFORE UPDATE ON branch
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER trg_department_updated_at BEFORE UPDATE ON department
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER trg_app_user_updated_at BEFORE UPDATE ON app_user
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER trg_org_membership_updated_at BEFORE UPDATE ON org_membership
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER trg_role_updated_at BEFORE UPDATE ON role
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER trg_branch_assignment_updated_at BEFORE UPDATE ON branch_assignment
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ---------- Seed: system roles & core permissions ----------

INSERT INTO role (organization_id, code, name, description, is_system) VALUES
    (NULL, 'clinical_lead',         'Klinik Yöneticisi',     'Tüm klinik modüllerde okuma + yönetim',            TRUE),
    (NULL, 'doctor',                'Doktor',                'Hasta, muayene, reçete, lab/rad istek, tanı',      TRUE),
    (NULL, 'doctor_assistant',      'Doktor Asistanı',       'Doktorun yaptığı her şey ama imzalayamaz',         TRUE),
    (NULL, 'nurse',                 'Hemşire',               'Vital signs, ilaç verme, yatak ataması',           TRUE),
    (NULL, 'reception',             'Hasta Kabul',           'Hasta kayıt, randevu, kabul',                       TRUE),
    (NULL, 'lab_technician',        'Lab Teknisyeni',        'Numune, sonuç girişi',                              TRUE),
    (NULL, 'radiology_technician',  'Radyoloji Teknisyeni',  'Tetkik çekim, görüntü yükleme',                     TRUE),
    (NULL, 'radiologist',           'Radyolog',              'Radyoloji raporu',                                  TRUE),
    (NULL, 'pharmacist',            'Eczacı',                'Eczane tam yetki',                                  TRUE),
    (NULL, 'cashier',               'Vezne',                 'Ödeme alma, kasa açma/kapama, iade (audit)',        TRUE),
    (NULL, 'finance_lead',          'Mali İşler Yöneticisi', 'Fatura, hakediş, mali raporlar',                    TRUE),
    (NULL, 'accountant',            'Muhasebeci',            'Mali okuma yetkisi',                                TRUE),
    (NULL, 'admin_clerk',           'İdari Memur',           'İdari raporlar, hasta dosyası okuma',               TRUE);

INSERT INTO permission (code, module, action, description) VALUES
    -- Tenant
    ('tenant.organization.read',        'tenant',    'read',     'Hastane grubunu okuma'),
    ('tenant.organization.write',       'tenant',    'write',    'Hastane grubu ayarlarını düzenleme'),
    ('tenant.branch.read',              'tenant',    'read',     'Şube okuma'),
    ('tenant.branch.write',             'tenant',    'write',    'Şube oluşturma/düzenleme'),
    ('tenant.user.invite',              'tenant',    'invite',   'Yeni kullanıcı davet etme'),
    ('tenant.user.deactivate',          'tenant',    'write',    'Kullanıcı askıya alma'),
    ('tenant.role.write',               'tenant',    'write',    'Rol/izin matrisini düzenleme'),
    -- Clinical: patient
    ('clinical.patient.read',           'clinical',  'read',     'Hasta dosyası okuma'),
    ('clinical.patient.write',          'clinical',  'write',    'Hasta oluşturma/düzenleme'),
    ('clinical.patient.delete',         'clinical',  'delete',   'Hasta kaydını silme/birleştirme'),
    -- Clinical: appointment / visit
    ('clinical.appointment.read',       'clinical',  'read',     'Randevu okuma'),
    ('clinical.appointment.write',      'clinical',  'write',    'Randevu oluşturma/düzenleme'),
    ('clinical.visit.read',             'clinical',  'read',     'Muayene okuma'),
    ('clinical.visit.write',            'clinical',  'write',    'Muayene oluşturma'),
    -- Clinical: prescription
    ('clinical.prescription.read',      'clinical',  'read',     'Reçete okuma'),
    ('clinical.prescription.write',     'clinical',  'write',    'Reçete taslağı oluşturma'),
    ('clinical.prescription.sign',      'clinical',  'sign',     'Reçete imzalama'),
    -- Clinical: inpatient
    ('clinical.admission.read',         'clinical',  'read',     'Yatış okuma'),
    ('clinical.admission.write',        'clinical',  'write',    'Yatış kabul/transfer/taburcu'),
    ('clinical.vital_signs.write',      'clinical',  'write',    'Vital signs girişi'),
    -- Lab
    ('lab.order.read',                  'lab',       'read',     'Lab istek okuma'),
    ('lab.order.write',                 'lab',       'write',    'Lab istek oluşturma'),
    ('lab.result.write',                'lab',       'write',    'Lab sonuç girişi'),
    ('lab.result.verify',               'lab',       'verify',   'Lab sonuç onaylama'),
    -- Radiology
    ('radiology.order.read',            'radiology', 'read',     'Radyoloji istek okuma'),
    ('radiology.order.write',           'radiology', 'write',    'Radyoloji istek oluşturma'),
    ('radiology.report.sign',           'radiology', 'sign',     'Radyoloji raporu imzalama'),
    -- Pharmacy
    ('pharmacy.medication.read',        'pharmacy',  'read',     'İlaç kataloğu okuma'),
    ('pharmacy.prescription.dispense',  'pharmacy',  'write',    'Reçete karşılama'),
    ('pharmacy.stock.read',             'pharmacy',  'read',     'Eczane stoğu okuma'),
    ('pharmacy.stock.write',            'pharmacy',  'write',    'Eczane stok hareketi'),
    -- Inventory (depo)
    ('inventory.stock.read',            'inventory', 'read',     'Depo stoğu okuma'),
    ('inventory.stock.write',           'inventory', 'write',    'Depo stok hareketi'),
    ('inventory.purchase_order.write',  'inventory', 'write',    'Satınalma siparişi'),
    -- Finance: invoice / cashier / earnings
    ('finance.invoice.read',            'finance',   'read',     'Fatura okuma'),
    ('finance.invoice.write',           'finance',   'write',    'Fatura kesme'),
    ('finance.cashier.open',            'finance',   'write',    'Kasa açma'),
    ('finance.cashier.close',           'finance',   'write',    'Kasa kapama'),
    ('finance.payment.write',           'finance',   'write',    'Ödeme alma'),
    ('finance.cashier.refund',          'finance',   'write',    'İade işlemi (audit)'),
    ('finance.doctor_earning.read',     'finance',   'read',     'Hekim hakedişi okuma'),
    ('finance.doctor_earning.close',    'finance',   'write',    'Hakediş dönemi kapatma'),
    -- Reports
    ('reports.cash.read',               'reports',   'read',     'Kasa raporları'),
    ('reports.clinical.read',           'reports',   'read',     'Klinik raporlar'),
    ('reports.management.read',         'reports',   'read',     'Yönetsel KPI raporları'),
    -- Integration
    ('integration.medula.submit',       'integration','write',   'Medula SGK çağrısı yapma'),
    ('integration.mernis.verify',       'integration','write',   'MERNIS TC sorgulama');

COMMIT;
