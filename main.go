package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/kisielk/gotool"

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
		Files      int64
		Size       int64
		Complexity int64
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

func (info *PkgInfo) CountSize() int64 {
	var total int64
	info.Walk(func(info *PkgInfo) {
		total += info.Stats.Size
	})
	return total
}

func (info *PkgInfo) CountComplexity() int64 {
	var total int64
	info.Walk(func(info *PkgInfo) {
		total += info.Stats.Complexity
	})
	return total
}

func main() {
	log.SetFlags(0)
	color.NoColor = false

	flag.Parse()
	args := flag.Args()
	if len(args) < 1 {
		log.Fatal("usage: pkgcost PACKAGE")
	}

	infos := make(map[string]*PkgInfo)
	rootInfo := &PkgInfo{
		ImportPath: "<root>",
	}

	for _, importPath := range gotool.ImportPaths(args) {
		cmd := exec.Command("go", "get", "-v", "-d", importPath)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err := cmd.Run()
		if err != nil {
			log.Fatalf("while fetching dependencies: %+v", err)
		}

		log.Printf("Entry point: %s", importPath)
		pkgInfo := getInfo(importPath)
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

	<-done
	fmt.Printf("\n")

	for _, info := range infos {
		if info.Goroot {
			continue
		}

		info.Stats.Files = int64(len(info.GoFiles) + len(info.CgoFiles))

		if info.Dir != "" {
			processFile := func(f string) {
				fpath := filepath.Join(info.Dir, f)
				stat, err := os.Stat(fpath)
				if err != nil {
					log.Fatalf("while getting file size: %+v", err)
				}
				info.Stats.Size += stat.Size()

				cOutput, err := exec.Command("gocyclo", fpath).Output()
				if err != nil {
					log.Fatalf("while running gocyclo: %+v", err)
				}
				s := bufio.NewScanner(bytes.NewReader(cOutput))
				for s.Scan() {
					line := s.Text()
					firstToken := strings.Split(line, " ")[0]
					complex, err := strconv.ParseInt(firstToken, 10, 64)
					if err != nil {
						log.Fatalf("while parsing gocyclo output: %+v", err)
					}
					info.Stats.Complexity += complex
				}
			}
			for _, f := range info.GoFiles {
				processFile(f)
			}
			for _, f := range info.CgoFiles {
				processFile(f)
			}
		}
	}

	yellow := color.New(color.FgYellow).SprintFunc()
	green := color.New(color.FgGreen).SprintfFunc()
	blue := color.New(color.FgBlue).SprintFunc()

	var mktree func(info *PkgInfo) gotree.Tree
	mktree = func(info *PkgInfo) gotree.Tree {
		ip := info.ImportPath
		ip = strings.Replace(ip, "github.com/", "@", 1)
		tree := gotree.New(fmt.Sprintf("%s (%s) [%s]",
			yellow(fmt.Sprintf("%d", info.CountComplexity())),
			green(humanize.IBytes(uint64(info.CountSize()))),
			blue(ip),
		))

		if !strings.Contains(info.ImportPath, "/vendor") {
			for _, depInfo := range info.PkgDeps {
				depTree := mktree(depInfo)
				tree.AddTree(depTree)
			}
		} else {
			tree.Add("(...ignoring deps of vendored package)")
		}
		return tree
	}

	var tree gotree.Tree
	if len(rootInfo.PkgDeps) == 1 {
		tree = mktree(rootInfo.PkgDeps[0])
	} else {
		tree = mktree(rootInfo)
	}
	fmt.Print(tree.Print())
}

func getInfos(importPath string) []*PkgInfo {
	before := time.Now()
	payload, err := exec.Command("go", "list", "-json", importPath).Output()
	log.Printf("%s: %s", importPath, time.Since(before))
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
