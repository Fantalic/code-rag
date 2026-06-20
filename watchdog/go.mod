module github.com/code-lightrag/watchdog

go 1.22

require (
	github.com/code-lightrag/coderag v0.0.0
	github.com/fsnotify/fsnotify v1.8.0
	github.com/urfave/cli/v2 v2.27.7
)

require (
	github.com/cpuguy83/go-md2man/v2 v2.0.7 // indirect
	github.com/russross/blackfriday/v2 v2.1.0 // indirect
	github.com/xrash/smetrics v0.0.0-20240521201337-686a1a2994c1 // indirect
	golang.org/x/sys v0.13.0 // indirect
)

replace github.com/code-lightrag/coderag => ../
