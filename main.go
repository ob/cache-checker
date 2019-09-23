package main

import (
	"crypto/sha256"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"runtime"
)

type result struct {
	result bool
	msg    string
}

func hasher(paths <-chan string, results chan<- result, done chan<- bool) {
	for p := range paths {
		data, err := ioutil.ReadFile(p)
		if err != nil {
			fmt.Printf("%s FAILED [%v]\n", p, err)
			continue
		}
		bn := path.Base(p)
		sum := sha256.Sum256(data)
		strsum := fmt.Sprintf("%x", sum)
		ret := result{true, fmt.Sprintf("%s OK", bn)}
		if bn != strsum {
			ret = result{false, fmt.Sprintf("%s CHECKSUM MISMATCH", bn)}
		}
		results <- ret
	}
	done <- true
}

func dirScanner(paths <-chan string, hasherQueue chan<- string, done chan<- bool) {
	for p := range paths {
		n := 0
		err := filepath.Walk(p, func(path string, info os.FileInfo, err error) error {
			if info == nil {
				fmt.Printf("%s: [%v]\n", path, err)
				return nil
			}
			if !info.Mode().IsRegular() {
				return nil
			}
			n++
			hasherQueue <- path
			return nil
		})
		if err != nil {
			fmt.Printf("%s: walk error [%v]\n", p, err)
		}
	}
	done <- true
}

func main() {

	flag.Parse()
	if flag.NArg() < 1 {
		flag.Usage()
		os.Exit(1)
	}
	numHashers := runtime.NumCPU()
	numDirScanners := 16
	fmt.Printf("Running with %d hashers and %d parallel dir scanners\n", numHashers, numDirScanners)
	rootpath := flag.Arg(0)

	hasherChannel := make(chan string, 100)
	dirsChannel := make(chan string, 100)
	results := make(chan result, 100)
	hashersDone := make(chan bool, 100)
	dirScannersDone := make(chan bool, 100)

	for w := 1; w <= numHashers; w++ {
		go hasher(hasherChannel, results, hashersDone)
	}

	for w := 1; w <= numDirScanners; w++ {
		go dirScanner(dirsChannel, hasherChannel, dirScannersDone)
	}

	go func() {
		// do a 1-deep parallel scan
		files, err := ioutil.ReadDir(rootpath)
		if err != nil {
			fmt.Printf("failed: %v", err)
			os.Exit(1)
		}
		for _, file := range files {
			if file.Mode().IsDir() {
				p := path.Join(rootpath, file.Name())
				dirsChannel <- p
			}
		}
		close(dirsChannel)
		// now wait for the dir scanners to be done
		for i := 1; i <= numDirScanners; i++ {
			<-dirScannersDone
		}
		close(hasherChannel)
	}()

	printerDone := make(chan bool, 1)
	go func() {
		n := 0
		for r := range results {
			// only print failures
			if !r.result {
				fmt.Printf("\n%s\n", r.msg)
			}
			if n%10 == 0 {
				fmt.Printf("\r%d", n)
			}
			n++
		}
		fmt.Printf("\rchecked %d files\n", n)
		printerDone <- true
		close(printerDone)
	}()

	fmt.Printf("Waiting for hashers...\n")
	// wait for all hashers
	for w := 1; w <= numHashers; w++ {
		<-hashersDone
	}
	close(results)
	// wait for printer
	<-printerDone
	fmt.Printf("done\n")
}
