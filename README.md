MPDViz
------
This is a console visualizer for MPD. It has two modes:

![spectrum](http://goput.it/qyp.gif "spectrum")

![wave](http://goput.it/511.gif "wave")

	Usage of mpdviz:
	  -c, --color="blue"           Color to use
	  -d, --dim=false              Turn off bold
	  -f, --file="/tmp/mpd.fifo"   Where to read fifo output from
	  -i, --intensitycolor=false   color bars based on intensity (spectrum)
	      --scale=2                Scale divisor (spectrum)
	      --step=2                 Samples to average in each column (wave)
	  -v, --viz="wave"             Visualization (spectrum or wave)
