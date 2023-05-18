package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"

	"github.com/pion/webrtc/v3"
)

func whep(url string, offer string) (string, error) {
	resp, err := http.Post(url, "application/json", bytes.NewBuffer([]byte(offer)))

	if err != nil {
		return "", err
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)

	return string(body), err
}

func handleData(track *webrtc.TrackRemote) {

	for {
		rtpPacket, _, err := track.ReadRTP()
		if err != nil {
			panic(err)
		}
		fmt.Println(rtpPacket)
	}
}

func main() {
	url := "http://localhost:8000/api/whep?url=Zeeland&options=rtptransport%3dtcp%26timeout%3d60"

	// Create a MediaEngine object to configure the supported codec
	m := &webrtc.MediaEngine{}

	if err := m.RegisterCodec(webrtc.RTPCodecParameters{
		RTPCodecCapability: webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeVP8, ClockRate: 90000, Channels: 0, SDPFmtpLine: "", RTCPFeedback: nil},
		PayloadType:        96,
	}, webrtc.RTPCodecTypeVideo); err != nil {
		panic(err)
	}
	if err := m.RegisterCodec(webrtc.RTPCodecParameters{
		RTPCodecCapability: webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeH264, ClockRate: 90000, Channels: 0, SDPFmtpLine: "", RTCPFeedback: nil},
		PayloadType:        97,
	}, webrtc.RTPCodecTypeVideo); err != nil {
		panic(err)
	}

	// Create the API object with the MediaEngine
	api := webrtc.NewAPI(webrtc.WithMediaEngine(m))

	// Prepare the configuration
	config := webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{
				URLs: []string{"stun:stun.l.google.com:19302"},
			},
		},
	}

	// Create a new RTCPeerConnection
	peerConnection, err := api.NewPeerConnection(config)
	if err != nil {
		panic(err)
	}

	// Allow us to 1 video track
	init := webrtc.RTPTransceiverInit{Direction: webrtc.RTPTransceiverDirectionRecvonly}
	if _, err = peerConnection.AddTransceiverFromKind(webrtc.RTPCodecTypeVideo, init); err != nil {
		panic(err)
	}

	// Set a handler for when a new remote track starts
	peerConnection.OnTrack(func(track *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
		codec := track.Codec()
		if strings.EqualFold(codec.MimeType, webrtc.MimeTypeVP8) {
			fmt.Println("Got VP8 track")
			handleData(track)
		} else if strings.EqualFold(codec.MimeType, webrtc.MimeTypeH264) {
			fmt.Println("Got H264 track")
			handleData(track)
		}
	})

	// Set the handler for ICE connection state
	// This will notify you when the peer has connected/disconnected
	peerConnection.OnICEConnectionStateChange(func(connectionState webrtc.ICEConnectionState) {
		fmt.Printf("Connection State has changed %s \n", connectionState.String())

		if connectionState == webrtc.ICEConnectionStateConnected {
			fmt.Println("Ctrl+C to interrupt")
		} else if connectionState == webrtc.ICEConnectionStateFailed {
			fmt.Println("Done")

			// Gracefully shutdown the peer connection
			if closeErr := peerConnection.Close(); closeErr != nil {
				panic(closeErr)
			}

			os.Exit(0)
		}
	})

	// Wait for the offer to be pasted
	options := webrtc.OfferOptions{}
	offer, err := peerConnection.CreateOffer(&options)
	if err != nil {
		panic(err)
	}

	// Set the local SessionDescription
	err = peerConnection.SetLocalDescription(offer)
	if err != nil {
		panic(err)
	}

	// Wait ICE Gathering is complete
	gatherComplete := webrtc.GatheringCompletePromise(peerConnection)
	<-gatherComplete

	// WHEP
	answerStr, err := whep(url, offer.SDP)
	if err != nil {
		panic(err)
	}
	fmt.Println(answerStr)

	// Sets the LocalDescription, and starts our UDP listeners
	answer := webrtc.SessionDescription{Type: webrtc.SDPTypeAnswer, SDP: answerStr}
	err = peerConnection.SetRemoteDescription(answer)
	if err != nil {
		panic(err)
	}

	// Block forever
	select {}
}
