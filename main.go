package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// go build -o md2anki main.go toggle.go

var VERBOSE bool

func main() {
	var mutations []options
	FlagToMathJax := flag.Bool("math", false, "convert to mathjax")
	FlagIncludeMedia := flag.Bool("media", false, "include media")
	
	OnlyMedia := flag.Bool("only-media", false, "only move media files")

	Verbose := flag.Bool("verbose", false, "see what is happening")

	if len(os.Args) < 2 || strings.HasPrefix(os.Args[1], "-") {
		fmt.Println("Usage:\n\t./md2anki <filepath> [-math] [-media] [-verbose]")
		return
	}

	flag.CommandLine.Parse(os.Args[2:])
	VERBOSE = *Verbose
	if *OnlyMedia {
		addMedia(os.Args[1])
		return
	}

	if *FlagToMathJax {
		mutations = append(mutations, toMathJax)
	}
	if *FlagIncludeMedia {
		mutations = append(mutations, includeMedia)
	}

	if err := Process(os.Args[1], mutations...); err != nil {
		log.Fatal(err)
	}
	if *FlagIncludeMedia {
		addMedia(os.Args[1])
	}
}

func saveClose(f io.Closer) {
	if err := f.Close(); err != nil {
		panic(err)
	}
}

func addMedia(forFp string) {
	ext := filepath.Ext(forFp)
	exportedMediaDp := forFp[:len(forFp)-len(ext)]
	if !exists(exportedMediaDp) {
		fmt.Printf("Cannot locate the exported media folder. Tried %q\n", exportedMediaDp)
		return
	}

	var failed = true
	defer func() {
		if failed {
			fmt.Printf(
`To add the media files manually, please locate your Anki2/collection.media folder. See https://docs.ankiweb.net/files.html to learn how.
Now rename all files in the %q folder with the pattern foldername_filename.
Next, select them all and place them in the media folder. Do ONLY copy the files, not the folder.
Retry to move only the media files with:
	./md2anki %q -only-media
`, exportedMediaDp, forFp)
		}
	}()

	// see https://docs.ankiweb.net/files.html
	var dp string
	switch runtime.GOOS {
	case "windows":
		var ok bool
		dp, ok := os.LookupEnv("APPDATA")
		if !ok {
			log.Println("Failed to find %APPDATA%")
			return
		}
		dp += `/Anki2`
	case "darwin":
		var ok bool
		dp, ok = os.LookupEnv("HOME")
		if !ok {
			log.Println("Failed to find $HOME")
			return
		}
		dp += `/Library/Application Support/Anki2`
	case "linux":
		var ok bool
		dp, ok = os.LookupEnv("XDG_DATA_HOME")
		dp += `/Anki2`
		if !ok || !exists(dp) {
			dp, ok = os.LookupEnv("HOME")
			if !ok {
				log.Println("Failed to find $HOME")
				return
			}
			dp += `/.local/share/Anki2`
		}
	}
	var username string
	fmt.Println("What is your Anki profile name?")
	_, _ = fmt.Scanln(&username)

	dp = filepath.Join(dp, username, "collection.media")
	if !exists(dp) {
		fmt.Printf("Could not find mediapath %q.\n", dp)
		return
	}

	dir, err := os.Open(exportedMediaDp)
	if err != nil {
		log.Println(err)
		return
	}
	names, err := dir.Readdirnames(-1)
	if err != nil {
		log.Println(err)
		return
	}
	if len(names) == 0 {
		log.Printf("There are no files in %q.\n", dp)
		failed = false
		return
	}

	exportedDirname := filepath.Base(exportedMediaDp)
	var once bool
	for _, name := range names {
		oldFp := filepath.Join(exportedMediaDp, name)
		newName := exportedDirname + "_" + name
		newFp := filepath.Join(dp, newName)

		if VERBOSE {
			log.Printf("mv %q %q\n", oldFp, newFp)
		}

		if err := os.Rename(oldFp, newFp); err != nil {
			log.Printf("Could not move %q to %q\n:%v\n", oldFp, newFp, err)
			once = true
		}
	}

	failed = once
}

func exists(fp string) bool {
	_, err := os.Stat(fp)
	return err == nil || errors.Is(err, os.ErrExist)
}
