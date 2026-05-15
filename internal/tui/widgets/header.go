package widgets

import (
	"strings"

	"charm.land/lipgloss/v2"
)

type Header struct {
	style lipgloss.Style
	width int
	title string
}

func NewHeader(style lipgloss.Style, title string) *Header {
	return &Header{
		style: style,
		title: title,
	}
}

func (h *Header) SetWidth(width int) {
	h.width = width
}

func (h *Header) GetWidth() int {
	return h.width
}

func (h *Header) UpdateStyles(style lipgloss.Style) {
	h.style = style
}

func (h Header) View() string {
	if h.width <= 0 || len(h.title) >= h.width {
		return h.style.Render(h.title)
	}
	padding := (h.width - len(h.title)) / 2
	if padding < 0 {
		padding = 0
	}
	spaces := strings.Repeat(" ", padding)
	return h.style.Render(spaces + h.title + spaces)
}
