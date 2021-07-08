package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
    "bufio"
)

// Linux, Darwin:
// 		go build -o md2anki main.go toggle.go
// Windows:
//		go build -o md2anki.exe main.go toggle.go

var NAME string
var CallPrefix string

func init() {
	switch runtime.GOOS {
	case "windows":
		NAME = "md2anki.exe"
	case "linux", "darwin":
		CallPrefix = "./"
		NAME = "md2anki"
	default:
		log.Fatalf("%s is not a supported platform.", runtime.GOOS)
	}
}

var VERBOSE bool

func main() {
	var mutations []options
	FlagToMathJax := flag.Bool("math", false, "convert to mathjax")
	FlagIncludeMedia := flag.Bool("media", false, "include media")

	OnlyMedia := flag.Bool("only-media", false, "only move media files")

	Verbose := flag.Bool("verbose", false, "see what is happening")

	if len(os.Args) < 2 || strings.HasPrefix(os.Args[1], "-") {
		fmt.Printf("Usage:\n\t%s%s <filepath> [-math] [-media] [-verbose]\n", CallPrefix, NAME)
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
	%s%s %q -only-media
`, exportedMediaDp, CallPrefix, NAME, forFp)
		}
	}()

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
		log.Printf("There are no files in %q.\n", exportedMediaDp)
		failed = false
		return
	}

	
	// see https://docs.ankiweb.net/files.html
	var dp string
	var ok bool
	switch runtime.GOOS {
	case "windows":
		dp, ok = os.LookupEnv("APPDATA")
		if !ok {
			log.Println("Failed to find %APPDATA%")
			return
		}
		dp += `\Anki2`
	case "darwin":
		dp, ok = os.LookupEnv("HOME")
		if !ok {
			log.Println("Failed to find $HOME")
			return
		}
		dp += `/Library/Application Support/Anki2`
	case "linux":
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
	default:
		log.Fatalf("%q is not supported.", runtime.GOOS)
	}
	var username string
	fmt.Println("What is your Anki profile name?")
    scanner := bufio.NewScanner(os.Stdin)
    if scanner.Scan() {
        username = scanner.Text()
    }

    dp = filepath.Join(dp, username, "collection.media")
	if !exists(dp) {
		fmt.Printf("Could not find mediapath %q.\n", dp)
		return
	}

	var moved int
	exportedDirname := filepath.Base(exportedMediaDp)
	for _, name := range names {
		oldFp := filepath.Join(exportedMediaDp, name)
		newName := exportedDirname + "_" + name
		// location of media files url escaped by notion, thus in note links.
		newFp := strings.Replace(filepath.Join(dp, newName), " ", "%20", -1) 

		if VERBOSE {
			log.Printf("mv %q %q\n", oldFp, newFp)
		}

		if err := os.Rename(oldFp, newFp); err != nil {
			log.Printf("Could not move %q to %q\n:%v\n", oldFp, newFp, err)
		} else {
			moved++
		}
	}

	log.Printf("Moved %d media files to %q.\n", moved, dp)
	failed = len(names) != moved
}

func exists(fp string) bool {
	_, err := os.Stat(fp)
	return err == nil || errors.Is(err, os.ErrExist)
}
