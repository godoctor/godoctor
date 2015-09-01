// Copyright 2015 Auburn University. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package doc

import (
	"flag"
	"io"

	"text/template"
)

// PrintInstallGuide outputs the (HTML) Installation Guide for the Go Doctor.
func PrintInstallGuide(aboutText string, flags *flag.FlagSet, out io.Writer) {
	tmpl := template.Must(template.New("installGuide").Parse(installGuide))
	err := tmpl.Execute(out, struct{ AboutText string }{aboutText})
	if err != nil {
		panic(err)
	}
}

const installGuide = `<!DOCTYPE html PUBLIC "-//W3C//DTD XHTML 1.0 Strict//EN" "http://www.w3.org/TR/xhtml1/DTD/xhtml1-strict.dtd">
<html xmlns="http://www.w3.org/1999/xhtml" xml:lang="en" lang="en">
<head>
  <title>{{.AboutText}} Installation Guide</title>
  <meta http-equiv="Content-Type" content="text/html; charset=utf-8" />
  <style>
  html {
    font-family: Arial;
    font-size: 0.688em;
    line-height: 1.364em;
    background-color: white;
    color: black;
  }
  a {
    color: black;
    text-decoration: underline;
  }
  a: hover {
    text-decoration: none;
  }
  tt {
    font-size: 1.2em;
  }
  h1 {
    text-align: center;
    font-size: 2.6em;
    font-weight: bold;
    padding: 20px 0 20px 0;
    background-color: #e0e0e0;
  }
  h2 {
    text-align: left;
    font-size: 2.5em;
    font-weight: bold;
    padding: 10px 0 10px 0;
    background-color: #e0e0e0;
  }
  h3 {
    text-align: left;
    font-size: 1.75em;
    font-weight: bold;
    padding: 5px 0 2px 0;
    border-bottom: 2px dashed #c0c0c0;
  }
  h4 {
    text-align: left;
    font-size: 1.5em;
    font-weight: bold;
    padding: 5px 0 0 0;
  }
  .highlight {
    background-color: yellow;
  }
  .dotted {
    border: 1px dotted;
  }
 
  .clicktoshow {
    display: none;
    font-size: 10px;
    font-weight: normal;
    color: #808080;
  }
  .showable {
    display: block;
  }

  #toc2col .column1 {
    width: 250px;
    padding: 0;
    position: fixed;
    right: 0px;
    top: 0px;
  }
  #toc2col .column2 {
    width: 628px;
    padding: 10px 0 10px 0;
  }

  .box {
    background-color: #c0c0c0;
    width: 210px;
  }
  .box h2 {
    text-align: center;
    font-size: 1.364em;
    line-height: 1em;
    font-weight: bold;
    padding: 3px 0 3px 0;
    margin-top: 40px;
    color:#ffffff;
    background-color: #000000;
  }
  .box ul       { list-style: none; padding: 0; margin: 0; }
  .box ul li    { padding: 5px 0 1px 15px; font-weight: bold;}
  .box ul ul li { padding: 1px 0 1px 30px; font-weight: normal;}

  .man h1 {
    text-align: center;
    font-size: 1.8em;
    line-height: 1em;
    font-weight: bold;
    padding: 3px 0 3px 0;
    margin-top: 5px;
    background-color: #ffffff;
    color: black;
  }
  .man h2 {
    text-align: left;
    font-size: 1.4em;
    line-height: 1em;
    font-weight: bold;
    padding: 3px 0 3px 0;
    margin-top: 20px;
    background-color: #ffffff;
  }

  .vimdoc pre {
    font-size:1.0em;
    margin-left: 20px;
  }
  </style>
  <script language="JavaScript">
    function setDisplay(selectors, value) {
      var divs = document.querySelectorAll(selectors);
      for (var i = 0; i < divs.length; i++) {
        divs[i].style.display = value;
      }
    }

    function show(id) {
      setDisplay('.showable', 'none');
      setDisplay('.clicktoshow', 'block');
      document.getElementById(id).style.display = 'block';
      document.getElementById(id + '-click').style.display = 'none';
    }

    function showAll() {
      setDisplay('.showable', 'block');
      setDisplay('.clicktoshow', 'none');
    }

    function hideAll() {
      setDisplay('.showable', 'none');
      setDisplay('.clicktoshow', 'block');
    }
  </script>
</head>
<body id="toc2col">
    <!-- BEGIN BODY -->
    <div id="middle">
      <div class="container">
        <div class="column1">
          <div class="box">
            <div class="corner_bottom_left">
              <div class="corner_top_right">
                <div class="corner_top_left">
                  <div class="indent">
                    <!-- BEGIN TOC -->
                    <h2>Installation</h2>
                      <ul class="toc2">
                      <li><a onClick="show('install-godoctor');" href="#install-godoctor">Installing the Go Doctor</a></li>
                      <li><a onClick="show('install-vim');" href="#install-vim">Installing the Vim Plug-in</a></li>
                      <li><a onClick="show('install-docs');" href="#install-docs">Installing Documentation</a></li>
                    </ul>
		    <p style="text-align: center; font-size: 10px; color: #808080;">
                      <a href="#" onClick="showAll();">Show All</a> |
                      <a href="#" onClick="hideAll();">Hide All</a>
                    </p>
                    <!-- END TOC -->
                    <br/>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>
        <div class="column2">
          <div class="indent">
            <!-- BEGIN CONTENT -->
<h1>{{.AboutText}} Installation Guide</h1>
<a name="install"></a>
<!--h2>Installation</h2-->
<div id="install-click" class="clicktoshow">
  <a href="#install" onClick="show('install');">Show&nbsp;&raquo;</a>
</div>
<div id="install" class="showable">
  <p>The Go Doctor consists of:</p>
  <ul>
    <li>The <tt>godoctor</tt> command line tool</li>
    <li>An optional plug-in for the Vim text editor</li>
  </ul>
</div>
<a name="install-godoctor"></a>
<h2>Installing the <tt>godoctor</tt> Command Line Tool</h2>
<div id="install-godoctor-click" class="clicktoshow">
  <a href="#install-godoctor" onClick="show('install-godoctor');">Show&nbsp;&raquo;</a>
</div>
<div id="install-godoctor" class="showable">
  <ol class="enum">
    <li><tt>go get github.com/godoctor/godoctor/cmd/godoctor</tt><br/>
    <tt>go install github.com/godoctor/godoctor/cmd/godoctor</tt><br/>
    <tt>$GOPATH/bin/godoctor</tt><br/><br/>
    The final <tt>godoctor</tt> command should output a brief help message.</li>
    <li>The Vim plug-in assumes that the <tt>godoctor</tt> command is on your
    PATH.  Most Go programmers will want to put $GOPATH/bin on their PATH.
    Alternatively, you may install the Go Doctor into a more permanent
    location, e.g.,<br/><br/>
    <tt>sudo install $GOPATH/bin/godoctor /usr/local/bin/</tt></li>
    <li><i>(Optional)</i> The <tt>godoctor</tt> tool can generate a Unix man
    page for itself.  The man page should be saved as <i>godoctor.1</i> in a
    directory on your MANPATH.  For example, if you installed <tt>godoctor</tt>
    into /usr/local/bin, you will probably want to install the man page into
    /usr/local/share/man:<br/><br/>
      <pre>sudo mkdir -p /usr/local/share/man/man1</pre><br/>
      <pre>sudo sh -c 'godoctor -doc man &gt;/usr/local/share/man/man1/godoctor.1'</pre><br/></li>
  </ol>
</div>
<a name="install-vim"></a>
<h2>Installing the Go Doctor Vim Plug-in</h2>
<div id="install-vim-click" class="clicktoshow">
  <a href="#install-vim" onClick="show('install-vim');">Show&nbsp;&raquo;</a>
</div>
<div id="install-vim" class="showable">
  <p><i>Note: In addition to the Go Doctor, you may want to install
  <a target="_blank" href="https://github.com/fatih/vim-go">vim-go</a>,
  which provides Vim integration for several other Go programming tools.</i></p>
  <p>The Go Doctor Vim plug-in has been tested with Vim 7.3 and 7.4.</p>
  <p>To install the plug-in, there are two options: one if you use Pathogen to
  manage Vim plug-ins, and another if you prefer to install plug-ins
  manually.</p>
  <h3>Option 1: Using Pathogen</h3>
  <p><i><a target="_blank"
  href="https://github.com/tpope/vim-pathogen">Pathogen</a> makes it easy to
  add new plug-ins to Vim by eliminating the need to manually edit Vim's
  <tt>runtimepath</tt> when a new plug-in is installed.  If you have installed
  Pathogen, follow these instructions; if you have not installed Pathogen (or
  if you are not sure), follow the instructions for Option&nbsp;2 (Manual
  Installation) instead.</i></p>
  <ol class="enum">
    <li>Link the <b>vim</b> directory from the Go Doctor source tree into your
    ~/.vim/bundle directory.  For example:<br/><br/>
    <tt>ln -s \<br/>
    &nbsp;&nbsp;$GOPATH/src/github.com/godoctor/godoctor/cmd/godoctor/vim \<br/>
    &nbsp;&nbsp;$HOME/.vim/bundle/godoctor-vim</tt><br/><br/>
    The resulting file structure should look something like this:<br/>
    <span style="margin-left: 2em;">$HOME/</span><br/>
    <span style="margin-left: 3.5em;">.vim/</span><br/>
    <span style="margin-left: 5em;">bundle/</span><br/>
    <span style="margin-left: 6.5em;">godoctor-vim/</span><br/>
    <span style="margin-left: 8em;">doc/</span><br/>
    <span style="margin-left: 8em;">ftdetect/</span><br/>
    <span style="margin-left: 8em;">ftplugin/</span><br/>
    </li>
    <li>Verify that the <tt>godoctor</tt> command is on your PATH.  If you type
    <tt>godoctor</tt> at a shell prompt, you should get a brief help
    message.</li>
    <li>Open a <tt>.go</tt> source file in Vim, and make sure that the
    <tt>:GoRefactor</tt> command is available.  (Note that it will <i>only</i>
    be available when a file with a .go filename extension is being edited.)
    Type <tt>:GoRefac</tt>, press the <b>Tab</b> key, and verify that the
    command autocompletes to <tt>:GoRefactor</tt>.  Exit Vim.</li>
    <li><i>(Optional)</i> The Vim plug-in includes stub documentation pointing
    to the Web site.  You may want to overwrite this with more complete
    documentation, which is generated by the <tt>godoctor</tt> command itself.
    To do so, simply overwrite the <b>godoctor-vim.txt</b> file in the Vim
    plug-in's <i>doc</i> directory.  For example:<br/><br/>
    <pre>godoctor -doc vim &gt;.vim/bundle/godoctor-vim/doc/godoctor-vim.txt</pre></li>
    <li>In Vim, run <tt>:Helptags</tt> to generate help tags from the
    plug-in documentation.</li>
    <li>To view the Vim plug-in documentation, execute <tt>:help godoctor</tt>
    in Vim.</li>
  </ol>
  <h3>Option 2: Manual Installation (without Pathogen)</h3>
  <ol class="enum">
    <li>Add these lines to ~/.vimrc:
    <pre>
    if exists("g:did_load_filetypes")
      filetype off
      filetype plugin indent off
    endif
    set rtp+=$GOPATH/src/github.com/godoctor/godoctor/cmd/godoctor/vim
    filetype plugin indent on
    syntax on</pre></li>
    <li>Verify that the <tt>godoctor</tt> command is on your PATH.  If you type
    <tt>godoctor</tt> at a shell prompt, you should get a brief help
    message.</li>
    <li>Open a <tt>.go</tt> source file in Vim, and make sure that the
    <tt>:GoRefactor</tt> command is available.  (Note that it will <i>only</i>
    be available when a file with a .go filename extension is being edited.)
    Type <tt>:GoRefac</tt>, press the <b>Tab</b> key, and verify that the
    command autocompletes to <tt>:GoRefactor</tt>.  Exit Vim.</li>
    <li><i>(Optional)</i> The Vim plug-in includes stub documentation pointing
    to the Web site.  You may want to overwrite this with more complete
    documentation, which is generated by the <tt>godoctor</tt> command itself.
    To do so, simply overwrite the <b>godoctor-vim.txt</b> file in the Vim
    plug-in's <i>doc</i> directory.  For example:<br/><br/>
    <pre>godoctor -doc vim \<br/>
&gt;$GOPATH/src/github.com/godoctor/godoctor/cmd/godoctor/vim/doc/godoctor-vim.txt</pre></li>
    <li>In Vim, run
    <pre>:helptags $GOPATH/src/github.com/godoctor/godoctor/cmd/godoctor/vim/doc</pre>
    to generate help tags from the
    plug-in documentation.</li>
    <li>To view the Vim plug-in documentation, execute <tt>:help godoctor</tt>
    in Vim.</li>
  </ol>
</div>
<a name="install-docs"></a>
<h2>Installing Documentation Locally</h2>
<div id="install-docs-click" class="clicktoshow">
  <a href="#install-docs" onClick="show('install-docs');">Show&nbsp;&raquo;</a>
</div>
<div id="install-docs" class="showable">
  <p>Documentation for the Go Doctor is generated by the Go Doctor itself.  This
  can be used to create a local copy of the documentation available on the Go
  Doctor Web site.</p>
  <ul>
    <li>See above for instructions on how to generate the man page for the
    <tt>godoctor</tt> command, as well as the documentation for the Vim
    plug-in.</li>
    <li>To generate a local copy of the HTML documentation:<br/><br/>
      &nbsp;&nbsp;&nbsp;&nbsp;
      <tt>godoctor -doc install &gt;install.html</tt><br/>
      &nbsp;&nbsp;&nbsp;&nbsp;
      <tt>godoctor -doc user &gt;user.html</tt></li>
  </ul>
</div>
            <!-- END CONTENT -->
          </div>
        </div>
      </div>
    </div>
    <!-- END BODY -->
</body>
</html>
`
