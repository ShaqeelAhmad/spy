PREFIX ?= /usr/local

# Note: you may want to set ETCDIR to only /etc when using /usr as PREFIX
ETCDIR ?= $(PREFIX)/etc
BINDIR ?= $(PREFIX)/bin
MANDIR ?= $(PREFIX)/share/man/man1

GO ?= go

all: spy spy.1

spy.1: spy.1.scd
	sed 's|EXAMPLE_CONFIG_PATH|$(ETCDIR)/spy/config|g' spy.1.scd | scdoc > spy.1

spy: main.go
	$(GO) build -o spy

install: all
	mkdir -p $(BINDIR)
	cp -f spy spy-list_package_files spy-list_packages $(BINDIR)
	mkdir -p $(MANDIR)
	cp -f spy.1 $(MANDIR)
	mkdir -p $(ETCDIR)/spy
	cp -f example.config $(ETCDIR)/spy/config

uninstall:
	rm -f $(BINDIR)/spy $(BINDIR)/spy-list_package_files $(BINDIR)/spy-list_packages
	rm -f $(MANDIR)/spy.1
	rm -f $(ETCDIR)/spy/config
	rmdir $(ETCDIR)/spy

clean:
	rm -f spy spy.1
	go clean

.PHONY: install uninstall all clean
