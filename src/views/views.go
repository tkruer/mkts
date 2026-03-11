package views

import (
	"fmt"
	"image/color"
	"math"
	"math/rand"
	"regexp"
	"sort"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/mattn/go-runewidth"
)

const (
	defaultWidth  = 140
	defaultHeight = 40
	minWidth      = 96
	minHeight     = 28
	headerHeight  = 3
	footerHeight  = 2
	tickInterval  = 1200 * time.Millisecond
	ellipsis      = "…"
)

var (
	ansiPattern = regexp.MustCompile(`\x1b\[[0-9;]*m`)

	paletteText     = [3]int{230, 236, 245}
	paletteMuted    = [3]int{136, 153, 177}
	paletteBorder   = [3]int{55, 77, 110}
	paletteAccent   = [3]int{255, 191, 71}
	paletteCyan     = [3]int{89, 214, 255}
	palettePositive = [3]int{71, 214, 138}
	paletteNegative = [3]int{255, 103, 121}
	paletteFocusBg  = [3]int{31, 49, 77}
	paletteHeaderBg = [3]int{10, 23, 41}
	paletteFooterBg = [3]int{8, 18, 34}
	palettePanelBg  = [3]int{12, 24, 40}
)

type focusArea int

const (
	focusWatchlist focusArea = iota
	focusMovers
	focusNews
)

func (f focusArea) label() string {
	switch f {
	case focusWatchlist:
		return "WATCHLIST"
	case focusMovers:
		return "MOVERS"
	case focusNews:
		return "NEWS"
	default:
		return "PANEL"
	}
}

type timeframe int

const (
	timeframe1D timeframe = iota
	timeframe1W
	timeframe1M
)

func (t timeframe) label() string {
	switch t {
	case timeframe1D:
		return "1D"
	case timeframe1W:
		return "1W"
	case timeframe1M:
		return "1M"
	default:
		return "1D"
	}
}

type headline struct {
	Time   string
	Source string
	Title  string
}

type quote struct {
	Symbol    string
	Name      string
	Sector    string
	Signal    string
	MarketCap string
	PE        string
	Dividend  string
	Beta      float64
	Price     float64
	PrevClose float64
	Open      float64
	High      float64
	Low       float64
	YearLow   float64
	YearHigh  float64
	Volume    int64
	AvgVolume int64
	Series1D  []float64
	Series1W  []float64
	Series1M  []float64
	Headlines []headline
}

func (q quote) change() float64 {
	return q.Price - q.PrevClose
}

func (q quote) percentChange() float64 {
	if q.PrevClose == 0 {
		return 0
	}
	return (q.change() / q.PrevClose) * 100
}

func (q quote) seriesFor(tf timeframe) []float64 {
	switch tf {
	case timeframe1W:
		return q.Series1W
	case timeframe1M:
		return q.Series1M
	default:
		return q.Series1D
	}
}

type marketIndex struct {
	Symbol    string
	Value     float64
	Change    float64
	Percent   float64
	Precision int
}

type mover struct {
	Index int
}

type tickMsg time.Time

type model struct {
	width           int
	height          int
	focus           focusArea
	timeframe       timeframe
	watchlistCursor int
	moversCursor    int
	newsCursor      int
	quotes          []quote
	indices         []marketIndex
	clock           time.Time
	status          string
	rng             *rand.Rand
}

func InitialModel() *model {
	now := time.Now()

	return &model{
		focus:     focusWatchlist,
		timeframe: timeframe1D,
		quotes:    seedQuotes(),
		indices:   seedIndices(),
		clock:     now,
		status:    "SIM FEED · delayed market tape running locally",
		rng:       rand.New(rand.NewSource(42)),
	}
}

func (m *model) Init() tea.Cmd {
	return tickCmd()
}

func tickCmd() tea.Cmd {
	return tea.Tick(tickInterval, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case tickMsg:
		m.clock = time.Time(msg)
		m.advanceMarket()
		return m, tickCmd()
	case tea.KeyPressMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "tab", "right", "l":
			m.focus = focusArea((int(m.focus) + 1) % 3)
			m.status = fmt.Sprintf("Focus moved to %s", m.focus.label())
		case "shift+tab", "left", "h":
			m.focus = focusArea((int(m.focus) + 2) % 3)
			m.status = fmt.Sprintf("Focus moved to %s", m.focus.label())
		case "up", "k":
			m.moveCursor(-1)
		case "down", "j":
			m.moveCursor(1)
		case "1":
			m.timeframe = timeframe1D
			m.status = "Chart range set to 1D"
		case "2":
			m.timeframe = timeframe1W
			m.status = "Chart range set to 1W"
		case "3":
			m.timeframe = timeframe1M
			m.status = "Chart range set to 1M"
		case "enter":
			if m.focus == focusMovers {
				m.loadMoverSelection()
			}
		}
	}

	return m, nil
}

func (m *model) View() tea.View {
	width := m.width
	if width == 0 {
		width = defaultWidth
	}

	height := m.height
	if height == 0 {
		height = defaultHeight
	}

	var content string
	if width < minWidth || height < minHeight {
		content = m.renderTooSmall(width, height)
	} else {
		content = m.renderDashboard(width, height)
	}

	v := tea.NewView(content)
	v.WindowTitle = "MKTS // Terminal"
	v.AltScreen = true
	v.BackgroundColor = color.RGBA{R: 7, G: 16, B: 29, A: 255}
	v.ForegroundColor = color.RGBA{R: 230, G: 236, B: 245, A: 255}
	return v
}

