// Contains the daemon helper *not* the daemon itself
package main

import (
	"bytes"
	"net/http"

	jsoniter "github.com/json-iterator/go"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

type daemonMsg struct {
	URL  string
	Body map[string]any
}

var sendRoutineChan = make(chan daemonMsg)

var httpCli = &http.Client{}

func sendRoutine() {
	go func() {
		for msg := range sendRoutineChan {
			var buf bytes.Buffer
			json.NewEncoder(&buf).Encode(msg.Body)

			req, err := http.NewRequest("POST", "http://localhost:3939"+msg.URL, &buf)

			if err != nil {
				panic(err)
			}

			_, err = httpCli.Do(req)

			if err != nil {
				panic(err)
			}
		}
	}()
}

func notifyMsg(level string, msg string) {
	// Send message to daemon
	sendRoutineChan <- daemonMsg{
		URL: "/notify",
		Body: map[string]any{
			"loglevel": level,
			"message":  msg,
		},
	}
}

func sendProgress(done int, total int, col string) {
	// Send message to daemon
	sendRoutineChan <- daemonMsg{
		URL: "/progress",
		Body: map[string]any{
			"done":  done,
			"total": total,
			"col":   col,
		},
	}
}
