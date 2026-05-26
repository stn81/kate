package codegen

import (
	"fmt"
	"strings"
)

// Table is the parsed representation of one DDL statement.
type Table struct {
	Schema  string // e.g. "hmct"
	Name    string // e.g. "dim_user_reg"
	Columns []Column
	IsView  bool
	// Comment is the optional table-level COMMENT '...' value.
	Comment string
}

// Column is one parsed column from a CREATE TABLE list.
type Column struct {
	Name     string // column name (unquoted)
	RawType  string // verbatim CK type expression, e.g. "Nullable(DateTime64(3))"
	GoType   string // resolved Go type, e.g. "*time.Time"
	GoImport string // optional extra import package required by GoType
	Comment  string // optional COMMENT 'xxx'
}

// Parse extracts a single CREATE TABLE or CREATE VIEW from src. It returns
// ErrNoStmt if neither is present.
func Parse(src string) (*Table, error) {
	// Strip line comments (-- ... \n) and block comments (/* ... */) first.
	src = stripComments(src)

	low := strings.ToLower(src)
	// Find CREATE TABLE / CREATE VIEW (handle MATERIALIZED / IF NOT EXISTS variants).
	tblIdx := indexOfStmt(low, "create table")
	viewIdx := indexOfStmt(low, "create view")
	mvIdx := indexOfStmt(low, "create materialized view")

	if tblIdx >= 0 && (viewIdx < 0 || tblIdx < viewIdx) {
		return parseTable(src[tblIdx:])
	}
	if mvIdx >= 0 {
		// Materialized views often carry types — treat as view (skip).
		name, schema := parseViewHead(src[mvIdx:])
		return &Table{Schema: schema, Name: name, IsView: true}, nil
	}
	if viewIdx >= 0 {
		name, schema := parseViewHead(src[viewIdx:])
		return &Table{Schema: schema, Name: name, IsView: true}, nil
	}
	return nil, fmt.Errorf("no CREATE TABLE / CREATE VIEW statement found")
}

// indexOfStmt finds the first occurrence of stmt in low that starts at a
// statement boundary (start-of-line or after whitespace), case-insensitive.
// Returns -1 if not found.
func indexOfStmt(low, stmt string) int {
	from := 0
	for {
		i := strings.Index(low[from:], stmt)
		if i < 0 {
			return -1
		}
		abs := from + i
		// boundary check: previous char must be whitespace or start-of-buf
		if abs == 0 || isSpace(low[abs-1]) {
			return abs
		}
		from = abs + len(stmt)
	}
}

func isSpace(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\r'
}

// stripComments removes -- line comments and /* ... */ block comments.
func stripComments(src string) string {
	var out strings.Builder
	i := 0
	for i < len(src) {
		// -- line comment
		if i+1 < len(src) && src[i] == '-' && src[i+1] == '-' {
			j := strings.IndexByte(src[i:], '\n')
			if j < 0 {
				return out.String()
			}
			i += j // skip up to (but not including) newline; loop will pick it up
			continue
		}
		// /* block */
		if i+1 < len(src) && src[i] == '/' && src[i+1] == '*' {
			j := strings.Index(src[i:], "*/")
			if j < 0 {
				return out.String()
			}
			i += j + 2
			continue
		}
		out.WriteByte(src[i])
		i++
	}
	return out.String()
}

// parseViewHead extracts (name, schema) from a CREATE [MATERIALIZED] VIEW
// statement head. Only the name is needed; we don't parse view bodies.
func parseViewHead(src string) (name, schema string) {
	// Skip "CREATE [MATERIALIZED] VIEW [IF NOT EXISTS]"
	tokens := tokenize(src)
	// Consume the leading "CREATE" / "MATERIALIZED" / "VIEW" tokens.
	idx := 0
	for idx < len(tokens) {
		t := strings.ToUpper(tokens[idx])
		if t == "VIEW" {
			idx++
			break
		}
		idx++
	}
	// Optional IF NOT EXISTS
	if idx+2 < len(tokens) && strings.EqualFold(tokens[idx], "IF") &&
		strings.EqualFold(tokens[idx+1], "NOT") && strings.EqualFold(tokens[idx+2], "EXISTS") {
		idx += 3
	}
	if idx >= len(tokens) {
		return "", ""
	}
	full := stripIdent(tokens[idx])
	if i := strings.IndexByte(full, '.'); i >= 0 {
		return full[i+1:], full[:i]
	}
	return full, ""
}

