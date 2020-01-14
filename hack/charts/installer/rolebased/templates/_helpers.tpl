{{/* vim: set filetype=mustache: */}}
{{/*
Create a image repository and tag name
*/}}
{{- define "controller.image" -}}
{{- printf "%s:%s" .Values.image.repository .Values.image.tag -}}
{{- end -}}