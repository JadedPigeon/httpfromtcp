package main

import (
	"fmt"
	"io"
	"os"
	"strings"
)

func main() {
	messages, err := os.Open("messages.txt")
	if err != nil {
		fmt.Println("Error opening file:", err)
	}
	defer messages.Close()

	byteslice := make([]byte, 8)
	currentline := ""

	for {
		n, err := messages.Read(byteslice)
		if err != nil && err != io.EOF {
			fmt.Println("Error reading file:", err)
			break
		}

		if n > 0 {
			chunk := string(byteslice[:n])
			parts := strings.Split(chunk, "\n")

			// print all complete lines
			for i := 0; i < len(parts)-1; i++ {
				fmt.Printf("read: %s\n", currentline+parts[i])
				currentline = ""
			}
			// keep the trailing partial (possibly empty if chunk ended with \n)
			currentline += parts[len(parts)-1]
		}

		if err == io.EOF {
			if currentline != "" {
				fmt.Printf("read: %s\n", currentline)
			}
			break
		}

	}

}
