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
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
)


// https://github.com/google/re2/wiki/Syntax

// Make a deck from a notion file.
// The Title of the page will be the Anki deck title.

// The front of each note will be the words of the toggle when collapsed, the content is the back.
// Nested toggels will not be respected.

// All headings are used as tags for the cards following that tag.
// Subsequent headings will inherit the tags of the previous titles with the larger formatting
/*
The abc  -> deck: The abc
/h1 abc  -> tag"abc" stack push
... -> add tag abc

/h2 cba -> tag"cba" stack push
... -> add abc, cba tags

/h2 bca -> peek and if >= 2 pop. push tag"bca" on stack // i. e. prior heading with same or smaller size rendering will get dropped.
... -> add tag abc, bca

/h

*/
// all tags will be kept. Else, all tags

/*
notion:
> abc

|
V

md:
- abc

	body

*/

type tag struct {
	// name can contain spaces, which will be replaced with underscores, quotes
	// must be omitted.
	// see https://anki.tenderapp.com/discussions/ankidesktop/28088-which-characters-can-tags-contain
	name []byte

	// level is 1, 2 and 3 for the according headings in anki.
	level int8
}

// NL is the platform agnostic new line character. (CRLF on windows, else jsut LF)
var NL string = func() string {
	switch runtime.GOOS {
	case "windows":
		return "\r\n"
	default:
		return "\n"
	}
}()

const INDENT = `[\t| {4}]`

var (
	// headingExp will be called with Submatch method, idx 1 returns the heading
	// with # and without NL.
	/* heading matches:
	...
	# foo bar foo2
	...
	or:
	...
	## foo bar foo2
	...
	or:
	...
	### foo bar foo2
	...
	*/
	headingExp = `(#{1,3}\s.+)` + NL

	/* toggle matches:
	....
	- foo bar foo2

		foo bar bar bar, foo
		foo bar foo bar.

	....
	*/
	toggleExp = "" + // capture group 0 includes hole toogle.
		// head
		`-\s(.+)` + NL + // capture group 1: use!
		// body
		NL +
		// only last capture group will be stored, thus this cg is only ment
		// for grouping.
		`(` + INDENT + `.+` + NL +
		`|` + NL + `)+` + // capture group 2: do not use, only includes last occurance.
		NL
)

// pageTitleToDeckNames takes the filepath of the exported file and returns the
// title of the page, which is included in the exported files filename.
// "{name inc. spaces} {id}.md"
func pageTitleToDeckName(fp string) string {
	name := filepath.Base(fp)
	i := strings.LastIndex(name, " ")
	return strings.ReplaceAll(name[0:i], " ", "_") // anki expects underscores.
}

func MdToAnkiFilename(fp string) string {
	return pageTitleToDeckName(fp) + ".txt"
}

type card2 struct {
	front []byte
	back  []byte
	tags  [][]byte
}

func (c card2) String() string {
	var sb strings.Builder
	sb.WriteString("Card{Front: " + string(c.front))
	sb.WriteString(fmt.Sprintf(" Back: %q Tags: ", string(c.back)))
	sb.WriteString(string(bytes.Join(c.tags, []byte{' '})))
	sb.WriteByte('}')

	return sb.String()
}

// Process is the many entry point which turns a file into an importable anki deck.
func Process(fp string, mutations ...options) error {
	raw, err := os.ReadFile(fp)
	if err != nil {
		return err
	}
	errc := make(chan error)
	hIdxc := make(chan [2]int) // header: start:end of file byte array excluding newline.
	tIdxc := make(chan [4]int) // toggle: front, back: fstart:fend bstart, bend; excluding newline and indentation.
	cards := make(chan card2)
	editedCards := make(chan card2)

	filename := MdToAnkiFilename(fp)

	var wg sync.WaitGroup
	wg.Add(5)
	go findHeadings(raw, hIdxc, &wg)
	go findToggles(raw, tIdxc, &wg)
	go combine(raw, hIdxc, tIdxc, cards, errc, &wg)
	go Prompter(cards, editedCards, &wg, mutations...)
	go Serialiser(filename, editedCards, &wg)
	wg.Wait()

	return nil
}

