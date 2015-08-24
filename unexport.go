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

	"golang.org/x/tools/go/loader"
	"golang.org/x/tools/go/types"
	"golang.org/x/tools/refactor/importgraph"
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

	if err := runMain(args[0]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// runMain runs the actual command. It's an helper function so we can easily
// calls defers or return errors.
func runMain(path string) error {
	ctxt := &build.Default
	prog, err := loadProgram(ctxt, map[string]bool{path: true})
	if err != nil {
		return err
	}

	_, rev, errors := importgraph.Build(ctxt)
	if len(errors) > 0 {
		// With a large GOPATH tree, errors are inevitable.
		// Report them but proceed.
		fmt.Fprintf(os.Stderr, "While scanning Go workspace:\n")
		for path, err := range errors {
			fmt.Fprintf(os.Stderr, "Package %q: %s.\n", path, err)
		}
	}

	// Enumerate the set of potentially affected packages.
	possiblePackages := make(map[string]bool)
	for _, obj := range findExportedObjects(prog, path) {
		// External test packages are never imported,
		// so they will never appear in the graph.
		for path := range rev.Search(obj.Pkg().Path()) {
			possiblePackages[path] = true
		}
	}

	fmt.Println("Possible affected packages:")
	for pkg := range possiblePackages {
		fmt.Println("\t", pkg)
	}

	// reload the program with all possible packages to fetch the packageinfo's
	globalProg, err := loadProgram(ctxt, possiblePackages)
	if err != nil {
		return err
	}

	objsToUpdate := make(map[types.Object]bool, 0)
	objects := findExportedObjects(globalProg, path)

	fmt.Println("Exported identififers are:")
	for _, obj := range objects {
		fmt.Println("\t", obj)
	}

	for _, info := range globalProg.Imported {
		safeObjects := filterObjects(info, objects)
		for _, obj := range safeObjects {
			objsToUpdate[obj] = true
		}
	}

	fmt.Println("Safe to unexport identifiers are:")
	for obj := range objsToUpdate {
		fmt.Println("\t", obj)
	}

	var nidents int
	var filesToUpdate = make(map[*token.File]bool)
	for _, info := range globalProg.Imported {
		for id, obj := range info.Defs {
			if objsToUpdate[obj] {
				nidents++
				id.Name = strings.ToLower(obj.Name())
				filesToUpdate[globalProg.Fset.File(id.Pos())] = true
			}
		}
		for id, obj := range info.Uses {
			if objsToUpdate[obj] {
				nidents++
				id.Name = strings.ToLower(obj.Name())
				filesToUpdate[globalProg.Fset.File(id.Pos())] = true
			}
		}
	}

	return nil
}

// filterObjects filters the given objects and returns objects which are not in use by the given info package
func filterObjects(info *loader.PackageInfo, exported map[*ast.Ident]types.Object) map[*ast.Ident]types.Object {
	filtered := make(map[*ast.Ident]types.Object, 0)
	for id, ex := range exported {
		if !hasUse(info, ex) {
			filtered[id] = ex
		}
	}

	return filtered
}

// hasUse returns true if the given obj is part of the use in info
func hasUse(info *loader.PackageInfo, obj types.Object) bool {
	for _, o := range info.Uses {
		if o == obj {
			return true
		}
	}
	return false
}

// exportedObjects returns objects which are exported only
func exportedObjects(info *loader.PackageInfo) map[*ast.Ident]types.Object {
	objects := make(map[*ast.Ident]types.Object, 0)
	for id, obj := range info.Defs {
		if obj == nil {
			continue
		}

		if obj.Exported() {
			objects[id] = obj
		}
	}

	return objects
}

func findExportedObjects(prog *loader.Program, path string) map[*ast.Ident]types.Object {
	var pkgObj *types.Package
	for pkg := range prog.AllPackages {
		if pkg.Path() == path {
			pkgObj = pkg
			break
		}
	}

	info := prog.AllPackages[pkgObj]
	return exportedObjects(info)
}

func loadProgram(ctxt *build.Context, pkgs map[string]bool) (*loader.Program, error) {
	conf := loader.Config{
		Build:       ctxt,
		ParserMode:  parser.ParseComments,
		AllowErrors: false,
	}

	for pkg := range pkgs {
		conf.ImportWithTests(pkg)
	}
	return conf.Load()
}