// parseTable handles a CREATE TABLE statement starting at src[0].
func parseTable(src string) (*Table, error) {
	// Find the head: "CREATE TABLE [IF NOT EXISTS] schema.name ("
	tokens := tokenize(src)
	idx := 0
	for idx < len(tokens) && !strings.EqualFold(tokens[idx], "TABLE") {
		idx++
	}
	idx++ // past TABLE
	if idx+2 < len(tokens) && strings.EqualFold(tokens[idx], "IF") &&
		strings.EqualFold(tokens[idx+1], "NOT") && strings.EqualFold(tokens[idx+2], "EXISTS") {
		idx += 3
	}
	if idx >= len(tokens) {
		return nil, fmt.Errorf("CREATE TABLE: unexpected end of input")
	}
	full := stripIdent(tokens[idx])
	var schema, name string
	if i := strings.IndexByte(full, '.'); i >= 0 {
		schema, name = full[:i], full[i+1:]
	} else {
		name = full
	}

	// Find the column list — slice the source between the matching
	// parens after the table name.
	openIdx := strings.IndexByte(src, '(')
	if openIdx < 0 {
		return nil, fmt.Errorf("CREATE TABLE %s: missing '('", name)
	}
	closeIdx := matchParen(src, openIdx)
	if closeIdx < 0 {
		return nil, fmt.Errorf("CREATE TABLE %s: unbalanced parens", name)
	}
	body := src[openIdx+1 : closeIdx]
	cols, err := parseColumnList(body)
	if err != nil {
		return nil, fmt.Errorf("CREATE TABLE %s: %w", name, err)
	}
	tbl := &Table{Schema: schema, Name: name, Columns: cols}
	// trailing comment
	if c := findTableComment(src[closeIdx:]); c != "" {
		tbl.Comment = c
	}
	return tbl, nil
}

// matchParen returns the index of the closing ')' that matches src[open].
// Respects nested parens; returns -1 on no match.
func matchParen(src string, open int) int {
	depth := 1
	inStr := byte(0)
	for i := open + 1; i < len(src); i++ {
		c := src[i]
		if inStr != 0 {
			if c == inStr && (i == 0 || src[i-1] != '\\') {
				inStr = 0
			}
			continue
		}
		switch c {
		case '\'', '"', '`':
			inStr = c
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

// parseColumnList parses the comma-separated entries inside the
// CREATE TABLE parens. Skips INDEX / CONSTRAINT / PROJECTION clauses.
func parseColumnList(body string) ([]Column, error) {
	parts := splitTopLevelCommas(body)
	out := make([]Column, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		up := strings.ToUpper(p)
		if strings.HasPrefix(up, "INDEX ") || strings.HasPrefix(up, "CONSTRAINT ") ||
			strings.HasPrefix(up, "PROJECTION ") || strings.HasPrefix(up, "PRIMARY ") {
			continue
		}
		col, err := parseColumn(p)
		if err != nil {
			return nil, err
		}
		out = append(out, col)
	}
	return out, nil
}

// splitTopLevelCommas splits body on commas that are not inside parens or
// quoted strings.
func splitTopLevelCommas(body string) []string {
	var parts []string
	var cur strings.Builder
	depth := 0
	inStr := byte(0)
	for i := 0; i < len(body); i++ {
		c := body[i]
		if inStr != 0 {
			cur.WriteByte(c)
			if c == inStr && (i == 0 || body[i-1] != '\\') {
				inStr = 0
			}
			continue
		}
		switch c {
		case '\'', '"', '`':
			inStr = c
			cur.WriteByte(c)
		case '(':
			depth++
			cur.WriteByte(c)
		case ')':
			depth--
			cur.WriteByte(c)
		case ',':
			if depth == 0 {
				parts = append(parts, cur.String())
				cur.Reset()
				continue
			}
			cur.WriteByte(c)
		default:
			cur.WriteByte(c)
		}
	}
	if cur.Len() > 0 {
		parts = append(parts, cur.String())
	}
	return parts
}

// parseColumn parses one column spec: "`name` Type [DEFAULT ...] [COMMENT '...']"
func parseColumn(spec string) (Column, error) {
	spec = strings.TrimSpace(spec)
	// name
	if len(spec) == 0 || spec[0] != '`' {
		return Column{}, fmt.Errorf("column does not start with `: %q", spec)
	}
	closeTick := strings.IndexByte(spec[1:], '`')
	if closeTick < 0 {
		return Column{}, fmt.Errorf("unterminated column name: %q", spec)
	}
	name := spec[1 : 1+closeTick]
	rest := strings.TrimSpace(spec[1+closeTick+1:])
	// type expression — read up to next whitespace at depth 0 or end.
	typeExpr, after := readTypeExpr(rest)
	if typeExpr == "" {
		return Column{}, fmt.Errorf("column %q has empty type", name)
	}
	// comment
	comment := extractColumnComment(after)
	goType, goImport := mapCKToGo(typeExpr)
	return Column{
		Name:     name,
		RawType:  typeExpr,
		GoType:   goType,
		GoImport: goImport,
		Comment:  comment,
	}, nil
}

// readTypeExpr reads a CK type expression respecting nested parens. Returns
// (typeExpr, remaining-after-type).
func readTypeExpr(s string) (string, string) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", ""
	}
	depth := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '(' {
			depth++
			continue
		}
		if c == ')' {
			depth--
			continue
		}
		if depth == 0 && isSpace(c) {
			return strings.TrimSpace(s[:i]), strings.TrimSpace(s[i:])
		}
	}
	return strings.TrimSpace(s), ""
}

