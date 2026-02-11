package main

import (
	"fmt"
	"log"
	"math"
	"net/http"
	_ "net/http/pprof"
	"sort"
	"strconv"
)

func main() {
	http.HandleFunc("/fib", handleFib)
	http.HandleFunc("/sort", handleSort)
	http.HandleFunc("/prime", handlePrime)
	http.HandleFunc("/health", handleHealth)

	log.Println("server listening on :8080")
	log.Println("pprof available at /debug/pprof/")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

// handleHealth はヘルスチェック用のエンドポイント
func handleHealth(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(w, "ok")
}

// handleFib はフィボナッチ数列を計算するエンドポイント
// ?n=40 のようにクエリパラメータで指定
func handleFib(w http.ResponseWriter, r *http.Request) {
	n, err := strconv.Atoi(r.URL.Query().Get("n"))
	if err != nil || n < 0 {
		n = 35
	}
	result := fib(n)
	fmt.Fprintf(w, "fib(%d) = %d\n", n, result)
}

// handleSort はランダムなスライスのソートを行うエンドポイント
// ?size=10000 のようにクエリパラメータで指定
func handleSort(w http.ResponseWriter, r *http.Request) {
	size, err := strconv.Atoi(r.URL.Query().Get("size"))
	if err != nil || size <= 0 {
		size = 100000
	}
	data := generateData(size)
	sort.Ints(data)
	fmt.Fprintf(w, "sorted %d elements, first=%d, last=%d\n", size, data[0], data[len(data)-1])
}

// handlePrime は指定された数以下の素数を数えるエンドポイント
// ?n=100000 のようにクエリパラメータで指定
func handlePrime(w http.ResponseWriter, r *http.Request) {
	n, err := strconv.Atoi(r.URL.Query().Get("n"))
	if err != nil || n <= 0 {
		n = 100000
	}
	count := countPrimes(n)
	fmt.Fprintf(w, "primes up to %d: %d\n", n, count)
}

// fib は再帰的にフィボナッチ数を計算する（CPU 負荷が高い）
func fib(n int) int {
	if n <= 1 {
		return n
	}
	return fib(n-1) + fib(n-2)
}

// generateData は疑似乱数的なデータスライスを生成する
func generateData(size int) []int {
	data := make([]int, size)
	for i := range data {
		data[i] = (i*2654435761 + 1) % (size * 10)
	}
	return data
}

// countPrimes はエラトステネスの篩で n 以下の素数を数える
func countPrimes(n int) int {
	if n < 2 {
		return 0
	}
	sieve := make([]bool, n+1)
	for i := 2; i <= int(math.Sqrt(float64(n))); i++ {
		if !sieve[i] {
			for j := i * i; j <= n; j += i {
				sieve[j] = true
			}
		}
	}
	count := 0
	for i := 2; i <= n; i++ {
		if !sieve[i] {
			count++
		}
	}
	return count
}
