package main

import (
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/pion/stun"
)

func main() {
	// Parse a STUN URI
	u, err := stun.ParseURI("stun:stun.l.google.com:19302"); if err != nil {
		panic(err)
	}
	
	// Create a connection to the STUN server
	c, err := stun.DialURI(u, &stun.DialConfig{})
	if err != nil {
		panic(err)
	}

	// Building binding request with random transaction
	// Transaction ID is a random 96-bit identifier sent to the server so
	// we can later identify that specific request
	// Binding Request is sent telling the server we want an IP
	message := stun.MustBuild(stun.TransactionID, stun.BindingRequest)
	
	// Sending request to STUN server, waiting for respo
	if err := c.Do(message, func(res stun.Event) {
		if res.Error != nil {
			panic(res.Error)
		}

		// Decoding XOR-MAPPED-ADDRESS attribute from message.
		var xorAddr stun.XORMappedAddress
		if err := xorAddr.GetFrom(res.Message); err != nil {
			panic(err)
		}
		fmt.Printf("Your IP & PORT is %s:%d\n", xorAddr.IP, xorAddr.Port)
	});
	err != nil {
		panic(err)
	}
	
	// Take user inputt Port and IP address
	var clientIpAddrAndPort string
	fmt.Print("Input the other client's IP and Port: ")
	fmt.Scan(&clientIpAddrAndPort) 
	parts := strings.Split(clientIpAddrAndPort, ":")

	clientIp := parts[0]
	clientPort, err := strconv.Atoi(parts[1])
	if err != nil {
		panic(err)
	}

	// Open UDP connection for listening
	conn, err := net.ListenPacket("udp", ":0")
	if err != nil {
		panic(err)
	}
	// Close connection once everything is shut down	
	defer conn.Close()

	// Create a goroutine to hole punch through NAT
	go func () {
		for {
			_,  err := conn.WriteTo([]byte("Hole punching packet"), &net.UDPAddr{
				IP: net.ParseIP(clientIp),
				Port: clientPort,
			})
			if err != nil {
				fmt.Println("Error sending packet: ", err)
				panic(err)
			}
			fmt.Println("Packet sent succesfully, sleeping")
			time.Sleep(1 * time.Second)
		}
	}()


	// Listen for incoming packets
	buffer := make([]byte, 1024)
	for {
		n, addr, err := conn.ReadFrom(buffer)
		if err != nil {
			fmt.Println("Error receiving packet:", err)
			continue
		}
		fmt.Printf("Received '%s' from %s\n", string(buffer[:n]), addr)
	}
}
