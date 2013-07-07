/*
Copyright (C) 2013 Lucy

Permission is hereby granted, free of charge, to any person obtaining a
copy of this software and associated documentation files (the "Software"),
to deal in the Software without restriction, including without limitation
the rights to use, copy, modify, merge, publish, distribute, sublicense,
and/or sell copies of the Software, and to permit persons to whom the
Software is furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in
all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING
FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER
DEALINGS IN THE SOFTWARE.package main
*/

package main

import (
	"encoding/binary"
	"flag"
	"fmt"
	"os"

	"github.com/nsf/termbox-go"
)

var dbuf [][]bool

var (
	step  = flag.Int("s", 2, "number of samples to average in each column")
	dim   = flag.Bool("d", false, "don't use bold")
	color = flag.String("c", "blue", "which color to use")
	file  = flag.String("f", "/tmp/mpd.fifo",
		"where to read fifo output from")
)

var colors = map[string]termbox.Attribute{
	"default": termbox.ColorDefault,
	"black":   termbox.ColorBlack,
	"red":     termbox.ColorRed,
	"green":   termbox.ColorGreen,
	"yellow":  termbox.ColorYellow,
	"blue":    termbox.ColorBlue,
	"magenta": termbox.ColorMagenta,
	"cyan":    termbox.ColorCyan,
	"white":   termbox.ColorWhite,
}

var on termbox.Attribute
var off = termbox.ColorBlack

func main() {
	flag.Parse()
	var ok bool
	on, ok = colors[*color]
	if !ok {
		die("unknown color " + *color)
	}
	if !*dim {
		on = on | termbox.AttrBold
	}

	file, err := os.Open(*file)
	if err != nil {
		die(err)
	}

	err = termbox.Init()
	if err != nil {
		die(err)
	}
	defer termbox.Close()

	clear()

	// input handler
	go func() {
		for {
			ev := termbox.PollEvent()
			if ev.Ch == 0 && ev.Key == termbox.KeyCtrlC {
				termbox.Close()
				os.Exit(0)
			}
		}
	}()

	ch := make(chan int16, 128)
	go draw(ch)
	for {
		var i int16
		binary.Read(file, binary.LittleEndian, &i)
		ch <- i
	}
}

func flush() {
	w, h := len(dbuf[0]), len(dbuf)
	for x := 0; x < h; x++ {
		for y := 0; y < w; y++ {
			if y%2 != 0 {
				continue
			}

			up, down := dbuf[x][y], dbuf[x][y+1]
			switch {
			case up:
				termbox.SetCell(x, y/2, '▀', on, off)
			case down:
				termbox.SetCell(x, y/2, '▄', on, off)
			}
		}
	}
	termbox.Flush()
}

func clear() {
	termbox.Clear(0, 0)
	w, h := termbox.Size()
	h *= 2
	dbuf = make([][]bool, w)
	for i := 0; i < w; i++ {
		dbuf[i] = make([]bool, h)
		for j := 0; j < h; j++ {
			dbuf[i][j] = false
		}
	}
}

func draw(c chan int16) {
	for pos := 0; ; pos++ {
		w, h := len(dbuf), len(dbuf[0])
		if pos >= w {
			flush()
			clear()
			pos = 0
		}

		var v float64
		for i := 0; i < *step; i++ {
			v += float64(<-c)
		}

		half_h := float64(h / 2)
		v = (v/float64(*step))/(32768/half_h) + half_h
		dbuf[pos][int(v)] = true
	}
}

func die(args ...interface{}) {
	fmt.Fprintf(os.Stderr, "mpdviz: %s\n", fmt.Sprint(args...))
	os.Exit(1)
}