func (m *model) renderTooSmall(width, height int) string {
	lines := make([]string, 0, height)
	lines = append(lines, bannerLine(width, " MKTS // MARKET TERMINAL ", true))
	lines = append(lines, bannerLine(width, fmt.Sprintf(" terminal too small: need at least %dx%d ", minWidth, minHeight), false))
	lines = append(lines, bannerLine(width, fmt.Sprintf(" current size: %dx%d ", width, height), false))
	lines = append(lines, "")

	selected := m.selectedQuote()
	lines = append(lines, paint(fmt.Sprintf("SELECTED %s %.2f %s", selected.Symbol, selected.Price, formatSignedPct(selected.percentChange())), &paletteAccent, nil, true))
	lines = append(lines, fmt.Sprintf("%s | %s", selected.Name, selected.Sector))
	lines = append(lines, fmt.Sprintf("Status: %s", m.status))
	lines = append(lines, "")
	lines = append(lines, "Resize the terminal to unlock the full dashboard.")
	lines = append(lines, "Controls: tab cycle panels, j/k move, 1/2/3 timeframe, q quit")

	return strings.Join(lines, "\n")
}

func (m *model) renderDashboard(width, height int) string {
	mainHeight := height - headerHeight - footerHeight
	gapWidth := 1

	leftWidth := width * 34 / 100
	centerWidth := width * 36 / 100
	rightWidth := width - leftWidth - centerWidth - (gapWidth * 2)

	if leftWidth < 30 {
		leftWidth = 30
	}
	if centerWidth < 34 {
		centerWidth = 34
	}
	if rightWidth < 26 {
		deficit := 26 - rightWidth
		rightWidth = 26
		centerWidth -= deficit
	}

	if centerWidth < 30 {
		centerWidth = 30
		rightWidth = width - leftWidth - centerWidth - (gapWidth * 2)
	}

	detailHeight := 10
	if mainHeight >= 34 {
		detailHeight = 11
	}
	chartHeight := mainHeight - detailHeight - 1
	if chartHeight < 10 {
		chartHeight = 10
		detailHeight = mainHeight - chartHeight - 1
	}

	moversHeight := 10
	if mainHeight >= 34 {
		moversHeight = 11
	}
	newsHeight := mainHeight - moversHeight - 1
	if newsHeight < 10 {
		newsHeight = 10
		moversHeight = mainHeight - newsHeight - 1
	}

	header := m.renderHeader(width)
	leftPanel := m.renderWatchlistPanel(leftWidth, mainHeight)
	centerColumn := stackPanels(
		m.renderDetailPanel(centerWidth, detailHeight),
		m.renderChartPanel(centerWidth, chartHeight),
		centerWidth,
	)
	rightColumn := stackPanels(
		m.renderMoversPanel(rightWidth, moversHeight),
		m.renderNewsPanel(rightWidth, newsHeight),
		rightWidth,
	)
	body := joinHorizontal(" ", leftPanel, centerColumn, rightColumn)
	footer := m.renderFooter(width)

	lines := make([]string, 0, len(header)+len(body)+len(footer))
	lines = append(lines, header...)
	lines = append(lines, body...)
	lines = append(lines, footer...)
	return strings.Join(lines, "\n")
}

func (m *model) renderHeader(width int) []string {
	selected := m.selectedQuote()
	adv, dec := m.marketBreadth()

	clockLine := fmt.Sprintf(" MKTS // MARKET TERMINAL  %s  ", m.clock.Format("Mon Jan 02 15:04:05"))
	clockLine += fmt.Sprintf("FOCUS %s  TF %s", m.focus.label(), m.timeframe.label())

	indexParts := make([]string, 0, len(m.indices))
	for _, idx := range m.indices {
		value := fmt.Sprintf("%s %s %s", idx.Symbol, formatFloat(idx.Value, idx.Precision), formatSignedPct(idx.Percent))
		if idx.Percent >= 0 {
			indexParts = append(indexParts, paint(value, &palettePositive, nil, true))
		} else {
			indexParts = append(indexParts, paint(value, &paletteNegative, nil, true))
		}
	}

	selectedLine := fmt.Sprintf(
		" SELECTED %s %s  %s  ADV %d  DEC %d  %s ",
		selected.Symbol,
		formatFloat(selected.Price, 2),
		formatSignedPct(selected.percentChange()),
		adv,
		dec,
		m.status,
	)

	return []string{
		bannerLine(width, clockLine, true),
		bannerStyledLine(width, strings.Join(indexParts, "   ")),
		bannerLine(width, selectedLine, false),
	}
}

func (m *model) renderFooter(width int) []string {
	line1 := " tab cycle panel   j/k move   enter load mover   1/2/3 change chart   q quit "
	line2 := " simulated data only   no live brokerage connectivity "

	return []string{
		footerLine(width, line1, true),
		footerLine(width, line2, false),
	}
}

