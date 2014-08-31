// This code is fucking awful because I didn't know anything about audio
// when it was written (and probably still don't by the time anyone reads
// this).

/*
Copyright (C) 2013-2014 Lucy

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
DEALINGS IN THE SOFTWARE.
*/

package main

import (
	"fmt"
	"io"
	"math/cmplx"
	"os"

	"github.com/lucy/go-fftw"
	flag "github.com/lucy/pflag"
	"github.com/lucy/termbox-go"
)

var (
	color = flag.StringP("color", "c", "default", "Color to use")
	dim = flag.BoolP("dim", "d", false,
		"Turn off bright colors where possible")

	step  = flag.Int("step", 2, "Samples for each step (wave/lines)")
	scale = flag.Float64("scale", 2, "Scale divisor (spectrum)")

	icolor = flag.BoolP("icolor", "i", false,
		"Color bars according to intensity (spectrum/lines)")
	imode = flag.String("imode", "dumb",
		"Mode for colorisation (dumb, 256 or grayscale)")

	filename = flag.StringP("file", "f", "/tmp/mpd.fifo",
		"Where to read pcm data from")
	vis = flag.StringP("viz", "v", "wave",
		"Visualisation (spectrum, wave or lines)")
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

var iColors []termbox.Attribute

var (
	on  = termbox.ColorDefault
	off = termbox.ColorDefault
)

var maxInt16 float64 = 1<<15 - 1

func warn(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, format, args...)
}

func main() {
	flag.Parse()

	if cl, ok := colors[*color]; !ok {
		warn("Unknown color \"%s\"\n", *color)
		return
	} else {
		on = cl
	}

	if !*dim {
		on = on | termbox.AttrBold
	}

	switch *imode {
	case "dumb":
		iColors = []termbox.Attribute{
			termbox.ColorBlue,
			termbox.ColorCyan,
			termbox.ColorGreen,
			termbox.ColorYellow,
			termbox.ColorRed,
		}
		if !*dim {
			for i := range iColors {
				iColors[i] = iColors[i] + 8
			}
		}
	case "256":
		iColors = []termbox.Attribute{
			21, 27, 39, 45, 51, 86, 85, 84, 82,
			154, 192, 220, 214, 208, 202, 196,
		}
	case "grayscale":
		const num = 19
		iColors = make([]termbox.Attribute, num)
		for i := termbox.Attribute(0); i < num; i++ {
			iColors[i] = i + 255 - num
		}
	default:
		warn("Unsupported mode: \"%s\"\n", *imode)
		return
	}

	var draw func(*os.File, chan bool)
	switch *vis {
	case "spectrum":
		draw = drawSpectrum
	case "wave":
		draw = drawWave
	case "lines":
		draw = drawLines
	default:
		warn("Unknown visualisation \"%s\"\n"+
			"Supported: spectrum, wave\n", *vis)
		return
	}

	file, err := os.Open(*filename)
	if err != nil {
		warn("%s\n", err)
		return
	}
	defer file.Close()

	err = termbox.Init()
	if err != nil {
		warn("%s\b", err)
		return
	}
	defer termbox.Close()

	end := make(chan bool)
	go draw(file, end)

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

	<-end
}

func size() (int, int) {
	w, h := termbox.Size()
	return w, h * 2
}

func drawWave(file *os.File, end chan bool) {
	defer close(end)
	var (
		inRaw  []int16
		ibound = len(iColors) - 1
		ilen   = float64(len(iColors))
	)
	for {
		w, h := size()
		if s := w * *step; len(inRaw) != s {
			inRaw = make([]int16, s)
		}

		if readInt16s(file, inRaw) != nil {
			return
		}

		half_h := float64(h / 2)
		div := maxInt16 / half_h
		for pos := 0; pos < w; pos++ {
			var v float64
			for i := 0; i < *step; i++ {
				v += float64(inRaw[pos**step+i])
			}
			v /= float64(*step)

			if *icolor {
				on = iColors[min(ibound, abs(int(v/(maxInt16/ilen))))]
			}

			vi := int(v/div + half_h)
			if vi%2 == 0 {
				termbox.SetCell(pos, vi/2, '▀', on, off)
			} else {
				termbox.SetCell(pos, vi/2, '▄', on, off)
			}
		}

		termbox.Flush()
		termbox.Clear(off, off)
	}
}

func drawSpectrum(file *os.File, end chan bool) {
	defer close(end)
	var (
		ilen  = len(iColors) - 1
		flen  = float64(len(iColors))
		resn  = -1
		in    []float64
		inRaw []int16
		out   []complex128
		plan  *fftw.Plan
	)

	for {
		w, h := size()
		if resn != w {
			w := max(2, w)
			if out != nil {
				fftw.Free1d(out)
			}
			resn = w
			samples := (w - 1) * 2
			in = make([]float64, samples)
			inRaw = make([]int16, samples)
			out = fftw.Alloc1d(resn)
			plan = fftw.PlanDftR2C1d(in, out, fftw.Measure)
		}

		if readInt16s(file, inRaw) != nil {
			return
		}

		for i := range inRaw {
			in[i] = float64(inRaw[i])
		}

		plan.Execute()
		for i := 0; i < w; i++ {
			v := cmplx.Abs(out[i]) / 1e5 / *scale
			if *icolor {
				on = iColors[min(ilen, int(v*flen))]
			}
			hd := int(v * float64(h))
			for j := h - 1; j > h-hd; j-- {
				termbox.SetCell(i, j/2, '┃', on, off)
			}
			if hd%2 == 0 {
				termbox.SetCell(i, (h-hd)/2, '╻', on, off)
			}
		}

		termbox.Flush()
		termbox.Clear(off, off)
	}
}

type pair struct{ x, y int }

var dirs = [9]pair{
	{1, 1}, {-1, -1},
	{1, -1}, {-1, 1},
	{1, 0}, {-1, 0},
	{0, 1}, {0, -1},
	{0, 0},
}

type coord struct{ x, y, dir int }

func (c *coord) step(x, y int) {
	c.x += dirs[c.dir].x
	c.y += dirs[c.dir].y
	c.x, c.y = mod(c.x, x), mod(c.y, y)
}

func drawLines(file *os.File, end chan bool) {
	defer close(end)
	var (
		c, bc coord
		inraw = make([]int16, *step)
		hist  = make([]pair, 1000)
		ilen  = len(iColors) - 1
		filen = float64(ilen)
	)

	for {
		if readInt16s(file, inraw) == io.EOF {
			return
		}

		var raw float64
		for i := range inraw {
			raw += float64(inraw[i])
		}
		raw /= float64(*step)
		c.dir = min(8, abs(int(raw/maxInt16*8)))
		bc.dir = 8 - c.dir
		if *icolor {
			on = iColors[min(ilen, abs(int(raw/maxInt16*filen)))]
		}

		w, h := termbox.Size()
		bc.step(w, h)
		c.step(w, h)

		// TODO: make this more efficient, somehow
		hist = append(hist[1:], pair{c.x, c.y})

		termbox.SetCell(hist[0].x, hist[0].y, ' ', off, off)
		termbox.SetCell(c.x, c.y, '#', on, off)

		termbox.Flush()
	}
}
