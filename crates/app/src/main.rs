use std::cmp::min;
use std::io;
use std::time::{Duration, Instant};

use crossterm::event::{self, Event, KeyCode, KeyEventKind};
use crossterm::execute;
use crossterm::terminal::{disable_raw_mode, enable_raw_mode, EnterAlternateScreen, LeaveAlternateScreen};
use rand::Rng;
use ratatui::layout::{Constraint, Direction, Layout, Rect};
use ratatui::prelude::*;
use ratatui::style::{Color, Modifier, Style};
use ratatui::text::{Line, Span};
use ratatui::widgets::{
    Block, Borders, Cell, Clear, Gauge, List, ListItem, Paragraph, Row, Sparkline, Table, Wrap,
};

const APP_TITLE: &str = "MKTS // MINI BLOOMBERG";
const TICK_RATE: Duration = Duration::from_millis(250);
const PRICE_UPDATE_RATE: Duration = Duration::from_millis(900);
const HISTORY_LEN: usize = 64;
const BANNER_TICK_RATE: Duration = Duration::from_millis(120);

fn main() -> io::Result<()> {
    enable_raw_mode()?;
    let mut stdout = io::stdout();
    execute!(stdout, EnterAlternateScreen)?;
    let backend = CrosstermBackend::new(stdout);
    let mut terminal = Terminal::new(backend)?;

    let result = run_app(&mut terminal);

    disable_raw_mode()?;
    execute!(terminal.backend_mut(), LeaveAlternateScreen)?;
    terminal.show_cursor()?;

    result
}

fn run_app(terminal: &mut Terminal<CrosstermBackend<io::Stdout>>) -> io::Result<()> {
    let mut app = App::new();
    let mut last_tick = Instant::now();
    let mut last_price_update = Instant::now();
    let mut last_banner_tick = Instant::now();

    loop {
        terminal.draw(|f| ui(f, &app))?;

        let timeout = TICK_RATE
            .checked_sub(last_tick.elapsed())
            .unwrap_or(Duration::from_secs(0));

        if event::poll(timeout)? {
            if let Event::Key(key) = event::read()? {
                if key.kind == KeyEventKind::Press {
                    if handle_key(&mut app, key.code) {
                        return Ok(());
                    }
                }
            }
        }

        if last_tick.elapsed() >= TICK_RATE {
            last_tick = Instant::now();
        }

        if last_price_update.elapsed() >= PRICE_UPDATE_RATE {
            app.update_prices();
            last_price_update = Instant::now();
        }

        if last_banner_tick.elapsed() >= BANNER_TICK_RATE {
            app.advance_banner();
            last_banner_tick = Instant::now();
        }
    }
}

fn handle_key(app: &mut App, code: KeyCode) -> bool {
    match code {
        KeyCode::Char('q') => true,
        KeyCode::Char('j') | KeyCode::Down => {
            app.select_next();
            false
        }
        KeyCode::Char('k') | KeyCode::Up => {
            app.select_prev();
            false
        }
        KeyCode::Char('r') => {
            app.reset_selection();
            false
        }
        _ => false,
    }
}

fn ui(frame: &mut Frame, app: &App) {
    let size = frame.size();
    frame.render_widget(Clear, size);

    let main_chunks = Layout::default()
        .direction(Direction::Vertical)
        .constraints([
            Constraint::Length(3),
            Constraint::Length(2),
            Constraint::Min(10),
            Constraint::Length(1),
        ])
        .split(size);

    render_header(frame, main_chunks[0], app);
    render_banner(frame, main_chunks[1], app);
    render_body(frame, main_chunks[2], app);
    render_footer(frame, main_chunks[3], app);
}

fn render_header(frame: &mut Frame, area: Rect, app: &App) {
    let title = Line::from(vec![
        Span::styled(APP_TITLE, Style::default().fg(Color::Black).bg(Color::Green)),
        Span::raw("  "),
        Span::styled(
            format!("SESSION {}  |  SYMBOLS {}", app.session, app.stocks.len()),
            Style::default().fg(Color::Green),
        ),
    ]);

    let block = Block::default().borders(Borders::ALL).style(Style::default().bg(Color::Black));
    let header = Paragraph::new(title).block(block).alignment(Alignment::Left);
    frame.render_widget(header, area);
}

fn render_banner(frame: &mut Frame, area: Rect, app: &App) {
    let text = format!(" {} ", app.banner_text());
    let banner = Paragraph::new(text)
        .block(Block::default().borders(Borders::ALL).title("NEWS TICKER"))
        .style(Style::default().fg(Color::Yellow))
        .alignment(Alignment::Left);
    frame.render_widget(banner, area);
}

