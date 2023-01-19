package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/pion/interceptor"
	"github.com/pion/rtcp"
	"github.com/pion/webrtc/v3"
	"github.com/pion/webrtc/v3/pkg/media"
	"github.com/pion/webrtc/v3/pkg/media/h264writer"
	"github.com/pion/webrtc/v3/pkg/media/ivfwriter"
)

var rw sync.Mutex

type SDPRequest struct {
	SDP string `json:"sdp"`
}

func WebRTCStart(w http.ResponseWriter, r *http.Request, stdin io.WriteCloser, wg *sync.WaitGroup) {
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
