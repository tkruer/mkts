# mkts

`mkts` is now a Bubble Tea-powered stock terminal with a Bloomberg-style layout and a simulated market tape.

## Run

```bash
go run ./cmd
```

## Controls

- `tab` / `shift+tab`: cycle focus between watchlist, movers, and news
- `j` / `k` or arrow keys: move inside the focused panel
- `enter`: load the highlighted mover into the detail pane
- `1`, `2`, `3`: switch the chart between `1D`, `1W`, and `1M`
- `q`: quit

## Layout

- Left: watchlist with prices, percent change, and volume
- Center: selected symbol detail plus an intraday-style chart
- Right: top movers and headline flow for the selected symbol

The current feed is simulated locally. There is no live brokerage or market-data connection yet.
