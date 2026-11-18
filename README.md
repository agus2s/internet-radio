# inetradio

**Internet Radio Player** — pemutar radio streaming untuk terminal dengan tampilan pemutar media klasik era 90-an: dua panel (player + playlist), layar LCD hijau dengan marquee judul lagu, spectrum analyzer dari audio asli, dan tombol kontrol bergaya perangkat pemutar jaman dulu.

Ditulis dalam Go dengan [Bubble Tea](https://github.com/charmbracelet/bubbletea) untuk UI dan [beep](https://github.com/gopxl/beep) untuk decoding/pemutaran MP3.

```
╭──────────────────────────────╮ ╭──────────────────────────╮
│ INTERNET RADIO PLAYER    ... │ │ PLAYLIST           1/30  │
│──────────────────────────────│ │──────────────────────────│
│ ╭──────────────────────────╮ │ │ #  CHANNEL               │
│ │ Now playing: ...         │ │ │ >  1. ANTENNE BAYERN     │
│ │ █ ██ ███ ██ █            │ │ │    2. Top 40             │
│ │ 00:12:45  128k 44kHz ST  │ │ │    3. Rock Antenne       │
│ ╰──────────────────────────╯ │ │    ...                   │
│ VOL [███████░░░]  7/10       │ │──────────────────────────│
│                              │ │ x play  c pause  v stop  │
│ [░░██░░░░░░░░░░] LIVE        │ │ z/b or <-> ch up/dn vol  │
│ [PREV] [PLAY] [PAUSE] [STOP] │ │                          │
│                              │ │                          │
│ Status: Memutar (live) 12MB  │ │                          │
╰──────────────────────────────╯ ╰──────────────────────────╯
```

## Fitur

- **30 channel bawaan**: 18 channel ANTENNE BAYERN (Top 40, Rock Antenne, Chillout, 80er, 90er, 2000er, 2010er, Schlager, Weihnachts-Hits, dll) plus 12 radio Indonesia (iSwara, Hard Rock FM, V Radio, MNC Trijaya, RDI, Suara Giri, Elshinta, Radio Rodja, Retjo Buntung, Rama FM, Dahlia FM, Radio Sholawat).
- **Judul lagu real-time** dari metadata ICY (`StreamTitle`), ditampilkan sebagai teks berjalan (marquee).
- **Spectrum analyzer** 64 bar yang dihitung dari sampel audio yang benar-benar diputar, bukan animasi palsu.
- **Auto-reconnect**: stream yang terputus disambung ulang otomatis (channel yang sama), dengan jeda singkat.
- **Pindah channel mulus** tanpa restart aplikasi; metadata dari koneksi lama tidak menimpa channel baru.
- **Volume 10 tingkat + mute**, diimplementasikan lewat `effects.Volume` dan `beep.Ctrl`.
- **Info stream**: bitrate, sample rate, dan mono/stereo dari header ICY.
- **Penghitung data** yang sudah diunduh (MB) per sesi channel.
- **Layout responsif**: menyesuaikan lebar terminal, teks terpotong rapi dengan elipsis.
- **URL kustom** bisa ditambahkan sebagai channel pertama lewat flag `-url`.
- **Tambah URL dari layar**: tekan `a`, masukkan URL stream, lalu Enter. Channel baru langsung disisipkan di atas item pertama dan diputar (berlaku untuk sesi ini; tambahkan ke `channels.txt` bila ingin permanen).

## Kebutuhan

- Go (versi mengikuti `go.mod`, saat ini 1.27.3)
- Perangkat audio yang bisa diakses proses
- Terminal dengan dukungan warna 256/truecolor (Windows Terminal, iTerm2, GNOME Terminal, dll.)

Ketergantungan native untuk audio (library `oto` di balik `beep`):

| OS | Kebutuhan |
| --- | --- |
| Windows | Tidak ada tambahan (WASAPI) |
| macOS | Tidak ada tambahan (CoreAudio) |
| Linux | `libasound2-dev` dan C compiler (ALSA, butuh CGO) |

## Build

```bash
git clone <repo> inetradio
cd inetradio
go build -o inetradio.exe .
```

Untuk Linux/macOS:

```bash
go build -o inetradio .
```

Menjalankan langsung tanpa build:

```bash
go run .
```

## Penggunaan

```bash
./inetradio            # mulai dari channel 1
./inetradio -ch 3      # mulai dari channel 3
./inetradio -url https://contoh.com/stream.mp3
```

### Flag

| Flag | Default | Keterangan |
| --- | --- | --- |
| `-ch` | `1` | Nomor channel awal (1 s/d jumlah channel). Nilai di luar rentang otomatis dikoreksi ke 1. |
| `-url` | kosong | URL stream MP3 kustom. Jika diisi, URL tersebut ditambahkan sebagai channel pertama bernama "Kustom" (sehingga `-ch 1` memilihnya). |

### Kontrol keyboard

| Tombol | Aksi |
| --- | --- |
| `x` / `X` | Play / lanjutkan (setelah stop akan menyambung ulang) |
| `c` / `C` / `Space` | Pause / resume |
| `v` / `V` | Stop (memutus stream) |
| `b` / `n` / `→` | Channel berikutnya |
| `z` / `p` / `←` | Channel sebelumnya |
| `↑` / `+` / `=` | Volume naik |
| `↓` / `-` / `_` | Volume turun |
| `m` / `M` | Mute / unmute |
| `a` / `A` | Tambah URL stream baru di atas item pertama (Enter simpan, Esc batal) |
| `1`–`9` | Lompat langsung ke channel 1–9 |
| `q` / `Q` / `Esc` / `Ctrl+C` | Keluar |

> Catatan: pada radio live `stop` benar-benar memutus koneksi, karena tidak ada posisi yang bisa dilanjutkan seperti file biasa. Setelah `x`, stream disambung ulang dari titik live saat itu.

## Daftar channel

| # | Channel |
| --- | --- |
| 1 | ANTENNE BAYERN |
| 2 | Top 40 |
| 3 | Rock Antenne |
| 4 | Chillout |
| 5 | 80er Kulthits |
| 6 | 90er Hits |
| 7 | 2000er Hits |
| 8 | 2010er Hits |
| 9 | Oldies but Goldies |
| 10 | Classic Rock Live |
| 11 | Coffee Music |
| 12 | Lovesongs |
| 13 | Fresh 4 You |
| 14 | Event |
| 15 | Workout Hits |
| 16 | Hits fuer Kids |
| 17 | Schlager Karussell |
| 18 | Weihnachts-Hits |
| 19 | iSwara Jakarta |
| 20 | Hard Rock FM Jakarta |
| 21 | V Radio Jakarta |
| 22 | MNC Trijaya FM Jakarta |
| 23 | RDI FM Jakarta (Dangdut) |
| 24 | Suara Giri FM Surabaya |
| 25 | Elshinta Jakarta |
| 26 | Radio Rodja |
| 27 | Retjo Buntung FM Jogja |
| 28 | Rama FM Bandung |
| 29 | Radio Dahlia FM Bandung |
| 30 | Radio Sholawat |

Channel 1–18 memakai basis URL `https://mp3channels.webradio.antenne.de/`, sedangkan channel 19–30 adalah stream radio Indonesia. Semua stream di daftar bawaan berformat MP3 agar cocok dengan decoder aplikasi (stream AAC/HLS tidak didukung). Daftar bawaan didefinisikan di `channels.go` (variabel `defaultChannels`), dan bisa ditimpa lewat `channels.txt` atau flag `-channels`.

> URL stream radio bisa berubah sewaktu-waktu. Jika sebuah channel gagal (`HTTP 403`/`404` atau `Gagal decode`), perbarui URL-nya di `channels.txt` atau ganti dengan stream MP3 lain lewat tombol `a`.

## Cara kerja singkat

Alur audio:

```
HTTP stream (ICY/MP3)
   └─ countReader      → statistik byte
   └─ icyReader        → buang blok metadata ICY, ambil StreamTitle
   └─ mp3.Decode       → streamer MP3
   └─ Resample (bila perlu) → 44100 Hz
   └─ switcher         → bisa diganti saat pindah channel/reconnect
   └─ beep.Ctrl        → pause/resume
   └─ spectrum         → snapshot level untuk visualizer
   └─ effects.Volume   → volume & mute
   └─ speaker
```

- `connectLoop` berjalan di goroutine terpisah dan bertanggung jawab membuka koneksi, men-decode, lalu menunggu sinyal `switcher.ended` (putus) atau `app.reload` (pindah channel/stop/play).
- `switcher` mengisi sisa buffer dengan keheningan ketika tidak ada stream aktif, sehingga audio tidak "nyangkut" saat transisi.
- UI memakai tick 250 ms dari Bubble Tea; angka waktu dan animasi memakai `time.Now()` saat render, jadi tidak perlu state tambahan.

Struktur file:

| File | Isi |
| --- | --- |
| `main.go` | State aplikasi, pembaca ICY, koneksi & loop pemutaran, entry point. |
| `channels.go` | Daftar channel bawaan, parsing/validasi `channels.txt`, dan penentuan file daftar channel. |
| `ui.go` | Model Bubble Tea, penanganan tombol, spectrum analyzer, tata letak dan semua komponen visual. |
| `go.mod` / `go.sum` | Dependensi (bubbletea, lipgloss, beep v2). |

## Troubleshooting

| Gejala | Penyebab umum / solusi |
| --- | --- |
| `Gagal inisialisasi audio` | Tidak ada perangkat audio yang bisa diakses, atau driver/budi (Linux) bermasalah. Pastikan `libasound2-dev` terpasang saat build. |
| `Gagal: HTTP 403` / `404` | URL channel berubah atau diblokir jaringan/proxy. Coba `-url` dengan stream lain. |
| Status `Terputus, sambung ulang` berulang | Koneksi tidak stabil; aplikasi otomatis mencoba lagi tiap ±3 detik. Cek proxy (`HTTP_PROXY`/`HTTPS_PROXY` ikut dipakai). |
| Judul lagu tidak muncul | Server tidak mengirim metadata ICY, atau `Icy-Metaint` tidak ada. Audio tetap diputar normal. |
| Spectrum tidak bergerak | Stream sedang buffering/hening, atau volume 0. |
| Tampilan berantakan | Terminal terlalu sempit; lebar minimum yang nyaman ±40 kolom, dan layout dua panel butuh ±80 kolom. |

## Disclaimer

`inetradio` adalah klien pemutar pihak ketiga yang **tidak resmi** dan **tidak berafiliasi** dengan ANTENNE BAYERN GmbH & Co. KG, pemilik merek dan layanan stream yang diputar. Aplikasi ini tidak menyalin aset visual milik produk pemutar media mana pun: seluruh tampilan digambar dari kode.

Seluruh hak atas nama channel, merek, dan konten audio tetap milik pemiliknya masing-masing. Gunakan aplikasi ini untuk mendengarkan stream yang memang disediakan publik. Jika kamu ingin memakai atau mendistribusikannya secara luas, periksa sendiri ketentuan layanan dan hukum merek/hak cipta yang berlaku di wilayahmu.

## Lisensi

Dirilis di bawah lisensi [MIT](LICENSE) © 2026 Agus Supriyadi.