// findHeading drops the first found heading, because it is the page name.
func findHeadings(raw []byte, hc chan<- [2]int, wg *sync.WaitGroup) {
	re := regexp.MustCompile(headingExp)
	// res is an slice of 4 int slices. These 4 ints describe 2 sections on the
	// raw byte array. The first section is the hole regexp match, the second
	// only the capture group.
	res := re.FindAllSubmatchIndex(raw, -1)

	// drop first heading because its the page name
	for _, idxs := range res[1:] {
		// we only send the capturing group.
		hc <- [2]int{idxs[2], idxs[3]}
	}
	close(hc)
	wg.Done()
}

func findToggles(raw []byte, tc chan<- [4]int, wg *sync.WaitGroup) {
	re := regexp.MustCompile(toggleExp)
	// res returns 3 sections per match, each described by two consecutive indexes.
	// The zeroth section is the hole match, the first the front without dash
	// and without newlines. The third one is the last match in the toggle body,
	// and does NOT represent the hole body.
	res := re.FindAllSubmatchIndex(raw, -1)

	for _, idxs := range res {
		// sections by index in idxs:
		// full (0, 1)
		// heading (2, 3), excluding dash and newline
		// last body cg (4, 5)
		// negative lookahead workaround char (6, 7)
		hstart, hend := idxs[2], idxs[3]

		// transform the full match to the body part.
		var bstart, bend int
		// bstart is the start of the body including identation.
		off := idxs[0]
		body := raw[off:idxs[1]]
		sep := []byte(NL + NL)
		i := bytes.Index(body, sep)
		if i == -1 {
			// cannot occur.... still check
			log.Panic("shoud never happen because regexp shouldn't allow this.")
		}
		bstart = off + i + len(sep) // includes spaceing! remove later with ReplaceAll(body, "\n    ", "\n"), only removes at linestart

		// bend is the end of the the body including one newline.
		// use the end of the last matching body capture group, section 2
		bend = idxs[5]

		tc <- [4]int{hstart, hend, bstart, bend}
	}
	close(tc)
	wg.Done()
}

// tagStack will always hold the latest headings with decreasing order.
// when a h1 and a h2 is in there and a h3 is found, add that to the end.
// when a h2 is encountered, replace h2 and remove h3.
// DO NOT ITERATE over tagStack without checking if the default tag values are used.
type tagStack struct {
	tags [3]tag // three heading levels
	len  int
}

func (stk *tagStack) push(t tag) {
	if stk.len == 0 {
		stk.tags[0] = t
		stk.len++
		return
	}
	var i int
	for ; i < stk.len; i++ {
		if t.level <= stk.tags[i].level {
			stk.tags[i] = t
			stk.len = 1 + i
			return
		}
	}
	stk.tags[i] = t
	stk.len++
}

func (stk tagStack) bytes() [][]byte {
	var bss [][]byte
	for i := 0; i < stk.len; i++ {
		bss = append(bss, stk.tags[i].name)
	}
	return bss
}

func newTag(bs []byte) tag {
	t := tag{}
	for i, c := range bs {
		if c == '#' {
			t.level++
			continue
		}
		bs = bytes.Trim(bs[i:], "\t ")
		t.name = bytes.ReplaceAll(bs, []byte{' '}, []byte{'_'})
		return t
	}
	panic("invalid byte slice.")
}

