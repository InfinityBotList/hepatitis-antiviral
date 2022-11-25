package main

import (
	"hepatitis-antiviral/cli"
	"math/rand"
	"strings"
	"time"
	"unsafe"

	"golang.org/x/exp/slices"
)

type link struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

func parseLink(key string, link string) string {
	if strings.ToLower(link) == "none" {
		return ""
	}

	if strings.HasPrefix(link, "http://") {
		return strings.Replace(link, "http://", "https://", 1)
	}

	if strings.HasPrefix(link, "https://") {
		return link
	}

	cli.NotifyMsg("info", "Possibly Invalid URL found: "+link)

	if key == "Support" && !strings.Contains(link, " ") {
		link = strings.Replace(link, "www", "", 1)
		if strings.HasPrefix(link, "discord.gg/") {
			link = "https://discord.gg/" + link[11:]
		} else if strings.HasPrefix(link, "discord.com/invite/") {
			link = "https://discord.gg/" + link[19:]
		} else if strings.HasPrefix(link, "discord.com/") {
			link = "https://discord.gg/" + link[12:]
		} else {
			link = "https://discord.gg/" + link
		}
		cli.NotifyMsg("info", "Succesfully fixed support link to"+link)
		return link
	} else {
		// But wait, it may be safe still
		split := strings.Split(link, "/")[0]
		tldLst := strings.Split(split, ".")

		if len(tldLst) > 1 && (len(tldLst[len(tldLst)-1]) == 2 || slices.Contains([]string{
			"com",
			"net",
			"org",
			"fun",
			"app",
			"dev",
			"xyz",
		}, tldLst[len(tldLst)-1])) {
			cli.NotifyMsg("info", "Fixed found URL link to https://"+link)
			return "https://" + link
		} else {
			if strings.HasPrefix(link, "https://") {
				return link
			}

			cli.NotifyMsg("warning", "Removing invalid link: "+link)
			return ""
		}
	}
}

const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
const (
	letterIdxBits = 6                    // 6 bits to represent a letter index
	letterIdxMask = 1<<letterIdxBits - 1 // All 1-bits, as many as letterIdxBits
	letterIdxMax  = 63 / letterIdxBits   // # of letter indices fitting in 63 bits
)

func RandString(n int) string {
	// https://stackoverflow.com/questions/22892120/how-to-generate-a-random-string-of-a-fixed-length-in-go

	var src = rand.NewSource(time.Now().UnixNano())

	b := make([]byte, n)
	// A src.Int63() generates 63 random bits, enough for letterIdxMax characters!
	for i, cache, remain := n-1, src.Int63(), letterIdxMax; i >= 0; {
		if remain == 0 {
			cache, remain = src.Int63(), letterIdxMax
		}
		if idx := int(cache & letterIdxMask); idx < len(letterBytes) {
			b[i] = letterBytes[idx]
			i--
		}
		cache >>= letterIdxBits
		remain--
	}

	return *(*string)(unsafe.Pointer(&b))
}
