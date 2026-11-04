package main

import (
	"bufio"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// ---------- daftar channel ----------

type channel struct {
	Name string
	URL  string
}

const base = "https://mp3channels.webradio.antenne.de/"

// defaultChannels dipakai saat file daftar channel belum ada atau tidak valid.
var defaultChannels = []channel{
	{"ANTENNE BAYERN", base + "antenne"},
	{"Top 40", base + "top-40"},
	{"Rock Antenne", base + "rockantenne"},
	{"Chillout", base + "chillout"},
	{"80er Kulthits", base + "80er-kulthits"},
	{"90er Hits", base + "90er-hits"},
	{"Oldies but Goldies", base + "oldies-but-goldies"},
	{"Classic Rock Live", base + "classic-rock-live"},
	{"New Hits", base + "new-hits"},
	{"Lovesongs", base + "lovesongs"},
	{"Fresh 4 You", base + "fresh"},
	{"Event", base + "event"},
	{"Workout Hits", base + "workout-hits"},
	{"Hits fuer Kids", base + "hits-fuer-kids"},
	{"Schlager Karussell", base + "das-schlager-karussell"},
	{"Weihnachts-Hits", base + "weihnachts-hits"},
}

// channels diisi dari file daftar channel (lihat resolveChannels).
var channels []channel

const channelsFileName = "channels.txt"

const channelsHeader = `# Daftar channel inetradio
#
# Format satu channel per baris:
#   Nama Channel | https://contoh.com/stream.mp3
#
# Catatan:
#   - Baris yang diawali # adalah komentar dan akan diabaikan.
#   - Nama boleh dikosongkan; kalau kosong, nama diambil dari URL.
#   - Baris tanpa tanda | dianggap URL saja.
#   - Perubahan pada file ini berlaku setelah aplikasi dijalankan ulang.
`

// defaultFileContent mengembalikan isi channels.txt bawaan, lengkap dengan
// komentar petunjuk supaya user tahu cara mengeditnya.
func defaultFileContent() string {
	var b strings.Builder
	b.WriteString(channelsHeader)
	b.WriteString("\n")
	for _, c := range defaultChannels {
		fmt.Fprintf(&b, "%s | %s\n", c.Name, c.URL)
	}
	return b.String()
}

// parseChannels membaca format "Nama | URL" per baris. Baris kosong dan
// komentar dilewati; baris yang tidak valid dilaporkan lewat bad.
func parseChannels(r io.Reader) (out []channel, bad []string, err error) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64<<10), 1<<20)

	line := 0
	for sc.Scan() {
		line++
		s := strings.TrimSpace(sc.Text())
		if s == "" || strings.HasPrefix(s, "#") {
			continue
		}

		name, raw := "", s
		if i := strings.Index(s, "|"); i >= 0 {
			name = strings.TrimSpace(s[:i])
			raw = strings.TrimSpace(s[i+1:])
		}
		if verr := validateStreamURL(raw); verr != nil {
			bad = append(bad, fmt.Sprintf("baris %d: %v", line, verr))
			continue
		}
		if name == "" {
			name = nameFromURL(raw)
		}
		out = append(out, channel{Name: name, URL: raw})
	}
	if serr := sc.Err(); serr != nil {
		return nil, bad, serr
	}
	return out, bad, nil
}

func validateStreamURL(raw string) error {
	if raw == "" {
		return fmt.Errorf("URL kosong")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("URL tidak valid: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("skema harus http atau https, bukan %q", u.Scheme)
	}
	if u.Host == "" {
		return fmt.Errorf("URL tidak punya host")
	}
	return nil
}

// nameFromURL membuat nama channel yang enak dibaca dari URL, misalnya
// ".../rockantenne" menjadi "rockantenne" dan ".../stream.mp3" menjadi "stream".
func nameFromURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return raw
	}
	p := strings.Trim(u.Path, "/")
	if p == "" {
		return u.Host
	}
	seg := p[strings.LastIndex(p, "/")+1:]
	seg = strings.TrimSuffix(seg, filepath.Ext(seg))
	if seg == "" {
		return u.Host
	}
	return strings.ReplaceAll(strings.ReplaceAll(seg, "-", " "), "_", " ")
}

func loadChannelsFile(path string) ([]channel, []string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()
	return parseChannels(f)
}

func userChannelsPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "inetradio", channelsFileName), nil
}

// candidatePaths adalah lokasi yang dicari otomatis, berurutan: direktori
// kerja, direktori executable, lalu folder config user.
func candidatePaths() []string {
	var out []string
	if wd, err := os.Getwd(); err == nil {
		out = append(out, filepath.Join(wd, channelsFileName))
	}
	if exe, err := os.Executable(); err == nil {
		out = append(out, filepath.Join(filepath.Dir(exe), channelsFileName))
	}
	if p, err := userChannelsPath(); err == nil {
		out = append(out, p)
	}
	return out
}

// ensureChannelsFile membuat file beserta folder induknya bila belum ada,
// tanpa menimpa file yang sudah diedit user.
func ensureChannelsFile(path string) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(defaultFileContent()), 0o644)
}

// resolveChannels menentukan file daftar channel yang dipakai.
//
// Urutan: path dari flag -channels, lalu channels.txt di direktori kerja,
// direktori executable, dan folder config user. Bila belum ada file sama
// sekali, file bawaan dibuat di folder config user.
//
// Nilai baliknya: path file yang dipakai (bisa kosong), daftar channel, dan
// peringatan yang layak ditampilkan ke user.
func resolveChannels(explicit string) (path string, chans []channel, warnings []string) {
	if explicit != "" {
		path = explicit
		if _, err := os.Stat(path); err != nil {
			if werr := ensureChannelsFile(path); werr != nil {
				return "", defaultChannels, append(warnings, "tidak bisa membuat "+path+": "+werr.Error())
			}
		}
		got, bad, err := loadChannelsFile(path)
		warnings = append(warnings, bad...)
		switch {
		case err != nil:
			warnings = append(warnings, "gagal membaca "+path+": "+err.Error())
			return path, defaultChannels, warnings
		case len(got) == 0:
			warnings = append(warnings, "tidak ada channel valid di "+path+"; memakai daftar bawaan")
			return path, defaultChannels, warnings
		}
		return path, got, warnings
	}

	for _, p := range candidatePaths() {
		if _, err := os.Stat(p); err != nil {
			continue
		}
		got, bad, err := loadChannelsFile(p)
		if err != nil {
			warnings = append(warnings, "gagal membaca "+p+": "+err.Error())
			continue
		}
		if len(got) == 0 {
			continue
		}
		return p, got, append(warnings, bad...)
	}

	p, err := userChannelsPath()
	if err != nil {
		return "", defaultChannels, warnings
	}
	if err := ensureChannelsFile(p); err != nil {
		warnings = append(warnings, "tidak bisa membuat "+p+": "+err.Error())
		return "", defaultChannels, warnings
	}
	return p, defaultChannels, warnings
}
