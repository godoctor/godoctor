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
"
" I would also add: 'set hidden' to your ~/.vimrc

if exists("b:did_ftplugin_doc")
    finish
endif

if exists("g:loaded_doctor")
    finish
endif
let g:loaded_doctor = 1

command! -nargs=1 -buffer Rename cal s:Rename(<f-args>)
command! DocThis cal s:doctorPWD()

"command! -buffer -complete=custom,<sid>list Doc cal s:Doctor(<f-args>)

" $GOPATH package path
fu! GoPWD()
    return substitute($PWD, $GOPATH."/src/", '', '')
endf

fu! s:doctorPWD()
    let g:doctor_scope=GoPWD()
endfu

fun! s:Rename(name)
    if !exists("g:doctor_scope")
        let s:scope = ""
    else
        let s:scope = " -scope=".g:doctor_scope." "
    endif

    let view = winsaveview()
    let cursor = line(".").",".col(".")
    let cursor = cursor.":".cursor

    let out = system("go-doctor -format json ".s:scope." -pos ".cursor." ".expand("%:t")." rename ".a:name)
    let js = parsejson#ParseJSON(out)

    if v:shell_error
        echohl Error | echom "Rename encountered an error" | echohl None
        if len(js["log"]["entries"]) > 0
            echom js["log"] " TODO(reed) put this somewhere...
            return
        endif
    endif

    let ch = (js["changes"])
    let qflist = []
    let og = bufnr("%")
    for f in keys(ch)
        " get/make buffer, fill with contents from go-doctor
        exec "badd ".fnameescape(f)
        let nr = bufnr(fnameescape(f))
        let contents = shellescape(ch[f], 1)

        silent exec "b! ".nr
        silent execute "%!echo ".contents

        let item = {
                    \ 'filename' : f,
                    \ 'lnum' : 1,
                    \ 'bufnr' : nr,
                    \}

        call add(qflist, item)
    endfor

    " restore to original w/ same cursor, windows, etc.
    silent exec "b! ".og
    call winrestview(view)

    if len(qflist) > 1 
        call setqflist(qflist)
        cwindow
    endif
endfun

let b:did_ftplugin_doc = 1

" vim:ts=4:sw=4:et
