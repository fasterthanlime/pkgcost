package main

import (
	"bufio"
	"bytes"
	"encoding/json"
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
	"github.com/pkg/errors"
)

func process(args []string) (*PkgInfo, error) {
	infos := make(map[string]*PkgInfo)
	rootInfo := &PkgInfo{
		ImportPath: "<root>",
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

			depInfo, walked := infos[dep]
			if !walked {
				var err error
				depInfo, err = getInfo(dep)
				if err != nil {
					return errors.WithStack(err)
				}
				infos[dep] = depInfo
				walk(depInfo)
			}
			info.PkgDeps = append(info.PkgDeps, depInfo)
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

		info.Stats.Files = int64(len(info.GoFiles) + len(info.CgoFiles))

		if info.Dir != "" {
			processFile := func(f string) error {
				fpath := filepath.Join(info.Dir, f)
				stat, err := os.Stat(fpath)
				if err != nil {
					return errors.WithMessage(err, "while getting file size")
				}
				info.Stats.Size += stat.Size()

				cOutput, err := exec.Command("gocyclo", fpath).Output()
				if err != nil {
					return errors.WithMessage(err, "while running gocyclo")
				}
				s := bufio.NewScanner(bytes.NewReader(cOutput))
				for s.Scan() {
					line := s.Text()
					firstToken := strings.Split(line, " ")[0]
					complex, err := strconv.ParseInt(firstToken, 10, 64)
					if err != nil {
						return errors.WithMessage(err, "while parsing gocyclo output")
					}
					info.Stats.Complexity += complex
				}
				return nil
			}
			for _, f := range info.GoFiles {
				err := processFile(f)
				if err != nil {
					return nil, err
				}
			}
			for _, f := range info.CgoFiles {
				err := processFile(f)
				if err != nil {
					return nil, err
				}
			}
		}
	}
	return rootInfo, nil
}

func getInfos(importPath string) ([]*PkgInfo, error) {
	before := time.Now()
	payload, err := exec.Command("go", "list", "-json", importPath).CombinedOutput()
	log.Printf("%s: %s", importPath, time.Since(before))
	if err != nil {
		return nil, errors.WithMessage(err, fmt.Sprintf("while walking %s - output: %s", importPath, string(payload)))
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

	return infos, nil
}

func getInfo(importPath string) (*PkgInfo, error) {
	infos, err := getInfos(importPath)
	if err != nil {
		return nil, err
	}
	if len(infos) != 1 {
		return nil, errors.Errorf("expected 1 info")
	}
	return infos[0], nil
}
