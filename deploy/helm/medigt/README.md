# MediGt HBYS Helm Chart

Production deployment chart for MediGt — backend (Go API) + web frontend
(Next.js) + optional in-chart Postgres + ingress / OpenShift Route.

## Quick start

```bash
# Dev / single-node — uses the in-chart Postgres.
helm install medigt deploy/helm/medigt

# OpenShift — Routes + external Postgres.
helm install medigt deploy/helm/medigt -f deploy/helm/medigt/values-openshift.yaml

# Hospital on-prem — k8s Ingress + in-chart PG + 500Gi PVC for uploads.
helm install medigt deploy/helm/medigt -f deploy/helm/medigt/values-onprem.yaml
```

## What you need to override

At minimum:

```yaml
backend:
  jwtSecret: <32+ char random>
  fieldEncryptionKey: <32+ char random>
  appUrl: https://medigt.example.com
postgres:
  auth:
    password: <strong random>
ingress:   # or `route` on OpenShift
  enabled: true
  host: medigt.example.com
```

For SGK / e-Nabız / TURKKEP production swap-in, set the matching `baseUrl`
+ credential fields under `backend.medula` / `backend.enabiz` /
`backend.turkkep` / `backend.mernis`. Empty values keep the mock client.

## Layout

```
templates/
  resources.yaml   # ServiceAccount, Secret, ConfigMap, Deployments,
                   # Services, PVC, StatefulSet (Postgres), Ingress,
                   # Routes (OCP), NetworkPolicy, ResourceQuota,
                   # LimitRange, PDBs, HPAs, ServiceMonitor.
  _helpers.tpl     # name/fullname/labels/serviceAccountName/databaseUrl
  NOTES.txt        # printed after `helm install` — first-login pointer
                   # + pre-cert checklist
```

Values files:

- `values.yaml`           — base defaults (dev-friendly)
- `values-openshift.yaml` — OCP overrides (Routes + external PG + HPA)
- `values-onprem.yaml`    — hospital LAN (Ingress + in-chart PG + 500Gi PVC)

## KVKK ile uyumlu pratikler

- `backend.fieldEncryptionKey` — TC, password, TOTP secret encryption-at-rest
- `backend.auditRetentionDays: 3650` — 10 yıl (zorunlu)
- `backend.storage.s3Region: eu-central-1` — Türkiye / AB içi
- `backend.storage.localUploadBaseUrl` — yurt dışına çıkmasın
- KVKK denetimi öncesi `medigt audit-log export` çıktısı hazır olmalı
  (audit-log viewer UI + endpoint hazır)
