package main

import (
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/gopxl/beep/v2"
)

// ---------- model Bubble Tea ----------

type uiModel struct {
	app      *app
	width    int
	height   int
	adding   bool
	input    string
	inputErr string
}

func newUIModel(a *app) uiModel {
	return uiModel{app: a, width: 80, height: 24}
}

type tickMsg time.Time

func tickCmd() tea.Cmd {
	return tea.Tick(250*time.Millisecond, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func (m uiModel) Init() tea.Cmd { return tickCmd() }

func (m uiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case tickMsg:
		return m, tickCmd()
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		if m.adding {
			m = m.handleAddKey(msg)
			return m, nil
		}
		if msg.String() == "a" || msg.String() == "A" {
			m.adding = true
			m.input = ""
			m.inputErr = ""
			return m, nil
		}
		if m.handleKey(msg.String()) {
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m uiModel) handleKey(key string) bool {
	switch key {
	case "q", "Q", "ctrl+c", "esc":
		return true
	case "x", "X": // play
		m.app.play()
	case "c", "C", " ": // pause
		m.app.togglePause()
	case "v", "V": // stop
		m.app.stop()
	case "b", "B", "n", "N", "right": // channel berikutnya
		m.app.switchTo(m.app.index() + 1)
	case "z", "Z", "p", "P", "left": // channel sebelumnya
		m.app.switchTo(m.app.index() - 1)
	case "up", "+", "=": // volume naik
		m.app.adjustVolume(1)
	case "down", "-", "_": // volume turun
		m.app.adjustVolume(-1)
	case "m", "M":
		m.app.toggleMute()
	default:
		if len(key) == 1 && key[0] >= '1' && key[0] <= '9' {
			if i := int(key[0] - '1'); i < len(channels) {
				m.app.switchTo(i)
			}
		}
	}
	return false
}

// handleAddKey memproses input URL saat mode tambah channel aktif.
func (m uiModel) handleAddKey(msg tea.KeyMsg) uiModel {
	switch msg.String() {
	case "esc":
		m.adding = false
		m.input = ""
		m.inputErr = ""
	case "enter":
		raw := strings.TrimSpace(m.input)
		if err := validateStreamURL(raw); err != nil {
			m.inputErr = err.Error()
			return m
		}
		m.app.addChannel(channel{Name: nameFromURL(raw), URL: raw})
		m.adding = false
		m.input = ""
		m.inputErr = ""
	case "backspace", "delete":
		if r := []rune(m.input); len(r) > 0 {
			m.input = string(r[:len(r)-1])
		}
		m.inputErr = ""
	case "ctrl+u":
		m.input = ""
		m.inputErr = ""
	default:
		if msg.Type == tea.KeyRunes || msg.Type == tea.KeySpace {
			m.input += string(msg.Runes)
			m.inputErr = ""
		}
	}
	return m
}

// footer menampilkan baris input URL saat mode tambah channel aktif.
func (m uiModel) footer() string {
	if !m.adding {
		return ""
	}
	if m.inputErr != "" {
		return "URL baru: " + m.input + "\n! " + m.inputErr
	}
	return "URL baru: " + m.input + "\nEnter simpan · Esc batal"
}

func (m uiModel) View() string {
	w, h := m.width, m.height
	if w <= 0 {
		w = 80
	}
	if h <= 0 {
		h = 24
	}
	return m.app.view(w, h, m.footer())
}

// ---------- visualizer ----------

// spectrum mengambil peak amplitudo dari audio yang benar-benar diputar,
// lalu menampilkannya sebagai bar seperti equalizer pemutar media klasik.
type spectrum struct {
	streamer beep.Streamer
	mu       sync.Mutex
	bars     [64]float64
}

func (s *spectrum) Stream(samples [][2]float64) (int, bool) {
	n, ok := s.streamer.Stream(samples)
	if n > 0 {
		var next [64]float64
		for c := range next {
			start := c * n / len(next)
			end := (c + 1) * n / len(next)
			if start >= n {
				start = n - 1
			}
			if end <= start {
				end = start + 1
			}
			if end > n {
				end = n
			}
			peak := 0.0
			for i := start; i < end; i++ {
				v := (samples[i][0] + samples[i][1]) / 2
				if v < 0 {
					v = -v
				}
				if v > peak {
					peak = v
				}
			}
			next[c] = peak
		}

		s.mu.Lock()
		for c := range s.bars {
			if next[c] > s.bars[c] {
				s.bars[c] = next[c]
			} else {
				s.bars[c] *= 0.72
			}
		}
		s.mu.Unlock()
	}
	return n, ok
}

func (s *spectrum) Err() error { return s.streamer.Err() }

func (s *spectrum) snapshot(n int) []float64 {
	if n < 1 {
		n = 1
	}
	out := make([]float64, n)
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range out {
		v := math.Sqrt(s.bars[i*len(s.bars)/n])
		if v > 1 {
			v = 1
		}
		out[i] = v
	}
	return out
}

// ---------- layout ----------

// panelHeight adalah tinggi isi panel tanpa border. Dipakai bersama oleh
// player dan playlist supaya keduanya sejajar tanpa baris kosong di bawah.
const panelHeight = 17

// copyright ditampilkan di kaki panel playlist.
const copyright = "© 2026 Agus Supriyadi"

func (a *app) view(width, height int, footer string) string {
	a.mu.Lock()
	defer a.mu.Unlock()

	winW := (width - 1) / 2
	if winW > 49 {
		winW = 49
	}
	if winW < 18 {
		winW = 18
	}

	player := a.playerView(winW)
	playlist := a.playlistView(winW)
	layout := lipgloss.JoinHorizontal(lipgloss.Top, player, " ", playlist)

	if footer != "" {
		fw := lipgloss.Width(layout)
		lines := strings.Split(footer, "\n")
		for i := range lines {
			if i == 0 {
				lines[i] = truncateLeft(lines[i], fw)
			} else {
				lines[i] = truncate(lines[i], fw)
			}
			lines[i] = lipgloss.NewStyle().
				Background(colBlue).
				Foreground(colWhite).
				Bold(true).
				Width(fw).
				Render(lines[i])
		}
		layout = lipgloss.JoinVertical(lipgloss.Left, layout, strings.Join(lines, "\n"))
	}

	if height > 21 {
		return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, layout)
	}
	return lipgloss.PlaceHorizontal(width, lipgloss.Center, layout)
}

func (a *app) playerView(winW int) string {
	cw := winW - 2
	if cw < 16 {
		cw = 16
	}

	lines := []string{
		a.titleLine(cw, "INTERNET RADIO PLAYER", "INETRADIO"),
		a.dividerLine(cw),
		insetBlock(a.lcdView(cw-2), 1, 1),
		blankLine(cw),
		bodyLine(cw, a.volumeLine(cw-2)),
		blankLine(cw), // pengganti baris BAL supaya posisi seekbar tidak bergeser
		bodyLine(cw, a.seekLine(cw-2)),
		blankLine(cw),
		bodyLine(cw, a.buttonLine(cw-2)),
		blankLine(cw),
		bodyLine(cw, a.statusLine(cw-2)),
	}

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colGray).
		Width(cw).
		Height(panelHeight).
		Render(lipgloss.JoinVertical(lipgloss.Left, lines...))
}

func (a *app) playlistView(winW int) string {
	cw := winW - 2
	if cw < 16 {
		cw = 16
	}

	lines := []string{
		a.titleLine(cw, "PLAYLIST", fmt.Sprintf("%d/%d", a.idx+1, len(channels))),
		a.dividerLine(cw),
		bodyLine(cw, lipgloss.NewStyle().Foreground(colYellow).Bold(true).Width(cw-2).Render("#  CHANNEL")),
	}

	n := len(channels)
	size := 10
	if n < size {
		size = n
	}
	first := a.idx - size/2
	if first < 0 {
		first = 0
	}
	if first > n-size {
		first = n - size
	}
	for i := first; i < first+size; i++ {
		prefix := "  "
		style := lipgloss.NewStyle().Foreground(colSilver)
		if i == a.idx {
			prefix = "> "
			style = style.Background(colBlueHi).Foreground(colWhite).Bold(true)
		}
		text := truncate(prefix+fmt.Sprintf("%2d. %s", i+1, channels[i].Name), cw-2)
		lines = append(lines, bodyLine(cw, style.Width(cw-2).Render(text)))
	}

	lines = append(lines,
		a.dividerLine(cw),
		bodyLine(cw, lipgloss.NewStyle().Foreground(colGray).Width(cw-2).Render(truncate("x play  c pause  v stop  m mute", cw-2))),
		bodyLine(cw, lipgloss.NewStyle().Foreground(colGray).Width(cw-2).Render(truncate("z/b ch  up/dn vol  a add  q quit", cw-2))),
		bodyLine(cw, lipgloss.NewStyle().Foreground(colGray).Width(cw-2).Render(truncate(copyright, cw-2))),
	)

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colGray).
		Width(cw).
		Height(panelHeight).
		Render(lipgloss.JoinVertical(lipgloss.Left, lines...))
}

