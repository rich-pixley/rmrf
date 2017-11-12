# Copyright © 2017 K Richard Pixley

all: rmrf

rmrf: rmrf.go
	go build rmrf

check: rmrf
	./rmrf_test.sh ./rmrf

install:
	$(INSTALL) -m 0755 ./rmrf $(INSTALLDIR)

all-gccgo: rmrf-gccgo

rmrf-gccgo: rmrf.go
	gccgo -Wall -O2 -g -o $@ $<

check-gccgo: rmrf-gccgo
	./rmrf_test.sh ./rmrf-gccgo

TAGS: rmrf.go
	etags rmrf.go

clean: ; rm -rf rmrf *~ TAGS foo bar baz
