/* rmr is a small go program to do the same thing as "rm -r" but with
   a high degree of concurrency.

   Copyright © 2017 K Richard Pixley

   This program is free software: you can redistribute it and/or
   modify it under the terms of the GNU General Public License as
   published by the Free Software Foundation, either version 3 of the
   License, or (at your option) any later version.

   This program is distributed in the hope that it will be useful, but
   WITHOUT ANY WARRANTY; without even the implied warranty of
   MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the GNU
   General Public License for more details.

   You should have received a copy of the GNU General Public License
   along with this program.  If not, see
   <http://www.gnu.org/licenses/>.
   */

package main

// Here's the general strategy.
// * a single traversal across the forest, (could be multiple trees on the command line).
// * use a single queue, (called "queue"), for all items to be removed.
// * remove files as soon as we find them in the queue.
// * collect all directory names into a "map", (stored as an array),
//   which both acts as a sequencing device, (so we only remove empty
//   directories), and also as a barrier in that we can remove all
//   empty directories concurrently.

// FIXME: highwater mark for number of goroutines

// FIXME: in the case of two directories on the command line, if they
// are at vastly different places in the file system, we could end up
// removing one entirely before the other.  These could be "flattened"
// out if we "normalized" their depth rather than using a string slash
// count.

// FIXME: an optional progress counter, perhaps with removals/minute,
// would be nice.

// FIXME: track global retval, would like to continue after many
// errors, but return non-zero exit status.

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// collect a stream of directory names into a "map", (stored as an
// array), where array index is the number of "/" in the directory
// name.  This generates both our removal ordering as well as a
// sequence of sets of directories that can be removed in parallel,
// (bottom up).
//
// returns a stream of []string that are names of directories that can
// be removed concurrently.

func dirTracker(dirs chan string, dirReturn chan []string)  {
	dirArray := make([][]string, 1)

	for d := range dirs {
		i := strings.Count(d, "/")
		log.Printf("collecing dir \"%v\" depth %v\n", d, i)

		if cap(dirArray) <= i {
			log.Printf("expanding dirArray to %v\n", i * 10)
			newArray := make([][]string, i * 10, i * 10)
			copy(newArray, dirArray)
			dirArray = newArray
		}
		
		dirArray[i] = append(dirArray[i], d)
	}

	count := 0
	batches := 0
	if len(dirArray) > 0 {
		log.Printf("max directory depth is %v\n", len(dirArray))

		for i := len(dirArray) - 1; i >= 0; i-- {
			count += len(dirArray[i])
			if len(dirArray[i]) > 0 {
				dirReturn <- dirArray[i]
				batches++
			}
		}
	}

	log.Printf("%v directories collected, sent in %v batches\n", count, batches)

	close(dirReturn)
}

type dirItem struct {
	name string
	info os.FileInfo
}

// process a single FileInfo.  For files, remove.  For directories,
// queue each element in the directory, and record the directory name
// with the dirTracker.

func process(item dirItem, queue chan dirItem, done chan bool, dirs chan string, verbose bool) {
	if item.info.Mode().IsDir() {
		dir, error := os.Open(item.name)
		if error != nil {
			fmt.Fprintf(os.Stderr, "ERROR: %v \"%v\"\n", error, item.name)
			log.Fatalf("ERROR: %v \"%v\"\n", error, item.name)
		} else {
			contents, error := dir.Readdir(0)
			if error != nil {
				fmt.Fprintf(os.Stderr, "ERROR: %v \"%v\"\n", error, item.name)
				log.Fatalf("ERROR: %v \"%v\"\n", error, item.name)
			} else {
				if error := dir.Close(); error != nil {
					fmt.Fprintf(os.Stderr, "ERROR: %v\n", error)
					log.Fatalf("ERROR: %v\n", error)
				}
				// process contents even if close fails

				for _, c := range contents {
					n := filepath.Join(item.name, c.Name())
					log.Printf("queueing \"%v\"\n", n)
					queue <- dirItem{n, c}
				}

				log.Printf("pushing dir \"%v\"\n", item.name)
				dirs <- item.name
			}
		}
	} else {
		if verbose {
			log.Printf("+ rm %v", item.name)
		}

		if error := os.Remove(item.name); error != nil {
			fmt.Fprintf(os.Stderr, "ERROR: %v\n", error)
			log.Fatalf("ERROR: %v\n", error)
		}
	}

	done <- true
}

