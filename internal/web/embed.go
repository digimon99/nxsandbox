package web

import "embed"

//go:embed templates/*.html
var templatesFS embed.FS

//go:embed static/* static/css/*
var staticFS embed.FS
