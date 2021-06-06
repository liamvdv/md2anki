package main

import (
	"fmt"
	"log"
	"os"
	"regexp"
	"testing"
)

func TestPattern(t *testing.T) {
	testPattern()
	fp := "" // required you to export a page.
	process(fp)
}

func TestMediaPattern(t *testing.T) {
	c := &card2{
		front: []byte("What is the mathmatical formula to calculate the information content of a piece of data in bits?"),
		back: []byte(`In information theory, the entropy $H(X)$ is the average amount of information contained in each piece of data received about the value of X. Calculate the weighted average:
		$$H(X)=E(I(X))= \sum_{i=1}^Np_i\cdot\log_2(\frac{1}{p_i})$$
	
		Getting less than H(X) means that ambiguity cannot be resolved. Sending more than H(X) bits means where resolving the ambiguity, but doing to quite inefficiently. Thus, entropy is the best possible encoding (theoratical lower bound).
	
		![somedirpath/Untitled 1.png](somedirpath/Untitled 1.png) aklsdf asdlfj asdklf`),
	}

	pattern := `!\[(.+)/(.+)\]\(.+\)` // split the dirpath and the basename of file.
	re := regexp.MustCompile(pattern)
	fmt.Println(re.String())
	fmt.Print(re.FindAllSubmatchIndex(c.front, -1))
	for i, s := range re.FindAllSubmatchIndex(c.front, -1) {
		fmt.Println("ran")
		fmt.Println(i, s)
	}
	c.back = re.ReplaceAll(c.back, []byte(`<img src="${1}_${2}">`))
	fmt.Printf("%q\n", c.back)
}

// test func
func process(fp string) {
	raw, err := os.ReadFile(fp)
	if err != nil {
		log.Panic(err)
	}
	hr := regexp.MustCompile(headingExp)
	hMatches := hr.FindAllSubmatch(raw, -1)
	idxMatches := hr.FindAllSubmatchIndex(raw, -1)
	fmt.Println("Heading matches:", len(hMatches))
	fmt.Println(idxMatches)
	for _, match := range hMatches {
		// match is an array of the found matches as bytes.
		for _, s := range match {
			fmt.Printf("%q\n", string(s))
		}
		// build tag stack... need order of regexp... use index? and then describe the underlying raw data?
	}

	tr := regexp.MustCompile(toggleExp)
	tMatches := tr.FindAllSubmatch(raw, -1)
	idxMatches = tr.FindAllSubmatchIndex(raw, -1)
	fmt.Println(idxMatches)
	fmt.Println("toggle matches:")
	for _, match := range tMatches {
		// match is an array of the found matches as bytes.
		for _, s := range match {
			fmt.Printf("%q\n", string(s))
		}
		// build tag stack... need order of regexp... use index? and then describe the underlying raw data?
	}

	fmt.Printf("%#v\n", string(raw))
}

func testPattern() error {
	tog := `
- How to switch to a certain window (labelled with a number)

	first, you need to finish work. 
	Then you can go home.

- How to switch to another pane

	<ident>

`
	tr := regexp.MustCompile(toggleExp)
	fmt.Println("Match toggle:")
	fmt.Printf("Pattern: %q\n", tr.String())
	fmt.Printf("Search: %#v\n", tog)
	fmt.Println("Matches:")
	for _, slice := range tr.FindAllStringSubmatch(tog, -1) {
		fmt.Printf("%#v\n", slice)
	}

	head := `
# Terminal multiplexer
bar foo
## Modes
### Commands foo bar 
`

	hr := regexp.MustCompile(headingExp)
	fmt.Println("\nMatch heading:")
	fmt.Printf("Pattern: %q\n", hr.String())
	fmt.Printf("Search: %#v\n", head)
	hMatches := hr.FindAllStringSubmatch(head, -1)
	fmt.Printf("Matches: %#v\n", hMatches)
	fmt.Printf("%v\n", newTag([]byte(hMatches[0][0]))) // not 0, 0 is full string
	return nil
}