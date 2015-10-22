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
	dim   = flag.BoolP("dim", "d", false,
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

type cell struct {
	on bool
	ic int
}

type x4 struct {
	arr   []cell
	w, h  int
	color bool
}

func (x *x4) fix() {
	x.color = *icolor
	x.w, x.h = termbox.Size()
	x.w, x.h = x.w*2, x.h*2
	l := x.w * x.h
	if l > cap(x.arr) {
		x.arr = make([]cell, l)
		x.arr = x.arr[:l]
	} else {
		x.arr = x.arr[:l]
		for i := range x.arr {
			x.arr[i] = cell{}
		}
	}
}

func (b *x4) set(x, y int, c cell) {
	if y*b.w+x >= len(b.arr) || x < 0 || y < 0 {
		return
	}
	b.arr[y*b.w+x] = c
}

func (b *x4) get(x, y int) cell {
	if y*b.w+x >= len(b.arr) || x < 0 || y < 0 {
		return cell{}
	}
	return b.arr[y*b.w+x]
}

func (b *x4) do() {
	w, h := b.w/2, b.h/2
	for x := 0; x < w; x++ {
		for y := 0; y < h; y++ {
			xx, yy := x*2, y*2
			ulc, urc, blc, brc := b.get(xx, yy), b.get(xx+1, yy), b.get(xx, yy+1), b.get(xx+1, yy+1)
			if !(ulc.on || urc.on || blc.on || brc.on) {
				continue
			}
			c := cellc(ulc.on, urc.on, blc.on, brc.on)
			var ic int
			var a int
			if b.color {
				if ulc.on {
					ic += ulc.ic
					a++
				}
				if urc.on {
					ic += urc.ic
					a++
				}
				if blc.on {
					ic += blc.ic
					a++
				}
				if brc.on {
					ic += brc.ic
					a++
				}
				if a > 0 {
					on = iColors[ic/a]
				}
			}
			termbox.SetCell(x, y, c, on, off)
		}
	}
}

func cellc(ul bool, ur bool, bl bool, br bool) rune {
	if ul {
		if ur {
			if bl {
				if br {
					return '█'
				} else {
					return '▛'
				}
			} else {
				if br {
					return '▜'
				} else {
					return '▀'
				}
			}
		} else {
			if bl {
				if br {
					return '▙'
				} else {
					return '▌'
				}
			} else {
				if br {
					return '▚'
				} else {
					return '▘'
				}
			}
		}
	} else {
		if ur {
			if bl {
				if br {
					return '▟'
				} else {
					return '▞'
				}
			} else {
				if br {
					return '▐'
				} else {
					return '▝'
				}
			}
		} else {
			if bl {
				if br {
					return '▄'
				} else {
					return '▖'
				}
			} else {
				if br {
					return '▗'
				} else {
					return ' '
				}
			}
		}
	}
}

func drawWave(file *os.File, end chan bool) {
	defer close(end)
	var (
		inRaw  []int16
		ibound = len(iColors) - 1
		ilen   = float64(len(iColors))
		back   = x4{}
	)
	for {
		back.fix()
		w, h := back.w, back.h
		if s := 1 + w * *step; len(inRaw) != s {
			inRaw = make([]int16, s)
		}

		if readInt16s(file, inRaw) != nil {
			return
		}

		half_h := float64(h / 2)
		div := maxInt16 / half_h
		var vi1, vi3 int
		var v1, v3 float64
		pos := 0
		{
			for i := 0; i < *step; i++ {
				v1 += float64(inRaw[pos**step+i])
			}
			v1 /= float64(*step)
			vi1 = int(v1/div + half_h)
		}

		for pos := 0; pos < w; pos++ {
			for i := 0; i < *step; i++ {
				v3 += float64(inRaw[pos**step+i])
			}
			v3 /= float64(*step)
			vi3 = int(v3/div + half_h)

			up, down := vi1, vi1
			if vi3 > up {
				up = vi3
			}
			if vi3 < down {
				down = vi3
			}

			if up-down < 1 {
				down--
			}

			for ; down < up; down++ {
				back.set(pos, down, cell{true, min(ibound, abs(int(v1/maxInt16*ilen)))})
			}

			vi1, vi3 = vi3, 0
			v1, v3 = v3, 0
		}
		back.do()
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
		back  = x4{}
	)

	for {
		back.fix()
		w, h := back.w, back.h
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
			hd := int(v * float64(h))
			for j := h; j > h-hd; j-- {
				back.set(i, j, cell{true, min(ilen, int(v*flen))})
			}
		}

		back.do()
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

type ringbuf struct {
	buf   []pair
	start int
}

func (r *ringbuf) push(p pair) {
	r.buf[r.start] = p
	r.start = (r.start + 1) % len(r.buf)
}

func (r *ringbuf) get(i int) pair {
	return r.buf[(r.start+i)%len(r.buf)]
}

func drawLines(file *os.File, end chan bool) {
	defer close(end)
	var (
		c, bc coord
		inraw = make([]int16, *step)
		hist  = ringbuf{make([]pair, 1000), 0}
		ilen  = len(iColors) - 1
		filen = float64(ilen)
	)

	for {
		if readInt16s(file, inraw) == io.EOF {
			return
		}

		var raw float64
		for _, rawi := range inraw {
			raw = float64(rawi)
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
			hist.push(pair{c.x, c.y})
			a := hist.get(len(hist.buf) - 1)

			termbox.SetCell(a.x, a.y, '#', on, off)
			//termbox.SetCell(c.x, c.y, '#', on, off)

			termbox.Flush()
		}
	}
}
