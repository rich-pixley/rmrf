#!/bin/sh
# Copyright © 2017, K Richard Pixley

#VERBOSE=-v

set -x
set -e

[ -n "$1" ] # tell me what binary to test

touch foo
$1 ${VERBOSE} foo
[ ! -e foo ]

mkdir bar
$1 ${VERBOSE} bar
[ ! -e bar ]

touch foo bar baz
$1 ${VERBOSE} foo bar baz
[ ! -e foo ]
[ ! -e bar ]
[ ! -e baz ]

# touch foo baz
# $1 ${VERBOSE} foo bar baz
# [ ! -e foo ]
# [ ! -e bar ]
# [ ! -e baz ]

mkdir foo bar baz
$1 ${VERBOSE} foo bar baz
[ ! -e foo ]
[ ! -e bar ]
[ ! -e baz ]

mkdir -p foo/bar/baz
touch foo/bar/baz/whoahdup
$1 ${VERBOSE} foo
[ ! -e foo ]

mkdir -p foo
touch foo/bar foo/baz foo/whoahdup
$1 ${VERBOSE} foo
[ ! -e foo/bar ]
[ ! -e foo/baz ]
[ ! -e foo/whoahdup ]
[ ! -e foo ]

: successful completion
