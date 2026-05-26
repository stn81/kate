// Package codegen produces typed Go bindings for ClickHouse / ByteHouse
// tables described by DDL .sql files. Each table becomes a Go subpackage
// containing a Row struct plus a descriptor `T` with typed Col[T] values
// for every column.
//
// codegen is intentionally tiny: a minimal DDL tokenizer + a static type
// map. It does not try to be a full SQL parser — only the column-list
// portion of CREATE TABLE statements is consumed.
package codegen

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Config controls a kate-gen run.
type Config struct {
	InputDir  string // directory of .sql files
	OutputDir string // root of generated packages
	PkgPrefix string // prepended to each generated package name
	SkipViews bool   // ignore CREATE VIEW (recommended)
}

// Run executes codegen end-to-end: scan input, parse each DDL, emit files.
func Run(cfg Config) error {
	if err := os.MkdirAll(cfg.OutputDir, 0o755); err != nil {
		return fmt.Errorf("mkdir output: %w", err)
	}
	entries, err := os.ReadDir(cfg.InputDir)
	if err != nil {
		return fmt.Errorf("read input dir: %w", err)
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		path := filepath.Join(cfg.InputDir, e.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read %s: %w", path, err)
		}
		tbl, err := Parse(string(data))
		if err != nil {
			// Skip parse errors with a warning; partial DDL or view-only
			// files shouldn't fail the whole run.
			fmt.Fprintf(os.Stderr, "kate-gen: %s: %v (skipped)\n", e.Name(), err)
			continue
		}
		if tbl.IsView && cfg.SkipViews {
			continue
		}
		if tbl.IsView {
			// Views are skipped for now — see SkipViews godoc.
			continue
		}
		pkgName := goPackageName(cfg.PkgPrefix, tbl.Name)
		pkgDir := filepath.Join(cfg.OutputDir, tbl.Name)
		if err := os.MkdirAll(pkgDir, 0o755); err != nil {
			return fmt.Errorf("mkdir %s: %w", pkgDir, err)
		}
		outPath := filepath.Join(pkgDir, tbl.Name+".go")
		src, err := Emit(tbl, pkgName)
		if err != nil {
			return fmt.Errorf("emit %s: %w", tbl.Name, err)
		}
		if err := os.WriteFile(outPath, []byte(src), 0o644); err != nil {
			return fmt.Errorf("write %s: %w", outPath, err)
		}
		fmt.Printf("kate-gen: %s → %s (%d cols)\n", e.Name(), outPath, len(tbl.Columns))
	}
	return nil
}

// goPackageName builds a valid Go package name from the table name plus
// an optional prefix.
func goPackageName(prefix, tableName string) string {
	// Replace any character that isn't [a-z0-9_] with _.
	var b strings.Builder
	b.WriteString(prefix)
	for _, r := range tableName {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '_':
			b.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r + ('a' - 'A'))
		default:
			b.WriteByte('_')
		}
	}
	return b.String()
}