fn render_body(frame: &mut Frame, area: Rect, app: &App) {
    let chunks = Layout::default()
        .direction(Direction::Horizontal)
        .constraints([Constraint::Percentage(70), Constraint::Percentage(30)])
        .split(area);

    render_main(frame, chunks[0], app);
    render_sidebar(frame, chunks[1], app);
}

fn render_footer(frame: &mut Frame, area: Rect, app: &App) {
    let status = format!(
        "VIM KEYS: q quit  j/k move  r reset  |  {}",
        app.market_status()
    );
    let footer = Paragraph::new(status)
        .style(Style::default().fg(Color::DarkGray))
        .alignment(Alignment::Left);
    frame.render_widget(footer, area);
}

fn render_main(frame: &mut Frame, area: Rect, app: &App) {
    let chunks = Layout::default()
        .direction(Direction::Vertical)
        .constraints([Constraint::Length(5), Constraint::Min(10)])
        .split(area);
    render_user_section(frame, chunks[0], app);
    let lower = Layout::default()
        .direction(Direction::Horizontal)
        .constraints([Constraint::Percentage(45), Constraint::Percentage(55)])
        .split(chunks[1]);
    render_watchlist(frame, lower[0], app);
    render_details(frame, lower[1], app);
}

fn render_user_section(frame: &mut Frame, area: Rect, app: &App) {
    let api_display = if app.api_key.is_empty() {
        "<not set>"
    } else {
        "********"
    };
    let text = vec![
        Line::from(vec![
            Span::styled("USER", Style::default().fg(Color::Cyan).add_modifier(Modifier::BOLD)),
            Span::raw("  "),
            Span::styled(app.user.as_str(), Style::default().fg(Color::White)),
        ]),
        Line::from(vec![
            Span::raw("API KEY "),
            Span::styled(api_display, Style::default().fg(Color::Yellow)),
            Span::raw("  "),
            Span::styled("press 'k' to edit (coming soon)", Style::default().fg(Color::DarkGray)),
        ]),
    ];
    let panel = Paragraph::new(text)
        .block(Block::default().borders(Borders::ALL).title("SETTINGS"))
        .wrap(Wrap { trim: true });
    frame.render_widget(panel, area);
}

fn render_watchlist(frame: &mut Frame, area: Rect, app: &App) {
    let header_cells = ["SYMBOL", "LAST", "CHG", "CHG%"]
        .iter()
        .map(|h| Cell::from(*h).style(Style::default().fg(Color::Gray)));
    let header = Row::new(header_cells).height(1).bottom_margin(0);

    let rows = app.stocks.iter().enumerate().map(|(idx, stock)| {
        let is_selected = idx == app.selected;
        let row_style = if is_selected {
            Style::default().bg(Color::DarkGray)
        } else {
            Style::default()
        };
        let chg_style = if stock.change >= 0.0 {
            Style::default().fg(Color::Green)
        } else {
            Style::default().fg(Color::Red)
        };
        Row::new(vec![
            Cell::from(stock.symbol.as_str()),
            Cell::from(format!("{:.2}", stock.price)),
            Cell::from(format!("{:+.2}", stock.change)).style(chg_style),
            Cell::from(format!("{:+.2}%", stock.change_pct)).style(chg_style),
        ])
        .style(row_style)
    });

    let table = Table::new(rows, [Constraint::Length(8), Constraint::Length(10), Constraint::Length(8), Constraint::Length(8)])
        .header(header)
        .block(Block::default().borders(Borders::ALL).title("WATCHLIST"))
        .column_spacing(1);
    frame.render_widget(table, area);
}

fn render_details(frame: &mut Frame, area: Rect, app: &App) {
    let chunks = Layout::default()
        .direction(Direction::Vertical)
        .constraints([Constraint::Length(7), Constraint::Min(10), Constraint::Length(5)])
        .split(area);

    render_quote(frame, chunks[0], app);
    render_chart(frame, chunks[1], app);
    render_news(frame, chunks[2], app);
}

fn render_sidebar(frame: &mut Frame, area: Rect, app: &App) {
    let items: Vec<ListItem> = app
        .explorer_items
        .iter()
        .enumerate()
        .map(|(idx, item)| {
            let style = if idx == app.explorer_selected {
                Style::default().fg(Color::Black).bg(Color::Cyan)
            } else {
                Style::default().fg(Color::Gray)
            };
            ListItem::new(Line::from(Span::styled(item.as_str(), style)))
        })
        .collect();

    let list = List::new(items)
        .block(Block::default().borders(Borders::ALL).title("EXPLORER"))
        .highlight_style(Style::default().fg(Color::Black).bg(Color::Cyan));
    frame.render_widget(list, area);
}

