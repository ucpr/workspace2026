// loadgen sends GET requests at a fixed rate (open model: new requests keep
// arriving even if the server is slow, like real user traffic) and prints a
// per-second summary.
package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

func main() {
	url := flag.String("url", "http://localhost:8080/work", "target URL (query string included)")
	rps := flag.Int("rps", 10, "requests per second")
	duration := flag.Duration("duration", 30*time.Second, "how long to send requests")
	flag.Parse()

	client := &http.Client{Timeout: 60 * time.Second}
	var sent, ok, failed, inflight atomic.Int64
	var wg sync.WaitGroup

	done := make(chan struct{})
	go func() {
		t := time.NewTicker(time.Second)
		defer t.Stop()
		for {
			select {
			case <-done:
				return
			case <-t.C:
				log.Printf("sent=%d ok=%d failed=%d inflight=%d", sent.Load(), ok.Load(), failed.Load(), inflight.Load())
			}
		}
	}()

	tick := time.NewTicker(time.Second / time.Duration(*rps))
	defer tick.Stop()
	end := time.After(*duration)
loop:
	for {
		select {
		case <-end:
			break loop
		case <-tick.C:
			sent.Add(1)
			inflight.Add(1)
			wg.Add(1)
			go func() {
				defer wg.Done()
				defer inflight.Add(-1)
				resp, err := client.Get(*url)
				if err != nil {
					failed.Add(1)
					return
				}
				_, _ = io.Copy(io.Discard, resp.Body)
				resp.Body.Close()
				if resp.StatusCode == http.StatusOK {
					ok.Add(1)
				} else {
					failed.Add(1)
				}
			}()
		}
	}
	wg.Wait()
	close(done)
	fmt.Printf("done: sent=%d ok=%d failed=%d\n", sent.Load(), ok.Load(), failed.Load())
}
