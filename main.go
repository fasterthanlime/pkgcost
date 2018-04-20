package main

import (
	"fmt"
	"html"
	"log"
	"net/http"
	"path"
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
			padding: 8px;
			padding-bottom: 8px;
			border-left: 1px solid #d9d8d9;
			transition: border .4s;
		}

		.import-path {
			font-size: 120%;
			color: #aeaeae;
		}

		.package:hover {
			border-color: #ccc;
		}

		.lbl-toggle:hover {
			cursor: pointer;
		}

		.lbl-toggle.has-deps-false::before {
			cursor: forbidden;
			opacity: .2;
		}

		.lbl-toggle::before {
			content: ' ';
			display: inline-block;
			
			border-top: 5px solid transparent;
			border-bottom: 5px solid transparent;
			border-left: 5px solid currentColor;
			vertical-align: middle;
			margin-right: .7rem;
			transform: translateY(-2px);
			
			transition: transform .2s ease-out;
		}

		.collapsible {
			display: none;
		}

		.lbl-toggle {
			margin-left: 8px;
		}

		.toggle {
			display: none;
		}
		
		.toggle:checked + .lbl-toggle::before {
			transform: rotate(90deg) translateX(-3px);
		}

		.toggle:checked + .lbl-toggle + .collapsible {
			display: initial;
		}

		a, a:visited, a:focus, a:hover {
			text-decoration: none;
			color: #0890df;
		}

		a:hover {
			text-decoration: underline;
		}

		.category {
			color: rgb(150, 150, 150);
			margin-left: 2em;
		}

		.tag {
			margin: 4px;
			padding: 4px;
			background: rgb(233, 233, 255);
			border-radius: 4px;
		}

		.size {
			color: rgb(60, 80, 80);
			background: rgb(255, 255, 255);
		}
		.size-64k {
			color: rgb(120, 80, 80);
			background: rgb(255, 240, 240);
		}
		.size-128k {
			color: rgb(160, 80, 80);
			background: rgb(255, 220, 220);
		}
		.size-512k {
			color: rgb(200, 80, 80);
			background: rgb(255, 210, 210);
		}
		.size-1m {
			color: rgb(240, 80, 80);
			background: rgb(255, 190, 190);
		}

		.category {

		}

		.complexity {
			color: #999;
		}

		ul {
			list-style-type: none;
			padding: 0 2em;
		}

		ul li {
			margin: 0;
			padding: 0;
		}

		li.dependency ul {
			visibility: hidden;
		}

		li.dependency:focus ul {
			visibility: visible;
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
			var walk func(id string, info *PkgInfo)
			walk = func(id string, info *PkgInfo) {
				log.Printf("walking %s", id)
				write(`<li>`)
				write(`<div class="package">`)

				hasDeps := len(info.ImportedPkgs) > 0
				write(`<input id=%#v class="toggle" type="checkbox">`, id)
				write(`<label for=%#v class="lbl-toggle has-deps-%v">`, id, hasDeps)

				ip := info.ImportPath
				vendored := false
				vendorTokens := strings.SplitN(ip, "/vendor/", 2)
				if len(vendorTokens) == 2 {
					vendored = true
					ip = vendorTokens[1]
				}

				pathTokens := strings.Split(ip, "/")

				write(`<span class="import-path">`)
				var partialPath string
				for i, pathToken := range pathTokens {
					if i > 0 {
						write(` / `)
					}

					if pathToken == "github.com" {
						partialPath = path.Join(partialPath, "github.com")
						write(`@`)
					} else {
						partialPath = path.Join(partialPath, pathToken)
						url := r.URL.Host + "/" + partialPath
						write(`<a href=%#v>%s</a>`, url, html.EscapeString(pathToken))
					}
				}
				write(`</span>`)

				writeSize := func(size int64) {
					var sizeClasses = []string{"size"}
					var KB int64 = 1024
					var MB int64 = 1024 * KB
					if size > 1*MB {
						sizeClasses = append(sizeClasses, "size-1m")
					} else if size > 700*KB {
						sizeClasses = append(sizeClasses, "size-700k")
					} else if size > 400*KB {
						sizeClasses = append(sizeClasses, "size-400k")
					} else if size > 200*KB {
						sizeClasses = append(sizeClasses, "size-200k")
					}
					write(` <span class="tag %s">%s</span>`, strings.Join(sizeClasses, " "), humanize.IBytes(uint64(size)))
				}

				writeComplexity := func(size int64) {
					return
					write(` <span class="tag complexity">%d</span>`, info.CountComplexity())
				}

				write(`<br>`)
				write(`<span class="category">self</span>`)
				writeSize(info.Stats.Size)
				writeComplexity(info.Stats.Complexity)

				if hasDeps {
					write(`<span class="category">total</span>`)
					writeSize(info.CountSize())
					writeComplexity(info.CountComplexity())
				}

				write(`<span class="category"/>`)
				if hasDeps {
					write(`<span class="tag">%d imports</span>`, len(info.ImportedPkgs))
				}

				if strings.HasSuffix(info.ImportPath, "/...") {
					url := r.URL.Host + "/" + strings.TrimSuffix(info.ImportPath, "/...")
					write(`<a href=%#v>view single</a>`, url)
				} else {
					url := r.URL.Host + "/" + info.ImportPath + "/..."
					write(`<a href=%#v>view recursive</a>`, url)
				}

				write(`<br>`)

				if vendored {
					write(` <span class="tag vendor">Vendored</span>`)
				}

				write(`</label>`)
				if hasDeps {
					write(`<div class="collapsible">`)
					for _, importedInfo := range info.ImportedPkgs {
						write(`<ul>`)
						walk(id+"_"+importedInfo.ImportPath, importedInfo)
						write(`</ul>`)
					}
					write(`</div>`)
				}
				write(`</div>`)
				write(`</li>`)
			}

			write(`<ul>`)
			walk(rootInfo.ImportPath, rootInfo)
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
