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
	"io"
	"math"
	"math/cmplx"
	"os"

	"github.com/jackvalmadre/go-fftw"
	"github.com/nsf/termbox-go"
)

var (
	color = flag.String("c", "blue", "which color to use")
	dim   = flag.Bool("d", false, "don't use bold")

	step = flag.Int("step", 2,
		"number of samples to average in each column (for wave)")
	scale = flag.Float64("scale", 2,
		"scale divisor (for spectrum)")

	filename = flag.String("f", "/tmp/mpd.fifo",
		"where to read fifo output from")
	vis = flag.String("v", "wave",
		"choose visualization (spectrum or wave)")
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
var off = termbox.ColorDefault
var upc, downc, bothc rune

var draw func(chan int16)
var dbuf [][]bool

var termboxRan bool

func die(format string, args ...interface{}) {
	if termboxRan {
		termbox.Close()
	}
	fmt.Fprintf(os.Stderr, format, args...)
	os.Exit(1)
}

func init() {
	flag.Parse()
	var ok bool
	on, ok = colors[*color]
	if !ok {
		die("Unknown color \"%s\"\n", *color)
	}
	if !*dim {
		on = on | termbox.AttrBold
	}

	switch *vis {
	case "spectrum":
		draw = drawSpectrum
	case "wave":
		draw = drawWave
	default:
		die("Unknown visualisation \"%s\"\n"+
			"Supported: spectrum, wave\n", *vis)
	}
}

func main() {
	file, err := os.Open(*filename)
	if err != nil {
		die("%s\n", err)
	}
	defer file.Close()

	err = termbox.Init()
	if err != nil {
		die("%s\b", err)
	}
	termboxRan = true
	defer termbox.Close()

	ch := make(chan int16, 128)
	end := make(chan string)

	// drawer
	clear()
	go draw(ch)

	// input handler
	go func() {
		for {
			ev := termbox.PollEvent()
			if ev.Ch == 0 && ev.Key == termbox.KeyCtrlC {
				close(end)
			}
		}
	}()

	// file reader
	go func() {
		var i int16
		for binary.Read(file, binary.LittleEndian, &i) != io.EOF {
			ch <- i
		}
		close(end)
	}()

	<-end
}

// print everything in buffer
func flush() {
	w, h := len(dbuf[0]), len(dbuf)
	for x := 0; x < h; x++ {
		for y := 0; y < w; y += 2 {
			up, down := dbuf[x][y], dbuf[x][y+1]
			switch {
			case up && down:
				termbox.SetCell(x, y/2, bothc, on, off)
			case up:
				termbox.SetCell(x, y/2, upc, on, off)
			case down:
				termbox.SetCell(x, y/2, downc, on, off)
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

func drawWave(c chan int16) {
	bothc, upc, downc = '█', '▀', '▄'
	w, h := len(dbuf), len(dbuf[0])
	for pos := 0; ; pos++ {
		if pos >= w {
			flush()
			clear()
			pos = 0
			w, h = len(dbuf), len(dbuf[0])
		}

		var v float64
		for i := 0; i < *step; i++ {
			v += float64(<-c)
		}

		half_h := float64(h / 2)
		v = v/float64(*step)/(32768/half_h) + half_h
		vi := int(v)
		if vi > h-1 {
			vi = h - 1
		} else if v < 0 {
			vi = 0
		}
		dbuf[pos][vi] = true
	}
}

func drawSpectrum(c chan int16) {
	bothc, upc, downc = '┃', '╹', '╻'
	var (
		samples int
		resn    int = -1
		in      []float64
		out     = fftw.Alloc1d(1) // hack to make the Free1d call not panic
		plan    *fftw.Plan
	)

	for {
		w, h := len(dbuf), len(dbuf[0])
		if resn != w && !(w <= 1) {
			fftw.Free1d(out)
			resn = w
			samples = (w - 1) * 2
			in = make([]float64, samples)
			out = fftw.Alloc1d(resn)
			plan = fftw.PlanDftR2C1d(in, out, fftw.Estimate)
		}

		for i := 0; i < samples; i++ {
			in[i] = float64(<-c)
		}

		plan.Execute()
		for i := 0; i < w; i++ {
			v := cmplx.Abs(out[i]) / 1e5 * float64(h) / *scale
			v = math.Min(float64(h), v)
			for j := h - 1; j > h-int(v); j-- {
				dbuf[i][j] = true
			}
		}

		flush()
		clear()
	}
}