func (a *app) titleLine(w int, left, right string) string {
	l := " " + left
	r := right + " "
	if lipgloss.Width(l)+lipgloss.Width(r) > w {
		r = ""
	}
	if lipgloss.Width(l) > w {
		l = truncate(left, w)
	}
	gap := w - lipgloss.Width(l) - lipgloss.Width(r)
	if gap < 0 {
		gap = 0
	}
	text := l + strings.Repeat(" ", gap) + r
	return lipgloss.NewStyle().
		Background(colBlue).
		Foreground(colWhite).
		Bold(true).
		Width(w).
		Render(text)
}

func (a *app) dividerLine(w int) string {
	return lipgloss.NewStyle().
		Foreground(colGray).
		Width(w).
		Render(strings.Repeat("─", w))
}

func blankLine(w int) string {
	return strings.Repeat(" ", w)
}

func bodyLine(w int, content string) string {
	return " " + padLine(w-2, content) + " "
}

func insetBlock(s string, left, right int) string {
	prefix := strings.Repeat(" ", left)
	suffix := strings.Repeat(" ", right)
	lines := strings.Split(s, "\n")
	for i := range lines {
		lines[i] = prefix + lines[i] + suffix
	}
	return strings.Join(lines, "\n")
}

func (a *app) lcdView(cw int) string {
	inner := cw - 2
	if inner < 1 {
		inner = 1
	}

	song := a.title
	if a.stopped {
		song = "(berhenti)"
	} else if song == "" {
		song = channels[a.idx].Name
	}

	body := []string{marquee(song, inner, time.Now())}
	body = append(body, a.scopeRows(inner)...)
	body = append(body, a.timeInfoLine(inner))

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colGreenDim).
		Foreground(colGreen).
		Background(colBlack).
		Bold(true).
		Width(inner).
		Render(lipgloss.JoinVertical(lipgloss.Left, body...))
}

