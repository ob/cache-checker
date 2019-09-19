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

func worker(paths <-chan string, results chan<- string, done chan<- bool) {
	for p := range paths {
		data, err := ioutil.ReadFile(p)
		if err != nil {
			fmt.Printf("%s FAILED [%v]\n", p, err)
			continue
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
		fmt.Printf("Done Scanning\n")
		close(jobs)
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
