# The Go Doctor

> **⚠️ Deprecated — this project is no longer actively maintained.**
>
> [**gopls**](https://github.com/golang/tools/tree/master/gopls) — the official
> Go language server maintained by the Go team — has superseded the Go Doctor.
> It provides equivalent (and more) refactoring functionality and is actively
> developed. New users should use gopls; existing users are encouraged to
> migrate.
>
> The source remains available here for historical reference. The previous
> project website at `gorefactor.org` is being retired. The user manual and
> install guide are now hosted via GitHub Pages:
> https://godoctor.github.io/godoctor/

[![GoDoc](https://godoc.org/github.com/godoctor/godoctor?status.png)](https://godoc.org/github.com/godoctor/godoctor)
![Project Status](https://img.shields.io/badge/status-deprecated-red.svg)

## A golang refactoring tool

```
                                 .,;;::;;;;::;,.                    
                           .'.......         ......'.               
                  .'.'. .,,'......        .''...''. .,;:'.''.       
                ':;,..::..,'.   ..',     ,.       .,.  ';...,c.     
              ',ccdXXx. .; .       .:   c.,c;       l    dXXklc,,.  
            .:;cc .:O.  ;,0XO,      ;.  l;X0d;      c.   .kl'::,;:' 
           .l,, ''.:'   ,:kOc'      ;.  ,:lo;.     .:     'l'.  .;;'
           ;,;    'c     ;,       .;''clc:;'.    .,.       :     ;.;
           ;,;    :,      ........:',kKXKx';:.....         :.    ;.;
           .c,'   l'              :    .    :.             ;.   .;',
            ,;;.  l.              .',.,:.',.'              ;;   ;,: 
             .;;, l.                ; .: .;                ,: ';;,  
              .';:c:                ..',.'.                ;l,:'.   
                 .,c:,.                                 .';c;'      
                   c',c:,'                           .',c,,cc       
         .'....'.  ;   .';c;...                  ..,c:'..  .l       
        ,' .''. ,. :       ''::''.           .'';c,'.      .o       
        ;  :  : .l.c.         ..';ll.......:l:'...         .x'',,   
        .:..''..c,.l.             ;;.     .,:               d;.cl.  
          ....:cc..c                .''.''.                 l,..    
              .c;' :                  lc;                   c'      
               .:;l:                  l:,                   c,      
                 ;::.                 l:,                   :,      
                  :c;'                l:'                   :,      
                  ,o;;,.             ;;;                    c'      
                  '; ';:,'.        ';::                     l.      
                   :   ..;c:,'.',;c;'.                     .l       
                   :       .......                         ;.       
                   ':                                     ;,        
                    ,;                                   :'         
                     .;.                               .l.          
                     .,:''.                        .','..,,         
                    :;'  .:l,''''............''''''..,. .;:,        
                    'c;.''       ............         .'.,.         
                      ,:.                                           
```

## Installation

```
git clone https://github.com/godoctor/godoctor
cd godoctor && go install
```

For vim users see https://github.com/godoctor/godoctor.vim

## Usage

It's recommended to use godoctor with an editor, though godoctor can be used
directly.

```
$ godoctor
```

The godoctor command without any arguments provides usage information. For
full documentation, see:

```
$ godoctor -doc
```
