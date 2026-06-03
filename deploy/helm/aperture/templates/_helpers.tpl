{{- define "aperture.fullname" -}}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "aperture.labels" }}
app.kubernetes.io/name: aperture
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/version: {{ .Chart.AppVersion }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
helm.sh/chart: {{ .Chart.Name }}-{{ .Chart.Version }}
{{- end }}

{{- define "aperture.selectorLabels" }}
app.kubernetes.io/name: aperture
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}