func (m *model) renderWatchlistPanel(width, height int) []string {
	innerWidth := maxInt(1, width-2)
	bodyHeight := maxInt(1, height-2)
	header := mutedText(fitPlain(
		fixedCell("SYM", 6, false)+" "+
			fixedCell("LAST", 10, true)+" "+
			fixedCell("CHG%", 8, true)+" "+
			fixedCell("VOL", maxInt(5, innerWidth-27), true),
		innerWidth,
	))

	lines := []string{header}
	visible := maxInt(1, bodyHeight-1)
	start := scrollStart(m.watchlistCursor, len(m.quotes), visible)

	for i := 0; i < visible && start+i < len(m.quotes); i++ {
		quoteIndex := start + i
		q := m.quotes[quoteIndex]
		row := fixedCell(q.Symbol, 6, false) + " " +
			fixedCell(formatFloat(q.Price, 2), 10, true) + " " +
			fixedCell(formatSignedPct(q.percentChange()), 8, true) + " " +
			fixedCell(formatVolume(q.Volume), maxInt(5, innerWidth-27), true)
		row = fitPlain(row, innerWidth)

		if quoteIndex == m.watchlistCursor {
			if m.focus == focusWatchlist {
				lines = append(lines, paint(row, &paletteText, &paletteAccent, true))
			} else {
				lines = append(lines, paint(row, &paletteText, &paletteFocusBg, true))
			}
			continue
		}

		colorChoice := &palettePositive
		if q.percentChange() < 0 {
			colorChoice = &paletteNegative
		}
		lines = append(lines, paint(row, colorChoice, nil, false))
	}

	return renderPanel(
		fmt.Sprintf("WATCHLIST %02d", len(m.quotes)),
		width,
		height,
		m.focus == focusWatchlist,
		fillBody(lines, bodyHeight, innerWidth),
	)
}

func (m *model) renderDetailPanel(width, height int) []string {
	q := m.selectedQuote()
	innerWidth := maxInt(1, width-2)
	bodyHeight := maxInt(1, height-2)
	barWidth := maxInt(10, innerWidth-20)

	changeColor := &palettePositive
	if q.change() < 0 {
		changeColor = &paletteNegative
	}

	lines := []string{
		paint(fitPlain(fmt.Sprintf("%s  %s", q.Symbol, q.Name), innerWidth), &paletteAccent, nil, true),
		mutedText(fitPlain(fmt.Sprintf("%s  |  %s", q.Sector, q.Signal), innerWidth)),
		fitStyled(
			paint(fixedCell(formatFloat(q.Price, 2)+" USD", 14, false), &paletteText, nil, true)+"  "+
				paint(fixedCell(formatSigned(q.change())+" / "+formatSignedPct(q.percentChange()), innerWidth-16, false), changeColor, nil, true),
			innerWidth,
		),
		mutedText(fitPlain(fmt.Sprintf("OPEN %s   HIGH %s   LOW %s", formatFloat(q.Open, 2), formatFloat(q.High, 2), formatFloat(q.Low, 2)), innerWidth)),
		mutedText(fitPlain(fmt.Sprintf("VOL %s   AVG %s   BETA %.2f", formatVolume(q.Volume), formatVolume(q.AvgVolume), q.Beta), innerWidth)),
		mutedText(fitPlain(fmt.Sprintf("CAP %s   P/E %s   DIV %s", q.MarketCap, q.PE, q.Dividend), innerWidth)),
		fitPlain(fmt.Sprintf("DAY  %s  %s", rangeBar(q.Price, q.Low, q.High, barWidth), formatFloat(q.Price, 2)), innerWidth),
		fitPlain(fmt.Sprintf("52W  %s  %s", rangeBar(q.Price, q.YearLow, q.YearHigh, barWidth), formatFloat(q.YearHigh, 2)), innerWidth),
	}

	return renderPanel("DETAIL", width, height, false, fillBody(lines, bodyHeight, innerWidth))
}

func (m *model) renderChartPanel(width, height int) []string {
	q := m.selectedQuote()
	innerWidth := maxInt(1, width-2)
	bodyHeight := maxInt(1, height-2)
	chartBodyHeight := maxInt(3, bodyHeight-2)
	chartLines := renderLineChart(q.seriesFor(m.timeframe), innerWidth, chartBodyHeight, q.change() >= 0)

	body := []string{
		mutedText(fitPlain(fmt.Sprintf("RANGE %s   VWAP %s   CLOSE-1 %s", m.timeframe.label(), formatFloat(average(q.seriesFor(m.timeframe)), 2), formatFloat(q.PrevClose, 2)), innerWidth)),
	}
	body = append(body, chartLines...)
	body = append(body, mutedText(fitPlain(fmt.Sprintf("SPREAD %s   TODAY %s / %s", formatSigned(q.change()), formatFloat(q.Low, 2), formatFloat(q.High, 2)), innerWidth)))

	return renderPanel("PRICE ACTION", width, height, false, fillBody(body, bodyHeight, innerWidth))
}

func (m *model) renderMoversPanel(width, height int) []string {
	innerWidth := maxInt(1, width-2)
	bodyHeight := maxInt(1, height-2)
	movers := m.rankMovers()

	lines := []string{
		mutedText(fitPlain(
			fixedCell("SYM", 6, false)+" "+
				fixedCell("MOVE", 8, true)+" "+
				fixedCell("LAST", maxInt(8, innerWidth-16), true),
			innerWidth,
		)),
	}

	visible := maxInt(1, bodyHeight-1)
	start := scrollStart(m.moversCursor, len(movers), visible)
	for i := 0; i < visible && start+i < len(movers); i++ {
		current := movers[start+i]
		q := m.quotes[current.Index]
		row := fixedCell(q.Symbol, 6, false) + " " +
			fixedCell(formatSignedPct(q.percentChange()), 8, true) + " " +
			fixedCell(formatFloat(q.Price, 2), maxInt(8, innerWidth-16), true)
		row = fitPlain(row, innerWidth)

		if start+i == m.moversCursor {
			if m.focus == focusMovers {
				lines = append(lines, paint(row, &paletteText, &paletteAccent, true))
			} else {
				lines = append(lines, paint(row, &paletteText, &paletteFocusBg, true))
			}
			continue
		}

		colorChoice := &palettePositive
		if q.percentChange() < 0 {
			colorChoice = &paletteNegative
		}
		lines = append(lines, paint(row, colorChoice, nil, false))
	}

	return renderPanel("TOP MOVERS", width, height, m.focus == focusMovers, fillBody(lines, bodyHeight, innerWidth))
}

