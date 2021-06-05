package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
)

// go build -o md2anki main.go toggle.go

func main() {
	var mutations []options
	FlagToMathJax := flag.Bool("math", false, "convert to mathjax")

	if len(os.Args) < 2 || strings.HasPrefix(os.Args[1], "-") {
		fmt.Println("Usage: ./md2anki <filepath> [-math]")
		return
	}

	flag.CommandLine.Parse(os.Args[2:])
	if *FlagToMathJax {
		mutations = append(mutations, toMathJax)
	}

	if err := Process(os.Args[1], mutations...); err != nil {
		log.Println(err)
	}
}

func saveClose(f io.Closer) {
	if err := f.Close(); err != nil {
		panic(err)
	}
}