// Combine should only be run once for every file and cannot be put behind a
// load balancer.
// Combiner takes all the input streams at channels them into a card, which will
// then be used by the prompter.
func combine(raw []byte, headings <-chan [2]int, toggles <-chan [4]int, cards chan<- card2, errc chan<- error, wg *sync.WaitGroup) {
	stack := tagStack{}

	// TODO(liamvdv): how could this be pipelined?
	var hs [][2]int
	for h := range headings {
		hs = append(hs, h)
	}

	var ts [][4]int
	for t := range toggles {
		//fmt.Printf("Toggle Name: %q\n", string(raw[t[0]:t[1]]))
		//fmt.Printf("Toggle Body: %q\n", strings.ReplaceAll(string(raw[t[2]:t[3]]), "    ", ""))
		ts = append(ts, t)
	}

	for _, t := range ts {
		card := card2{
			front: raw[t[0]:t[1]],
			back:  bytes.ReplaceAll(raw[t[2]:t[3]], []byte("    "), nil), // remove indent
		}

		for _, h := range hs {
			if h[1] <= t[0] { // if the heading comes before the toggle (= because index is one greater than real end)
				tag := newTag(raw[h[0]:h[1]])
				stack.push(tag)
				hs = hs[1:] // reduce the array
				continue
			}
			// headings within toggles should be ignored (includes code blocks, f. e. python comments)
			if h[1] <= t[3] {
				hs = hs[1:]
				continue
			}
			// headings comes after card, do not consume yet
			break
		}
		card.tags = stack.bytes()
		cards <- card
	}
	close(cards)
	wg.Done()
}

var (
	subsep   []byte = bytes.Repeat([]byte{'~'}, 5)
	frontSep []byte
	backSep  []byte
	tagsSep  []byte
)

func init() {
	var (
		front = []byte("Front")
		back  = []byte("Back")
		tags  = []byte("Tags")
		nl    = []byte{'\n'}
	)
	fill := func(dst []byte, srcs ...[]byte) int {
		for i := range srcs {
			dst = append(dst, srcs[i]...)
		}
		return len(dst)
	}

	frontSep = make([]byte, 0, 2*len(subsep)+len(front)+1)
	n := fill(frontSep, subsep, front, subsep, nl)
	frontSep = frontSep[0:n]

	backSep = make([]byte, 0, 2*len(subsep)+len(back)+1)
	n = fill(backSep, subsep, back, subsep, nl)
	backSep = backSep[0:n]

	tagsSep = make([]byte, 0, 2*len(subsep)+len(tags)+1)
	n = fill(tagsSep, subsep, tags, subsep, nl)
	tagsSep = tagsSep[0:n]
}

func Prompter(cards <-chan card2, editedCards chan<- card2, wg *sync.WaitGroup, mutations ...options) {
	name := "note.txt"
	dp, err := os.Getwd()
	if err != nil {
		log.Panic(err)
	}
	fp := filepath.Join(dp, name)
	defer os.Remove(fp)  // omit error, possible because card doesn't run

	doMutations := yieldDoMutation(mutations...)

	cmd := getCmd(fp)
	exe := exec.Command(cmd[0], cmd[1:]...)
	for card := range cards {
		// apply patches such as converting to mathjax format.
		doMutations(&card)

		if err := createPrompt(fp, &card); err != nil {
			log.Panic(err)
		}

		exec := *exe
		runCommand(&exec)

		cs, err := readPrompt(fp)
		if err != nil {
			if err == skipNote {
				continue
			}
			log.Panic(err)
		}
		for _, card := range cs {
			editedCards <- card
		}
	}
	close(editedCards)
	wg.Done()
}

// does not yet support vim, vim needs terminal emulated (shell)
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
			if editor == "vi" || editor == "vim" {
				fmt.Printf("%s does not currently support vim, because vim requires an emulated terminal.", NAME)
			} else {
				linuxEditor = editor
			}
		}
		cmd = append(linuxShell, linuxEditor+" "+fp)
	default:
		panic("unknown platform.")
	}
	return
}

// createPrompt serialises the card and seperates all fields with 10 tildes (~~~~~)
func createPrompt(fp string, card *card2) error {
	ufile, err := os.OpenFile(fp, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0755)
	if err != nil {
		return err
	}
	defer saveClose(ufile)
	file := bufio.NewWriter(ufile)

	file.Write(frontSep)
	file.Write(card.front)
	file.WriteRune('\n')
	file.Write(backSep)
	file.Write(card.back)
	file.Write(tagsSep)
	for i := range card.tags {
		file.Write(card.tags[i])
		file.WriteRune('\n')
	}
	return file.Flush()
}

var skipLit = []byte("skip")
var skipNote = errors.New("Skip this note.")

