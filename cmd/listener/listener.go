package main

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"net"
)

func main() {
	startServer()
}
func startServer() {
	l, err := net.Listen("tcp", "localhost:3338")
	if err != nil {
		log.Fatal(err)
	}
	defer l.Close()
	for {
		conn, err := l.Accept()
		if err != nil {
			log.Fatal(err)
		}
		go handleConnection(conn)
	}
}

func handleRequest(conn net.Conn) {
	defer conn.Close()

	reader := bufio.NewReader(conn)

	for {
		message, err := reader.ReadString('\n') // change the delimiter according to your messaging protocol
		if err != nil {
			if err != io.EOF {
				log.Fatal(err)
			}
			break
		}
		fmt.Printf("Received message: %s", message)
		_, err = conn.Write([]byte(message))
		if err != nil {
			log.Fatal(err)
		}
	}
}
func handleConnection(conn net.Conn) {
	defer conn.Close()
	for {
		var num int32
		// Read the integer from the connection
		err := binary.Read(conn, binary.BigEndian, &num)
		if err != nil {
			fmt.Println("Error reading from connection:", err)
			return
		}

		// Write the integer back to the connection
		err = binary.Write(conn, binary.BigEndian, num)
		if err != nil {
			fmt.Println("Error writing to connection:", err)
			return
		}

		fmt.Println("Received and sent back:", num)
	}

}
