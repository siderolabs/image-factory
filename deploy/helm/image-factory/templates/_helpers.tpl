{{/*
Expand the name of the chart.
*/}}
{{- define "imageFactory.name" -}}
    {{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "imageFactory.fullname" -}}
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

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "imageFactory.chart" -}}
    {{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "imageFactory.labels" -}}
helm.sh/chart: {{ include "imageFactory.chart" . }}
{{ include "imageFactory.selectorLabels" . }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "imageFactory.selectorLabels" -}}
app.kubernetes.io/name: {{ include "imageFactory.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "imageFactory.serviceAccountName" -}}
    {{- if .Values.serviceAccount.create }}
        {{- default (include "imageFactory.fullname" .) .Values.serviceAccount.name }}
    {{- else }}
        {{- default "default" .Values.serviceAccount.name }}
    {{- end }}
{{- end }}

{{/*
Create secret name used for configuring imageFactory.
*/}}
{{- define "imageFactory.secret" -}}
    {{- default (printf "%s" (include "imageFactory.fullname" .)) .Values.secret.name }}
{{- end }}

{{/*
Create TLS secret name used for configuring imageFactory.
*/}}
{{- define "imageFactory.secretTls" -}}
    {{- default (printf "%s-tls" (include "imageFactory.fullname" .)) .Values.secret.name }}
{{- end }}

{{/*
Create PXE ingress name used for configuring imageFactory.
*/}}
{{- define "imageFactory.ingressPxe" -}}
    {{- default (printf "%s-pxe" (include "imageFactory.fullname" .)) .Values.secret.name }}
{{- end }}