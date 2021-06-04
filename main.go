package main

import (
	"fmt"
	"io"
	"log"
	"os"
)

// go build -o md2anki main.go toggle.go

func main() {
	if len(os.Args) != 2 {
		fmt.Println("Usage: ./md2anki <filepath>")
		return
	}
	if err := Process(os.Args[1]); err != nil {
		log.Println(err)
	}
}

func saveClose(f io.Closer) {
	if err := f.Close(); err != nil {
		panic(err)
	}
}
