package main

import tea "github.com/charmbracelet/bubbletea"

type model struct{}

func (model) Init() tea.Cmd { return nil }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if k, ok := msg.(tea.KeyMsg); ok {
		switch k.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		}
	}
	return m, nil
}

func (model) View() string { return "gtui – press q to quit\n" }
