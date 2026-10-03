# GoRoutine Dad Jokes — Sistem Terdistribusi

Repository ini berisi implementasi **Tugas 1 Sistem Terdistribusi (SISTER)** menggunakan bahasa pemrograman **Go (Golang)**.

Program memanfaatkan **goroutine**, **channel**, **mutex**, dan **cache** untuk mengambil serta mengakses data Dad Joke dari API [icanhazdadjoke.com](https://icanhazdadjoke.com/).

## Tujuan

Program dibuat untuk menerapkan konsep concurrency pada sistem terdistribusi dengan menjalankan banyak request secara bersamaan.

Pada program ini terdapat **50 goroutine** yang meminta Dad Joke berdasarkan ID yang dipilih secara acak dari daftar ID yang diperoleh melalui endpoint `/search`.

Program juga menggunakan cache untuk menghindari pengambilan data yang sama secara berulang dari API.

## Teknologi

* **Go (Golang)**
* `goroutine` — menjalankan proses secara concurrent
* `sync.WaitGroup` — menunggu seluruh goroutine selesai
* `sync.Mutex` — menjaga akses cache agar aman saat digunakan secara concurrent
* `channel` — melakukan sinkronisasi antar-goroutine
* `net/http` — melakukan HTTP request
* `encoding/json` — melakukan proses JSON unmarshalling
* **icanhazdadjoke API** — sumber data Dad Joke

## Alur Program

Secara umum, alur program adalah:

```text
Start
  │
  ▼
Mengambil 8 ID Joke dari API /search
  │
  ▼
Membuat Cache
  │
  ▼
Menjalankan 50 Goroutine
  │
  ├── Memilih ID secara acak
  │
  ├── Mengecek Cache
  │      │
  │      ├── HIT
  │      │    └── Menggunakan data dari Cache
  │      │
  │      └── MISS
  │           └── Mengambil Joke dari API
  │
  ▼
Menampilkan hasil setiap Goroutine
  │
  ▼
Menampilkan statistik Cache
  │
  ▼
End
```

## Penjelasan Komponen

### 1. Struct `Joke`

```go
type Joke struct {
    ID     string `json:"id"`
    Joke   string `json:"joke"`
    Status int    `json:"status"`
}
```

Struct `Joke` digunakan untuk menampung hasil JSON dari API. Data yang disimpan terdiri dari ID joke, isi joke, dan status response.

### 2. Struct `entry`

```go
type entry struct {
    joke  *Joke
    ready chan struct{}
}
```

`entry` merupakan satu slot dalam cache untuk menyimpan satu joke berdasarkan ID.

Channel `ready` digunakan sebagai mekanisme sinkronisasi. Channel akan ditutup ketika proses pengambilan joke selesai. Goroutine lain yang meminta ID yang sama dapat menunggu melalui channel tersebut tanpa melakukan request API dan unmarshalling ulang.

### 3. Struct `Cache`

```go
type Cache struct {
    mu   sync.Mutex
    m    map[string]*entry
    hit  int
    miss int
}
```

`Cache` menyimpan joke yang sudah berhasil diambil dan di-unmarshal.

* `mu` digunakan untuk melindungi akses ke data cache.
* `m` merupakan map yang menyimpan data berdasarkan ID joke.
* `hit` menghitung request yang menemukan ID di cache.
* `miss` menghitung request yang belum memiliki ID di cache.

Penggunaan `sync.Mutex` memastikan akses terhadap map dan statistik tetap aman ketika digunakan oleh banyak goroutine.

## Proses HTTP Request

Fungsi `doGet()` digunakan untuk melakukan HTTP GET request.

```go
req.Header.Set("Accept", "application/json")
req.Header.Set("User-Agent", "Tugas Cache Goroutine (ITS)")
```

Header `Accept` digunakan untuk meminta response dalam format JSON.

Program juga menggunakan HTTP client dengan timeout 10 detik:

```go
var client = &http.Client{Timeout: 10 * time.Second}
```

Hal ini mencegah program menunggu request tanpa batas waktu apabila API mengalami masalah.

## Mengambil ID Joke

Fungsi `fetchIDs()` mengambil daftar joke dari endpoint:

```text
/search?limit=8
```

Dari hasil response JSON, program mengambil ID setiap joke dan menyimpannya dalam slice `ids`.

ID tersebut kemudian digunakan sebagai sumber ID yang akan dipilih oleh 50 goroutine.

## Mekanisme Cache

Bagian utama program terdapat pada fungsi:

```go
func (c *Cache) Get(id string) (Joke, bool)
```

Ketika sebuah goroutine meminta joke berdasarkan ID, program terlebih dahulu mengecek cache.

### Cache HIT

Jika ID sudah terdapat dalam cache:

```go
if exists {
    c.hit++
    c.mu.Unlock()
    <-e.ready
    ...
}
```

Request dihitung sebagai **cache hit**.

Goroutine kemudian menunggu channel `ready` apabila joke masih sedang diambil oleh goroutine lain.

Setelah proses selesai, data joke digunakan kembali tanpa melakukan request API dan unmarshalling ulang.

### Cache MISS

Jika ID belum terdapat dalam cache, program membuat entry baru:

```go
e = &entry{ready: make(chan struct{})}
c.m[id] = e
c.miss++
```

Request dihitung sebagai **cache miss**.

Setelah itu, goroutine mengambil joke dari API menggunakan:

```go
j, ok := retrieveJoke(id)
```

Proses pengambilan dilakukan **di luar mutex** agar proses HTTP request yang membutuhkan waktu tidak menghalangi goroutine lain untuk mengakses cache.

Setelah data berhasil diperoleh:

```go
e.joke = j
close(e.ready)
```

`close(e.ready)` memberi tanda bahwa data sudah siap digunakan oleh goroutine lain yang menunggu ID tersebut.

## Concurrency dengan Goroutine

Program menjalankan sebanyak **50 goroutine**:

```go
for i := 1; i <= 50; i++ {
    go func(n int) {
        ...
    }(i)
}
```

Setiap goroutine memilih satu ID secara acak:

```go
id := ids[rand.Intn(len(ids))]
```

Kemudian meminta data joke melalui:

```go
j, ok := cache.Get(id)
```

Dengan cara ini, beberapa goroutine dapat mengakses cache secara bersamaan.

## Sinkronisasi dengan WaitGroup

Program menggunakan `sync.WaitGroup` untuk memastikan `main()` menunggu seluruh goroutine selesai.

```go
var wg sync.WaitGroup
wg.Add(50)
```

Setiap goroutine memanggil:

```go
defer wg.Done()
```

Kemudian program menunggu seluruh goroutine:

```go
wg.Wait()
```

Program baru menampilkan statistik setelah seluruh proses selesai.

## Statistik Cache

Setelah 50 goroutine selesai, program menampilkan:

* Total request
* Cache hit
* Cache miss
* Hit rate
* Miss rate
* Waktu total eksekusi

Contoh format output:

```text
========== STATISTIK CACHE ==========
Total request : 50
Hit           : ...
Miss          : ...
Hit rate      : ...%
Miss rate     : ...%
Waktu total   : ...
```

Nilai hit dan miss dapat berbeda pada setiap eksekusi karena setiap goroutine memilih ID secara acak.

## Cara Menjalankan

Pastikan Go sudah terinstall pada komputer.

### 1. Clone repository

```bash
git clone https://github.com/K-naulita/GoRoutineDadJokes_SISTER.git
```

### 2. Masuk ke folder repository

```bash
cd GoRoutineDadJokes_SISTER
```

### 3. Jalankan program

```bash
go run .
```

Program membutuhkan koneksi internet karena mengambil data dari API `icanhazdadjoke.com`.

## Konsep yang Diterapkan

| Konsep                        | Implementasi         |
| ----------------------------- | -------------------- |
| Concurrency                   | 50 goroutine         |
| Synchronization               | `sync.WaitGroup`     |
| Mutual Exclusion              | `sync.Mutex`         |
| Communication/Synchronization | Channel `ready`      |
| Caching                       | `Cache` dengan `map` |
| HTTP Request                  | `net/http`           |
| JSON Processing               | `encoding/json`      |
| External API                  | icanhazdadjoke.com   |

## Kesimpulan

Program ini menunjukkan penerapan concurrency pada Go dengan menjalankan 50 goroutine yang mengakses data Dad Joke secara bersamaan. Cache digunakan untuk mengurangi pengambilan data yang sama secara berulang, sedangkan `sync.Mutex`, `sync.WaitGroup`, dan channel digunakan untuk menjaga sinkronisasi serta keamanan akses data ketika beberapa goroutine berjalan secara concurrent.
