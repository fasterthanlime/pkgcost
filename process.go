package main

import (
	"fmt"
	"go/parser"
	"go/token"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/kisielk/gotool"
	"github.com/pkg/errors"
)

func process(args []string) (*PkgInfo, error) {
	infos := make(map[string]*PkgInfo)
	rootInfo := &PkgInfo{
		ImportPath: fmt.Sprintf("glob %s", strings.Join(args, " ")),
	}

	for _, importPath := range gotool.ImportPaths(args) {
		log.Printf("Entry point: %s", importPath)
		pkgInfo, err := getInfo(importPath)
		if err != nil {
			return nil, err
		}
		rootInfo.Imports = append(rootInfo.Imports, pkgInfo.ImportPath)
	}

	infos[rootInfo.ImportPath] = rootInfo

	goroot := os.Getenv("GOROOT")
	if goroot == "" {
		if runtime.GOOS == "windows" {
			goroot = `C:\Go`
		} else {
			goroot = "/usr/local/go"
		}

		_, err := os.Stat(goroot)
		if err != nil {
			if err != nil {
				return nil, errors.Errorf("(%s) does not exist, please set $GOROOT", goroot)
			}
		}
	}

	rootcache := make(map[string]bool)

	isRoot := func(dep string) bool {
		if v, ok := rootcache[dep]; ok {
			return v
		}

		rootFolder := filepath.Join(goroot, "src", filepath.FromSlash(dep))
		_, err := os.Stat(rootFolder)
		rootcache[dep] = (err == nil)
		return rootcache[dep]
	}

	var walk func(info *PkgInfo) error
	walk = func(info *PkgInfo) error {
		for _, dep := range info.Imports {
			if isRoot(dep) {
				continue
			}

			if dep == "C" {
				continue
			}

			importedInfo, walked := infos[dep]
			if !walked {
				var err error
				importedInfo, err = getInfo(dep)
				if err != nil {
					return errors.WithStack(err)
				}
				infos[dep] = importedInfo
				walk(importedInfo)
			}
			info.ImportedPkgs = append(info.ImportedPkgs, importedInfo)
		}
		return nil
	}

	err := walk(rootInfo)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	for _, info := range infos {
		if info.Goroot {
			continue
		}

		info.Stats.Files = int64(len(info.GoFiles))

		if info.Dir != "" {
			processFile := func(fpath string) error {
				stat, err := os.Stat(fpath)
				if err != nil {
					return errors.WithMessage(err, "while getting file size")
				}
				info.Stats.Size += stat.Size()

				// cOutput, err := exec.Command("gocyclo", fpath).Output()
				// if err != nil {
				// 	return errors.WithMessage(err, "while running gocyclo")
				// }
				// s := bufio.NewScanner(bytes.NewReader(cOutput))
				// for s.Scan() {
				// 	line := s.Text()
				// 	firstToken := strings.Split(line, " ")[0]
				// 	complex, err := strconv.ParseInt(firstToken, 10, 64)
				// 	if err != nil {
				// 		return errors.WithMessage(err, "while parsing gocyclo output")
				// 	}
				// 	info.Stats.Complexity += complex
				// }
				return nil
			}
			for _, f := range info.GoFiles {
				err := processFile(f)
				if err != nil {
					return nil, err
				}
			}
		}
	}

	if len(rootInfo.ImportedPkgs) == 1 {
		return rootInfo.ImportedPkgs[0], nil
	}

	rootInfo.ImportPath = strings.TrimPrefix(rootInfo.ImportPath, "glob ")
	return rootInfo, nil
}

func filter(fi os.FileInfo) bool {
	if strings.HasSuffix(strings.ToLower(fi.Name()), "_test.go") {
		return false
	}
	return true
}

func getInfo(importPath string) (*PkgInfo, error) {
	fset := token.NewFileSet()

	gopath := os.Getenv("GOPATH")
	if gopath == "" {
		return nil, errors.Errorf("$GOPATH is not set")
	}

	dir := filepath.Join(gopath, "src", filepath.FromSlash(importPath))
	d, err := parser.ParseDir(fset, dir, filter, parser.ImportsOnly)
	if err != nil {
		return nil, errors.WithMessage(err, "while parsing files")
	}

	info := &PkgInfo{
		Dir:        dir,
		ImportPath: importPath,
	}

	importMap := make(map[string]bool)
	for _, pkg := range d {
		for name, f := range pkg.Files {
			info.GoFiles = append(info.GoFiles, name)

			for _, imp := range f.Imports {
				ip := imp.Path.Value
				ip = ip[1 : len(ip)-1]
				importMap[ip] = true
			}
		}
	}

	for imp := range importMap {
		info.Imports = append(info.Imports, imp)
	}
	sort.Strings(info.Imports)

	return info, nil
}
