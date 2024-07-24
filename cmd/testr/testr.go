package main

import (
	"encoding/binary"
	"fmt"
	"golang.org/x/net/proxy"
	"os"
)

func main() {
	// set up a socks5 dialer
	dialer, err := proxy.SOCKS5("tcp", "localhost:8882", nil, proxy.Direct)
	if err != nil {
		fmt.Fprintln(os.Stderr, "can't connect to the proxy:", err)
		os.Exit(1)
	}
	// use the dialer to connect to the server
	conn, err := dialer.Dial("tcp", "nprofile1qqs9ntc52tn0app0w7azwpj4s39lnz8h0frnzlhf6mun2ptq9ay36kspzemhxue69uhhyetvv9ujuwpnxvejuumsv93k20v2pva:3338")
	if err != nil {
		fmt.Fprintln(os.Stderr, "can't connect to the server:", err)
		os.Exit(1)
	}
	counter := int32(0)

	for {

		// Increment the counter
		counter++

		// Write the counter to the connection
		err = binary.Write(conn, binary.BigEndian, counter)
		if err != nil {
			fmt.Println("Error writing to connection:", err)
			break
		}

		// Read the response from the server
		var response int32
		err = binary.Read(conn, binary.BigEndian, &response)
		if err != nil {
			fmt.Println("Error reading from connection:", err)
			break
		}

		fmt.Println("Sent:", counter, "Received:", response)

	}
	_ = conn.Close()
}
