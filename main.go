package main

import (
	"fmt"
	"io"
	"os"
)

func main() {
	messages, err := os.Open("messages.txt")
	if err != nil {
		fmt.Println("Error opening file:", err)
	}
	defer messages.Close()

	byteslice := make([]byte, 8)

	for {
		n, err := messages.Read(byteslice)
		if err != nil && err != io.EOF {
			fmt.Println("Error reading file:", err)
			break
		}

		if n > 0 {
			fmt.Printf("read: %s\n", byteslice[:n])
		}

		if err == io.EOF {
			break
		}
	}

}
