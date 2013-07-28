MPDViz
------
This is a console visualizer for MPD. It has two modes:

![spectrum](http://goput.it/qyp.gif "spectrum")

![wave](http://goput.it/511.gif "wave")


	Usage of mpdviz:
	  -c, --color="default"        Color to use
	  -d, --dim=false              Turn off bright colors where possible
	  -f, --file="/tmp/mpd.fifo"   Where to read pcm data from
	  -i, --icolor=false           Color bars according to intensity (spectrum)
	      --imode="dumb"           Mode for colorisation (dumb, 256 or grayscale)
	      --scale=2                Scale divisor (spectrum)
	      --step=2                 Samples to average in each column (wave)
	  -t, --tick=40ms              Minimum time to spend on a frame, set higher to
	                               lower CPU usage. ncmpcpp uses 40ms (25fps).
	  -v, --viz="wave"             Visualisation (spectrum or wave)
