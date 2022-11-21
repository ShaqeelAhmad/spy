VERSION = 0.9
PREFIX ?= /usr/local

# Note: you may want to set ETCDIR to only /etc when using /usr as PREFIX
ETCDIR ?= $(PREFIX)/etc
BINDIR ?= $(PREFIX)/bin
MANDIR ?= $(PREFIX)/share/man/man1

CC ?= cc
CFLAGS = -Wall -Wextra -Werror -std=c99 -pedantic
CFLAGS += -DSPY_VERSION='"$(VERSION)"'

all: spy spy.1

spy.1: spy.1.scd
	sed 's|EXAMPLE_CONFIG_PATH|$(ETCDIR)/spy/|g' spy.1.scd | scdoc > spy.1

spy: main.c
	$(CC) $(CFLAGS) $< -o spy

install: all
	mkdir -p $(BINDIR)
	cp -f spy spy_list_package_files spy_list_packages $(BINDIR)
	mkdir -p $(MANDIR)
	cp -f spy.1 $(MANDIR)
	mkdir -p $(ETCDIR)/spy
	cp -f example.update $(ETCDIR)/spy/update

uninstall:
	rm -f $(BINDIR)/spy $(BINDIR)/spy_list_package_files $(BINDIR)/spy_list_packages
	rm -f $(MANDIR)/spy.1
	rm -f $(ETCDIR)/spy/update
	rmdir $(ETCDIR)/spy

clean:
	rm -f spy spy.1

.PHONY: install uninstall all clean
