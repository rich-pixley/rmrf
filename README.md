# Rmrf

**Rmrf** is a small go program to do the same thing as "rm -rf" but with a
high degree of concurrency.

## Rationale

The traditional UN*X "rm" command is single threaded.  Once upon a
time, with single cpu systems and single threaded file systems,
holding dozens or even hundreds of files, this was entirely
sufficient.  However, with modern multi-core systems, broadly parallel
file systems, and network and distributed file systems, holding
terrabytes of data over millions of files, it's quite common for the
removal of a tree to take nearly as long as it did to create the tree.
In some cases, much longer, perhaps even several times as long since
the creation of the tree was done with parallelization while removal
is just single threaded.  That can easily be hours or days in many
contexts.

**Rmrf** is an experiment in concurrency.  It is written in the [Go
programming language, (golang)](https://golang.org), makes heavy
use of Go's concurrency mechanism called "goroutines", and is intended
to partially address this problem.

### Usage

Naive use is trivial.  `rmr file1 directory2 ...`  However, there is
some tuning available once you understand some of how **rmrf** works.

### How It Works

**Rmrf** uses a work queue as embodied in a Go "channel".  The items
from the command line are added to the queue initially.  Thereafter,
separate "worker" goroutines are called to read an item from the queue
and "process" it.  "Processing" in this context means either removing
a file or reading a directory and adding all of the directories items
to the queue.  Both file removal and directory reading may be done
many times concurrently and concurrently with each other.  That's
rather the *point* of **rmrf**, afterall.

This is important to understand because the queue has a finite size.
If the number of items in a directory is larger than the size of the
queue, then the goroutine may not be able to queue all of the items
from the directory and **rmrf** would then deadlock.  More, since
there may be many goroutines each reading directories and queueing up
items, the sum total number of items being queued needs to fit in the
queue.  At the same time, there will be other goroutines taking items
out of the queue, removing files, and reading directories and adding
new items to the queue.

The queue size needed for any particular tree is somewhat complex, not
entirely deterministic, and depends on the shape of your tree.
Basically, if you run into deadlock, try again with a higher queue
size, (the "-q size" command line option), perhaps an order of
magnitude larger, even.

There is also a command line switch for the number of goroutines to
run concurrently, (the "-n count" command line option).  In practice,
this will coincide highly with the number of outstanding operating
system calls, (either unlink(2) or readdir(3)).  For a local file
system, you'll need to tune it, but my guess is that the right number
will likely be about 4 times the number of actual CPUs you have
available.  If you want to optimize your removal times then your goal
here is to saturate the disk subsystem, but only just.  That is,
oversaturation will quickly lead to *slower* removal times, not
faster, due to thrashing.  But experiment.

For NFS, your optimum concurrency number will likely be much higher.
Don't be surprised if the right number is two or three orders of
magnitude higher than your optimum number on local disk.

### Directories Are Special

Traditional graph traversals of trees are either depth first or
breadth first, and removals are post order.  Most tree removal
algorythms do exactly this.  However, those algorythms are single
threaded.

There's an argument to be made that even with an infinitely parallel
machine, the tree depth will be your minimal removal time for
directories since lower level directories must be removed before
higher level ones.  But we can still remove many empty directories
concurrently.

The way this works in **rmrf** is that as we're doing our first, (and
only), traversal of the directory tree, we record the name of each of
the directories we find, in a hash, where the hash is the number of
"/" characters in the directory's name.  Once we've collected them
all, (and removed all of the files), we walk down the hash counts.  So
if the longest path is 24 slash characters, then we remove all of the
24 slash character directories concurrently.  When that's done, we
remove all of the 23 slash character directories, etc, and so on, and
so forth.

What this means is twofold.  First, if you're watching things be
removed, you'll notice that *files* are removed first, left to right,
top to bottom.  (This is *very* different from traditional removal
order.)  Then directories are removed bottom up, but "generationally"
left to right.
