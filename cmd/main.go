package main

import (
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"
	"github.com/tkruer/mkts/src/views"
)

func main() {
	p := tea.NewProgram(views.InitialModel())
	if _, err := p.Run(); err != nil {
		fmt.Printf("failed to run: %s", err)
		os.Exit(1)
	}
}
