# md2anki
This is not a full featured script. Consider it a simple script to convert [notion](https://www.notion.so/) pages into Anki flashcards. Specifically, md2Anki only looks at 2 blocks:
1) Toggles
2) Headings

The algorithm will find all the toggles and use the "title" as the front and the body as the back side.
The headings will be converted to tags. For that, spaces will become underscores. A note only gets the tag of the last heading and all heading levels above (see example below). This adheres to the logical structure of headings. 
Example:
```
h1: ABC
...
toggles have tag: ABC
...
h2: b
...
toggles have tags: ABC b
...
h3: more detailed 
...
toggles have tags: ABC b more_detailed
...
h2: c
...
toggles have tags: ABC c   # note that the h3 tag was dropped and the h2 tag replaced.
...

h1: DEF
...
toggles have tag: DEF   # note that the h2 tag was dropped and the h1 tag replaced.
...

```

## Usage on Linux
First build the tool with go@1.16=<
```
$ go build -o md2anki main.go toggle.go
```
Then use with
```
$ ./md2anki <filepath to exported notion page>
```
It will open the default editor (nano, vim requires emulated terminal) and allow the user to modify the cards before they will be added to the importable file. The filename will be the `"{page_name}.txt"`.
To skip a note, type "skip" at the start of the file or delete the whole content and save.