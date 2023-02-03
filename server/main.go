package main

import (
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"sync"

	"github.com/cvhariharan/raycast/handlers"
	"github.com/cvhariharan/raycast/video"
	"github.com/hashicorp/mdns"
	"github.com/skip2/go-qrcode"
)

var compress = false

const HOSTNAME = "raycast"

func handleSDP(stdin io.WriteCloser, wg *sync.WaitGroup) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlers.WebRTCStart(w, r, stdin, wg)
	})
}

func main() {
	if _, err := os.Stat("/dev/video10"); err != nil {
		log.Println("v4l2loopback module not loaded")
		fmt.Println("Execute as root: modprobe v4l2loopback video_nr=10 exclusive_caps=1 card_label=\"VirtualCam\"")
		return
	}

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

	info := []string{"Raycast webcam"}
	service, err := mdns.NewMDNSService(HOSTNAME, "_http._tcp", "", "", 8080, nil, info)
	if err != nil {
		log.Println(err)
	}

	server, err := mdns.NewServer(&mdns.Config{Zone: service})
	if err != nil {
		log.Println(err)
	}
	defer server.Shutdown()

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
