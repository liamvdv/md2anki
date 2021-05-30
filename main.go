package main

import (
	"bufio"
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	_ "os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"unicode"
)

var wg sync.WaitGroup

func main() {
	inFp := "test.md"
	outFp := "md2anki.txt"

	cards := make(chan card)
	editedCards := make(chan card)
	wg.Add(3)
	go Parser(inFp, cards)
	go Prompter(cards, editedCards) // get card, serialise it to file, allow user to change it, then read an serialise it again, send to chan
	go Serialiser(outFp, editedCards)
	wg.Wait()
}

const CMD_LIT = '@'

var TITLE_LIT = []string{"t", "T", "tit", "Tit", "title", "Title"}
var BODY_LIT = []string{"b", "B", "bo", "Bo", "body", "Body"}
var TAG_LIT = []string{"ta", "Ta", "tag", "Tag"}

const ESCAPE_LIT = "`'\""

// state for operations, short ops
const (
	TITLE int = iota
	BODY
	TAG
	numOp
)

// could be made for efficiently with string buffers.

func Parser(fp string, cards chan<- card) {
	md, err := os.Open(fp)
	if err != nil {
		panic(err)
	}
	defer saveClose(md)
	r := bufio.NewReader(md)

	var escape bool // flag for escaping block code, i. e. ``` \n....\n ```\n

	var op int = TAG // init with last val for next func.
	next := func() {
		if escape {
			panic("md2Anki detected a code block which was not closed across cards.")
		}
		op = (op + 1) % numOp // wrap
	}

	// check if valid op line.
	// TODO(liamvdv): return errors instead of panics.
	valid := func(line string) (lineLeft string) {
		// line: "@tit asd def" => search: "tit" ; line: "@a" => search: "a"
		var search string
		if i := strings.Index(line, " "); i == -1 {
			search = line[len("@"):] // todo CMD_LIT
			defer func() { lineLeft = "" }()
		} else {
			search = line[len("@"):i] // todo CMD_LIT
			defer func() { lineLeft = line[i+len(" "):] }()
		}

		var fineSyntax bool
		var nextOp int
		switch c := string(search[0]); {
		case c == TITLE_LIT[0] || c == TITLE_LIT[1]:
			if in(search, TITLE_LIT) {
				nextOp = TITLE
				fineSyntax = true
			} else if in(search, TAG_LIT) {
				nextOp = TAG
				fineSyntax = true
			}
		case c == BODY_LIT[0] || c == BODY_LIT[1]:
			if in(search, BODY_LIT) {
				nextOp = BODY
				fineSyntax = true
			}
		default:
			// TODO(liamvdv): tell user file name and line number.
			panic(fmt.Sprintf("md2Anki does not allow %v or %v after %v", TITLE_LIT, BODY_LIT, CMD_LIT))
		}
		if !fineSyntax {
			// TODO(liamvdv): tell user file name and line number.
			panic(fmt.Sprintf("md2Anki does not allow %v or %v after %v", TITLE_LIT, BODY_LIT, CMD_LIT))
		}

		// check if current operation is operation prior to the next op
		if op != (nextOp+(numOp-1))%numOp {
			// TODO(liamvdv): tell user file name and line number.
			panic("md2Anki expects a title, a body and tags. They must be set in that order for every note.")
		}
		return // see defered funcs.
	}

	var card card
	isPlaceholder := func() bool {
		return card.front == "" && card.back == ""
	}

	consume := func(s string) {
		s = s + "\n"
		switch op {
		case TITLE:
			if isPlaceholder() { // new card
				card.front += s
			} else { // also old card ends, thus send on channel
				fmt.Printf("Sending %v to cards\n", card)
				cards <- card
				// card.tags = []string{} // TODO(liamvdv): carry over tags?
				card.back = ""
				card.front = s
			}
		case BODY:
			card.back += s
		case TAG:
			card.tags = append(card.tags, s)
		}
	}

	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text() // does NOT include \n

		var i int
		var c rune
		for i, c = range line {
			if unicode.IsSpace(c) {
				continue
			}
			// escape needed to allow f. e. python decorators in code blocks.
			if c == '`' && len(line[i:]) >= 3 && line[i:i+3] == "```" {
				escape = !escape
			}
			if c != CMD_LIT || escape {
				consume(line)
				break
			} // else this is a command literal
			s := valid(line) // panics if not
			next()
			if s := strings.TrimSpace(s); s != "" {
				consume(s)
			}
			break
		}
	}
	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}
	// send last card because next card wasn't started, thus old card not added.
	next()
	consume("")
	close(cards)
	wg.Done()
}

