// Command gensite generates the static documentation site that GitHub
// Pages serves from the /docs directory. It runs the same generators used
// by the godoctor command's -doc flag, plus a small landing page.
//
// Run from the repository root:
//
//	go run ./hack/gensite
//
// The man page is embedded into the user guide as a <pre> block rather
// than piped through groff, so this tool has no system dependencies.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"html"
	"io"
	"os"
	"path/filepath"

	"github.com/godoctor/godoctor/doc"
	"github.com/godoctor/godoctor/engine"
)

const (
	aboutText = "Go Doctor 0.8 (Beta)"
	outDir    = "docs"
)

func main() {
	engine.AddDefaultRefactorings()

	if err := os.MkdirAll(outDir, 0755); err != nil {
		fatal(err)
	}

	write("godoctor.1", func(w io.Writer) {
		doc.PrintManPage(aboutText, newFlags(), w)
	})
	write("install.html", func(w io.Writer) {
		doc.PrintInstallGuide(aboutText, newFlags(), w)
	})
	write("godoctor-vim.txt", func(w io.Writer) {
		doc.PrintVimdoc(aboutText, newFlags(), w)
	})

	var manRaw bytes.Buffer
	doc.PrintManPage(aboutText, newFlags(), &manRaw)
	var vimRaw bytes.Buffer
	doc.PrintVimdoc(aboutText, newFlags(), &vimRaw)
	manPre := "<pre>\n" + html.EscapeString(manRaw.String()) + "\n</pre>"
	vimPre := "<pre>\n" + html.EscapeString(vimRaw.String()) + "\n</pre>"

	write("doc.html", func(w io.Writer) {
		doc.PrintUserGuideAsGiven(aboutText, newFlags(),
			&doc.UserGuideContent{
				ManPageHTML: manPre,
				VimdocHTML:  vimPre,
			}, w)
	})

	if err := os.WriteFile(filepath.Join(outDir, "index.html"), []byte(indexHTML), 0644); err != nil {
		fatal(err)
	}

	fmt.Println("generated site in", outDir)
}

func newFlags() *flag.FlagSet {
	return flag.NewFlagSet("godoctor", flag.PanicOnError)
}

func write(name string, gen func(io.Writer)) {
	path := filepath.Join(outDir, name)
	f, err := os.Create(path)
	if err != nil {
		fatal(err)
	}
	defer f.Close()
	gen(f)
	fmt.Println("wrote", path)
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}

const indexHTML = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8" />
  <title>The Go Doctor (deprecated)</title>
  <meta name="viewport" content="width=device-width, initial-scale=1" />
  <style>
    body {
      font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Helvetica, Arial, sans-serif;
      max-width: 720px;
      margin: 40px auto;
      padding: 0 20px;
      color: #222;
      line-height: 1.5;
    }
    h1 { font-size: 2.4em; margin-bottom: 0.2em; }
    .tagline { color: #666; margin-top: 0; }
    .banner {
      background-color: #fff4d6;
      border: 1px solid #d4a017;
      border-left: 6px solid #d4a017;
      padding: 16px 20px;
      margin: 24px 0;
      border-radius: 4px;
    }
    .banner strong { font-size: 1.1em; }
    a { color: #0366d6; }
    code, pre {
      font-family: SFMono-Regular, Consolas, "Liberation Mono", Menlo, monospace;
      background: #f6f8fa;
      padding: 2px 6px;
      border-radius: 3px;
      font-size: 0.95em;
    }
    pre { padding: 12px 16px; overflow-x: auto; }
    ul { padding-left: 22px; }
    li { margin: 6px 0; }
    footer { margin-top: 60px; color: #888; font-size: 0.9em; border-top: 1px solid #eee; padding-top: 16px; }
  </style>
</head>
<body>
  <h1>The Go Doctor</h1>
  <p class="tagline">A Go refactoring tool.</p>

  <div class="banner">
    <strong>This project is deprecated.</strong><br/>
    <a href="https://github.com/golang/tools/tree/master/gopls">gopls</a> &mdash;
    the official Go language server maintained by the Go team &mdash; has
    superseded the Go Doctor and provides equivalent (and more) refactoring
    functionality. New users should use gopls; existing users are encouraged
    to migrate. The source code and these archived docs remain available for
    historical reference.
  </div>

  <h2>Archived documentation</h2>
  <ul>
    <li><a href="install.html">Installation Guide</a></li>
    <li><a href="doc.html">User's Guide</a> (includes the <code>godoctor</code> man page and Vim plug-in reference)</li>
    <li><a href="godoctor.1">Raw <code>godoctor.1</code> man page source</a></li>
    <li><a href="godoctor-vim.txt">Raw Vim help (<code>godoctor-vim.txt</code>)</a></li>
  </ul>

  <h2>Source</h2>
  <p>The Go Doctor repository:
    <a href="https://github.com/godoctor/godoctor">github.com/godoctor/godoctor</a>.
    The Vim plug-in:
    <a href="https://github.com/godoctor/godoctor.vim">github.com/godoctor/godoctor.vim</a>.
  </p>

  <h2>Regenerating these pages</h2>
  <p>This site is generated from the Go Doctor source. From the repository root:</p>
  <pre>go run ./hack/gensite</pre>

  <footer>
    Copyright &copy; Auburn University and others. Licensed under a BSD-style license.
  </footer>
</body>
</html>
`
