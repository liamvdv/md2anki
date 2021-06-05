package main

import (
	"fmt"
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
