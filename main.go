package main

import (
	"bufio"
	"encoding/csv"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"
	"unicode"
	"strings"
)

func main() {
	fp := "test.md"
	in, err := os.Open(fp)
	if err != nil {
		panic(err)
	}
	defer saveClose(in)
	rd := bufio.NewReader(in)

	out, err := os.OpenFile("md2anki.txt", os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0700)
	if err != nil {
		panic(err)
	}
	defer saveClose(out)
	wr := csv.NewWriter(out)

	cards := make(chan card)
	filepaths := make(chan string)

}

const CMD_LIT = '@'
const TITLE_LIT = []string{"t", "T", "tit", "Tit", "title", "Title"}
const BODY_LIT = []string{"b", "B", "bo", "Bo", "body", "Body"}
const ESCAPE_LIT = "`'\""

// state for operations, short ops
const (
	TITLE int = iota
	BODY
	TAG
	numOp
)

// could be made for efficiently with string buffers.

func Parse(r io.Reader, cards chan<- card) {
	var escape bool  // flag for escaping block code, i. e. ``` \n....\n ```\n

	var op int
	next := func () {
		if escape {
			panic("md2Anki detected a code block which was not closed across cards.")
		}
		op = (op + 1) % numOp  // wrap
	}

	// check if valid op line.
	// TODO(liamvdv): return errors instead of panics.
	valid := func (line string) (lineLeft string) {
		// line: "@tit asd def" => search: "tit" ; line: "@a" => search: "a"
		var search string	
		if i := strings.Index(line, " "); i == -1 {
			search = line[len(CMD_LIT):]
			defer func() { lineLeft = "" }()
		} else {
			search = line[len(CMD_LIT):i]
			defer func() { lineLeft = line[i+len(" "):] }()
		}

		var fineSyntax bool
		var nextOp int
		switch {
		case c == TITLE_LIT[0] || c == TITLE_LIT[1]:
			if in(search, TITLE_LIT) {
				nextOp = TITLE
				fine = true
			}
		case c == BODY_LIT[0] || c == BODY_LIT[1]:
			if in(search, BODY_LIT) {
				nextOp = BODY
				fine = true
			}
		default:
			// TODO(liamvdv): tell user file name and line number.
			panic("md2Anki does not allow %v or %v after %s", TITLE_LIT, BODY_LIT, CMD_LIT)
		}
		if !fineSyntax {
			// TODO(liamvdv): tell user file name and line number.
			panic("md2Anki does not allow %v or %v after %s", TITLE_LIT, BODY_LIT, CMD_LIT)
		}

		// check if current operation is operation prior to the next op
		if op != (nextOp + (numOp - 1)) % numOp {
			// TODO(liamvdv): tell user file name and line number.
			panic("md2Anki expects a title, a body and tags. They must be set in that order for every note.")
		}
		return // see defered funcs.
	}

	var card card
	isPlaceholder := func () bool {
		return card.front == "" && card.back == ""
	}
	reset := func () {
		card.front = ""
		card.back = ""
		card.tags = []string{}
	}

	consume := func (s string) {
		s = s + "\n"
		switch op {
		case TITLE:
			if isPlaceholder() {  // new card
				card.front += s
			} else {  // also old card ends, thus send on channel
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
		line := scanner.Text()  // does NOT include \n

		var i int
		var c rune
		for i, c = range line {
			if unicode.IsSpace(c) {
				continue
			}
			// escape needed to allow f. e. python decorators in code blocks.
			if c == "`" && len(line[i:]) >= 3 && line[i+1:i+3] == "``" {  
				escape = !escape
			}
			if c != CMD_LIT || escape {
				consume(line)
				break
			} // else this is a command literal
			s := valid(line[i:])  // panics if not
			next()
			consume(s)
			break
		}
	}
	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}
}

func Prompt(cs <-chan card) {
	tmpFile := os.OpenFile("note.txt", os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0755)

}

// does not yet support vim, vim needs terminal emulated (shell)
var defaultEditor = "code"

func prompt(fp string) {
	if editor := os.Getenv("EDITOR"); editor != "" {
		defaultEditor = editor
	}
	exe, err := exec.LookPath(defaultEditor)
	if err != nil {
		panic(err)
	}
	cmd := exec.Command(exe, []string{fp})
}

func Serialise(fp chan<- string, w io.Writer) {

}

type parser struct {
	line string
	lineNum int
	op int
	escape bool

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

func (c *card) serialise() []string {
	return
}

func saveClose(f io.Closer) {
	if err := f.Close(); err != nil {
		panic(err)
	}
}


func in(s string, slice []string) bool {
	for _, str := slice {
		if s == str {
			return true
		}
	}
	return false
}