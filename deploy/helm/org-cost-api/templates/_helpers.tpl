{{/*
Expand the name of the chart.
*/}}
{{- define "org-cost-api.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
*/}}
{{- define "org-cost-api.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.nameOverride }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}

{{- define "org-cost-api.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "org-cost-api.labels" -}}
helm.sh/chart: {{ include "org-cost-api.chart" . }}
{{ include "org-cost-api.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{- define "org-cost-api.selectorLabels" -}}
app.kubernetes.io/name: {{ include "org-cost-api.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{- define "org-cost-api.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "org-cost-api.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{- define "org-cost-api.configMapName" -}}
{{- printf "%s-config" (include "org-cost-api.fullname" .) }}
{{- end }}

{{- define "org-cost-api.chat.fullname" -}}
{{- printf "%s-chat" (include "org-cost-api.fullname" .) }}
{{- end }}

{{- define "org-cost-api.chat.selectorLabels" -}}
app.kubernetes.io/name: {{ include "org-cost-api.name" . }}-chat
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/component: chat
{{- end }}

{{- define "org-cost-api.mcp.fullname" -}}
{{- printf "%s-mcp" (include "org-cost-api.fullname" .) }}
{{- end }}

{{- define "org-cost-api.mcp.selectorLabels" -}}
app.kubernetes.io/name: {{ include "org-cost-api.name" . }}-mcp
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/component: mcp
{{- end }}