func (m *model) renderNewsPanel(width, height int) []string {
	q := m.selectedQuote()
	innerWidth := maxInt(1, width-2)
	bodyHeight := maxInt(1, height-2)
	lines := []string{
		mutedText(fitPlain(fmt.Sprintf("%s headlines", q.Symbol), innerWidth)),
	}

	visible := maxInt(1, bodyHeight-1)
	start := scrollStart(m.newsCursor, len(q.Headlines), visible)
	for i := 0; i < visible && start+i < len(q.Headlines); i++ {
		item := q.Headlines[start+i]
		row := fixedCell(item.Time, 5, false) + " " +
			fixedCell(item.Source, 4, false) + " " +
			fixedCell(item.Title, maxInt(8, innerWidth-11), false)
		row = fitPlain(row, innerWidth)

		if start+i == m.newsCursor {
			if m.focus == focusNews {
				lines = append(lines, paint(row, &paletteText, &paletteAccent, true))
			} else {
				lines = append(lines, paint(row, &paletteText, &paletteFocusBg, true))
			}
			continue
		}

		lines = append(lines, paint(row, &paletteCyan, nil, false))
	}

	return renderPanel("NEWS FLOW", width, height, m.focus == focusNews, fillBody(lines, bodyHeight, innerWidth))
}

func (m *model) moveCursor(delta int) {
	switch m.focus {
	case focusWatchlist:
		m.watchlistCursor = clampInt(m.watchlistCursor+delta, 0, len(m.quotes)-1)
		m.newsCursor = 0
		m.status = fmt.Sprintf("Loaded %s into the detail pane", m.selectedQuote().Symbol)
	case focusMovers:
		m.moversCursor = clampInt(m.moversCursor+delta, 0, len(m.rankMovers())-1)
		m.status = fmt.Sprintf("Mover shortlist row %d selected", m.moversCursor+1)
	case focusNews:
		total := len(m.selectedQuote().Headlines)
		m.newsCursor = clampInt(m.newsCursor+delta, 0, total-1)
		m.status = fmt.Sprintf("News headline %d in focus", m.newsCursor+1)
	}
}

func (m *model) loadMoverSelection() {
	movers := m.rankMovers()
	if len(movers) == 0 {
		return
	}

	m.watchlistCursor = movers[clampInt(m.moversCursor, 0, len(movers)-1)].Index
	m.newsCursor = 0
	m.focus = focusWatchlist
	m.status = fmt.Sprintf("Loaded mover %s into the detail pane", m.selectedQuote().Symbol)
}

func (m *model) selectedQuote() *quote {
	return &m.quotes[clampInt(m.watchlistCursor, 0, len(m.quotes)-1)]
}

func (m *model) marketBreadth() (int, int) {
	advancers := 0
	decliners := 0
	for _, q := range m.quotes {
		switch {
		case q.percentChange() > 0:
			advancers++
		case q.percentChange() < 0:
			decliners++
		}
	}
	return advancers, decliners
}

func (m *model) rankMovers() []mover {
	result := make([]mover, len(m.quotes))
	for i := range m.quotes {
		result[i] = mover{Index: i}
	}

	sort.Slice(result, func(i, j int) bool {
		left := math.Abs(m.quotes[result[i].Index].percentChange())
		right := math.Abs(m.quotes[result[j].Index].percentChange())
		if left == right {
			return m.quotes[result[i].Index].Symbol < m.quotes[result[j].Index].Symbol
		}
		return left > right
	})

	return result
}

func (m *model) advanceMarket() {
	for i := range m.quotes {
		q := &m.quotes[i]
		move := (m.rng.Float64() - 0.49) * q.Price * 0.0022
		next := math.Max(1, q.Price+move)
		q.Price = next
		q.High = math.Max(q.High, next)
		q.Low = math.Min(q.Low, next)
		q.Volume += int64(50000 + m.rng.Intn(140000))
		q.Series1D = append(q.Series1D[1:], next)
		q.Signal = signalForQuote(*q)
	}

	for i := range m.indices {
		idx := &m.indices[i]
		move := (m.rng.Float64() - 0.48) * idx.Value * 0.0007
		idx.Value += move
		idx.Change += move
		if idx.Value != 0 {
			idx.Percent = (idx.Change / (idx.Value - idx.Change)) * 100
		}
	}
}

