package ui

import "github.com/charmbracelet/lipgloss"

var (
    TitleStyle = lipgloss.NewStyle().Bold(true)
    KeyStyle   = lipgloss.NewStyle().Bold(true)
    ErrStyle   = lipgloss.NewStyle().Bold(true)
)

func Title(s string) string { return TitleStyle.Render(s) }
func Key(s string) string   { return KeyStyle.Render(s) }
func Err(s string) string   { return ErrStyle.Render(s) }
