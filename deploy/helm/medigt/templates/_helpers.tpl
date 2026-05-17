{{- define "medigt.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "medigt.fullname" -}}
{{- if .Values.fullnameOverride -}}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- printf "%s-%s" .Release.Name (include "medigt.name" .) | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}

{{- define "medigt.labels" -}}
app.kubernetes.io/name: {{ include "medigt.name" . }}
helm.sh/chart: {{ .Chart.Name }}-{{ .Chart.Version | replace "+" "_" }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
medigt.com/tenant-id: {{ .Values.tenant.id | quote }}
{{- range $key, $value := .Values.tenant.labels }}
{{ $key }}: {{ $value | quote }}
{{- end }}
{{- end -}}

{{- define "medigt.serviceAccountName" -}}
{{- if .Values.serviceAccount.create -}}
{{- default (include "medigt.fullname" .) .Values.serviceAccount.name -}}
{{- else -}}
{{- default "default" .Values.serviceAccount.name -}}
{{- end -}}
{{- end -}}

{{- /*
  Database URL — explicit override wins; otherwise build from the in-chart
  Postgres StatefulSet's auth values. KVKK reality: production deployments
  almost always use an external managed Postgres so the override path is
  the common one. The fallback is for dev/onprem PoC clusters.
*/ -}}
{{- define "medigt.backendDatabaseUrl" -}}
{{- if .Values.backend.databaseUrl -}}
{{- .Values.backend.databaseUrl -}}
{{- else -}}
{{- printf "postgres://%s:%s@%s-postgres:%d/%s?sslmode=disable" .Values.postgres.auth.username .Values.postgres.auth.password (include "medigt.fullname" .) (int .Values.service.postgres.port) .Values.postgres.auth.database -}}
{{- end -}}
{{- end -}}