// kickstart is used to queue all items from the command line.  This
// is done in sequence in order to "seed" the queue.

func kickstart(args []string, queue chan dirItem, done chan bool, verbose bool) {
	for _, arg := range args {
		info, error := os.Lstat(arg)
		if error != nil {
			fmt.Fprintf(os.Stderr, "ERROR: %v\n", error)
			log.Fatalf("ERROR: %v\n", error)
			continue
		}

		log.Printf("kickstart queueing \"%v\"\n", info.Name())
		queue <- dirItem{arg, info}
	}

	if verbose {
		log.Printf("kickstart complete: %v", strings.Join(args, " "))
	}

	done <- true
}

func loggedRemoval(directory string, verbose bool, done chan bool) {
	if verbose {
		log.Printf("+ rmdir %v\n", directory)
	}

	if error := os.Remove(directory); error != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", error)
		log.Fatalf("ERROR: %v\n", error)
	}

	done <- true
}

func main() {
	depthPtr := flag.Int("n", 20, "number of outstanding goroutines")
	queuePtr := flag.Int("q", 20000, "size of work queue")
	verbosePtr := flag.Bool("v", false, "verbosity")
	flag.Parse()

	if !*verbosePtr {
		log.SetOutput(ioutil.Discard)
	}

	log.Printf("runtime.GOMAXPROCS(0) = %v\n", runtime.GOMAXPROCS(0))
	log.Printf("runtime.NumCPU() = %v\n", runtime.NumCPU())

	// the degenerate case doesn't work very well since we use a
	// lack of goroutines to indicate completion.  So test for it
	// explicitly.

	if len(flag.Args()) == 0 {
		return
	}

	queue := make(chan dirItem, *queuePtr)
	done := make(chan bool)
	dirs := make(chan string)
	dirReturn := make(chan []string)

	// start the dirTracker
	go dirTracker(dirs, dirReturn)

	// put everything from the command line into the queue
	go kickstart(flag.Args(), queue, done, *verbosePtr)

	goroutinesOutstanding := 1 // including kickstart

	log.Printf("goroutinesOutstanding = %v, runtime.NumGoroutine() = %v\n",
		goroutinesOutstanding, runtime.NumGoroutine())

	// basic strategy:
	// * if there are any dones available, reap them.
	// * if we can start another goroutine, do so, otherwise wait for one to be done.

	for {
		select {
		case <-done:
			if goroutinesOutstanding--; goroutinesOutstanding == 0 {
				log.Printf("scanned done with %v outstanding\n", goroutinesOutstanding)
				log.Printf("goroutinesOutstanding = %v, runtime.NumGoroutine() = %v\n",
					goroutinesOutstanding, runtime.NumGoroutine())

				break
			}

		default:
			log.Print("no dones scanned")
			// pass
		}

		if goroutinesOutstanding < *depthPtr {
			select {
			case item := <-queue:
				log.Printf("processing \"%v\" from queue\n", item.name)
				go process(item, queue, done, dirs, *verbosePtr)
				goroutinesOutstanding++
				log.Printf("goroutinesOutstanding = %v, runtime.NumGoroutine() = %v\n",
					goroutinesOutstanding, runtime.NumGoroutine())


			default:
				log.Printf("queue empty, %v goroutinesOutstanding, %v runtime.NumGoroutine()",
					goroutinesOutstanding, runtime.NumGoroutine())
				// means queue has emptied, probably still need to wait for dones
				if goroutinesOutstanding == 0 {
					goto finished
				}
			}
		}

		log.Print("blocking on done...")
		<- done
		goroutinesOutstanding--
		log.Printf("blocking done with %v outstanding\n", goroutinesOutstanding)
	}

finished:

	log.Printf("goroutinesOutstanding = %v, runtime.NumGoroutine() = %v\n",
		goroutinesOutstanding, runtime.NumGoroutine())

	// at this point all files have been processed, (removed), and
	// all directories recorded.  Closing dirs tells dirTracker to
	// stop collecting and start regurgitating.

	close(dirs)
	close(done)

	// process dirs

	for dlist := range dirReturn {
		done := make(chan bool)

		log.Printf("caught directory batch %v\n", dlist)

		for _, d := range dlist {
			go loggedRemoval(d, *verbosePtr, done)
		}

		for i := len(dlist); i > 0; i-- {
			<-done
		}
	}
}
