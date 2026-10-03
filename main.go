package main

import (
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"sync"
	"time"
)

// Hasil unmarshal dari DadJokes
type Joke struct {
	ID     string `json:"id"`
	Joke   string `json:"joke"`
	Status int    `json:"status"`
}

// entry adalah satu slot di cache untuk satu ID.
// Channel ready ditutup setelah proses pengambilan selesai, sehingga goroutine
// lain yang meminta ID yang sama cukup menunggu dan memakai hasilnya.
type entry struct {
	joke  *Joke
	ready chan struct{}
}

// Cache menyimpan Joke yang SUDAH di-unmarshal.
type Cache struct {
	mu   sync.Mutex
	m    map[string]*entry
	hit  int
	miss int
}

var client = &http.Client{Timeout: 10 * time.Second}

// doGet: HTTP GET ke icanhazdadjoke dengan header yang benar
func doGet(url string) ([]byte, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Tugas Cache Goroutine (ITS)")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

// fetchIDs: ambil n ID joke valid dari endpoint /search
func fetchIDs(n int) ([]string, error) {
	body, err := doGet(fmt.Sprintf("https://icanhazdadjoke.com/search?limit=%d", n))
	if err != nil {
		return nil, err
	}
	var r struct {
		Results []Joke `json:"results"`
	}
	if err := json.Unmarshal(body, &r); err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(r.Results))
	for _, j := range r.Results {
		ids = append(ids, j.ID)
	}
	return ids, nil
}

// retrieveJoke: fetch + unmarshal. Hanya dipanggil saat cache miss.
func retrieveJoke(id string) (*Joke, bool) {
	body, err := doGet("https://icanhazdadjoke.com/j/" + id)
	if err != nil {
		return nil, false
	}
	var j Joke
	if err := json.Unmarshal(body, &j); err != nil {
		return nil, false
	}
	return &j, true
}

func (c *Cache) Get(id string) (Joke, bool) {
	c.mu.Lock()
	e, exists := c.m[id]
	if exists {
		// HIT: ID sudah ada (selesai atau sedang diambil goroutine lain)
		c.hit++
		c.mu.Unlock()
		<-e.ready // tunggu sampai data siap, tanpa unmarshal lagi
		if e.joke == nil {
			return Joke{}, false
		}
		return *e.joke, true
	}

	// MISS: daftarkan entry baru sebelum lock dibuka
	e = &entry{ready: make(chan struct{})}
	c.m[id] = e
	c.miss++
	c.mu.Unlock()

	// fetch + unmarshal di luar lock
	j, ok := retrieveJoke(id)
	if ok {
		e.joke = j
	}
	close(e.ready) // bangunkan goroutine yang menunggu

	if !ok {
		// gagal: hapus entry agar bisa dicoba lagi
		c.mu.Lock()
		delete(c.m, id)
		c.mu.Unlock()
		return Joke{}, false
	}
	return *j, true
}

func (c *Cache) Stats() (hit, miss int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.hit, c.miss
}

func main() {
	start := time.Now()

	ids, err := fetchIDs(8)
	if err != nil || len(ids) == 0 {
		fmt.Println("Gagal mengambil daftar ID:", err)
		return
	}
	fmt.Println("ID yang dipakai:", ids)

	cache := Cache{m: make(map[string]*entry)}

	var wg sync.WaitGroup
	wg.Add(50)
	for i := 1; i <= 50; i++ {
		go func(n int) {
			defer wg.Done()
			id := ids[rand.Intn(len(ids))]
			j, ok := cache.Get(id)
			if !ok {
				fmt.Printf("[goroutine %2d] gagal mengambil %s\n", n, id)
				return
			}
			fmt.Printf("[goroutine %2d] (%s) %s\n", n, j.ID, j.Joke)
		}(i)
	}
	wg.Wait()

	hit, miss := cache.Stats()
	total := hit + miss
	fmt.Println("\n========== STATISTIK CACHE ==========")
	fmt.Printf("Total request : %d\n", total)
	fmt.Printf("Hit           : %d\n", hit)
	fmt.Printf("Miss          : %d\n", miss)
	fmt.Printf("Hit rate      : %.2f%%\n", float64(hit)/float64(total)*100)
	fmt.Printf("Miss rate     : %.2f%%\n", float64(miss)/float64(total)*100)
	fmt.Printf("Waktu total   : %v\n", time.Since(start))
}