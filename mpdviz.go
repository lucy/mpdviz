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
	scale = flag.Float64("scale", 3,
		"scale divisor (for spectrum)")

	file = flag.String("f", "/tmp/mpd.fifo",
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

var dbuf [][]bool

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

	ch := make(chan int16, 128)
	switch *vis {
	case "spectrum":
		go drawSpectrum(ch)
	case "wave":
		go drawWave(ch)
	default:
		fmt.Fprintf(os.Stderr, "mpdviz: unknown visualization %s\n"+
			"supported visualizations: spectrum, wave", *vis)
		os.Exit(1)
	}

	go func() {
		for {
			var i int16
			binary.Read(file, binary.LittleEndian, &i)
			ch <- i
		}
	}()

	// input handler
	for {
		ev := termbox.PollEvent()
		if ev.Ch == 0 && ev.Key == termbox.KeyCtrlC {
			termbox.Close()
			os.Exit(0)
		}
	}

}

func flush(both, upc, downc rune) {
	w, h := len(dbuf[0]), len(dbuf)
	for x := 0; x < h; x++ {
		for y := 0; y < w; y++ {
			if y%2 != 0 {
				continue
			}

			up, down := dbuf[x][y], dbuf[x][y+1]
			switch {
			case up && down:
				termbox.SetCell(x, y/2, both, on, off)
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
	for pos := 0; ; pos++ {
		w, h := len(dbuf), len(dbuf[0])
		if pos >= w {
			flush('█', '▀', '▄')
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

func drawSpectrum(c chan int16) {
	var (
		samples = 2048
		resn    = samples/2 + 1
		mag     = make([]float64, resn)
		in      = make([]float64, samples)
		out     = fftw.Alloc1d(resn)
		plan    = fftw.PlanDftR2C1d(in, out, fftw.Estimate)
	)

	// TODO: improve efficiency, possibly dither more frames
	for {
		w, h := len(dbuf), len(dbuf[0])
		for i := 0; i < samples; i++ {
			in[i] = float64(<-c)
		}

		plan.Execute()
		for i := 0; i < resn; i++ {
			mag[i] = cmplx.Abs(out[i]) / 1e5 * float64(h) / *scale
		}

		mlen := resn / w
		for i := 0; i < w; i++ {
			v := 0.0
			for _, m := range mag[mlen*i:][:mlen] {
				v += m
			}
			v /= float64(mlen)
			v = math.Min(float64(h), v)
			for j := h - 1; j > h-int(v); j-- {
				dbuf[i][j] = true
			}
		}

		flush('┃', '╹', '╻')
		clear()
	}
}

func die(args ...interface{}) {
	fmt.Fprintf(os.Stderr, "mpdviz: %s\n", fmt.Sprint(args...))
	os.Exit(1)
}
