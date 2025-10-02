package main

import (
	"fmt"
	"io"
	"net"
	"strings"
)

func getLinesChannel(f io.ReadCloser) <-chan string {
	ch := make(chan string)

	go func() {
		defer close(ch)
		defer f.Close()

		buf := make([]byte, 8)
		var accum []byte
		for {
			n, err := f.Read(buf)
			if n > 0 {
				chunk := buf[:n]
				accum = append(accum, chunk...)
				parts := strings.Split(string(accum), "\n")
				for i := 0; i < len(parts)-1; i++ {
					ch <- parts[i]
				}
				accum = []byte(parts[len(parts)-1])
			}
			if err != nil {
				if err == io.EOF {
					if len(accum) > 0 {
						ch <- string(accum)
					}
				} else {
					// optional: log or ignore, but exit
					fmt.Println("Error reading file:", err)
				}
				return
			}
		}
	}()

	return ch
}

func main() {

	tcplistener, err := net.Listen("tcp", ":42069")
	if err != nil {
		fmt.Println("Error listening:", err)
		return
	}
	defer tcplistener.Close()

	for {
		// Wait for a connection.
		conn, err := tcplistener.Accept()
		if err != nil {
			fmt.Println("Error accepting: ", err)
		}

		fmt.Println("accepted connection")

		go func(c net.Conn) {
			defer c.Close()
			for line := range getLinesChannel(c) {
				fmt.Println(line)
			}
			fmt.Println("connection closed")
		}(conn)
	}
}
