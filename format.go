package main

import (
	"strings"

	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

var printer *message.Printer = message.NewPrinter(language.English)

func formatUnderscores(value int) string {
	return strings.ReplaceAll(printer.Sprintf("%d", value), ",", "_")
}

func formatUnderscoresFloat(value float64) string {
	return strings.ReplaceAll(printer.Sprintf("%.2f", value), ",", "_")
}
