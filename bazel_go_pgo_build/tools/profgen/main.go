package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sync"
	"time"
)

const (
	serverAddr     = "http://localhost:8080"
	profileSeconds = 30
	concurrency    = 4
)

func main() {
	log.Println("starting load generation and profile collection")
	log.Printf("target: %s, profile duration: %ds, concurrency: %d", serverAddr, profileSeconds, concurrency)

	// ヘルスチェック
	if err := waitForServer(); err != nil {
		log.Fatalf("server not available: %v", err)
	}
	log.Println("server is ready")

	// バックグラウンドで負荷をかけながらプロファイルを収集
	ctx := make(chan struct{})
	var wg sync.WaitGroup

	// 負荷生成ゴルーチン
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			generateLoad(id, ctx)
		}(i)
	}

	// プロファイル収集
	log.Printf("collecting %d-second CPU profile...", profileSeconds)
	if err := collectProfile(); err != nil {
		close(ctx)
		wg.Wait()
		log.Fatalf("failed to collect profile: %v", err)
	}

	close(ctx)
	wg.Wait()

	log.Println("done! profile saved to profile/default.pgo")
	log.Println("you can now rebuild with PGO: bazel build //cmd/server")
}

func waitForServer() error {
	client := &http.Client{Timeout: 2 * time.Second}
	for i := 0; i < 30; i++ {
		resp, err := client.Get(serverAddr + "/health")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}
		time.Sleep(1 * time.Second)
	}
	return fmt.Errorf("server did not become ready within 30s")
}

func generateLoad(id int, done <-chan struct{}) {
	client := &http.Client{Timeout: 30 * time.Second}
	endpoints := []string{
		"/fib?n=35",
		"/sort?size=100000",
		"/prime?n=100000",
		"/fib?n=30",
		"/sort?size=50000",
		"/prime?n=50000",
	}

	i := 0
	for {
		select {
		case <-done:
			return
		default:
		}

		url := serverAddr + endpoints[i%len(endpoints)]
		resp, err := client.Get(url)
		if err != nil {
			log.Printf("worker %d: request error: %v", id, err)
			time.Sleep(100 * time.Millisecond)
		} else {
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
		}
		i++
	}
}

func collectProfile() error {
	url := fmt.Sprintf("%s/debug/pprof/profile?seconds=%d", serverAddr, profileSeconds)
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("GET %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status: %s", resp.Status)
	}

	if err := os.MkdirAll("profile", 0o755); err != nil {
		return fmt.Errorf("mkdir profile: %w", err)
	}

	f, err := os.Create("profile/default.pgo")
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}
	defer f.Close()

	n, err := io.Copy(f, resp.Body)
	if err != nil {
		return fmt.Errorf("write profile: %w", err)
	}
	log.Printf("wrote %d bytes to profile/default.pgo", n)
	return nil
}
