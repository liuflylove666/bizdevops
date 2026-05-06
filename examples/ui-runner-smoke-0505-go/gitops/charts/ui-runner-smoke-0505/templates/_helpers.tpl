{{- define "ui-runner-smoke-0505.name" -}}
ui-runner-smoke-0505
{{- end -}}

{{- define "ui-runner-smoke-0505.labels" -}}
app: {{ include "ui-runner-smoke-0505.name" . }}
app.kubernetes.io/name: {{ include "ui-runner-smoke-0505.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end -}}

{{- define "ui-runner-smoke-0505.selectorLabels" -}}
app: {{ include "ui-runner-smoke-0505.name" . }}
{{- end -}}
