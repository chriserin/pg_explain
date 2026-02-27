package main

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/viewport"
	"charm.land/lipgloss/v2"
)

type Section struct {
	viewport viewport.Model
	title    string
	subtitle string
}

var borderColor = lipgloss.Color("#452297")
var titleStyle lipgloss.Style = lipgloss.NewStyle().Background(borderColor)
var topBarStyle lipgloss.Style = lipgloss.NewStyle().Foreground(borderColor)

func (s Section) View() string {
	var buf strings.Builder
	buf.WriteString(titleStyle.Render(fmt.Sprintf(" %s ", s.title)))
	var subtitle string
	if s.subtitle != "" {
		subtitle = fmt.Sprintf(" %s ", s.subtitle)
		buf.WriteString(subtitle)
	}
	topBarLength := s.viewport.Width() - lipgloss.Width(s.title) - lipgloss.Width(subtitle) - 3
	buf.WriteString(topBarStyle.Render(strings.Repeat("─", max(0, topBarLength)) + "╗"))
	buf.WriteString("\n")
	buf.WriteString(s.viewport.View())
	return buf.String()
}

func NewSection(title string, width, height int) Section {
	vp := viewport.New(viewport.WithWidth(width-1), viewport.WithHeight(height-1))
	vp.Style = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder(), false, true, true, true).
		BorderForeground(borderColor).
		PaddingRight(2).PaddingLeft(2)

	return Section{title: title, viewport: vp}
}

func (s *Section) SetContent(content string) {
	s.viewport.SetContent("\n" + content)
}

func (s *Section) SetDimensions(width, height int) {
	s.viewport.SetWidth(width)
	s.viewport.SetHeight(height)
}

func (s *Section) LineUp(lines int) {
	s.viewport.ScrollUp(lines)
}

func (s *Section) LineDown(lines int) {
	s.viewport.ScrollDown(lines)
}
