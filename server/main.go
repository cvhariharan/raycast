package main

import (
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"sync"

	"github.com/cvhariharan/raycast/handlers"
	"github.com/cvhariharan/raycast/video"
	"github.com/skip2/go-qrcode"
)

var compress = false

func handleSDP(stdin io.WriteCloser, wg *sync.WaitGroup) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlers.WebRTCStart(w, r, stdin, wg)
	})
}

func main() {
	var wg sync.WaitGroup

	ffmpeg := video.NewFFmpegEncoder(&wg)

	fs := http.FileServer(http.Dir("../build"))
	http.Handle("/", fs)

	http.Handle("/sdp", handleSDP(ffmpeg.Stdin, &wg))

	log.Println("Serving at https://" + getLocalIP() + ":8080")

	qr, err := qrcode.New("https://"+getLocalIP()+":8080", qrcode.Medium)
	if err != nil {
		log.Println(err)
	}
	fmt.Println(qr.ToSmallString(true))

	http.ListenAndServeTLS(":8080", "./localhost.crt", "./localhost.key", nil)

	wg.Wait()
}

func getLocalIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return ""
	}
	for _, address := range addrs {
		if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String()
			}
		}
	}
	return ""
}
