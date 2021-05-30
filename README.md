# md2anki
This is a simple script I wrote to test some programming patterns and prototype this idea.
There is a lot of room for improvement, especially when it comes to how I handle strings.
In particular, I should not use the bufio.Scanner but rather a custom implementation and then work
with byte slices all the way through. Where that is to difficult, I should use strings.Builder.


## Gotchas
Tags are carried over from the first declaration. You must add the @tag at line
start, but do not have to type it out.