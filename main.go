// inetradio - pemutar radio streaming multi-channel berbasis console.
//
// Build:
//
//	go mod init inetradio
//	go get github.com/gopxl/beep/v2 github.com/charmbracelet/bubbletea github.com/charmbracelet/lipgloss
//	go build -o inetradio.exe .
package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/gopxl/beep/v2"
	"github.com/gopxl/beep/v2/effects"
	"github.com/gopxl/beep/v2/mp3"
	"github.com/gopxl/beep/v2/speaker"
)

// Palet warna pemutar media klasik era 90-an.
var (
	colBlue     = lipgloss.Color("#000080")
	colBlueHi   = lipgloss.Color("#1e40af")
	colWhite    = lipgloss.Color("#f8fafc")
	colSilver   = lipgloss.Color("#c0c0c0")
	colGray     = lipgloss.Color("#6b7280")
	colDark     = lipgloss.Color("#111827")
	colBlack    = lipgloss.Color("#000000")
	colGreen    = lipgloss.Color("#22ff88")
	colGreenDim = lipgloss.Color("#0f9d58")
	colYellow   = lipgloss.Color("#fbbf24")
	colRed      = lipgloss.Color("#ef4444")
	colPanel    = lipgloss.Color("#1f2937")
)

const sampleRate = beep.SampleRate(44100)

// ---------- daftar channel ----------

// channels diisi saat start dari file daftar channel (lihat channels.go).
var channels []channel

// ---------- streamer yang bisa diganti saat reconnect/pindah channel ----------

type switcher struct {
	mu    sync.Mutex
	s     beep.Streamer
	ended chan struct{}
}

func newSwitcher() *switcher { return &switcher{ended: make(chan struct{}, 1)} }

func (w *switcher) set(s beep.Streamer) {
	select { // buang sinyal lama
	case <-w.ended:
	default:
	}
	w.mu.Lock()
	w.s = s
	w.mu.Unlock()
}

func (w *switcher) Stream(samples [][2]float64) (int, bool) {
	w.mu.Lock()
	s := w.s
	w.mu.Unlock()

	filled := 0
	if s != nil {
		n, ok := s.Stream(samples)
		filled = n
		if !ok || (n == 0 && s.Err() != nil) {
			w.mu.Lock()
			if w.s == s { // jangan timpa stream baru
				w.s = nil
				select {
				case w.ended <- struct{}{}:
				default:
				}
			}
			w.mu.Unlock()
		}
	}
	for i := filled; i < len(samples); i++ { // isi sisanya dengan hening
		samples[i] = [2]float64{}
	}
	return len(samples), true
}

func (w *switcher) Err() error { return nil }

// ---------- pembaca stream ----------

type countReader struct {
	r io.Reader
	n *atomic.Int64
}

func (c *countReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.n.Add(int64(n))
	return n, err
}

type readCloser struct {
	io.Reader
	io.Closer
}

// ---------- state aplikasi ----------

type streamInfo struct {
	bitrate  int
	rate     int
	channels int
}

type app struct {
	mu      sync.Mutex
	status  string
	title   string
	idx     int
	level   int // 0..10
	muted   bool
	paused  bool
	stopped bool
	started time.Time
	info    streamInfo
	cnt     atomic.Pointer[atomic.Int64]
	reload  chan struct{}
	ctrl    *beep.Ctrl
	vol     *effects.Volume
	spec    *spectrum
}

func (a *app) current() channel {
	a.mu.Lock()
	defer a.mu.Unlock()
	return channels[a.idx]
}

func (a *app) index() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.idx
}

func (a *app) setStatus(s string) {
	a.mu.Lock()
	a.status = s
	a.mu.Unlock()
}

// setTitle hanya menerima judul dari stream channel yang masih aktif, supaya
// metadata dari koneksi lama tidak menimpa channel baru.
func (a *app) setTitle(url, title string) {
	a.mu.Lock()
	if channels[a.idx].URL == url {
		a.title = title
	}
	a.mu.Unlock()
}

func (a *app) setInfo(info streamInfo) {
	a.mu.Lock()
	a.info = info
	a.mu.Unlock()
}

// wake membangunkan connectLoop dari tidur/stream yang sedang berjalan.
func (a *app) wake() {
	select {
	case a.reload <- struct{}{}:
	default:
	}
}

func (a *app) switchTo(i int) {
	n := len(channels)
	i = ((i % n) + n) % n
	a.mu.Lock()
	a.idx = i
	a.started = time.Now()
	a.status = "Menyambung..."
	a.title = ""
	a.info = streamInfo{}
	a.stopped = false
	a.paused = false
	a.cnt.Store(nil)
	a.mu.Unlock()
	speaker.Lock()
	a.ctrl.Paused = false
	speaker.Unlock()
	a.wake()
}

