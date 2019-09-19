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

type Result struct {
	result bool
	msg    string
}

func worker(paths <-chan string, results chan<- Result, done chan<- bool) {
	for p := range paths {
		data, err := ioutil.ReadFile(p)
		if err != nil {
			fmt.Printf("%s FAILED [%v]\n", p, err)
			continue
		}
		bn := path.Base(p)
		sum := sha256.Sum256(data)
		strsum := fmt.Sprintf("%x", sum)
		ret := Result{true, fmt.Sprintf("%s OK", bn)}
		if bn != strsum {
			ret = Result{false, fmt.Sprintf("%s CHECKSUM MISMATCH", bn)}
		}
		results <- ret
	}
	done <- true
}

func main() {
	flag.Parse()
	if flag.NArg() < 1 {
		flag.Usage()
		os.Exit(1)
	}
	numWorkers := runtime.NumCPU()
	fmt.Printf("Running with %d workers\n", numWorkers)
	rootpath := flag.Arg(0)
	jobs := make(chan string, 100)
	results := make(chan Result, 100)
	done := make(chan bool, 100)

	for w := 1; w <= numWorkers; w++ {
		go worker(jobs, results, done)
	}

	go func() {
		parallelScanners := 0
		waitChan := make(chan bool, 100)
		// do a 1-deep parallel scan
		files, err := ioutil.ReadDir(rootpath)
		if err != nil {
			fmt.Printf("failed: %v", err)
			os.Exit(1)
		}
		for _, file := range files {
			if file.Mode().IsDir() {
				np := path.Join(rootpath, file.Name())
				parallelScanners += 1
				go func() {
					err := filepath.Walk(np, func(path string, info os.FileInfo, err error) error {
						if info == nil {
							fmt.Printf("%s: [%v]\n", path, err)
							return nil
						}
						if !info.Mode().IsRegular() {
							return nil
						}
						jobs <- path
						return nil
					})
					if err != nil {
						fmt.Printf("%s: walk error [%v]\n", np, err)
					}
					waitChan <- true
				}()
			}
		}
		fmt.Printf("Waiting for %d parallel scanners\n", parallelScanners)
		// wait for paralle scanners
		for w := 0; w < parallelScanners; w++ {
			<-waitChan
		}
		close(jobs)
	}()

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
			n += 1
		}
		fmt.Printf("\n")
	}()

	// wait for all workers
	for w := 1; w <= numWorkers; w++ {
		<-done
	}
}
