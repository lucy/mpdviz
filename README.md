MPDViz
------
This is a console visualizer for MPD. It has three modes:

![spectrum](http://goput.it/ji6.gif "spectrum")

![wave](http://goput.it/hnq.gif "wave")

![lines](http://goput.it/s9d.gif "lines")


    Usage of mpdviz:
      -c, --color="default"        Color to use
      -d, --dim=false              Turn off bright colors where possible
      -f, --file="/tmp/mpd.fifo"   Where to read pcm data from
      -i, --icolor=false           Color bars according to intensity (spectrum/lines)
          --imode="dumb"           Mode for colorisation (dumb, 256 or grayscale)
          --scale=2                Scale divisor (spectrum)
          --step=2                 Samples for each step (wave/lines)
      -v, --viz="wave"             Visualisation (spectrum, wave or lines)
