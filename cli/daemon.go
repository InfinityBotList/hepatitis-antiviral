// Contains the daemon helper *not* the daemon itself
package cli

import (
	"fmt"

	"github.com/fatih/color"
	"github.com/vbauerster/mpb/v8"
	"github.com/vbauerster/mpb/v8/decor"
)

var warningFunc = color.New(color.FgYellow).SprintFunc()
var errorFunc = color.New(color.FgRed).SprintFunc()
var debugFunc = color.New(color.FgHiCyan).SprintFunc()
var infoFunc = color.New(color.FgHiGreen).SprintFunc()

var mb *mpb.Progress
var Bar *mpb.Bar

func NotifyMsg(level string, msg string) {
	if level == "warning" {
		level = warningFunc(level)
	} else if level == "error" {
		level = errorFunc(level)
	} else if level == "debug" {
		level = debugFunc(level)
	} else if level == "info" {
		level = infoFunc(level)
	} else {
		panic("invalid log level")
	}

	// Send message to daemon
	mb.Write([]byte(fmt.Sprintln(level+":", msg)))
}

func StartBar(schemaName string, count int64, removeOld bool) (b *mpb.Bar) {
	if Bar != nil && removeOld {
		Bar.Abort(true)
		Bar.Wait()
		mb.Wait()
	}

	if removeOld {
		mb = mpb.New(mpb.WithWidth(64))
	}

	bar := mb.New(
		count,
		// BarFillerBuilder with custom style
		mpb.BarStyle(),
		mpb.PrependDecorators(
			// display our name with one space on the right
			decor.Name(schemaName, decor.WC{W: len(schemaName) + 1, C: decor.DidentRight}),
			// replace ETA decorator with "done" message, OnComplete event
			decor.OnComplete(
				decor.AverageETA(decor.ET_STYLE_GO, decor.WC{W: 4}), "done",
			),
		),
		mpb.AppendDecorators(
			// Percentage decorator with width reservation and no extra space
			decor.Percentage(),
			// Set a counter at the end of the bar
			decor.Counters(0, " [%d/%d]", decor.WC{W: len(schemaName) + 1, C: decor.DidentRight}),
		),
		mpb.BarRemoveOnComplete(),
	)

	bar.SetCurrent(0)

	if removeOld {
		Bar = bar
	}

	return bar
}
