package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/disiqueira/gotree"
	humanize "github.com/dustin/go-humanize"
	"github.com/fatih/color"
)

type PkgInfo struct {
	Goroot     bool
	Dir        string
	ImportPath string
	Name       string
	Imports    []string

	GoFiles  []string
	CgoFiles []string

	PkgDeps []*PkgInfo
	Stats   struct {
		Files int64
		Lines int64
	}
}

func (info *PkgInfo) Walk(f func(info *PkgInfo)) {
	walked := make(map[string]bool)
	var walk func(info *PkgInfo)
	walk = func(info *PkgInfo) {
		if walked[info.ImportPath] {
			return
		}
		walked[info.ImportPath] = true

		if info.Goroot {
			return
		}

		f(info)
		for _, depInfo := range info.PkgDeps {
			walk(depInfo)
		}
	}
	walk(info)
}

func (info *PkgInfo) CountFiles() int64 {
	var total int64
	info.Walk(func(info *PkgInfo) {
		total += info.Stats.Files
	})
	return total
}

func (info *PkgInfo) CountLines() int64 {
	var total int64
	info.Walk(func(info *PkgInfo) {
		total += info.Stats.Lines
	})
	return total
}

type GoloccOutput struct {
	NCLOC int64
}

func main() {
	log.SetFlags(0)
	log.SetOutput(os.Stdout)
	color.NoColor = false

	args := os.Args[1:]
	if len(args) < 1 {
		log.Fatal("usage: pkgcost PACKAGE")
	}

	pkg := args[0]

	infos := make(map[string]*PkgInfo)
	rootInfo := &PkgInfo{
		ImportPath: "<root>",
	}
	for _, info := range getInfos(pkg) {
		rootInfo.Imports = append(rootInfo.Imports, info.ImportPath)
	}
	infos[rootInfo.ImportPath] = rootInfo

	goroot := os.Getenv("GOROOT")
	if goroot == "" {
		if runtime.GOOS == "windows" {
			goroot = `C:\Go`
		} else {
			goroot = "/usr/local"
		}

		_, err := os.Stat(goroot)
		if err != nil {
			log.Fatalf("(%s) does not exist, please set $GOROOT", goroot)
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

	var walk func(info *PkgInfo)
	walk = func(info *PkgInfo) {
		for _, dep := range info.Imports {
			if isRoot(dep) {
				continue
			}

			if dep == "C" {
				continue
			}

			depInfo, walked := infos[dep]
			if !walked {
				depInfo = getInfo(dep)
				infos[dep] = depInfo
				walk(depInfo)
			}
			info.PkgDeps = append(info.PkgDeps, depInfo)
		}
	}

	done := make(chan bool)
	go func() {
		walk(rootInfo)
		done <- true
	}()

wait:
	for {
		select {
		case <-done:
			break wait
		case <-time.After(250 * time.Millisecond):
			fmt.Printf(".")
		}
	}
	fmt.Printf("\n")

	for _, info := range infos {
		if info.Goroot {
			continue
		}

		info.Stats.Files = int64(len(info.GoFiles) + len(info.CgoFiles))

		if info.Dir != "" {
			glb, err := exec.Command("golocc", "-o", "json", info.Dir).Output()
			if err != nil {
				panic(err)
			}

			glo := &GoloccOutput{}
			err = json.Unmarshal(glb, glo)
			if err != nil {
				panic(err)
			}

			info.Stats.Lines = glo.NCLOC
		}
	}

	printed := make(map[string]bool)

	yellow := color.New(color.FgYellow).SprintFunc()
	blue := color.New(color.FgBlue).SprintFunc()

	var mktree func(info *PkgInfo) gotree.Tree
	mktree = func(info *PkgInfo) gotree.Tree {
		if printed[info.ImportPath] {
			return nil
		}
		printed[info.ImportPath] = true
		ip := info.ImportPath
		ip = strings.Replace(ip, "github.com/", "@", 1)
		tree := gotree.New(fmt.Sprintf("%s [%s]", yellow(humanize.SI(float64(info.CountLines()), "LOC")), blue(ip)))

		var duplicates int
		for _, depInfo := range info.PkgDeps {
			depTree := mktree(depInfo)
			if depTree == nil {
				duplicates++
			} else {
				tree.AddTree(depTree)
			}
		}
		if duplicates > 0 {
			tree.Add(fmt.Sprintf("+ %d duplicates", duplicates))
		}
		return tree
	}

	var tree gotree.Tree
	if len(rootInfo.PkgDeps) == 1 {
		tree = mktree(rootInfo.PkgDeps[0])
	} else {
		tree = mktree(rootInfo)
	}
	log.Printf(tree.Print())
}

func getInfos(importPath string) []*PkgInfo {
	payload, err := exec.Command("go", "list", "-json", importPath).Output()
	if err != nil {
		log.Printf("While walking %s", importPath)
		panic(err)
	}

	dec := json.NewDecoder(bytes.NewReader(payload))
	var infos []*PkgInfo
	for {
		info := &PkgInfo{}
		err = dec.Decode(info)
		if err != nil {
			if err == io.EOF {
				break
			}
			panic(err)
		}
		infos = append(infos, info)
	}

	return infos
}

func getInfo(importPath string) *PkgInfo {
	infos := getInfos(importPath)
	if len(infos) != 1 {
		panic("expected 1 info")
	}
	return infos[0]
}
