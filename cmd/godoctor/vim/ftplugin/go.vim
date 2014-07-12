" Copyright 2014 The Go Authors. All rights reserved.  
" Use of this source code is governed by a BSD-style
" license that can be found in the LICENSE file.

" Vim integration for the Go Doctor.

" To use, add this to ~/.vimrc:
" set rtp+=$GOPATH/src/golang-refactoring.org/go-doctor/cmd/godoctor/vim

" TODO: If a refactoring only affects a single file, allow unsaved buffers
" and pipe the current buffer's contents into the godoctor via stdin
" (n.b. the quickfix list needs to be given a real filename to point to
" errors, so the godoctor's use of -.go and /dev/stdin in the log aren't good
" enough)
" TODO: Pass an option to the godoctor to limit the number of modifications.
" If it's going to try to open 100 new buffers, fail.  Consider a fallback
" option to write files in-place.

" NOTES
" -- Windows and buffers:
" http://vimdoc.sourceforge.net/htmldoc/windows.html
" -- Simulating hyperlinks in Vim:
" http://stackoverflow.com/questions/10925030/vimscriptl-file-browser-hyperlink
" -- Shell escaping/temp file problems under Windows
" http://vim.wikia.com/wiki/Fix_errors_that_relate_to_reading_or_creating_files_in_the_temp_or_tmp_environment_on_an_MS_Windows_PC
" -- Inserting the contents of a variable into a buffer
" http://stackoverflow.com/questions/16833217/set-buffer-content-with-variable
" -- Calling a varargs function
" http://stackoverflow.com/questions/11703297/how-can-i-pass-varargs-to-another-function-in-vimscript
" -- Go Doctor ASCII art uses the AMC 3 Line font, generated here:
" http://patorjk.com/software/taag/#p=display&v=3&f=AMC%203%20Line&t=Go%20Doctor
" -- General Vimscript reference:
" http://vimdoc.sourceforge.net/htmldoc/eval.html

if exists("b:did_ftplugin_doc")
    finish
endif

if exists("g:loaded_doctor")
    finish
endif
let g:loaded_doctor = 1

" Get the path to the godoctor executable.
func! s:go_doctor_bin()
  let [ext, sep] = (has('win32') || has('win64') ? ['.exe', ';'] : ['', ':'])
  let go_doctor = globpath(join(split($GOPATH, sep), ','), '/bin/godoctor' . ext)
  if go_doctor == ''
    let go_doctor = globpath($GOROOT, '/bin/doctor' . ext)
  endif
  return go_doctor
endfunction

" Path to the godoctor executable.
let s:go_doctor = s:go_doctor_bin()

" Parse the godoctor -complete output and store file contents in a dictionary
" mapping filenames to file contents
"
" When given the "-complete" flag, the output has the form:
"     log message
"     log message
"     ...
"     log message
"     @@@@@ filename1 @@@@@ num_bytes @@@@@
"     file1 contents
"     file1 contents
"     @@@@@ filename2 @@@@@ num_bytes @@@@@
"     file2 contents
"     ...
"     @@@@@ filenamen @@@@@ num_bytes @@@@@
"     filen contents
func! s:parsefiles(output)
  let result = {}
  let pat = '@@@@@ \([^@]\+\) @@@@@ \(\d\+\) @@@@@\(\|\r\)\n'
  let start = 0

  while start >= 0
    let match = matchlist(a:output, pat, start)
    let linelen = len(match[0])
  
    let filename = match[1]
    let nextmatch = match(a:output, pat, start+linelen)
    if nextmatch < 0
      let contents = a:output[start+linelen : ]
    else
      let contents = a:output[start+linelen : nextmatch-1]
    endif
    let result[filename] = contents
    let start = nextmatch
  endwhile

  return result
endfun

" List of all buffers modified by the most recent refactoring
let g:allbuffers = []

" List of new buffers opened by the most recent refactoring
let g:newbuffers = []

" Open all refactored files in (hidden) buffers
func! s:loadfiles(files)
  " Save original view
  let view = winsaveview()
  let orig = bufnr("%")

  if &hidden == 0
    set hidden
  endif
  let g:allbuffers = []
  let g:newbuffers = []
  for file in keys(a:files)
    " Get or create buffer, and fill with refactored file contents
    let oldnr = bufnr(fnameescape(file))
    exec "badd ".fnameescape(file)
    let nr = bufnr(fnameescape(file))
    call add(g:allbuffers, nr)
    if oldnr < 0
      call add(g:newbuffers, nr)
    endif
    silent exec "buffer! ".nr
    " silent execute "%!echo ".shellescape(a:files[file], 1)
    silent :1,$delete
    silent :put =a:files[file]
    silent :1delete _
  endfor

  " Restore original cursor position, windows, etc.
  silent exec "buffer! ".orig
  call winrestview(view)

  if len(a:files) > 1
    " Create a sort of dialog to make saving and undoing easy
    exec "topleft 3new"
    call setline(1, "Save Changes & Close New Buffers     " .
                  \ "  .-. .-.   .-. .-. .-. .-. .-. .-.  ")
    call setline(2, "Undo Changes & Close New Buffers     " .
                  \ "  |.. | |   |  )| | |    |  | | |(   ")
    call setline(3, "Save Changes                         " .
                  \ "  `-' `-'   `-' `-' `-'  '  `-' ' '  ")
    call setline(4, "Undo Changes")
    call setline(5, "Close This Window")
    setlocal nomodifiable buftype=nofile bufhidden=wipe nobuflisted noswapfile
    " Fix its height so, e.g., it doesn't grow when quickfix list is closed
    setlocal wfh
    " Hyperlink each line to be interpreted by b:interpret
    nnoremap <silent> <buffer> <CR> :call b:interpret(getline('.'))<CR>
  endif