func seedQuotes() []quote {
	rng := rand.New(rand.NewSource(7))
	quotes := []quote{
		{
			Symbol:    "AAPL",
			Name:      "Apple",
			Sector:    "Consumer Tech",
			MarketCap: "3.28T",
			PE:        "33.9",
			Dividend:  "0.49%",
			Beta:      1.21,
			Price:     219.34,
			PrevClose: 217.82,
			Open:      218.10,
			High:      220.41,
			Low:       216.74,
			YearLow:   164.08,
			YearHigh:  238.92,
			Volume:    68120000,
			AvgVolume: 59000000,
			Headlines: []headline{
				{Time: "09:12", Source: "BGN", Title: "Services mix offsets softer iPhone sell-through in channel checks"},
				{Time: "10:08", Source: "WSJ", Title: "Apple expands on-device AI controls ahead of enterprise push"},
				{Time: "11:22", Source: "RTR", Title: "Suppliers flag stable lead times into the next product cycle"},
				{Time: "13:41", Source: "FT", Title: "Investors lean defensive while keeping mega-cap exposure"},
			},
		},
		{
			Symbol:    "MSFT",
			Name:      "Microsoft",
			Sector:    "Software",
			MarketCap: "3.12T",
			PE:        "36.4",
			Dividend:  "0.67%",
			Beta:      0.92,
			Price:     428.67,
			PrevClose: 425.12,
			Open:      426.48,
			High:      430.25,
			Low:       423.88,
			YearLow:   362.90,
			YearHigh:  468.35,
			Volume:    22450000,
			AvgVolume: 21000000,
			Headlines: []headline{
				{Time: "08:58", Source: "BGN", Title: "Azure bookings stay firm as enterprise demand rotates to inference"},
				{Time: "09:47", Source: "CNBC", Title: "Cloud unit seen absorbing more AI capex without margin reset"},
				{Time: "12:06", Source: "RTR", Title: "Copilot pricing remains in focus for CIO budget meetings"},
				{Time: "14:18", Source: "DJN", Title: "Broker checks point to resilient seat expansion in productivity suite"},
			},
		},
		{
			Symbol:    "NVDA",
			Name:      "NVIDIA",
			Sector:    "Semiconductors",
			MarketCap: "2.84T",
			PE:        "48.2",
			Dividend:  "0.03%",
			Beta:      1.66,
			Price:     118.42,
			PrevClose: 115.11,
			Open:      116.84,
			High:      119.28,
			Low:       114.96,
			YearLow:   72.11,
			YearHigh:  153.13,
			Volume:    345200000,
			AvgVolume: 289000000,
			Headlines: []headline{
				{Time: "09:01", Source: "BGN", Title: "Hyperscaler checks keep GPU backlog narrative intact"},
				{Time: "10:34", Source: "RTR", Title: "Foundry supply remains tight for advanced packaging lanes"},
				{Time: "11:57", Source: "FT", Title: "Chip basket rallies on fresh sovereign AI spending talk"},
				{Time: "15:02", Source: "DJN", Title: "Options desks flag call demand returning after recent pullback"},
			},
		},
		{
			Symbol:    "AMZN",
			Name:      "Amazon",
			Sector:    "E-Commerce",
			MarketCap: "2.03T",
			PE:        "41.7",
			Dividend:  "n/a",
			Beta:      1.13,
			Price:     204.19,
			PrevClose: 202.94,
			Open:      203.16,
			High:      205.01,
			Low:       201.88,
			YearLow:   151.61,
			YearHigh:  242.52,
			Volume:    37580000,
			AvgVolume: 40800000,
			Headlines: []headline{
				{Time: "09:19", Source: "BGN", Title: "Retail basket data shows steady unit growth into quarter-end"},
				{Time: "10:52", Source: "RTR", Title: "AWS pricing discipline seen holding as model usage scales"},
				{Time: "13:14", Source: "FT", Title: "Logistics analysts note lower last-mile surcharge pressure"},
				{Time: "15:22", Source: "DJN", Title: "Street keeps margin focus on fulfillment network density gains"},
			},
		},
		{
			Symbol:    "META",
			Name:      "Meta",
			Sector:    "Digital Ads",
			MarketCap: "1.46T",
			PE:        "29.8",
			Dividend:  "0.36%",
			Beta:      1.27,
			Price:     582.75,
			PrevClose: 579.11,
			Open:      580.06,
			High:      584.41,
			Low:       575.80,
			YearLow:   414.50,
			YearHigh:  638.40,
			Volume:    16230000,
			AvgVolume: 15000000,
			Headlines: []headline{
				{Time: "08:43", Source: "BGN", Title: "Ad checks improve on broad-based demand from travel and retail"},
				{Time: "11:05", Source: "RTR", Title: "Investors eye AI spend cadence after latest model roadmap"},
				{Time: "12:31", Source: "FT", Title: "Short-video monetization gains offset softer Europe commerce ads"},
				{Time: "14:47", Source: "DJN", Title: "Analysts expect continued share capture in performance marketing"},
			},
		},
		{
			Symbol:    "GOOGL",
			Name:      "Alphabet",
			Sector:    "Internet",
			MarketCap: "2.06T",
			PE:        "25.6",
			Dividend:  "0.47%",
			Beta:      1.02,
			Price:     184.62,
			PrevClose: 185.34,
			Open:      185.08,
			High:      186.91,
			Low:       183.77,
			YearLow:   131.55,
			YearHigh:  207.05,
			Volume:    28110000,
			AvgVolume: 26200000,
			Headlines: []headline{
				{Time: "09:03", Source: "BGN", Title: "Search ad pacing eases but cloud profitability remains a cushion"},
				{Time: "10:16", Source: "RTR", Title: "Generative AI answer formats stay central to traffic questions"},
				{Time: "13:02", Source: "DJN", Title: "Broker note points to steady YouTube premium conversion"},
				{Time: "14:59", Source: "FT", Title: "Street balances antitrust risk against improving cost discipline"},
			},
		},
		{
			Symbol:    "TSLA",
			Name:      "Tesla",
			Sector:    "Automotive",
			MarketCap: "631B",
			PE:        "58.7",
			Dividend:  "n/a",
			Beta:      2.04,
			Price:     201.54,
			PrevClose: 207.80,
			Open:      205.42,
			High:      206.33,
			Low:       200.41,
			YearLow:   138.80,
			YearHigh:  299.29,
			Volume:    96840000,
			AvgVolume: 104000000,
			Headlines: []headline{
				{Time: "09:07", Source: "RTR", Title: "Delivery estimate revisions stay heavy after recent incentive checks"},
				{Time: "10:41", Source: "BGN", Title: "Margins stay under scrutiny as inventory mix shifts lower"},
				{Time: "12:50", Source: "CNBC", Title: "Analysts watch next autonomy rollout for demand reacceleration"},
				{Time: "14:26", Source: "DJN", Title: "Auto suppliers note continued cautious ordering behavior"},
			},
		},
		{
			Symbol:    "JPM",
			Name:      "JPMorgan",
			Sector:    "Banks",
			MarketCap: "598B",
			PE:        "13.4",
			Dividend:  "2.11%",
			Beta:      0.83,
			Price:     211.08,
			PrevClose: 209.94,
			Open:      210.22,
			High:      211.66,
			Low:       208.70,
			YearLow:   172.62,
			YearHigh:  226.75,
			Volume:    11260000,
			AvgVolume: 9900000,
			Headlines: []headline{
				{Time: "08:51", Source: "BGN", Title: "NII commentary steadies as curve volatility stays contained"},
				{Time: "10:11", Source: "RTR", Title: "Credit desks report constructive issuance tone across investment grade"},
				{Time: "13:38", Source: "FT", Title: "Bank basket edges up with capital return still in focus"},
				{Time: "15:07", Source: "DJN", Title: "Analysts highlight operating leverage from expense discipline"},
			},
		},
		{
			Symbol:    "XOM",
			Name:      "Exxon Mobil",
			Sector:    "Energy",
			MarketCap: "472B",
			PE:        "14.2",
			Dividend:  "3.18%",
			Beta:      0.88,
			Price:     119.46,
			PrevClose: 118.03,
			Open:      118.42,
			High:      120.08,
			Low:       117.98,
			YearLow:   95.77,
			YearHigh:  126.34,
			Volume:    17620000,
			AvgVolume: 16200000,
			Headlines: []headline{
				{Time: "09:24", Source: "RTR", Title: "Crude strength lifts integrated oils as refining margins stabilize"},
				{Time: "11:18", Source: "BGN", Title: "Permian productivity outlook supports steady buyback expectations"},
				{Time: "13:09", Source: "FT", Title: "Natural gas recovery tempers caution around chemical demand"},
				{Time: "14:55", Source: "DJN", Title: "Street expects upstream cash generation to remain elevated"},
			},
		},
	}

	for i := range quotes {
		quotes[i].Series1D = buildSeries(quotes[i].Open, quotes[i].Price, 48, quotes[i].Price*0.012, rng)
		quotes[i].Series1W = buildSeries(quotes[i].PrevClose*0.97, quotes[i].Price, 40, quotes[i].Price*0.025, rng)
		quotes[i].Series1M = buildSeries(quotes[i].YearLow+(quotes[i].YearHigh-quotes[i].YearLow)*0.36, quotes[i].Price, 56, quotes[i].Price*0.04, rng)
		quotes[i].Signal = signalForQuote(quotes[i])
	}

	return quotes
}

