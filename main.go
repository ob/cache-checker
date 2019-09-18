package main

import (
	"flag"
	"fmt"
	"runtime"
	"path/filepath"
	"os"
	"io/ioutil"
	"path"
	"crypto/sha256"
)

func worker(paths <-chan string, results chan<- string, done chan<- bool) {
    for p := range paths {
		data, err := ioutil.ReadFile(p)
		if err != nil {
			fmt.Printf("Failed to read %s\n", p)
		}
		bn := path.Base(p)
		sum := sha256.Sum256(data)
		strsum := fmt.Sprintf("%x", sum)
		ret := "OK"
		if bn != strsum {
			ret = "FAILED"
			results <- fmt.Sprintf("%s %s", bn, ret)
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
	numWorkers := runtime.NumCPU()
	fmt.Printf("Running with %d workers\n", numWorkers)
	rootpath := flag.Arg(0)
    jobs := make(chan string, 100)
	results := make(chan string, 100)
	done := make(chan bool, 100)

    for w := 1; w <= numWorkers; w++ {
        go worker(jobs, results, done)
    }

	go func() {
		err := filepath.Walk(rootpath, func(path string, info os.FileInfo, err error) error {
			if !info.Mode().IsRegular() {
				return nil
			}
			jobs <- path
			return nil
		})
		fmt.Printf("Done Scanning\n")
		close(jobs)
		if err != nil {
			fmt.Printf("walk error [%v]\n", err)
		}
	}()

	go func() {
    	for r := range results {
			fmt.Printf("%s\n", r)
		}
	}()

	// wait for all workers
	for w := 1; w <= numWorkers; w++ {
		<-done
	}
}