// readPrompt reads in the user modified file. It also handles skiping notes
// with the skipNote error. The user may include mutliple notes in one file.
// If the file is empty or begins with "skip", readPrompt returns the skipNote error.
func readPrompt(fp string) ([]card2, error) {
	raw, err := os.ReadFile(fp)
	if err != nil {
		return nil, err
	}

	// check if file should be skipped explicitly:
	if len(raw) < len(skipLit) {
		return nil, skipNote
	}
	if bytes.EqualFold(raw[0:len(skipLit)], skipLit) {
		return nil, skipNote
	}

	// Parse the file
	pattern := string(frontSep) +
		`(.*\s)+` +
		string(backSep) +
		`(.*\s)+` +
		string(tagsSep) +
		`(.*\s)+` // set s flag: . also matches newline
	re := regexp.MustCompile(pattern)

	matches := re.FindAllSubmatchIndex(raw, -1)
	cards := make([]card2, 0, len(matches))
	for _, idxs := range matches {
		// fmt.Printf("%q\n", raw[idxs[0]:idxs[1]]) // whole
		// fmt.Printf("%q\n", raw[idxs[2]:idxs[3]]) // front (no newlines permitted, so right match), includes newline
		// fmt.Printf("%q\n", raw[idxs[4]:idxs[5]]) // last match of back
		// fmt.Printf("%q\n", raw[idxs[6]:idxs[7]]) // last match of tags inc. \n
		card := card2{}

		// front
		start, end := idxs[2], idxs[3]
		card.front = raw[start : end-len("\n")]

		// back
		start, end = end+len(backSep), idxs[5]
		card.back = raw[start:end]

		// tags:
		start, end = end+len(tagsSep), idxs[7]
		card.tags = bytes.Split(raw[start:end], []byte{'\n'})

		if VERBOSE {
			log.Println(card)
		}
		cards = append(cards, card)
	}

	if len(matches) == 0 {
		log.Printf("Could not find any matching pattern in:\n%s\nBe sure not to mess with the separators.", string(raw))
		return nil, nil
	}

	return cards, nil
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

func Serialiser(fp string, cards <-chan card2, wg *sync.WaitGroup) {
	var n int
	out, err := os.OpenFile(fp, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0700)
	if err != nil {
		panic(err)
	}
	defer saveClose(out)
	w := csv.NewWriter(out)

	for card := range cards {
		w.Write([]string{string(card.front), string(card.back), string(bytes.Join(card.tags, []byte{' '}))})
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

type options int

const (
	// Add options here.
	toMathJax options = 1 << iota
	includeMedia
)

func yieldDoMutation(mutations ...options) func(c *card2) {
	if len(mutations) == 0 {
		return func(c *card2) { return }
	}

	return func(c *card2) {
		for _, mu := range mutations {
			switch mu {
			case toMathJax:
				// turn $$...$$ into \(...\)
				mlPattern := `\$\$(.+?)\$\$`
				mlRe := regexp.MustCompile(mlPattern)
				c.front = mlRe.ReplaceAll(c.front, []byte(`\[$1\]`))
				c.back = mlRe.ReplaceAll(c.back, []byte(`\[$1\]`))
				
				// turn $...$ into \(...\)
				slPattern := `\$(.+?)\$`
				slRe := regexp.MustCompile(slPattern)
				c.front = slRe.ReplaceAll(c.front, []byte(`\($1\)`))
				c.back = slRe.ReplaceAll(c.back, []byte(`\($1\)`))				
			case includeMedia:
				// turn ![somedir/somefile](somedir/somefile) to <img src="somedir_somefile">
				// https://docs.ankiweb.net/importing.html
				pattern := `!\[(.+)/(.+)\]\(.+\)` // split the dirpath and the basename of file.
				re := regexp.MustCompile(pattern)
				c.front = re.ReplaceAll(c.front, []byte(`<img src="${1}_${2}">`)) // provides scoping
				c.back = re.ReplaceAll(c.back, []byte(`<img src="${1}_${2}">`))  //provides scoping
			}
		}
	}
}

func saveClose(f io.Closer) {
	if err := f.Close(); err != nil {
		panic(err)
	}
}