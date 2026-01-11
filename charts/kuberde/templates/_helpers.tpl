{{/*
Expand the name of the chart.
*/}}
{{- define "kuberde.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
*/}}
{{- define "kuberde.fullname" -}}
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
{{- define "kuberde.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "kuberde.labels" -}}
helm.sh/chart: {{ include "kuberde.chart" . }}
{{ include "kuberde.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "kuberde.selectorLabels" -}}
app.kubernetes.io/name: {{ include "kuberde.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "kuberde.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "kuberde.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Create the name of the operator service account to use
*/}}
{{- define "kuberde.operator.serviceAccountName" -}}
{{- if .Values.operator.serviceAccount.create }}
{{- default (printf "%s-operator" (include "kuberde.fullname" .)) .Values.operator.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.operator.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Server component labels
*/}}
{{- define "kuberde.server.labels" -}}
{{ include "kuberde.labels" . }}
app.kubernetes.io/component: server
{{- end }}

{{/*
Server selector labels
*/}}
{{- define "kuberde.server.selectorLabels" -}}
app.kubernetes.io/name: kuberde-server
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/component: server
{{- end }}

{{/*
Operator component labels
*/}}
{{- define "kuberde.operator.labels" -}}
{{ include "kuberde.labels" . }}
app.kubernetes.io/component: operator
{{- end }}

{{/*
Operator selector labels
*/}}
{{- define "kuberde.operator.selectorLabels" -}}
app.kubernetes.io/name: kuberde-operator
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/component: operator
{{- end }}

{{/*
Web component labels
*/}}
{{- define "kuberde.web.labels" -}}
{{ include "kuberde.labels" . }}
app.kubernetes.io/component: web
{{- end }}

{{/*
Web selector labels
*/}}
{{- define "kuberde.web.selectorLabels" -}}
app.kubernetes.io/name: kuberde-web
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/component: web
{{- end }}

{{/*
Keycloak component labels
*/}}
{{- define "kuberde.keycloak.labels" -}}
{{ include "kuberde.labels" . }}
app.kubernetes.io/component: keycloak
{{- end }}

{{/*
Keycloak selector labels
*/}}
{{- define "kuberde.keycloak.selectorLabels" -}}
app.kubernetes.io/name: kuberde-keycloak
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/component: keycloak
{{- end }}

{{/*
PostgreSQL component labels
*/}}
{{- define "kuberde.postgresql.labels" -}}
{{ include "kuberde.labels" . }}
app.kubernetes.io/component: postgresql
{{- end }}

{{/*
PostgreSQL selector labels
*/}}
{{- define "kuberde.postgresql.selectorLabels" -}}
app.kubernetes.io/name: kuberde-postgresql
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/component: postgresql
{{- end }}

{{/*
Return the proper image name
*/}}
{{- define "kuberde.image" -}}
{{- $registryName := .Values.image.registry -}}
{{- $repositoryName := .Values.image.repository -}}
{{- $componentImage := .componentImage -}}
{{- $tag := .Values.image.tag | default "latest" -}}
{{- if $registryName }}
{{- printf "%s/%s/%s:%s" $registryName $repositoryName $componentImage $tag -}}
{{- else }}
{{- printf "%s/%s:%s" $repositoryName $componentImage $tag -}}
{{- end }}
{{- end }}

{{/*
Return the appropriate apiVersion for RBAC
*/}}
{{- define "rbac.apiVersion" -}}
{{- if .Capabilities.APIVersions.Has "rbac.authorization.k8s.io/v1" }}
{{- print "rbac.authorization.k8s.io/v1" }}
{{- else }}
{{- print "rbac.authorization.k8s.io/v1beta1" }}
{{- end }}
{{- end }}

{{/*
Return the appropriate apiVersion for Ingress
*/}}
{{- define "ingress.apiVersion" -}}
{{- if .Capabilities.APIVersions.Has "networking.k8s.io/v1" }}
{{- print "networking.k8s.io/v1" }}
{{- else if .Capabilities.APIVersions.Has "networking.k8s.io/v1beta1" }}
{{- print "networking.k8s.io/v1beta1" }}
{{- else }}
{{- print "extensions/v1beta1" }}
{{- end }}
{{- end }}