fn render_quote(frame: &mut Frame, area: Rect, app: &App) {
    let stock = app.current();
    let chg_style = if stock.change >= 0.0 {
        Style::default().fg(Color::Green)
    } else {
        Style::default().fg(Color::Red)
    };

    let gauge_ratio = if stock.day_range_high - stock.day_range_low <= 0.0 {
        0.0
    } else {
        (stock.price - stock.day_range_low) / (stock.day_range_high - stock.day_range_low)
    };
    let gauge = Gauge::default()
        .block(Block::default().title("DAY RANGE").borders(Borders::ALL))
        .gauge_style(Style::default().fg(Color::Cyan))
        .ratio(gauge_ratio.clamp(0.0, 1.0))
        .label(format!(
            "{:.2}  |  {:.2} - {:.2}",
            stock.price, stock.day_range_low, stock.day_range_high
        ));

    let quote = Paragraph::new(vec![
        Line::from(vec![
            Span::styled(stock.symbol.as_str(), Style::default().add_modifier(Modifier::BOLD)),
            Span::raw("  "),
            Span::styled(stock.name.as_str(), Style::default().fg(Color::Gray)),
        ]),
        Line::from(vec![
            Span::raw("LAST "),
            Span::styled(format!("{:.2}", stock.price), Style::default().fg(Color::White)),
            Span::raw("  CHG "),
            Span::styled(format!("{:+.2}", stock.change), chg_style),
            Span::raw("  CHG% "),
            Span::styled(format!("{:+.2}%", stock.change_pct), chg_style),
        ]),
        Line::from(vec![
            Span::raw("VOL "),
            Span::styled(format!("{:.2}M", stock.volume / 1_000_000.0), Style::default().fg(Color::Yellow)),
            Span::raw("  VWAP "),
            Span::styled(format!("{:.2}", stock.vwap), Style::default().fg(Color::White)),
            Span::raw("  OPEN "),
            Span::styled(format!("{:.2}", stock.open), Style::default().fg(Color::White)),
        ]),
    ])
    .block(Block::default().borders(Borders::ALL).title("QUOTE"))
    .wrap(Wrap { trim: true });

    let quote_chunks = Layout::default()
        .direction(Direction::Horizontal)
        .constraints([Constraint::Percentage(70), Constraint::Percentage(30)])
        .split(area);

    frame.render_widget(quote, quote_chunks[0]);
    frame.render_widget(gauge, quote_chunks[1]);
}

fn render_chart(frame: &mut Frame, area: Rect, app: &App) {
    let stock = app.current();
    let data = normalize_history(&stock.history);
    let spark = Sparkline::default()
        .block(Block::default().borders(Borders::ALL).title("INTRADAY"))
        .data(&data)
        .style(Style::default().fg(Color::Cyan));

    frame.render_widget(spark, area);
}

fn render_news(frame: &mut Frame, area: Rect, app: &App) {
    let items: Vec<ListItem> = app
        .headlines
        .iter()
        .take(3)
        .map(|h| ListItem::new(Line::from(vec![Span::styled(
            h.as_str(),
            Style::default().fg(Color::Gray),
        )])))
        .collect();

    let list = List::new(items)
        .block(Block::default().borders(Borders::ALL).title("TOP HEADLINES"));
    frame.render_widget(list, area);
}

fn normalize_history(history: &[f64]) -> Vec<u64> {
    if history.is_empty() {
        return vec![0];
    }
    let min_val = history
        .iter()
        .cloned()
        .fold(f64::INFINITY, f64::min);
    let max_val = history
        .iter()
        .cloned()
        .fold(f64::NEG_INFINITY, f64::max);
    let span = if max_val - min_val <= 0.0001 {
        1.0
    } else {
        max_val - min_val
    };
    history
        .iter()
        .map(|v| (((v - min_val) / span) * 100.0) as u64 + 1)
        .collect()
}

#[derive(Clone)]
struct Stock {
    symbol: String,
    name: String,
    price: f64,
    prev_close: f64,
    change: f64,
    change_pct: f64,
    volume: f64,
    vwap: f64,
    open: f64,
    day_range_low: f64,
    day_range_high: f64,
    history: Vec<f64>,
}

