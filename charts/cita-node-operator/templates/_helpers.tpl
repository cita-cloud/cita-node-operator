{{/*
Expand the name of the chart.
*/}}
{{- define "cita-node-operator.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}


{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "cita-node-operator.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "cita-node-operator.labels" -}}
helm.sh/chart: {{ include "cita-node-operator.chart" . }}
{{ include "cita-node-operator.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "cita-node-operator.selectorLabels" -}}
app.kubernetes.io/name: {{ include "cita-node-operator.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}
