// The unexport command unexports exported identifiers which are not imported
// by any other Go code.
package main

import (
	"flag"
	"fmt"
	"go/ast"
	"go/build"
	"go/parser"
	"go/token"
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

	log.SetPrefix("unexport: ")
	flag.Usage = Usage
	flag.Parse()

	identifiers := strings.Split(*flagIdentifier, ",")
	fmt.Printf("identifiers = %+v\n", identifiers)

	args := flag.Args()
	pkg, err := build.Import(args[0], "", build.ImportComment)
	if err != nil {
		log.Fatalf("%s", err)
	}
	parsePackage(pkg)

}

func parsePackage(pkg *build.Package) {
	fs := token.NewFileSet()

	include := func(info os.FileInfo) bool {
		for _, name := range pkg.GoFiles {
			if name == info.Name() {
				return true
			}
		}
		for _, name := range pkg.CgoFiles {
			if name == info.Name() {
				return true
			}
		}
		return false
	}

	pkgs, err := parser.ParseDir(fs, pkg.Dir, include, parser.ParseComments)
	if err != nil {
		log.Fatal(err)
	}

	// Make sure they are all in one package.
	if len(pkgs) != 1 {
		log.Fatalf("multiple packages in directory %s", pkg.Dir)
	}

	astPkg := pkgs[pkg.Name]

	for _, f := range astPkg.Files {
		ast.FileExports(f)
		for _, decl := range f.Decls {
			switch d := decl.(type) {
			case *ast.GenDecl:
				switch d.Tok {
				case token.IMPORT:
				case token.CONST:
					for _, spec := range d.Specs {
						if v, ok := spec.(*ast.ValueSpec); ok {
							fmt.Println("Const:", v.Names)
						}
					}
				case token.VAR:
					for _, spec := range d.Specs {
						if v, ok := spec.(*ast.ValueSpec); ok {
							fmt.Println("Var:", v.Names)
						}
					}
				case token.TYPE:
					for _, spec := range d.Specs {
						if s, ok := spec.(*ast.TypeSpec); ok {
							fmt.Println("Type:", s.Name.Name)
							switch t := s.Type.(type) {
							case *ast.StructType:
								for _, l := range t.Fields.List {
									fmt.Printf("\tField: %+v\n", l.Names)
								}
							}
						}
					}
				}
			case *ast.FuncDecl:
				// methods might bound to unexported types, show only if those
				// types are exported too
				if d.Recv != nil {
					for _, l := range d.Recv.List {
						for _, n := range l.Names {
							if ast.IsExported(n.Name) {
								fmt.Printf("Func: %s\n", d.Name.Name)
							}
						}
					}
				} else {
					fmt.Printf("Func: %s\n", d.Name.Name)
				}
			}
		}
	}
}