struct App {
    stocks: Vec<Stock>,
    selected: usize,
    headlines: Vec<String>,
    banner: Vec<String>,
    banner_offset: usize,
    user: String,
    api_key: String,
    explorer_items: Vec<String>,
    explorer_selected: usize,
    session: String,
    rng: rand::rngs::ThreadRng,
}

impl App {
    fn new() -> Self {
        let stocks = vec![
            Stock::seed("AAPL", "Apple Inc.", 182.42),
            Stock::seed("MSFT", "Microsoft", 413.18),
            Stock::seed("NVDA", "NVIDIA", 738.44),
            Stock::seed("TSLA", "Tesla", 196.08),
            Stock::seed("AMZN", "Amazon", 171.52),
            Stock::seed("META", "Meta Platforms", 485.36),
            Stock::seed("JPM", "JPMorgan", 178.22),
            Stock::seed("XOM", "Exxon Mobil", 104.26),
        ];

        let headlines = vec![
            "RATES: CPI cools, traders price first cut in Q3",
            "EARNINGS: Cloud spend accelerates across mega-cap",
            "ENERGY: OPEC+ signals steady supply through summer",
            "FX: USD softer as risk appetite improves",
        ]
        .into_iter()
        .map(String::from)
        .collect();

        let banner = vec![
            "MARKET: Futures edge higher ahead of Fed minutes",
            "TECH: Semis lead gains as AI capex expands",
            "MACRO: Treasury yields slip, curve steepens",
        ]
        .into_iter()
        .map(String::from)
        .collect();

        let explorer_items = vec!["Stocks", "Bonds", "Crypto", "Commodities", "FX", "News"]
            .into_iter()
            .map(String::from)
            .collect();

        Self {
            stocks,
            selected: 0,
            headlines,
            banner,
            banner_offset: 0,
            user: "guest".to_string(),
            api_key: String::new(),
            explorer_items,
            explorer_selected: 0,
            session: "OPEN".to_string(),
            rng: rand::thread_rng(),
        }
    }

    fn select_next(&mut self) {
        self.selected = min(self.selected + 1, self.stocks.len().saturating_sub(1));
    }

    fn select_prev(&mut self) {
        if self.selected > 0 {
            self.selected -= 1;
        }
    }

    fn reset_selection(&mut self) {
        self.selected = 0;
    }

    fn current(&self) -> &Stock {
        &self.stocks[self.selected]
    }

    fn banner_text(&self) -> String {
        let joined = self
            .banner
            .iter()
            .map(|s| format!("{}   ", s))
            .collect::<String>();
        if joined.is_empty() {
            return "NO HEADLINES".to_string();
        }
        let len = joined.chars().count();
        let offset = self.banner_offset % len;
        let mut rotated = joined.chars().cycle().skip(offset).take(len).collect::<String>();
        rotated.push(' ');
        rotated
    }

    fn advance_banner(&mut self) {
        if !self.banner.is_empty() {
            self.banner_offset = self.banner_offset.saturating_add(1);
        }
    }

    fn update_prices(&mut self) {
        for stock in &mut self.stocks {
            let delta = self.rng.gen_range(-0.8..0.9);
            stock.price = (stock.price + delta).max(1.0);
            stock.history.push(stock.price);
            if stock.history.len() > HISTORY_LEN {
                stock.history.remove(0);
            }
            stock.change = stock.price - stock.prev_close;
            stock.change_pct = (stock.change / stock.prev_close) * 100.0;
            stock.volume += self.rng.gen_range(20_000.0..180_000.0);
            stock.vwap = (stock.vwap + stock.price) / 2.0;
            stock.day_range_low = stock.day_range_low.min(stock.price);
            stock.day_range_high = stock.day_range_high.max(stock.price);
        }
    }

    fn market_status(&self) -> &'static str {
        "NYSE 09:30-16:00 ET"
    }
}

impl Stock {
    fn seed(symbol: &str, name: &str, price: f64) -> Self {
        let mut history = Vec::with_capacity(HISTORY_LEN);
        let mut val = price;
        for _ in 0..HISTORY_LEN {
            val *= 1.0 + ((rand::random::<f64>() - 0.5) * 0.003);
            history.push(val);
        }
        let prev_close = price * 0.995;
        let open = price * 0.99;
        let day_range_low = price * 0.98;
        let day_range_high = price * 1.02;
        let change = price - prev_close;
        let change_pct = (change / prev_close) * 100.0;

        Self {
            symbol: symbol.to_string(),
            name: name.to_string(),
            price,
            prev_close,
            change,
            change_pct,
            volume: 2_500_000.0,
            vwap: (price + open) / 2.0,
            open,
            day_range_low,
            day_range_high,
            history,
        }
    }
}
