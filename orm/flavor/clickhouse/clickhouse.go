// Package clickhouse provides the ClickHouse / ByteHouse flavor implementation.
package clickhouse

// Flavor is the package-level singleton. Pass it to db.Config.Flavor.
var Flavor = chFlavor{}

type chFlavor struct{}

func (chFlavor) Name() string         { return "ClickHouse" }
func (chFlavor) IsClickHouse()        {}
func (chFlavor) SupportsCTE() bool    { return true }
func (chFlavor) SupportsReturning() bool { return false }

// Quote wraps an identifier in backticks. CK also accepts double quotes but
// backticks are universal across MergeTree dialects and ByteHouse, and we
// match MySQL for visual consistency in mixed environments.
func (chFlavor) Quote(ident string) string {
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
			// CK supports backslash-escape or doubling; the driver accepts both.
			out = append(out, '`', '`')
			continue
		}
		out = append(out, c)
	}
	out = append(out, '`')
	return string(out)
}

// Placeholder returns `?`. clickhouse-go v2's stdlib facade rewrites these
// into the native protocol internally, so we use the MySQL-style placeholder
// for emit simplicity.
func (chFlavor) Placeholder(i int) string { return "?" }
