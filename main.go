package main

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/pion/webrtc/v4"
)

func main() {
	// Configure ICE servers with both STUN and TURN
	var iceServers = []webrtc.ICEServer{
		{
			URLs: []string{
				"stun:stun.l.google.com:19302",
				"stun:stun1.l.google.com:19302",
			},
		},
		{
			URLs: []string{"turn:global.relay.metered.ca:80"},
			Username:   "3d586cc56c428ce9e5435496",
			Credential: "t+/nUE9ITGlbRqzj",
		},
		{
			URLs: []string{"turn:global.relay.metered.ca:443"},
			Username:   "3d586cc56c428ce9e5435496",
			Credential: "t+/nUE9ITGlbRqzj",
		},
		{
			URLs: []string{"turn:global.relay.metered.ca:80?transport=tcp"},
			Username:   "3d586cc56c428ce9e5435496",
			Credential: "t+/nUE9ITGlbRqzj",
		},
		{
			URLs: []string{"turns:global.relay.metered.ca:443?transport=tcp"},
			Username:   "3d586cc56c428ce9e5435496",
			Credential: "t+/nUE9ITGlbRqzj",
		},
	}

	config := webrtc.Configuration{
		ICEServers: iceServers,
	}

	if len(os.Args) < 2 {
		fmt.Println("Usage: go run . <offer/accept>")
		return
	}

	switch clientType := strings.ToLower(os.Args[1]); clientType {
	case "offer":
		fmt.Println("You selected offer")
		runOffer(config)
	case "accept":
		fmt.Println("You selected accept")
		runAccept(config)
	default:
		fmt.Println("That is an invalid param. Select either accept or offer.")
	}
}

func runOffer(config webrtc.Configuration) {
	// Create a new RTCPeerConnection
	peerConnection, err := webrtc.NewPeerConnection(config)
	if err != nil {
		panic(err)
	}
	defer peerConnection.Close()

	// Create channels for signaling
	connected := make(chan struct{})
	failed := make(chan struct{})

	// Create the data channel
	dataChannel, err := peerConnection.CreateDataChannel("data", nil)
	if err != nil {
		panic(err)
	}

	// Set up data channel handlers
	dataChannel.OnOpen(func() {
		fmt.Println("Data channel is open. You can start typing messages.")
		close(connected)
	})

	dataChannel.OnMessage(func(msg webrtc.DataChannelMessage) {
		fmt.Printf("Received: %s\n", string(msg.Data))
	})

	// Monitor connection state
	peerConnection.OnICEConnectionStateChange(func(connectionState webrtc.ICEConnectionState) {
		fmt.Printf("ICE Connection State has changed: %s\n", connectionState.String())
		
		switch connectionState {
		case webrtc.ICEConnectionStateFailed:
			fmt.Println("ICE Connection failed. Please check your network connection and try again.")
			close(failed)
		case webrtc.ICEConnectionStateDisconnected:
			fmt.Println("ICE Connection disconnected. Attempting to reconnect...")
		case webrtc.ICEConnectionStateConnected:
			fmt.Println("ICE Connection established successfully!")
		}
	})

	// Monitor ICE candidate gathering
	peerConnection.OnICECandidate(func(ice *webrtc.ICECandidate) {
		if ice != nil {
			fmt.Printf("Found ICE candidate: %s\n", ice.String())
		}
	})

	// Create an offer
	offer, err := peerConnection.CreateOffer(nil)
	if err != nil {
		panic(err)
	}

	// Set the local description
	err = peerConnection.SetLocalDescription(offer)
	if err != nil {
		panic(err)
	}

	// Wait for ICE gathering to complete
	fmt.Println("Gathering ICE candidates...")
	time.Sleep(2 * time.Second)

	encodedOffer := encode(*peerConnection.LocalDescription())
	fmt.Println("\nSend this offer to the acceptor:")
	fmt.Println(encodedOffer)

	fmt.Println("\nPaste the acceptor's answer here:")
	answerBase64 := readInput()
	answer := webrtc.SessionDescription{}
	decode(answerBase64, &answer)

	err = peerConnection.SetRemoteDescription(answer)
	if err != nil {
		panic(err)
	}

	// Wait for either connection or failure
	select {
	case <-connected:
		fmt.Println("Connection established successfully!")
	case <-failed:
		fmt.Println("Connection failed!")
		return
	case <-time.After(30 * time.Second):
		fmt.Println("Connection timed out!")
		return
	}

	// Start chat
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		message := scanner.Text()
		if message == "quit" {
			return
		}
		err := dataChannel.SendText(message)
		if err != nil {
			fmt.Println("Error sending message:", err)
		}
	}
}