func seedIndices() []marketIndex {
	return []marketIndex{
		{Symbol: "SPX", Value: 5231.48, Change: 21.71, Percent: 0.42, Precision: 2},
		{Symbol: "NDX", Value: 18342.21, Change: 118.34, Percent: 0.65, Precision: 2},
		{Symbol: "DJI", Value: 39102.84, Change: -41.28, Percent: -0.11, Precision: 2},
		{Symbol: "RTY", Value: 2077.14, Change: 12.04, Percent: 0.58, Precision: 2},
		{Symbol: "VIX", Value: 14.21, Change: -0.31, Percent: -2.14, Precision: 2},
	}
}

func buildSeries(start, end float64, points int, noise float64, rng *rand.Rand) []float64 {
	if points < 2 {
		return []float64{end}
	}

	series := make([]float64, points)
	for i := 0; i < points; i++ {
		progress := float64(i) / float64(points-1)
		base := start + ((end - start) * progress)
		wiggle := (rng.Float64() - 0.5) * noise
		value := math.Max(1, base+wiggle)
		if i == 0 {
			value = start
		}
		if i == points-1 {
			value = end
		}

		if i > 0 {
			value = (series[i-1] * 0.35) + (value * 0.65)
		}

		series[i] = value
	}

	series[0] = start
	series[len(series)-1] = end
	return series
}

func signalForQuote(q quote) string {
	change := q.percentChange()
	switch {
	case change >= 2:
		return "Momentum bid"
	case change >= 0.5:
		return "Steady accumulation"
	case change <= -2:
		return "Risk unwind"
	case change <= -0.5:
		return "Heavy offer"
	case q.Price >= q.Open:
		return "Balanced tape"
	default:
		return "Range trade"
	}
}

