# Copyright © 2017 K Richard Pixley

all: rmr

rmr: rmr.go
	go build rmr

check: rmr
	./rmr_test.sh ./rmr

install:
	$(INSTALL) -m 0755 ./rmr $(INSTALLDIR)

all-gccgo: rmr-gccgo

rmr-gccgo: rmr.go
	gccgo -Wall -O2 -g -o $@ $<

check-gccgo: rmr-gccgo
	./rmr_test.sh ./rmr-gccgo

TAGS: rmr.go
	etags rmr.go

clean: ; rm -rf rmr *~ TAGS foo bar baz