// extractColumnComment finds COMMENT '...' in the trailing column spec.
func extractColumnComment(s string) string {
	up := strings.ToUpper(s)
	idx := strings.Index(up, "COMMENT ")
	if idx < 0 {
		return ""
	}
	rest := strings.TrimSpace(s[idx+len("COMMENT "):])
	if len(rest) == 0 || rest[0] != '\'' {
		return ""
	}
	end := strings.IndexByte(rest[1:], '\'')
	if end < 0 {
		return ""
	}
	return rest[1 : 1+end]
}

// findTableComment scans trailing DDL (after closing paren) for the
// table-level COMMENT '...' suffix.
func findTableComment(s string) string {
	up := strings.ToUpper(s)
	idx := strings.LastIndex(up, "COMMENT ")
	if idx < 0 {
		return ""
	}
	rest := strings.TrimSpace(s[idx+len("COMMENT "):])
	if len(rest) == 0 || rest[0] != '\'' {
		return ""
	}
	end := strings.IndexByte(rest[1:], '\'')
	if end < 0 {
		return ""
	}
	return rest[1 : 1+end]
}

// tokenize splits src on whitespace, treating "schema.name" / quoted
// identifiers as single tokens.
func tokenize(src string) []string {
	var tokens []string
	var cur strings.Builder
	inStr := byte(0)
	for i := 0; i < len(src); i++ {
		c := src[i]
		if inStr != 0 {
			cur.WriteByte(c)
			if c == inStr {
				inStr = 0
			}
			continue
		}
		switch {
		case c == '`' || c == '"' || c == '\'':
			inStr = c
			cur.WriteByte(c)
		case isSpace(c) || c == '(' || c == ',' || c == ';':
			if cur.Len() > 0 {
				tokens = append(tokens, cur.String())
				cur.Reset()
			}
		default:
			cur.WriteByte(c)
		}
	}
	if cur.Len() > 0 {
		tokens = append(tokens, cur.String())
	}
	return tokens
}

// stripIdent removes surrounding backticks / quotes from an identifier.
func stripIdent(s string) string {
	if len(s) >= 2 && (s[0] == '`' || s[0] == '"') {
		return s[1 : len(s)-1]
	}
	return s
}