func (a *app) scopeRows(w int) []string {
	const rows = 3
	if w < 1 {
		w = 1
	}
	// Tiap batang memakai satu kolom blok ditambah satu kolom spasi pemisah,
	// jadi tiap kolom analyzer terlihat terpisah seperti equalizer pemutar media klasik.
	n := (w + 1) / 2
	if n < 1 {
		n = 1
	}
	var bars []float64
	if a.spec != nil {
		bars = a.spec.snapshot(n)
	}
	colors := []lipgloss.Color{colRed, colYellow, colGreen}
	out := make([]string, rows)
	for r := 0; r < rows; r++ {
		var b strings.Builder
		b.Grow(w)
		for i, v := range bars {
			h := int(v*float64(rows) + 0.999)
			if h > rows {
				h = rows
			}
			if h > rows-1-r {
				b.WriteString("█")
			} else {
				b.WriteString(" ")
			}
			if i < len(bars)-1 {
				b.WriteString(" ")
			}
		}
		line := b.String()
		if n := w - lipgloss.Width(line); n > 0 {
			line += strings.Repeat(" ", n)
		}
		out[r] = lipgloss.NewStyle().
			Foreground(colors[r]).
			Background(colBlack).
			Render(line)
	}
	return out
}

func (a *app) timeInfoLine(w int) string {
	el := time.Since(a.started).Round(time.Second)
	h, m, s := int(el.Hours()), int(el.Minutes())%60, int(el.Seconds())%60
	clock := fmt.Sprintf("%02d:%02d:%02d", h, m, s)

	br := "---k"
	if a.info.bitrate > 0 {
		br = fmt.Sprintf("%dk", a.info.bitrate)
	}
	rate := "---kHz"
	if a.info.rate > 0 {
		rate = fmt.Sprintf("%dkHz", a.info.rate/1000)
	}
	ch := "ST"
	if a.info.channels == 1 {
		ch = "MONO"
	}
	info := fmt.Sprintf("%s %s %s", br, rate, ch)
	if w < 10+lipgloss.Width(info) {
		return lipgloss.NewStyle().Width(w).Render(truncate(clock, w))
	}
	return lipgloss.NewStyle().Width(w).Render(clock + "  " + info)
}

func (a *app) volumeLine(w int) string {
	label := lipgloss.NewStyle().Foreground(colGray).Render("VOL")
	if a.muted {
		return padLine(w, lipgloss.JoinHorizontal(lipgloss.Top,
			label, " ",
			lipgloss.NewStyle().Foreground(colRed).Bold(true).Render("[ MUTE ]")))
	}

	barW := 12
	if w < 24 {
		barW = w - 12
		if barW < 2 {
			barW = 2
		}
	}
	filled := a.level * barW / 10
	bar := strings.Repeat("█", filled) + strings.Repeat("░", barW-filled)
	level := lipgloss.NewStyle().Foreground(colSilver).Render(fmt.Sprintf("%2d/10", a.level))
	return padLine(w, lipgloss.JoinHorizontal(lipgloss.Top,
		label, " ",
		lipgloss.NewStyle().Foreground(colGray).Render("["),
		lipgloss.NewStyle().Foreground(colGreen).Render(bar),
		lipgloss.NewStyle().Foreground(colGray).Render("]"),
		" ", level))
}