func renderPanel(title string, width, height int, active bool, body []string) []string {
	if width < 4 {
		return []string{fitPlain("", width)}
	}
	if height < 3 {
		return []string{fitPlain(title, width)}
	}

	innerWidth := width - 2
	bodyHeight := height - 2
	lines := make([]string, 0, height)

	titleText := " " + title + " "
	titleWidth := runewidth.StringWidth(titleText)
	fillWidth := maxInt(0, width-2-titleWidth)
	borderColor := paletteBorder
	titleColor := paletteAccent
	titleBg := [3]int{}
	titleHasBg := false

	if active {
		borderColor = paletteAccent
		titleColor = palettePanelBg
		titleBg = paletteAccent
		titleHasBg = true
	}

	top := paint("┌", &borderColor, nil, false)
	if titleHasBg {
		top += paint(titleText, &titleColor, &titleBg, true)
	} else {
		top += paint(titleText, &titleColor, nil, true)
	}
	top += paint(strings.Repeat("─", fillWidth), &borderColor, nil, false)
	top += paint("┐", &borderColor, nil, false)
	lines = append(lines, top)

	for i := 0; i < bodyHeight; i++ {
		content := fitStyled("", innerWidth)
		if i < len(body) {
			content = fitStyled(body[i], innerWidth)
		}
		line := paint("│", &borderColor, nil, false) + content + paint("│", &borderColor, nil, false)
		lines = append(lines, line)
	}

	bottom := paint("└"+strings.Repeat("─", width-2)+"┘", &borderColor, nil, false)
	lines = append(lines, bottom)
	return lines
}

func renderLineChart(series []float64, width, height int, positive bool) []string {
	if len(series) == 0 {
		return fillBody(nil, height, width)
	}
	if width < 12 || height < 3 {
		return []string{fitPlain(renderSparkline(series, width), width)}
	}

	labelWidth := 8
	if width < 22 {
		labelWidth = 0
	}

	chartWidth := width
	if labelWidth > 0 {
		chartWidth = width - labelWidth - 1
	}
	if chartWidth < 6 {
		chartWidth = width
		labelWidth = 0
	}

	points := sampleSeries(series, chartWidth)
	minValue, maxValue := bounds(points)
	spread := maxValue - minValue
	if spread == 0 {
		spread = 1
	}

	grid := make([][]rune, height)
	for row := range grid {
		grid[row] = []rune(strings.Repeat(" ", chartWidth))
	}

	prevY := -1
	for x, value := range points {
		y := height - 1 - int(math.Round(((value-minValue)/spread)*float64(height-1)))
		y = clampInt(y, 0, height-1)

		if prevY >= 0 {
			step := 0
			switch {
			case y > prevY:
				step = 1
			case y < prevY:
				step = -1
			}
			if step == 0 {
				grid[y][x] = '─'
			} else {
				for fill := prevY; fill != y; fill += step {
					grid[fill][x] = '│'
				}
			}
		}

		if x == len(points)-1 {
			grid[y][x] = '●'
		} else {
			grid[y][x] = '•'
		}

		prevY = y
	}

	chartColor := palettePositive
	if !positive {
		chartColor = paletteNegative
	}

	lines := make([]string, 0, height)
	for row := 0; row < height; row++ {
		var prefix string
		if labelWidth > 0 {
			label := strings.Repeat(" ", labelWidth)
			switch row {
			case 0:
				label = fixedCell(formatFloat(maxValue, 2), labelWidth, true)
			case height / 2:
				label = fixedCell(formatFloat(minValue+(spread/2), 2), labelWidth, true)
			case height - 1:
				label = fixedCell(formatFloat(minValue, 2), labelWidth, true)
			}
			prefix = mutedText(label + " ")
		}

		lines = append(lines, prefix+paint(string(grid[row]), &chartColor, nil, false))
	}

	return lines
}

func renderSparkline(series []float64, width int) string {
	if len(series) == 0 || width <= 0 {
		return ""
	}

	blocks := []rune("▁▂▃▄▅▆▇█")
	points := sampleSeries(series, width)
	minValue, maxValue := bounds(points)
	spread := maxValue - minValue
	if spread == 0 {
		spread = 1
	}

	var builder strings.Builder
	for _, value := range points {
		position := int(math.Round(((value - minValue) / spread) * float64(len(blocks)-1)))
		position = clampInt(position, 0, len(blocks)-1)
		builder.WriteRune(blocks[position])
	}
	return builder.String()
}

func sampleSeries(series []float64, width int) []float64 {
	if width <= 1 {
		return []float64{series[len(series)-1]}
	}

	if len(series) <= width {
		out := make([]float64, 0, width)
		out = append(out, series...)
		for len(out) < width {
			out = append(out, series[len(series)-1])
		}
		return out
	}

	points := make([]float64, width)
	last := len(series) - 1
	for i := 0; i < width; i++ {
		position := int(math.Round(float64(i) / float64(width-1) * float64(last)))
		points[i] = series[position]
	}
	return points
}

func rangeBar(value, low, high float64, width int) string {
	if width <= 0 {
		return ""
	}

	ratio := 0.5
	if high > low {
		ratio = (value - low) / (high - low)
	}
	ratio = math.Max(0, math.Min(1, ratio))

	position := clampInt(int(math.Round(ratio*float64(width-1))), 0, width-1)
	bar := make([]rune, width)
	for i := range bar {
		bar[i] = '░'
	}

	for i := 0; i < position; i++ {
		bar[i] = '█'
	}
	bar[position] = '◆'
	return string(bar)
}

func stackPanels(top, bottom []string, width int) []string {
	out := make([]string, 0, len(top)+len(bottom)+1)
	out = append(out, top...)
	out = append(out, strings.Repeat(" ", width))
	out = append(out, bottom...)
	return out
}

