package openapi

//go:generate go tool oapi-codegen -response-type-suffix Resp -package db_inspector -generate client,spec,types -o clients/db_inspector/client.go /tmp/liara-openapi/spec/database-inspector.yaml
//go:generate go tool oapi-codegen -response-type-suffix Resp -package dbaas -generate client,spec,types -o clients/dbaas/client.go /tmp/liara-openapi/spec/dbaas.yaml
//go:generate go tool oapi-codegen -response-type-suffix Resp -package dns -generate client,spec,types -o clients/dns/client.go /tmp/liara-openapi/spec/dns.yaml
//go:generate go tool oapi-codegen -response-type-suffix Resp -package file_browser -generate client,spec,types -o clients/file_browser/client.go /tmp/liara-openapi/spec/file-browser.yaml
//go:generate go tool oapi-codegen -response-type-suffix Resp -package mail -generate client,spec,types -o clients/mail/client.go /tmp/liara-openapi/spec/mail.yaml
//go:generate go tool oapi-codegen -response-type-suffix Resp -package object_storage -generate client,spec,types -o clients/object_storage/client.go /tmp/liara-openapi/spec/object-storage.yaml
//go:generate go tool oapi-codegen -response-type-suffix Resp -package paas -generate client,spec,types -o clients/paas/client.go /tmp/liara-openapi/spec/paas.yaml
