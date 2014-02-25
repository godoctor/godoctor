" Copyright 2014 The Go Authors. All rights reserved.
" Use of this source code is governed by a BSD-style
" license that can be found in the LICENSE file.
"
" To add to yours (in shell):
" echo "set rtp+=$GOPATH/src/golang-refactoring.org/go-doctor/extras/vim" >> ~/.vimrc
"
" TODO(reed): add other refactorings support (I know, I know, will integrate
" into openrefactory... this is useful for now.)
" 
" currently only does rename on current cursor selection (in .go file)
" usage:
" :Rename <new name>

if !exists("g:doctor_scope")
  let g:doctor_scope = ""
endif

command! -nargs=1 -buffer Rename cal s:Rename(<f-args>)
"command! -buffer -complete=custom,<sid>list Doc cal s:Doctor(<f-args>)

fun! s:Rename(name)
  let view = winsaveview()
  let cursor = line(".").",".col(".")
  let cursor = cursor.":".cursor
  silent execute "%! go-doctor -pos ".cursor." rename ".a:name.""
  if v:shell_error
    undo
    echohl Error | echom "Rename encountered an error" | echohl None
  endif
  call winrestview(view)
endfun

" vim:ts=4:sw=4:et
