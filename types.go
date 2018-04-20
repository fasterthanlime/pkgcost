package main

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
