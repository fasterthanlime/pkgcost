# pkgcost

pkgcost:

  * Requires <https://github.com/warmans/golocc> and a go toolchains in your `$PATH`
  * Requires `$GOROOT` to be set
  * Lets you know how many lines of code a package pulls in, with a pretty tree like so:

```
$ pkgcost ./cmd/dbtest-gorm
.............................
23.989 kLOC [@fasterthanlime/dbtest/cmd/dbtest-gorm]
├── 8.52 kLOC [@itchio/go-itchio]
│   ├── 4.501 kLOC [@itchio/httpkit/timeout]
│   │   ├── 666 LOC [@efarrer/iothrottler]
│   │   ├── 2.72 kLOC [@getlantern/idletiming]
│   │   │   └── 2.179 kLOC [@getlantern/golog]
│   │   │       ├── 1.671 kLOC [@getlantern/errors]
│   │   │       │   ├── 400 LOC [@getlantern/context]
│   │   │       │   ├── 103 LOC [@getlantern/hidden]
│   │   │       │   │   ├── 42 LOC [@getlantern/hex]
│   │   │       │   │   └── + 1 duplicates
│   │   │       │   ├── 514 LOC [@getlantern/ops]
│   │   │       │   │   └── + 2 duplicates
│   │   │       │   └── 545 LOC [@getlantern/stack]
│   │   │       ├── 100 LOC [@oxtoacart/bpool]
│   │   │       └── + 2 duplicates
│   │   └── 1.004 kLOC [@pkg/errors]
│   ├── 2.984 kLOC [@mitchellh/mapstructure]
│   └── + 1 duplicates
├── 10.959 kLOC [@jinzhu/gorm]
│   └── 366 LOC [@jinzhu/inflection]
└── 4.485 kLOC [@jinzhu/gorm/dialects/sqlite]
    └── 4.483 kLOC [@mattn/go-sqlite3]
```

It even has colors but you'll have to try it to see them.

### License

MIT probably

