" ==============================================================
" JSON Parser
" 
" Parses a JSON string into a Vim data structure.
"
" Usage:
"   let result = ParseJSON(json_str)
"
" Version: 0.0.1
" License: Vim license. See :help license
" Language: Vim script
" Maintainer: Po Shan Cheah <morton@mortonfox.com>
" Created: February 7, 2011
" Last updated: February 8, 2011
" ==============================================================

" Load this module only once.
if exists('loaded_parsejson')
    finish
endif
let loaded_parsejson = 1

" Avoid side-effects from cpoptions setting.
let s:save_cpo = &cpo
set cpo&vim

let s:parse_string = {}

function! s:set_parse_string(s)
    let s:parse_string = { 'str' : a:s, 'ptr' : 0 }
endfunction

function! s:is_digit(c)
    return a:c =~ '\d'
endfunction

function! s:is_hexdigit(c)
    return a:c =~ '\x'
endfunction

function! s:is_alpha(c)
    return a:c =~ '\a'
endfunction

" Get next character. If peek is true, don't advance string pointer.
function! s:lookahead_char(peek)
    let str = s:parse_string.str
    let len = strlen(str)
    let ptr = s:parse_string.ptr
    if ptr >= len
	return ''
    endif
    if str[ptr] == '\'
	if ptr + 1 < len
	    if stridx('"\/bfnrt', str[ptr + 1]) >= 0
		let s = eval('"\'.str[ptr + 1].'"')
		if !a:peek
		    let s:parse_string.ptr = ptr + 2
		endif
		" Tokenizer needs to distinguish " from \" inside a string.
		return '\'.s
	    elseif str[ptr + 1] == 'u'
		let s = ''
		for i in range(ptr + 2, ptr + 5)
		    if i < len && s:is_hexdigit(str[i])
			let s .= str[i]
		    endif
		endfor
		if s != ''
		    let s2 = eval('"\u'.s.'"')
		    if !a:peek
			let s:parse_string.ptr = ptr + 2 + strlen(s)
		    endif
		    return s2
		endif
	    endif
	endif
    endif

    " If we don't recognize any longer char tokens, just return the current
    " char.
    let s = str[ptr]
    if !a:peek
	let s:parse_string.ptr = ptr + 1
    endif
    return s
endfunction

function! s:getchar()
    return s:lookahead_char(0)
endfunction

function! s:peekchar()
    return s:lookahead_char(1)
endfunction

function! s:peekstr(n)
    return strpart(s:parse_string.str, s:parse_string.ptr, a:n)
endfunction

function! s:parse_error_msg(what)
    return printf("Parse error near '%s': %s", s:peekstr(30), a:what)
endfunction

" Get next token from JSON string.
" Returns: [ tokentype, value ]
function! s:get_token()
    while 1
	let c = s:getchar()

	if c == ''
	    return [ 'eof', '' ]
	
	elseif c == '"'
	    let s = ''
	    while 1
		let c = s:getchar()
		if c == '"' || c == ''
		    return ['string', s]
		endif
		" Strip off the escaping backslash.
		if c[0] == '\'
		    let c = c[1]
		endif
		let s .= c
	    endwhile

	elseif stridx('{}[],:', c) >= 0
	    return [ 'char', c ]

	elseif s:is_alpha(c)
	    let s = c
	    while s:is_alpha(s:peekchar())
		let c = s:getchar()
		let s .= c
	    endwhile
	    return [ 'keyword', s ]

	elseif s:is_digit(c) || c == '-'
	    " number = [-]d[d...][.d[d...]][(e|E)[(-|+)]d[d...]]
	    let mode = 'int'
	    let s = c
	    while 1
		let c = s:peekchar()
		if s:is_digit(c)
		    let c = s:getchar()
		    let s .= c
		elseif c == '.' && mode == 'int'
		    let mode = 'frac'
		    let c = s:getchar()
		    let s .= c
		elseif (c == 'e' || c == 'E') && (mode == 'int' || mode == 'frac')
		    let mode = 'exp'
		    let c = s:getchar()
		    let s .= c

		    let c = s:peekchar()
		    if c == '-' || c == '+'
			let c = s:getchar()
			let s .= c
		    endif
		else
		    " Clean up some malformed floating-point numbers that Vim
		    " would reject.
		    let s = substitute(s, '^\.', '0.', '')
		    let s = substitute(s, '-\.', '-0.', '')
		    let s = substitute(s, '\.[eE]', '.0E', '')
		    let s = substitute(s, '[-+eE.]$', '&0', '')

		    " This takes care of the case where there is
		    " an exponent but no frac part.
		    if s =~ '[Ee]' && s != '\.'
			let s = substitute(s, '[Ee]', '.0&', '')
		    endif

		    return ['number', eval(s)]
		endif
	    endwhile
	endif
    endwhile
endfunction

" value = string | number | object | array | true | false | null
function! s:parse_value(tok)
    let tok = a:tok
    if tok[0] == 'string' || tok[0] == 'number'
	return [ tok[1], s:get_token() ]
    elseif tok == [ 'char', '{' ]
	return s:parse_object(tok)
    elseif tok == [ 'char', '[' ]
	return s:parse_array(tok)
    elseif tok[0] == 'keyword'
	if tok[1] == 'true'
	    return [ 1, s:get_token() ]
	elseif tok[1] == 'false'
	    return [ 0, s:get_token() ]
	elseif tok[1] == 'null'
	    return [ {}, s:get_token() ]
	else
	    throw s:parse_error_msg("unrecognized keyword '".tok[1]."'")
	endif
    elseif tok[0] == 'eof'
	throw s:parse_error_msg("unexpected EOF")
    endif
endfunction

" elements = value | value ',' elements
function! s:parse_elements(tok)
    let [ resultx, tok ] = s:parse_value(a:tok)
    let result = [ resultx ]
    if tok == [ 'char', ',' ]
	let [ result2, tok ] = s:parse_elements(s:get_token())
	call extend(result, result2)
    endif
    return [ result, tok ]
endfunction

" array = '[' ']' | '[' elements ']'
function! s:parse_array(tok)
    if a:tok == [ 'char', '[' ]
	let tok = s:get_token()
	if tok == [ 'char', ']' ]
	    return [ [], s:get_token() ]
	endif
	let [ result, tok ] = s:parse_elements(tok)
	if tok != [ 'char', ']' ]
	    throw s:parse_error_msg("']' expected")
	endif
	return [ result, s:get_token() ]
    else
	throw s:parse_error_msg("'[' expected")
    endif
endfunction

" pair = string ':' value
function! s:parse_pair(tok)
    if a:tok[0] == 'string'
	let key = a:tok[1]
	let tok = s:get_token()
	if tok == [ 'char', ':' ]
	    let [ result, tok ] = s:parse_value(s:get_token())
	    return [ { key : result }, tok ]
	else
	    throw s:parse_error_msg("':' expected")
	endif
    else
	throw s:parse_error_msg("string (key name) expected")
    endif
endfunction

" members = pair | pair ',' members
function! s:parse_members(tok)
    let [ result, tok ] = s:parse_pair(a:tok)
    if tok == [ 'char', ',' ]
	let [ result2, tok ] = s:parse_members(s:get_token())
	call extend(result, result2)
    endif
    return [ result, tok ]
endfunction

" object = '{' '}' | '{' members '}'
function! s:parse_object(tok)
    if a:tok == [ 'char', '{' ]
	let tok = s:get_token()
	if tok == [ 'char', '}' ]
	    return [ {}, s:get_token() ]
	endif
	let [ result, tok ] = s:parse_members(tok)
	if tok != [ 'char', '}' ]
	    throw s:parse_error_msg("'}' expected")
	endif
	return [ result, s:get_token() ]
    else
	throw s:parse_error_msg("'{' expected")
    endif
endfunction

function! parsejson#ParseJSON(str)
    try
	call s:set_parse_string(a:str)
	let [ result, tok ] = s:parse_object(s:get_token())
	return result
    catch /^Parse error/
	echoerr v:exception
    endtry
endfunction

" echo s:parse_json('{ "glossary" : { "title" : [ 1, 2, "hello", true ] } }')

" let s:example_json = ' {"widget": { "debug": "on", "window": { "title": "Sample Konfabulator Widget", "name": "main_window", "width": 500, "height": 500 }, "image": { "src": "Images/Sun.png", "name": "sun1", "hOffset": 250, "vOffset": 250, "alignment": "center" }, "text": { "data": "Click Here", "size": 36, "style": "bold", "name": "text1", "hOffset": 250, "vOffset": 100, "alignment": "center", "onMouseUp": "sun1.opacity = (sun1.opacity / 100) * 90;" } }} '

" echo ParseJSON(s:example_json)

let &cpo = s:save_cpo
finish

" vim:set tw=0:
