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
	"fmt"
	"io"
	"math/cmplx"
	"os"

	"github.com/jackvalmadre/go-fftw"
	flag "github.com/neeee/pflag"
	"github.com/nsf/termbox-go"
)

var (
	color = flag.StringP("color", "c", "blue", "Color to use")
	dim   = flag.BoolP("dim", "d", false, "Turn off bold")

	step = flag.Int("step", 2,
		"Number of samples to average in each column (for wave)")
	scale = flag.Float64("scale", 2,
		"Scale divisor (for spectrum)")

	filename = flag.StringP("file", "f", "/tmp/mpd.fifo",
		"Where to read fifo output from")
	vis = flag.StringP("viz", "v", "wave",
		"Visualization (spectrum or wave)")
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

var draw func(chan int16)

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
	go draw(ch)

	// input handler
	go func() {
		for {
			ev := termbox.PollEvent()
			if ev.Ch == 0 && ev.Key == termbox.KeyCtrlC {
				close(end)
				return
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

func drawWave(c chan int16) {
	w, h := termbox.Size()
	h *= 2
	for pos := 0; ; pos++ {
		if pos >= w {
			pos = 0
			w, h = termbox.Size()
			h *= 2
			termbox.Flush()
			termbox.Clear(0, 0)
		}

		var v float64
		for i := 0; i < *step; i++ {
			v += float64(<-c)
		}

		half_h := float64(h / 2)
		vi := int(v/float64(*step)/(32768/half_h) + half_h)
		if vi%2 == 0 {
			termbox.SetCell(pos, vi/2, '▀', on, off)
		} else {
			termbox.SetCell(pos, vi/2, '▄', on, off)
		}
	}
}

func drawSpectrum(c chan int16) {
	var (
		samples int
		resn    int = -1
		in      []float64
		out     = fftw.Alloc1d(1) // hack to make the Free1d call not panic
		plan    *fftw.Plan
	)

	for {
		w, h := termbox.Size()
		h *= 2
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
			vi := int(v)
			for j := h - 1; j > h-vi; j-- {
				termbox.SetCell(i, j/2, '┃', on, off)
			}
			if vi%2 != 0 {
				termbox.SetCell(i, (h-vi)/2, '╻', on, off)
			}
		}

		termbox.Flush()
		termbox.Clear(0, 0)
	}
}
