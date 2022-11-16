// Contains the daemon helper *not* the daemon itself
package cli

import (
	"fmt"

	"github.com/fatih/color"
	"github.com/vbauerster/mpb/v8"
)

var warningFunc = color.New(color.FgYellow).SprintFunc()
var errorFunc = color.New(color.FgRed).SprintFunc()
var debugFunc = color.New(color.FgHiCyan).SprintFunc()
var infoFunc = color.New(color.FgHiGreen).SprintFunc()

var mb *mpb.Progress
var bar *mpb.Bar

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

func notifyDone(done int, total int, col string) {
	// Send message to daemon
}