func (a *app) seekLine(w int) string {
	label, color := "LIVE", colGreen
	if a.stopped {
		label, color = "STOP", colRed
	} else if a.paused {
		label, color = "PAUSE", colYellow
	}

	barW := w - len(label) - 3
	if barW < 6 {
		barW = 6
	}
	cells := make([]rune, barW)
	for i := range cells {
		cells[i] = '░'
	}
	if a.stopped || a.paused {
		for i := 0; i < 4 && i < barW; i++ {
			cells[i] = '█'
		}
	} else {
		pos := int(time.Now().UnixMilli()/140) % barW
		for i := 0; i < 4 && i < barW; i++ {
			cells[(pos+i)%barW] = '█'
		}
	}
	return padLine(w, lipgloss.JoinHorizontal(lipgloss.Top,
		lipgloss.NewStyle().Foreground(colGray).Render("["),
		lipgloss.NewStyle().Foreground(colGreen).Render(string(cells)),
		lipgloss.NewStyle().Foreground(colGray).Render("]"),
		" ",
		lipgloss.NewStyle().Foreground(color).Bold(true).Render(label)))
}

func (a *app) buttonLine(w int) string {
	type button struct {
		label  string
		active bool
		color  lipgloss.Color
	}
	playing := !a.paused && !a.stopped && strings.HasPrefix(a.status, "Memutar")

	var buttons []button
	pad := 1
	switch {
	case w >= 35:
		buttons = []button{
			{"PREV", false, colGray},
			{"PLAY", playing, colGreen},
			{"PAUSE", a.paused, colYellow},
			{"STOP", a.stopped, colRed},
			{"NEXT", false, colGray},
		}
	case w >= 24:
		buttons = []button{
			{"<<", false, colGray},
			{">", playing, colGreen},
			{"||", a.paused, colYellow},
			{"#", a.stopped, colRed},
			{">>", false, colGray},
		}
	default:
		pad = 0
		buttons = []button{
			{"<", false, colGray},
			{">", playing, colGreen},
			{"|", a.paused, colYellow},
			{"#", a.stopped, colRed},
			{">", false, colGray},
		}
	}

	chips := make([]string, 0, len(buttons))
	for _, btn := range buttons {
		style := lipgloss.NewStyle().
			Foreground(colSilver).
			Background(colPanel).
			Padding(0, pad)
		if btn.active {
			style = style.Background(btn.color).Foreground(colBlack).Bold(true)
		}
		chips = append(chips, style.Render(btn.label))
	}
	return padLine(w, strings.Join(chips, " "))
}

func (a *app) statusLine(w int) string {
	status := a.status
	if a.paused {
		status = "Jeda"
	}
	if a.stopped {
		status = "Berhenti"
	}
	line := "Status: " + status
	if c := a.cnt.Load(); c != nil && !a.stopped {
		line += fmt.Sprintf("  %.1f MB", float64(c.Load())/(1024*1024))
	}
	return lipgloss.NewStyle().
		Foreground(a.stateLipColor()).
		Width(w).
		Render(truncate(line, w))
}

func (a *app) stateLipColor() lipgloss.Color {
	switch {
	case a.stopped:
		return colRed
	case a.paused:
		return colYellow
	case strings.HasPrefix(a.status, "Memutar"):
		return colGreen
	case strings.HasPrefix(a.status, "Gagal") || strings.HasPrefix(a.status, "Terputus"):
		return colRed
	default:
		return colYellow
	}
}

// ---------- helper teks ----------

func padLine(w int, s string) string {
	if n := w - lipgloss.Width(s); n > 0 {
		return s + strings.Repeat(" ", n)
	}
	return s
}

func truncate(s string, w int) string {
	if w < 1 {
		return ""
	}
	r := []rune(s)
	if len(r) <= w {
		return s
	}
	if w <= 3 {
		return string(r[:w])
	}
	return string(r[:w-3]) + "..."
}

// truncateLeft memotong dari kiri supaya bagian akhir teks (yang sedang
// diketik) tetap terlihat.
func truncateLeft(s string, w int) string {
	if w < 1 {
		return ""
	}
	if lipgloss.Width(s) <= w {
		return s
	}
	r := []rune(s)
	for len(r) > 0 && lipgloss.Width("…"+string(r)) > w {
		r = r[1:]
	}
	return "…" + string(r)
}

func marquee(s string, w int, now time.Time) string {
	if w < 1 {
		return ""
	}
	r := []rune(s)
	if len(r) <= w {
		return padLine(w, s)
	}
	text := append(append([]rune{}, r...), []rune("   ***   ")...)
	off := int(now.UnixMilli()/250) % len(text)
	out := make([]rune, w)
	for i := range out {
		out[i] = text[(off+i)%len(text)]
	}
	return string(out)
}