func joinHorizontal(gap string, columns ...[]string) []string {
	if len(columns) == 0 {
		return nil
	}

	maxHeight := 0
	widths := make([]int, len(columns))
	for i, column := range columns {
		if len(column) > maxHeight {
			maxHeight = len(column)
		}
		for _, line := range column {
			widths[i] = maxInt(widths[i], styledWidth(line))
		}
	}

	out := make([]string, maxHeight)
	for row := 0; row < maxHeight; row++ {
		var builder strings.Builder
		for columnIndex, column := range columns {
			if columnIndex > 0 {
				builder.WriteString(gap)
			}

			if row < len(column) {
				builder.WriteString(fitStyled(column[row], widths[columnIndex]))
			} else {
				builder.WriteString(strings.Repeat(" ", widths[columnIndex]))
			}
		}
		out[row] = builder.String()
	}

	return out
}

func fillBody(lines []string, bodyHeight, width int) []string {
	out := make([]string, 0, bodyHeight)
	for _, line := range lines {
		if len(out) == bodyHeight {
			break
		}
		out = append(out, fitStyled(line, width))
	}

	for len(out) < bodyHeight {
		out = append(out, strings.Repeat(" ", width))
	}

	return out
}

func bannerLine(width int, text string, accent bool) string {
	foreground := paletteText
	background := paletteHeaderBg
	if accent {
		foreground = paletteAccent
	}
	return paint(fitPlain(text, width), &foreground, &background, true)
}

func bannerStyledLine(width int, content string) string {
	return fitStyled(content, width)
}

func footerLine(width int, text string, accent bool) string {
	foreground := paletteMuted
	if accent {
		foreground = paletteCyan
	}
	return paint(fitPlain(text, width), &foreground, &paletteFooterBg, false)
}

func paint(s string, fg, bg *[3]int, bold bool) string {
	codes := make([]string, 0, 3)
	if bold {
		codes = append(codes, "1")
	}
	if fg != nil {
		codes = append(codes, fmt.Sprintf("38;2;%d;%d;%d", fg[0], fg[1], fg[2]))
	}
	if bg != nil {
		codes = append(codes, fmt.Sprintf("48;2;%d;%d;%d", bg[0], bg[1], bg[2]))
	}
	if len(codes) == 0 {
		return s
	}
	return "\033[" + strings.Join(codes, ";") + "m" + s + "\033[0m"
}

func mutedText(s string) string {
	return paint(s, &paletteMuted, nil, false)
}

func fixedCell(s string, width int, alignRight bool) string {
	if width <= 0 {
		return ""
	}

	if runewidth.StringWidth(s) > width {
		s = truncatePlain(s, width)
	}

	padding := strings.Repeat(" ", maxInt(0, width-runewidth.StringWidth(s)))
	if alignRight {
		return padding + s
	}
	return s + padding
}

func fitPlain(s string, width int) string {
	if width <= 0 {
		return ""
	}
	return fixedCell(s, width, false)
}

func fitStyled(s string, width int) string {
	if width <= 0 {
		return ""
	}

	plain := stripANSI(s)
	if runewidth.StringWidth(plain) > width {
		return fitPlain(plain, width)
	}

	return s + strings.Repeat(" ", width-runewidth.StringWidth(plain))
}

func stripANSI(s string) string {
	return ansiPattern.ReplaceAllString(s, "")
}

func styledWidth(s string) int {
	return runewidth.StringWidth(stripANSI(s))
}

func truncatePlain(s string, width int) string {
	if width <= 0 {
		return ""
	}
	if runewidth.StringWidth(s) <= width {
		return s
	}
	if width == 1 {
		return ellipsis
	}

	var builder strings.Builder
	consumed := 0
	for _, r := range s {
		runeWidth := runewidth.RuneWidth(r)
		if consumed+runeWidth > width-1 {
			break
		}
		builder.WriteRune(r)
		consumed += runeWidth
	}
	builder.WriteString(ellipsis)
	return builder.String()
}

func scrollStart(cursor, total, visible int) int {
	if total <= visible {
		return 0
	}

	start := cursor - (visible / 2)
	if start < 0 {
		start = 0
	}

	limit := total - visible
	if start > limit {
		start = limit
	}

	return start
}

func bounds(series []float64) (float64, float64) {
	minimum := series[0]
	maximum := series[0]
	for _, value := range series[1:] {
		minimum = math.Min(minimum, value)
		maximum = math.Max(maximum, value)
	}
	return minimum, maximum
}

func average(series []float64) float64 {
	if len(series) == 0 {
		return 0
	}
	total := 0.0
	for _, value := range series {
		total += value
	}
	return total / float64(len(series))
}

func formatVolume(value int64) string {
	switch {
	case value >= 1_000_000_000:
		return fmt.Sprintf("%.1fB", float64(value)/1_000_000_000)
	case value >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(value)/1_000_000)
	case value >= 1_000:
		return fmt.Sprintf("%.1fK", float64(value)/1_000)
	default:
		return fmt.Sprintf("%d", value)
	}
}

func formatSigned(value float64) string {
	if value >= 0 {
		return fmt.Sprintf("+%.2f", value)
	}
	return fmt.Sprintf("%.2f", value)
}

func formatSignedPct(value float64) string {
	if value >= 0 {
		return fmt.Sprintf("+%.2f%%", value)
	}
	return fmt.Sprintf("%.2f%%", value)
}

func formatFloat(value float64, precision int) string {
	return fmt.Sprintf("%.*f", precision, value)
}

func clampInt(value, min, max int) int {
	if max < min {
		return min
	}
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
