// Command scaffold generates or removes feature modules and CLI commands.
//
// Usage:
//
//	make feature NAME=product
//	make feature-remove NAME=product
//	make command NAME=health-check SIGNATURE=health:check
//	make command NAME=list-users SIGNATURE=user:list FEATURE=user
package main

import (
	"flag"
	"fmt"
	"path/filepath"
	"regexp"

	"github.com/jungo-dev/junkit/console"
	"github.com/jungo-dev/junkit/scaffold"
)

const (
	moduleName    = "jungo"
	featuresDir   = "internal/features"
	fxFile        = "internal/app/fx.go"
	featureMarker = "// FEATURES"
	migrationsDir = "internal/database/migrations"
	queriesDir    = "internal/database/queries"
	sqlcDir       = "internal/database/sqlc"
)

var featureNamePattern = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

func main() {
	kind := flag.String("type", "feature", "\"feature\" or \"command\"")
	name := flag.String("name", "", "Name in snake_case (required)")
	mode := flag.String("mode", "create", "\"create\" or \"remove\" (feature only)")
	table := flag.String("table", "", "Override the generated table name (feature only, default: \"<name>s\")")
	signature := flag.String("signature", "", "CLI signature, e.g. \"user:list\" (command only, required)")
	feature := flag.String("feature", "", "Attach the command to this existing feature instead of internal/console/commands (command only)")
	flag.Parse()

	switch *kind {
	case "feature":
		runFeatureScaffold(*name, *mode, *table)
	case "command":
		runCommandScaffold(*name, *signature, *feature)
	default:
		console.Fatalf("unknown -type %q: use \"feature\" or \"command\"", *kind)
	}
}

func runFeatureScaffold(name, mode, table string) {
	if name == "" {
		console.Fatalf("feature name is required. Usage: make feature NAME=<feature_name>")
	}
	if !featureNamePattern.MatchString(name) {
		console.Fatalf("invalid feature name %q: must be snake_case (lowercase letters, digits, underscores; must start with a letter)", name)
	}

	data := scaffold.NewFeatureData(name, moduleName, table)
	importPath := fmt.Sprintf("%s/internal/features/%s", moduleName, data.Name)
	moduleLine := fmt.Sprintf("%s.Module,", data.Name)

	switch mode {
	case "create":
		runCreate(data, importPath, moduleLine)
	case "remove":
		runRemove(data, importPath, moduleLine)
	default:
		console.Fatalf("unknown mode %q: use \"create\" or \"remove\"", mode)
	}
}

func runCreate(data scaffold.FeatureData, importPath, moduleLine string) {
	featureDir := filepath.Join(featuresDir, data.Name)

	seq, err := scaffold.NextMigrationSeq(migrationsDir)
	if err != nil {
		console.Fatalf("compute next migration sequence: %v", err)
	}

	console.Infof("Generating feature %q (table %q, migration %s)...", data.Name, data.Table, seq)

	cfg := scaffold.GenerateConfig{
		Data:       data,
		Files:      featureFiles(data, seq),
		FxFile:     fxFile,
		ImportPath: importPath,
		Marker:     featureMarker,
		ModuleLine: moduleLine,
	}

	if err := scaffold.Generate(cfg); err != nil {
		console.Fatalf("generate feature %q: %v", data.Name, err)
	}

	console.Successf("Feature %q generated in %s", data.Name, featureDir)
	console.Infof("Next: make migrate-up   (creates the %s table)", data.Table)
	console.Infof("Then: make sqlc         (generates internal/database/sqlc/%s.sql.go)", data.Name)
	console.Infof("Then: go build ./...    (verify everything compiles)")
}

func runRemove(data scaffold.FeatureData, importPath, moduleLine string) {
	cfg := scaffold.RemoveConfig{
		Dirs: []string{filepath.Join(featuresDir, data.Name)},
		ExtraFiles: []string{
			filepath.Join(sqlcDir, data.Name+".sql.go"),
			filepath.Join(queriesDir, data.Name+".sql"),
		},
		FxFile:     fxFile,
		ImportPath: importPath,
		ModuleLine: moduleLine,
	}

	if err := scaffold.Remove(cfg); err != nil {
		console.Fatalf("remove feature %q: %v", data.Name, err)
	}

	console.Warnf("migration files under %s were left in place — dropping a table is destructive, so roll it back yourself with `make migrate-down` and delete the files if you're sure", migrationsDir)
	console.Successf("Feature %q removed", data.Name)
}
