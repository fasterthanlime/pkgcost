package main

import (
	"fmt"
	"html"
	"log"
	"net/http"
	"strings"

	humanize "github.com/dustin/go-humanize"
)

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		tokens := strings.SplitN(r.URL.Path, "/", 2)
		pkg := tokens[1]

		var rootInfo *PkgInfo
		var err error
		if pkg != "" {
			rootInfo, err = process([]string{pkg})
		}

		write := func(f string, args ...interface{}) {
			fmt.Fprintf(w, f, args...)
		}

		write(`%s`, `
<html>
	<head>
		<title>pkgcost</title>
		<link href="https://fonts.googleapis.com/css?family=Lato" rel="stylesheet">
		<style>
		* {
			font-family: 'Lato', sans-serif;
			line-height: 1.6;
		}

		.package {
			margin: 8px 4px;
		}

		.import-path {}

		.tag {
			padding: 3px;
			margin: 0 4px;
			border-radius: 4px;
			font-size: 85%;
			color: white;
			background: grey;
		}

		.vendored {
			background: yellow;
		}

		.size {
			background: red;
		}

		.complexity {
			background: blue;
		}

		ul {
			list-style-type: none;
			padding: 0 2em;
		}

		ul li {
			margin: 0;
			padding: 0;
		}
		</style>
	</head>
	<body>
		<form>
			<input name="pkg" type="text" placeholder="github.com/example/package"/>
			<input type="submit" value="Explore!"/>
		</form>
		`)
		if err != nil {
			write(`<h3>Error: </h3>`)
			write(`<pre>`)
			write(`%+v`, err)
			write(`</pre>`)
		} else if rootInfo != nil {
			var walk func(info *PkgInfo)
			visited := make(map[string]bool)
			walk = func(info *PkgInfo) {
				write(`<li>`)
				write(`<div class="package">`)

				ip := info.ImportPath
				vendored := false
				vendorTokens := strings.SplitN(ip, "/vendor/", 2)
				if len(vendorTokens) == 2 {
					vendored = true
					ip = vendorTokens[1]
				}
				ip = strings.Replace(ip, "github.com/", "@", 1)

				write(`<span class="import-path">%s</span>`, html.EscapeString(ip))
				write(` <span class="tag size">%s</span>`, humanize.IBytes(uint64(info.CountSize())))
				write(` <span class="tag complexity">%d</span>`, info.CountComplexity())
				if vendored {
					write(` <span class="tag vendor"/>`, info.CountComplexity())
				}
				write(`</div>`)
				for _, depInfo := range info.PkgDeps {
					if visited[depInfo.ImportPath] {
						return
					}
					visited[depInfo.ImportPath] = true
					write(`<ul>`)
					walk(depInfo)
					write(`</ul>`)
				}
				write(`</li>`)
			}

			write(`<ul>`)
			walk(rootInfo)
			write(`</ul>`)
		}
		write(`
	</body>
</html>
			`)
	})
	addr := "localhost:9089"
	log.Printf("Listening on %s", addr)
	http.ListenAndServe(addr, http.DefaultServeMux)
}
