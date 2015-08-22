// The unexport command unexports exported identifiers which are not imported
// by any other Go code.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
)

// Usage is a replacement usage function for the flags package.
func Usage() {
	fmt.Fprintf(os.Stderr, "Usage of %s:\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "\tunexport [flags] -identifier T [packages]\n")
	fmt.Fprintf(os.Stderr, "Flags:\n")
	flag.PrintDefaults()
}

func main() {
	var (
		flagIdentifier = flag.String("identifier", "", "comma-separated list of identifiers names; if empty all identifiers are unexported")
	)

	log.SetPrefix("stringer: ")
	flag.Usage = Usage
	flag.Parse()

	identifiers := strings.Split(*flagIdentifier, ",")
	fmt.Printf("identifiers = %+v\n", identifiers)
}
