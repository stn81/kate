// Command kate-gen reads ClickHouse / ByteHouse DDL files (CREATE TABLE)
// and emits Go source files containing typed Row structs and Col[T]
// descriptors for use with kate/orm.
//
// Usage:
//
//	kate-gen \
//	    -in  ./catalog/bytehouse  \  # input directory of .sql files
//	    -out ./gen/db                 # output directory; one subpackage per table
//	    [-pkg-prefix tbl_]            # optional Go package name prefix
//	    [-skip-views=true]            # skip CREATE VIEW (recommended; views need
//	                                  # column types inferred from sources)
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/stn81/kate/orm/codegen"
)

func main() {
	var (
		in        = flag.String("in", "", "input directory of .sql DDL files")
		out       = flag.String("out", "", "output directory (one subpackage per table)")
		pkgPrefix = flag.String("pkg-prefix", "", "optional prefix for generated Go package names")
		skipViews = flag.Bool("skip-views", true, "skip CREATE VIEW statements")
	)
	flag.Parse()
	if *in == "" || *out == "" {
		flag.Usage()
		os.Exit(2)
	}
	cfg := codegen.Config{
		InputDir:   *in,
		OutputDir:  *out,
		PkgPrefix:  *pkgPrefix,
		SkipViews:  *skipViews,
	}
	if err := codegen.Run(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "kate-gen: %v\n", err)
		os.Exit(1)
	}
}
