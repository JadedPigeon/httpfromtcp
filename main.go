package main

import (
	"fmt"
	"io"
	"os"
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
	currentfile, err := os.Open("messages.txt")
	if err != nil {
		fmt.Println("Error opening file:", err)
		return
	}
	for line := range getLinesChannel(currentfile) {
		fmt.Printf("read: %s\n", line)
	}

}
