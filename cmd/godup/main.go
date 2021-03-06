package main

import (
	"context"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"

	"github.com/olekukonko/tablewriter"
	"github.com/tthanh/godup"
)

var (
	allFile map[int64][]*godup.File
	count   int64
	mutex   sync.Mutex
	wg      sync.WaitGroup
	ctx     context.Context
	cancel  context.CancelFunc
)

func main() {
	paths := os.Args[1:]

	allFile = make(map[int64][]*godup.File)

	if len(paths) < 1 {
		fmt.Println("path not found")
		return
	}

	for _, path := range paths {
		err := check(path)
		if err != nil {
			fmt.Println(err)
		}
	}

	if len(allFile) == 0 {
		fmt.Println("cannot find any file")
		return
	}

	fmt.Printf("found %d files\n", count)

	ctx, cancel = context.WithCancel(context.Background())
	defer cancel()

	cic := make(chan []*godup.File) // compare input channel
	coc := make(chan []*godup.File) // compare output channel

	workers := runtime.NumCPU()
	wg.Add(workers)

	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			godup.CompareWorker(ctx, cic, coc)
		}()
	}

	go func() {
		wg.Wait()
		close(coc)
	}()

	go func() {
		defer close(cic)
		for _, files := range allFile {
			if len(files) > 1 {
				cic <- files
			}
		}
	}()

	var tableData [][]string
	for files := range coc {
		if len(files) > 1 {
			for idx, file := range files {
				var size string
				var hash string
				if idx == 0 {
					size = strconv.FormatInt(file.Size, 10)
					hash = hex.EncodeToString(file.Hash)
				}

				tableData = append(tableData, []string{size, hash, file.Path})
				if idx != len(files)-1 {
					tableData = append(tableData, []string{"", "", ""})
				}
			}
		}

	}

	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Size", "Hash", "Path"})

	for _, r := range tableData {
		table.Append(r)
	}

	table.Render()
}

func check(path string) error {
	src, err := os.Stat(path)
	if err != nil {
		return err
	}

	if src.IsDir() {
		fullPath, err := filepath.Abs(path)
		if err != nil {
			return err
		}
		fmt.Printf("Checking %s\n", fullPath)
		if err = filepath.Walk(fullPath, walker); err != nil {
			return err
		}
	}
	return nil
}

func walker(path string, fi os.FileInfo, err error) error {
	if !fi.IsDir() {
		mutex.Lock()
		defer mutex.Unlock()

		count++

		file := &godup.File{
			Name: fi.Name(),
			Size: fi.Size(),
			Path: path,
		}

		files := allFile[file.Size]
		files = append(files, file)
		allFile[file.Size] = files
	}
	return nil
}
