// Package postgres provides the PostgreSQL flavor implementation.
package postgres

import "strconv"

// Flavor is the package-level singleton. Pass it to db.Config.Flavor.
var Flavor = pgFlavor{}

type pgFlavor struct{}

func (pgFlavor) Name() string { return "PostgreSQL" }

// Quote wraps an identifier in double quotes. Embedded double quotes are
// doubled per PG convention.
func (pgFlavor) Quote(ident string) string {
	for i := 0; i < len(ident); i++ {
		if ident[i] == '"' {
			return slowQuote(ident)
		}
	}
	return `"` + ident + `"`
}

func slowQuote(ident string) string {
	out := make([]byte, 0, len(ident)+2)
	out = append(out, '"')
	for i := 0; i < len(ident); i++ {
		c := ident[i]
		if c == '"' {
			out = append(out, '"', '"')
			continue
		}
		out = append(out, c)
	}
	out = append(out, '"')
	return string(out)
}

// Placeholder returns the PG numbered placeholder `$N`.
func (pgFlavor) Placeholder(i int) string { return "$" + strconv.Itoa(i) }

func (pgFlavor) SupportsCTE() bool       { return true }
func (pgFlavor) SupportsReturning() bool { return true }
