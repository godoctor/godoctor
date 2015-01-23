VERSION=0.1

BINARIES=godoctor-${VERSION}-darwin-386 #godoctor-${VERSION}-darwin-amd64

DOCARCHIVE=$(VERSION)/godoctor-${VERSION}-doc.zip
BINARCHIVES=$(patsubst %,$(VERSION)/%.zip,$(BINARIES))
CHECKSUMS=$(VERSION)/md5sums.txt $(VERSION)/sha1sums.txt

all: $(CHECKSUMS)

clean:
	rm -f godoctor godoctor.exe godoctor.1 godoctor.1.html godoctor-vim.html
	rm -rf $(BINARCHIVES) $(DOCARCHIVE) $(CHECKSUMS)
	rmdir $(VERSION)

godoctor.1:
	@echo ""
	@echo "Creating man page..."
	@echo ""
	go build \
	  -ldflags "-X main.version ${VERSION}" \
	  -o godoctor \
	  ../cmd/godoctor \
	  && \
	  ./godoctor -man >godoctor.1

godoctor.1.html: godoctor.1
	@echo ""
	@echo "Converting man page to HTML..."
	@echo ""
	groff -t -mandoc -Thtml godoctor.1 > godoctor.1.html

godoctor-vim.html:
	@echo ""
	@echo "Converting vimdoc to HTML..."
	@echo ""
	vim -f +"set filetype=help" +"syn on" +"run! syntax/2html.vim" +"wq" +"q" \
	  ../cmd/godoctor/vim/doc/godoctor-vim.txt \
	  && \
	  mv ../cmd/godoctor/vim/doc/godoctor-vim.txt.html \
	    godoctor-vim.html

$(DOCARCHIVE): godoctor.1.html godoctor-vim.html
	@echo ""
	@echo "Packaging" $@
	@echo ""
	mkdir -p ${VERSION}
	rm -f $@
	zip -9 --junk-paths $@ \
		../doc/godoctor.html \
		godoctor.1.html \
		godoctor-vim.html

$(BINARCHIVES): godoctor.1
	@echo ""
	@echo "Building" $@
	@echo ""
	GOOS=$(word 3,$(subst -, ,$@)) GOARCH=$(subst .zip,,$(word 4,$(subst -, ,$@))) \
	  go build \
	  -ldflags "-X main.version ${VERSION}" \
	  -o godoctor$(if $(findstring windows,$(word 3,$(subst -, ,$@))),.exe,) \
	  ../cmd/godoctor
	mkdir -p $(VERSION)
	rm -f $@
	zip -9 --junk-paths $@ \
		godoctor$(if $(findstring windows,$(word 3,$(subst -, ,$@))),.exe,) \
		godoctor.1 \
		../doc/godoctor.html

$(CHECKSUMS): $(DOCARCHIVE) $(BINARCHIVES)
	@echo ""
	@echo "Computing checksums..."
	@echo ""
	md5sum $(DOCARCHIVE) $(BINARCHIVES) | sed -e 's/$(VERSION)\///' >$(VERSION)/md5sums.txt
	sha1sum $(DOCARCHIVE) $(BINARCHIVES) | sed -e 's/$(VERSION)\///' >$(VERSION)/sha1sums.txt
	rm -f godoctor godoctor.exe godoctor.1 godoctor.1.html godoctor-vim.html
	@echo ""
	@echo "***************************************"
	@echo "* Release packages built successfully *"
	@echo "***************************************"
