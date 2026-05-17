-- 025_pacs_image_reference: DICOM görüntü referansları + PACS metadata.
--
-- Tek bir radiology_order altında 1+ DICOM "study" (genelde 1), her
-- study altında 1+ "series" (CT/MR'da onlarca), her series altında
-- 1+ "instance" olur. UI'da PACS viewer'a (OHIF / Weasis) açılacak
-- UID'leri burada saklarız. Gerçek görüntü baytları bizim DB'ye değil
-- PACS sunucusuna (Orthanc) gider.
--
-- Mock geliştirme: pacs/client.go order yaratıldığında deterministik
-- bir study UID üretip image_reference satırı yazar. Üretimde Orthanc
-- DICOM-Web GET çağrıları ile gerçek study metadata'sı çekilir.

BEGIN;

CREATE TABLE image_reference (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id     UUID NOT NULL REFERENCES organization(id) ON DELETE CASCADE,
    branch_id           UUID NOT NULL REFERENCES branch(id) ON DELETE CASCADE,
    radiology_order_id  UUID REFERENCES radiology_order(id) ON DELETE CASCADE,
    patient_id          UUID NOT NULL REFERENCES patient(id) ON DELETE RESTRICT,

    -- DICOM identifiers (resmi format: dotted decimal OID)
    study_instance_uid  TEXT NOT NULL,
    series_instance_uid TEXT,
    modality            TEXT NOT NULL,                  -- CR / CT / MR / US / DX / MG / NM / OT
    study_date          TIMESTAMPTZ,
    description         TEXT,
    instance_count      INTEGER NOT NULL DEFAULT 0,

    -- PACS endpoint pointer (Orthanc URL temeli — UI doğrudan iframe kurar)
    pacs_base_url       TEXT,
    thumbnail_url       TEXT,
    -- Yüksek çözünürlüklü ZIP / .dcm dosya download URL'i (op. operatör)
    download_url        TEXT,

    -- PACS submit/sync state
    submitted_at        TIMESTAMPTZ,
    last_synced_at      TIMESTAMPTZ,
    sync_error          TEXT,

    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    UNIQUE (organization_id, study_instance_uid, series_instance_uid)
);

CREATE INDEX idx_image_ref_order ON image_reference(radiology_order_id) WHERE radiology_order_id IS NOT NULL;
CREATE INDEX idx_image_ref_patient ON image_reference(patient_id, study_date DESC);
CREATE INDEX idx_image_ref_branch ON image_reference(branch_id, study_date DESC);

CREATE TRIGGER trg_image_ref_updated_at BEFORE UPDATE ON image_reference
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

COMMIT;