// play: tombol play. Melanjutkan jeda atau menyambung ulang setelah stop.
func (a *app) play() {
	a.mu.Lock()
	wasStopped := a.stopped
	a.stopped = false
	a.paused = false
	if wasStopped {
		a.started = time.Now()
		a.cnt.Store(nil)
		a.status = "Menyambung..."
	}
	a.mu.Unlock()

	speaker.Lock()
	a.ctrl.Paused = false
	speaker.Unlock()

	if wasStopped {
		a.wake()
	}
}

// togglePause: tombol pause.
func (a *app) togglePause() {
	a.mu.Lock()
	if a.stopped {
		a.mu.Unlock()
		return
	}
	a.paused = !a.paused
	p := a.paused
	a.mu.Unlock()

	speaker.Lock()
	a.ctrl.Paused = p
	speaker.Unlock()
}

// stop: tombol stop. Putuskan stream supaya tidak ada audio yang tertinggal
// (radio live tidak punya posisi untuk dilanjutkan seperti file biasa).
func (a *app) stop() {
	a.mu.Lock()
	if a.stopped {
		a.mu.Unlock()
		return
	}
	a.stopped = true
	a.paused = false
	a.status = "Berhenti"
	a.title = ""
	a.cnt.Store(nil)
	a.mu.Unlock()

	speaker.Lock()
	a.ctrl.Paused = false
	speaker.Unlock()

	a.wake()
}

func (a *app) adjustVolume(d int) {
	a.mu.Lock()
	a.level += d
	if a.level < 0 {
		a.level = 0
	}
	if a.level > 10 {
		a.level = 10
	}
	a.muted = false
	a.mu.Unlock()
	a.applyVolume()
}

func (a *app) toggleMute() {
	a.mu.Lock()
	a.muted = !a.muted
	a.mu.Unlock()
	a.applyVolume()
}

func (a *app) applyVolume() {
	speaker.Lock()
	a.vol.Silent = a.muted || a.level == 0
	a.vol.Volume = float64(a.level-10) * 0.5
	speaker.Unlock()
}

// ---------- koneksi ----------

func openStream(client *http.Client, url string, onTitle func(string)) (io.ReadCloser, *atomic.Int64, streamInfo, error) {
	var info streamInfo
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, nil, info, err
	}
	req.Header.Set("User-Agent", "inetradio/1.0 (Go)")
	req.Header.Set("Icy-MetaData", "1") // minta judul lagu (ICY metadata)
	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, info, err
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, nil, info, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	info.bitrate, _ = strconv.Atoi(strings.TrimSpace(resp.Header.Get("Icy-Br")))
	for _, part := range strings.Split(resp.Header.Get("Icy-Audio-Info"), ";") {
		kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(kv) != 2 {
			continue
		}
		n, err := strconv.Atoi(strings.TrimSpace(kv[1]))
		if err != nil {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(kv[0])) {
		case "ice-bitrate":
			if info.bitrate == 0 {
				info.bitrate = n
			}
		case "ice-samplerate":
			info.rate = n
		case "ice-channels":
			info.channels = n
		}
	}

	cnt := &atomic.Int64{}
	var r io.Reader = bufio.NewReaderSize(&countReader{r: resp.Body, n: cnt}, 256<<10)
	if mi, _ := strconv.Atoi(resp.Header.Get("Icy-Metaint")); mi > 0 {
		r = newICYReader(r, mi, onTitle)
	}
	return readCloser{r, resp.Body}, cnt, info, nil
}

// icyReader membuang blok metadata ICY dari stream dan meneruskan audio murni
// ke decoder MP3. Format: metaint byte audio, 1 byte panjang, lalu metadata.
type icyReader struct {
	r         io.Reader
	metaint   int
	remain    int
	lastTitle string
	onTitle   func(string)
}

func newICYReader(r io.Reader, metaint int, onTitle func(string)) *icyReader {
	return &icyReader{r: r, metaint: metaint, remain: metaint, onTitle: onTitle}
}

func (r *icyReader) Read(p []byte) (int, error) {
	n := 0
	for n < len(p) {
		if r.remain == 0 {
			if err := r.readMeta(); err != nil {
				if n > 0 {
					return n, nil
				}
				return 0, err
			}
			continue
		}

		want := len(p) - n
		if want > r.remain {
			want = r.remain
		}
		m, err := r.r.Read(p[n : n+want])
		n += m
		r.remain -= m
		if err != nil {
			if n > 0 {
				return n, nil
			}
			return 0, err
		}
		if m == 0 {
			return n, io.ErrNoProgress
		}
	}
	return n, nil
}