endfun

" Scratch buffer ("dialog") callback to save, undo, and close buffers
func! b:interpret(cmd)
  if winnr('$') > 1
    close
  endif
  let view = winsaveview()
  let orig = bufnr("%")

  if a:cmd =~ "Save Changes"
    for buf in g:allbuffers
      if bufexists(buf)
        silent exec "buffer! " . buf . " | w"
      endif
    endfor
  elseif a:cmd =~ "Undo Changes"
    for buf in g:allbuffers
      if bufexists(buf)
        silent exec "buffer! " . buf . " | undo"
      endif
    endfor
  endif
  cclose

  if bufexists(orig)
    silent exec "buffer! ".orig
  endif
  call winrestview(view)

  if a:cmd =~ "Close New"
    for buf in g:newbuffers
      if buf != orig && bufexists(buf)
        silent exec buf . "bwipeout!"
      endif
    endfor
  endif

  let g:allbuffers = []
  let g:newbuffers = []
endfunc

" Populate the quickfix list with the refactoring log, and populate each
" window's location list with the positions the refactoring modified
func! s:qfloclist(output)
  let qflist = []
  let loclists = {}
  " Parse GNU-style 'file:line.col-line.col: message' format.
  let mx = '^\(\a:[\\/][^:]\+\|[^:]\+\):\(\d\+\):\(\d\+\):\(.*\)$'
  for line in split(a:output, "\n")
    if line =~ '^@@@@@'
      " Log is displayed before file output, so this is the end of the log
      break
    endif
    let ml = matchlist(line, mx)
    " Ignore non-match lines or warnings
    if ml == []
      let item = {
      \  'text': line,
      \}
    else
      let item = {
      \  'filename': ml[1],
      \  'text': ml[4],
      \  'lnum': ml[2],
      \  'col': ml[3],
      \}
      let bnr = bufnr(fnameescape(ml[1]))
      if bnr != -1
        let item['bufnr'] = bnr
      endif
    endif
    if item['text'] =~ 'rror:'
      let item['type'] = 'E'
    elseif item['text'] =~ 'arning:'
      let item['type'] = 'W'
    else
      let item['type'] = 'I'
    endif
    if has_key(item, 'filename') && item['text'] =~ '^ | '
      if !has_key(loclists, item['filename'])
        let loclists[item['filename']] = []
      endif
      call add(loclists[item['filename']], item)
    else
      call add(qflist, item)
    endif
  endfor
  for f in keys(loclists)
    let list = loclists[f]
    let nr = bufwinnr(f)
    if nr > 0
      call setloclist(nr, list)
    endif
  endfor
  call setqflist(qflist)
  if empty(qflist)
    cclose
  else
    cwindow
  endif
endfun

" Run the Go Doctor with the given selection, refactoring name, and arguments
func! s:RunDoctor(selected, refac, ...) range abort
  let bufcount = bufnr('$')
  for i in range(1, bufcount)
    if bufexists(i) && getbufvar(i, "&mod")
      echohl Error
         \ | echom bufname(i) . " has unsaved changes; please save before refactoring"
         \ | echohl None
      return
    endif
  endfor

  if !exists("g:doctor_scope")
    let s:scope = ""
  else
    let s:scope = " -scope=".shellescape(g:doctor_scope)
  endif

  let file = expand('%:p')
  if file == ""
    echohl Error
       \ | echom "The current buffer must be saved before refactoring"
       \ | echohl None
    return
  endif
  let file = printf(" -file=%s", shellescape(file))

  if a:selected != -1
    let pos = printf(" -pos=%d,%d:%d,%d",
      \ line("'<"), col("'<"),
      \ line("'>"), col("'>"))
  else
    let pos = printf(" -pos=%d,%d:%d,%d",
      \ line('.'), col('.'),
      \ line('.'), col('.'))
  endif
  let cmd = printf('%s -v -complete%s%s%s %s %s',
    \ s:go_doctor,
    \ s:scope,
    \ file,
    \ pos,
    \ shellescape(a:refac),
    \ join(map(copy(a:000), 'shellescape(v:val)'), ' '))
  let out = system(cmd)
  " echo cmd
  " echo out
  if v:shell_error
    let lines = split(out, "\n")
    echohl Error | echom lines[0] | echohl None
  endif
  let files = s:parsefiles(out)
  call s:loadfiles(files)
  call s:qfloclist(out)
endfun

function! s:list_refacs(a, l, p)
  let out = system(printf('%s --list', s:go_doctor))
  if v:shell_error
    return ""
  endif
  return out
endfun

func! s:RunRename(selected, ...) range abort
  if len(a:000) > 0
    call call("s:RunDoctor", [a:selected, 'rename'] + a:000)
  else
    let input = inputdialog("Enter new name: ")
    if input == ""
      echo ""
    else
      call s:RunDoctor(a:selected, 'rename', input)
    endif
  endif
endfun

command! -range=% -nargs=* Rename
  \ call s:RunRename(<count>, <f-args>)

command! -range=% -nargs=+ -complete=custom,<sid>list_refacs GoRefactor
  \ call s:RunDoctor(<count>, <f-args>)

let b:did_ftplugin_doc = 1

" vim:ts=2:sw=2:et
