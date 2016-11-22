package ui

import (
	"fmt"
	"log"

	"github.com/lhz/considerate/hvsc"
	"github.com/lhz/considerate/player"
	"github.com/nsf/termbox-go"
)

var w, h int
var listOffset, listPos int

func Setup() {
	err := termbox.Init()
	if err != nil {
		log.Panicln(err)
	}

	w, h = termbox.Size()
	listOffset = 0
	listPos = 0

	termbox.SetInputMode(termbox.InputEsc | termbox.InputMouse)
	draw()
}

func Run() {
	defer termbox.Close()

	for quit := false; !quit; {
		switch ev := termbox.PollEvent(); ev.Type {
		case termbox.EventKey:
			switch ev.Key {
			case termbox.KeyCtrlC, termbox.KeyEsc:
				quit = true
			case termbox.KeyPgup:
				listOffset -= h
				if listOffset < 0 {
					listOffset = 0
				}
				draw()
			case termbox.KeyPgdn:
				listOffset += h
				if listOffset > hvsc.NumTunes-1 {
					listOffset = hvsc.NumTunes - 1
				}
				draw()
			case termbox.KeyArrowUp:
				listPos--
				if listPos < 0 {
					listPos = h - 1
					listOffset -= h
					if listOffset < 0 {
						listOffset = 0
					}
				}
				draw()
			case termbox.KeyArrowDown:
				listPos++
				if listPos >= h {
					listPos = 0
					listOffset += h
					if listOffset > hvsc.NumTunes-1 {
						listOffset = hvsc.NumTunes - 1
					}
				}
				draw()
			case termbox.KeyEnter:
				player.Play(hvsc.Tunes[listOffset + listPos].FullPath(), 1)
			}
		case termbox.EventResize:
			w, h = ev.Width, ev.Height
			draw()
		case termbox.EventError:
			panic(ev.Err)
		}
	}
}

func draw() {
	termbox.Clear(termbox.ColorDefault, termbox.ColorDefault)
	for y := 0; y < h; y++ {
		tune := hvsc.Tunes[y+listOffset]
		bg := termbox.ColorDefault
		if y == listPos {
			bg = termbox.ColorBlack
		}
		writeAt(0, y, fmt.Sprintf("%05d", y+listOffset+1), termbox.ColorRed, termbox.ColorDefault)
		writeAt(12, y, tune.Header.Name, termbox.ColorBlue, bg)
		writeAt(45, y, tune.Header.Author, termbox.ColorGreen, bg)
		writeAt(78, y, tune.Header.Released, termbox.ColorYellow, bg)
		writeAt(112, y, tune.Path, termbox.ColorDefault, bg)
	}
	termbox.Flush()
}

func writeAt(x, y int, value string, fg, bg termbox.Attribute) {
	i := 0
	for _, c := range value {
		termbox.SetCell(x+i, y, c, fg, bg)
		i++
	}
}
