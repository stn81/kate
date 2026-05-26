// Package mysql provides the MySQL flavor implementation.
package mysql

// Flavor is the package-level singleton. Pass it to db.Config.Flavor.
var Flavor = mysqlFlavor{}

type mysqlFlavor struct{}

func (mysqlFlavor) Name() string { return "MySQL" }

// Quote wraps an identifier in backticks. We escape embedded backticks by
// doubling per MySQL convention.
func (mysqlFlavor) Quote(ident string) string {
	// fast path: no backtick inside
	for i := 0; i < len(ident); i++ {
		if ident[i] == '`' {
			return slowQuote(ident)
		}
	}
	return "`" + ident + "`"
}

func slowQuote(ident string) string {
	out := make([]byte, 0, len(ident)+2)
	out = append(out, '`')
	for i := 0; i < len(ident); i++ {
		c := ident[i]
		if c == '`' {
			out = append(out, '`', '`')
			continue
		}
		out = append(out, c)
	}
	out = append(out, '`')
	return string(out)
}

// Placeholder returns the MySQL placeholder. MySQL uses `?` for all
// positional parameters regardless of index.
func (mysqlFlavor) Placeholder(i int) string { return "?" }

func (mysqlFlavor) SupportsCTE() bool       { return true } // MySQL 8.0+
func (mysqlFlavor) SupportsReturning() bool { return false }
