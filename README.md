# md2anki
🛑 This **is not and will not be maintained** anymore! There is a significantly better alternative for notion to flashcards: [zorbi.cards](https://zorbi.cards) 🛑

# About
This is a script to convert Notion toggles to Anki flashcards. The supported format is limited to front and back with tags. Tags are deduced from the headings, see more on that below. It supports media and math conversions. Additionally, it runs on 
- Windows
- Linux and
- MacOs (Darwin).

## Installation
First of all, you need Golang version 1.16 or higher installed. From there on, you need to differentiate by platform. Make sure you are in the directory containing `main.go`.
###  Linux and Mac 
Run
```
$ go build -o md2anki main.go toggle.go
```
If you want to use md2anki from anywhere on your filesystem, add it to path or better yet, move it to a location already in path, like `/usr/local/bin`.
```
$ sudo mv ./md2anki /usr/local/bin
```
Now you can enjoy md2anki by typing `md2anki` and hitting enter. Everything else will be explained there.
### Windows
Run 
```
$ go build -o md2anki.exe main.go toggle.go
```
If you want to use md2anki from anywhere on your filesystem, add it to path. To do so, first find the path to the current `md2anki.exe` file. We will assume that path is `C:\Users\jake\Downloads\md2anki\md2anki.exe`.
Run
```
$ setx /M path "%path%;<YOURPATH>"
$ setx /M path "%path%;C:\Users\jake\Downloads\md2anki\md2anki.exe"
```
Now you can enjoy md2anki by typing `md2anki.exe` and hitting enter.

## Usage
If you added md2anki to your path, you can access it by typing md2anki. If you haven't, then you must provide the relative path to md2anki. After installation, that is most likely to be on Linux.
```
$ ./md2anki <filepath to exported notion page> [-math] [-media] [-verbose]
```
On Windows it is most likely
```
$ .\md2anki.exe <filepath to exported notion page> [-math] [-media] [-verbose]
```
When running md2anki, it will read in the exported Notion page and create flashcards. For every card, an editor window pops und and you can make changes to the card. If the first 4 letters of the file start with `skip` or the file is saved with all content deleted, no flashcard will be created.

Depending on your options, md2anki will rewrite math expression so that they are supported by Anki natively. If the media option is provided, md2anki will also enable support for media files and move them accordingly.

After the program finished, you will find a `{page_name}.txt` file in the current directory. You should import that file to Anki. Remember to select "Allow HTML Tags" in Anki, else the media files cannot be referenced. 

## Inner workings
Consider it a simple script to convert [notion](https://www.notion.so/) pages into Anki flashcards. Specifically, md2Anki only looks at 2 blocks:
1) Toggles
2) Headings

The algorithm will find all the toggles and use the "title" as the front and the body as the back side.
The headings will be converted to tags. For that, spaces are converted to  underscores. A note only gets the tag of the last heading and all heading levels above (see example below). This adheres to the logical structure of headings. 
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

## Feedback
If you find any bugs, please open a GitHub issue. If it saved you any time, please consider starring the project. Thank you and have fun.