func (r *icyReader) readMeta() error {
	var lb [1]byte
	if _, err := io.ReadFull(r.r, lb[:]); err != nil {
		return err
	}
	size := int(lb[0]) * 16
	if size > 0 {
		buf := make([]byte, size)
		if _, err := io.ReadFull(r.r, buf); err != nil {
			return err
		}
		if t := parseStreamTitle(buf); t != "" && t != r.lastTitle {
			r.lastTitle = t
			if r.onTitle != nil {
				r.onTitle(t)
			}
		}
	}
	r.remain = r.metaint
	return nil
}

func parseStreamTitle(b []byte) string {
	s := string(b)
	const key = "StreamTitle='"
	i := strings.Index(s, key)
	if i < 0 {
		return ""
	}
	s = s[i+len(key):]
	if j := strings.Index(s, "';"); j >= 0 {
		s = s[:j]
	} else {
		s = strings.TrimRight(s, "\x00")
	}
	return strings.TrimSpace(s)
}

// pause menunggu d, tapi bangun lebih cepat bila ada permintaan pindah channel.
func (a *app) pause(d time.Duration) {
	select {
	case <-time.After(d):
	case <-a.reload:
	}
}

func (a *app) connectLoop(sw *switcher) {
	client := &http.Client{
		Transport: &http.Transport{
			ResponseHeaderTimeout: 15 * time.Second,
			Proxy:                 http.ProxyFromEnvironment,
		},
	}
	for {
		a.mu.Lock()
		stopped := a.stopped
		a.mu.Unlock()
		if stopped {
			a.setStatus("Berhenti")
			<-a.reload // tunggu tombol putar / pindah channel
			continue
		}

		ch := a.current()
		a.setStatus("Menyambung...")
		body, cnt, info, err := openStream(client, ch.URL, func(title string) {
			a.setTitle(ch.URL, title)
		})
		if err != nil {
			a.setStatus("Gagal: " + err.Error())
			a.pause(4 * time.Second)
			continue
		}
		streamer, format, err := mp3.Decode(body)
		if err != nil {
			body.Close()
			a.setStatus("Gagal decode")
			a.pause(4 * time.Second)
			continue
		}
		a.cnt.Store(cnt)
		info.rate = int(format.SampleRate)
		info.channels = format.NumChannels
		a.setInfo(info)

		var src beep.Streamer = streamer
		if format.SampleRate != sampleRate {
			src = beep.Resample(4, format.SampleRate, sampleRate, streamer)
		}
		sw.set(src)
		a.setStatus("Memutar (live)")

		select {
		case <-sw.ended: // stream putus -> sambung ulang channel yang sama
			body.Close()
			streamer.Close()
			a.setStatus("Terputus, sambung ulang")
			a.pause(3 * time.Second)
		case <-a.reload: // pindah channel
			sw.set(nil)
			body.Close()
			streamer.Close()
		}
	}
}

func main() {
	start := flag.Int("ch", 1, "nomor channel awal (lihat daftar di UI)")
	custom := flag.String("url", "", "URL stream MP3 kustom (ditambahkan sebagai channel pertama)")
	chFile := flag.String("channels", "", "file daftar channel (format: Nama | URL)")
	flag.Parse()

	cfgPath, chans, warnings := resolveChannels(*chFile)
	channels = chans

	if *custom != "" {
		if err := validateStreamURL(*custom); err != nil {
			fmt.Fprintln(os.Stderr, "URL kustom tidak valid:", err)
			os.Exit(2)
		}
		channels = append([]channel{{"Kustom", *custom}}, channels...)
	}
	if *start < 1 || *start > len(channels) {
		*start = 1
	}

	if err := speaker.Init(sampleRate, sampleRate.N(time.Second/5)); err != nil {
		fmt.Println("Gagal inisialisasi audio:", err)
		os.Exit(1)
	}

	sw := newSwitcher()
	ctrl := &beep.Ctrl{Streamer: sw}
	spec := &spectrum{streamer: ctrl}
	vol := &effects.Volume{Streamer: spec, Base: 2}
	a := &app{
		idx:     *start - 1,
		level:   7,
		started: time.Now(),
		status:  "Menyambung...",
		reload:  make(chan struct{}, 1),
		ctrl:    ctrl,
		vol:     vol,
		spec:    spec,
	}
	a.applyVolume()
	speaker.Play(vol)
	defer speaker.Close()

	go a.connectLoop(sw)

	p := tea.NewProgram(newUIModel(a), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Println("Gagal menjalankan UI:", err)
		os.Exit(1)
	}
}
