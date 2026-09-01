package migrations

import _ "embed"

//go:embed 000001_initial.up.sql
var Up string

//go:embed 000001_initial.down.sql
var Down string

const LatestVersion = 1
