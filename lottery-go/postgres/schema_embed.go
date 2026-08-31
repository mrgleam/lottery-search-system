package postgres

import _ "embed"

// Schema is the DDL, compiled into the binary so deployment is one file.
//
//go:embed schema.sql
var Schema string
