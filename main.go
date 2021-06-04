package main

import (
	"io"
	"log"
	_ "os/exec"
)

func main() {
	fp := "./tmux e8fe4b2ab4994109b56d915e7df0194f.md"
	err := Process(fp)
	if err != nil {
		log.Panic(err)
	}
}
func saveClose(f io.Closer) {
	if err := f.Close(); err != nil {
		panic(err)
	}
}