func runAccept(config webrtc.Configuration) {
	peerConnection, err := webrtc.NewPeerConnection(config)
	if err != nil {
		panic(err)
	}
	defer peerConnection.Close()

	connected := make(chan struct{})
	failed := make(chan struct{})
	
	var mu sync.Mutex
	var dataChannel *webrtc.DataChannel

	peerConnection.OnDataChannel(func(d *webrtc.DataChannel) {
		fmt.Println("New data channel:", d.Label())
		
		mu.Lock()
		dataChannel = d
		mu.Unlock()

		d.OnOpen(func() {
			fmt.Println("Data channel is open. You can start typing messages.")
			close(connected)
		})

		d.OnMessage(func(msg webrtc.DataChannelMessage) {
			fmt.Printf("Received: %s\n", string(msg.Data))
		})
	})

	// Monitor connection state
	peerConnection.OnICEConnectionStateChange(func(connectionState webrtc.ICEConnectionState) {
		fmt.Printf("ICE Connection State has changed: %s\n", connectionState.String())
		
		switch connectionState {
		case webrtc.ICEConnectionStateFailed:
			fmt.Println("ICE Connection failed. Please check your network connection and try again.")
			close(failed)
		case webrtc.ICEConnectionStateDisconnected:
			fmt.Println("ICE Connection disconnected. Attempting to reconnect...")
		case webrtc.ICEConnectionStateConnected:
			fmt.Println("ICE Connection established successfully!")
		}
	})

	// Monitor ICE candidate gathering
	peerConnection.OnICECandidate(func(ice *webrtc.ICECandidate) {
		if ice != nil {
			fmt.Printf("Found ICE candidate: %s\n", ice.String())
		}
	})

	fmt.Println("Paste the offer from the initiator here:")
	offerBase64 := readInput()
	offer := webrtc.SessionDescription{}
	decode(offerBase64, &offer)

	err = peerConnection.SetRemoteDescription(offer)
	if err != nil {
		panic(err)
	}

	// Create answer
	answer, err := peerConnection.CreateAnswer(nil)
	if err != nil {
		panic(err)
	}

	// Set local description
	err = peerConnection.SetLocalDescription(answer)
	if err != nil {
		panic(err)
	}

	// Wait for ICE gathering to complete
	fmt.Println("Gathering ICE candidates...")
	time.Sleep(2 * time.Second)

	answerBase64 := encode(*peerConnection.LocalDescription())
	fmt.Println("\nSend this answer to the initiator:")
	fmt.Println(answerBase64)

	// Wait for either connection or failure
	select {
	case <-connected:
		fmt.Println("Connection established successfully!")
	case <-failed:
		fmt.Println("Connection failed!")
		return
	case <-time.After(30 * time.Second):
		fmt.Println("Connection timed out!")
		return
	}

	// Start chat
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		message := scanner.Text()
		if message == "quit" {
			return
		}
		
		mu.Lock()
		if dataChannel != nil {
			err := dataChannel.SendText(message)
			if err != nil {
				fmt.Println("Error sending message:", err)
			}
		}
		mu.Unlock()
	}
}

func encode(sd webrtc.SessionDescription) string {
	b, err := json.Marshal(sd)
	if err != nil {
		panic(err)
	}

	var buffer bytes.Buffer
	gzipWriter := gzip.NewWriter(&buffer)
	_, err = gzipWriter.Write(b)
	if err != nil {
		panic(err)
	}
	gzipWriter.Close()

	return base64.StdEncoding.EncodeToString(buffer.Bytes())
}

// Decode and decompress a base64 string to a SessionDescription
func decode(s string, sd *webrtc.SessionDescription) {
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		panic(err)
	}

	gzipReader, err := gzip.NewReader(bytes.NewReader(b))
	if err != nil {
		panic(err)
	}
	defer gzipReader.Close()

	decompressed, err := io.ReadAll(gzipReader)
	if err != nil {
		panic(err)
	}

	err = json.Unmarshal(decompressed, sd)
	if err != nil {
		panic(err)
	}
}
func readInput() string {
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		text := strings.TrimSpace(scanner.Text())
		if text != "" {
			return text
		}
	}
	return ""
}
