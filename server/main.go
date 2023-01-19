package main

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/pion/interceptor"
	"github.com/pion/rtcp"
	"github.com/pion/webrtc/v3"
	"github.com/pion/webrtc/v3/pkg/media"
	"github.com/pion/webrtc/v3/pkg/media/h264writer"
	"github.com/pion/webrtc/v3/pkg/media/ivfwriter"
	"github.com/skip2/go-qrcode"
)

var compress = false

type SDPRequest struct {
	SDP string `json:"sdp"`
}

var wg sync.WaitGroup
var rw sync.Mutex

func webrtcStart(w http.ResponseWriter, r *http.Request, stdin io.WriteCloser) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "*")
	w.Header().Set("Content-Type", "application/json")

	rw.Lock()

	m := &webrtc.MediaEngine{}

	if err := m.RegisterCodec(webrtc.RTPCodecParameters{
		RTPCodecCapability: webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeVP8, ClockRate: 90000, Channels: 0, SDPFmtpLine: "", RTCPFeedback: nil},
		PayloadType:        106,
	}, webrtc.RTPCodecTypeVideo); err != nil {
		panic(err)
	}

	i := &interceptor.Registry{}

	if err := webrtc.RegisterDefaultInterceptors(m, i); err != nil {
		panic(err)
	}

	api := webrtc.NewAPI(webrtc.WithMediaEngine(m), webrtc.WithInterceptorRegistry(i))

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Println(err)
		return
	}

	var sdpReq SDPRequest
	err = json.Unmarshal(body, &sdpReq)
	if err != nil {
		log.Println(err)
		return
	}

	config := webrtc.Configuration{
		SDPSemantics: webrtc.SDPSemanticsUnifiedPlan,
	}

	peerConnection, err := api.NewPeerConnection(config)
	if err != nil {
		panic(err)
	}

	if _, err = peerConnection.AddTransceiverFromKind(webrtc.RTPCodecTypeVideo); err != nil {
		panic(err)
	}

	offer := webrtc.SessionDescription{}
	Decode(sdpReq.SDP, &offer)

	err = peerConnection.SetRemoteDescription(offer)
	if err != nil {
		panic(err)
	}

	answer, err := peerConnection.CreateAnswer(nil)
	if err != nil {
		log.Fatal(err)
	}

	gatherComplete := webrtc.GatheringCompletePromise(peerConnection)

	err = peerConnection.SetLocalDescription(answer)
	if err != nil {
		panic(err)
	}

	<-gatherComplete

	fmt.Println("Answer: ", Encode(answer))
	resp := json.NewEncoder(w)
	resp.Encode(SDPRequest{SDP: Encode(answer)})

	peerConnection.OnICEConnectionStateChange(func(connectionState webrtc.ICEConnectionState) {
		fmt.Printf("ICE Connection State has changed: %s\n", connectionState.String())
		if connectionState.String() == "failed" || connectionState.String() == "disconnected" {
			rw.Unlock()
		}
	})

	peerConnection.OnTrack(func(t *webrtc.TrackRemote, r *webrtc.RTPReceiver) {
		log.Println("Testing")
		var writer media.Writer
		wg.Add(1)
		log.Println(t.Codec())
		go func() {
			ticker := time.NewTicker(time.Millisecond * 100)
			for range ticker.C {
				rtcpSendErr := peerConnection.WriteRTCP([]rtcp.Packet{&rtcp.PictureLossIndication{MediaSSRC: uint32(t.SSRC())}})
				if rtcpSendErr != nil {
					fmt.Println(rtcpSendErr)
				}
			}
		}()
		if strings.Contains(strings.ToLower(t.Codec().MimeType), "vp8") || strings.Contains(strings.ToLower(t.Codec().MimeType), "vp9") {
			writer, _ = ivfwriter.NewWith(stdin)
		} else if strings.Contains(strings.ToLower(t.Codec().MimeType), "h264") {
			writer = h264writer.NewWith(stdin)
		}
		go func() {
			for {
				rtpPacket, _, err := t.ReadRTP()
				if err != nil {
					log.Println(err)
					return
				}

				writer.WriteRTP(rtpPacket)
			}
			wg.Done()
		}()
	})

}

func handleSDP(stdin io.WriteCloser) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		webrtcStart(w, r, stdin)
	})
}

func main() {

	cmd := exec.Command("ffmpeg", "-threads", "4", "-re", "-i", "pipe:", "-an", "-f", "v4l2", "-fflags", "nobuffer", "-vcodec", "rawvideo", "-pix_fmt", "yuv420p", "/dev/video10")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		log.Println(err)
		return
	}

	wg.Add(1)
	go func(cmd *exec.Cmd) {
		err = cmd.Run()
		if err != nil {
			log.Println(err)
		}
		wg.Done()
	}(cmd)

	fs := http.FileServer(http.Dir("../build"))
	http.Handle("/", fs)

	http.Handle("/sdp", handleSDP(stdin))

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

// Encode encodes the input in base64
// It can optionally zip the input before encoding
func Encode(obj interface{}) string {
	b, err := json.Marshal(obj)
	if err != nil {
		fmt.Println(err)
	}

	if compress {
		b = zip(b)
	}

	return base64.StdEncoding.EncodeToString(b)
}

// Decode decodes the input from base64
// It can optionally unzip the input after decoding
func Decode(in string, obj interface{}) {
	b, err := base64.StdEncoding.DecodeString(strings.Replace(in, "\n", "", -1))
	if err != nil {
		fmt.Println(err)
	}

	if compress {
		b = unzip(b)
	}

	fmt.Println(string(b))

	err = json.Unmarshal(b, obj)
	if err != nil {
		fmt.Println(err)
	}
}

func zip(in []byte) []byte {
	var b bytes.Buffer
	gz := gzip.NewWriter(&b)
	_, err := gz.Write(in)
	if err != nil {
		fmt.Println(err)
	}
	err = gz.Flush()
	if err != nil {
		fmt.Println(err)
	}
	err = gz.Close()
	if err != nil {
		fmt.Println(err)
	}
	return b.Bytes()
}

func unzip(in []byte) []byte {
	var b bytes.Buffer
	_, err := b.Write(in)
	if err != nil {
		fmt.Println(err)
	}
	r, err := gzip.NewReader(&b)
	if err != nil {
		fmt.Println(err)
	}
	res, err := ioutil.ReadAll(r)
	if err != nil {
		fmt.Println(err)
	}
	return res
}