func Prompter(cards <-chan card, editedCards chan<- card) {
	name := "note.txt"
	dp, err := os.Getwd()
	if err != nil {
		log.Panic(err)
	}
	fp := filepath.Join(dp, name)

	cmd := getCmd(fp)
	exe := exec.Command(cmd[0], cmd[1:]...)
	for card := range cards {
		fmt.Printf("Reading %v from cards.\n", card)
		// write to file
		if err := createPrompt(fp, &card); err != nil {
			log.Panic(err)
		}

		// run editor and allow user to make edits
		exec := *exe
		runCommand(&exec)

		// read from file
		c, err := readPrompt(fp)
		fmt.Printf("readPrompt returned: %v %v", c, err)
		fmt.Printf("Got user edited card: %v\n", *c)
		if err != nil {
			if err == skipNote {
				continue
			}
			log.Panic(err)
		}
		editedCards <- *c
	}
	if err := os.Remove(fp); err != nil {
		log.Panic(err)
	}
	fmt.Println("Closing editedCards channel after all cards have been read.")
	close(editedCards)
	wg.Done()
}

// does not yet support vim, vim needs terminal emulated (shell)
// var defaultEditor = "code"
var (
	windowsEditor = "notepad.exe"

	linuxShell  = []string{"bash", "-c"}
	linuxEditor = "nano"
)

// return a string array which can be passed to exec.Command([0], [1:]...)
func getCmd(fp string) (cmd []string) {
	switch runtime.GOOS {
	case "windows":
		cmdx, err := exec.LookPath(windowsEditor)
		if err != nil {
			log.Panic(err)
		}
		cmd = []string{cmdx, fp}
	case "linux", "darwin":
		if editor := os.Getenv("EDITOR"); editor != "" {
			linuxEditor = editor
		}
		cmd = append(linuxShell, linuxEditor+" "+fp)
	default:
		panic("unknown platform.")
	}
	// cmdx, err := exec.LookPath(defaultEditor)
	// if err != nil {
	// 	log.Panicf("Cannot find editor executable %q", defaultEditor)
	// }
	// return []string{cmdx, fp}
	return
}

var promptSep = append(bytes.Repeat([]byte{'~'}, 10), '\n')

// createPrompt serialises the card and seperates all fields with 10 tildes (~~~~~)
func createPrompt(fp string, card *card) error {
	file, err := os.OpenFile(fp, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0755)
	if err != nil {
		return err
	}
	defer saveClose(file)

	file.WriteString(card.front)
	file.Write(promptSep)
	file.WriteString(card.back)
	file.Write(promptSep)
	file.WriteString(strings.Join(card.tags, "\n") + "\n")

	return file.Sync()
}

var skipLit = []byte("!skip")
var skipNote = errors.New("Skip this note.")

// readPrompt reads in the user modified file. It also handles skiping notes
// with the skipNote error.
func readPrompt(fp string) (*card, error) {
	file, err := os.Open(fp)
	if err != nil {
		return nil, err
	}
	defer saveClose(file)
	r := bufio.NewReader(file)

	c := card{}
	var s strings.Builder
	var curOp = TITLE

	b, err := r.Peek(len(skipLit))
	if err != nil {
		log.Panic(err) // only occurs if user deletes hole file content
	}

	if bytes.Compare(b, skipLit) == 0 {
		return nil, skipNote
	}
	for {
		bline, err := r.ReadBytes('\n')
		if err == io.EOF {
			break
		} else if err != nil {
			return nil, err
		}
		if bytes.HasPrefix(bline, promptSep) {
			switch curOp {
			case TITLE:
				c.front = strings.ReplaceAll(s.String(), "\n", "<br>") 
			case BODY:
				c.back = strings.ReplaceAll(s.String(), "\n", "<br>") 
			default:
				log.Panicf("The section does not expect %q at the end.", promptSep)
			}
			curOp++
			s.Reset()
			continue
		}
		_, _ = s.Write(bline)
	}
	c.tags = strings.Split(s.String(), "\n")
	return &c, nil
}

// Blocks until finished. Panics on failure.
func runCommand(exe *exec.Cmd) {
	exe.Stdin = os.Stdin
	exe.Stdout = os.Stdout
	errp, err := exe.StderrPipe()
	if err != nil {
		log.Panic(err)
	}

	if err := exe.Start(); err != nil {
		log.Panic(err)
	}

	raw, err := io.ReadAll(errp)
	if err != nil {
		log.Println(string(raw))
		log.Panic(err)
	}

	if err := exe.Wait(); err != nil {
		log.Panic(err)
	}
}

func Serialiser(fp string, cards <-chan card) {
	var n int
	out, err := os.OpenFile(fp, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0700)
	if err != nil {
		panic(err)
	}
	defer saveClose(out)
	w := csv.NewWriter(out)

	for card := range cards {
		fmt.Printf("Serialiser read card: %v\n", card)
		w.Write([]string{card.front, card.back, strings.Join(card.tags, " ")})
		n++
		w.Flush()
	}
	if err := w.Error(); err != nil {
		log.Println(err)
		return
	}
	log.Printf("Done. Added %d cards to %q.\n", n, fp)
	wg.Done()
}

type parser struct {
	line    string
	lineNum int
	op      int
	escape  bool
}

type state struct {
	ncards    int
	userCheck bool
}

type card struct {
	front string // title
	back  string // body
	tags  []string
}

func saveClose(f io.Closer) {
	if err := f.Close(); err != nil {
		panic(err)
	}
}

func in(s string, slice []string) bool {
	for _, str := range slice {
		if s == str {
			return true
		}
	}
	return false
}
