package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/briandowns/spinner"
	"github.com/fatih/color"
)

var (
	cGreen  = color.New(color.FgHiGreen)
	cRed    = color.New(color.FgHiRed)
	cCyan   = color.New(color.FgHiCyan)
	cBlue   = color.New(color.FgHiBlue)
	cWhite  = color.New(color.FgHiWhite, color.Bold)
	cDim    = color.New(color.Faint)
	cYellow = color.New(color.FgHiYellow)
)

func SymbolPass() string {
	return cGreen.Sprint("✔")
}

func SymbolFail() string {
	return cRed.Sprint("✖")
}

func SymbolInfo() string {
	return cBlue.Sprint("●")
}

func SymbolWarn() string {
	return cYellow.Sprint("▲")
}

func Title(text string) string {
	return cWhite.Sprint(text)
}

func Accent(text string) string {
	return cCyan.Sprint(text)
}

func Muted(text string) string {
	return cDim.Sprint(text)
}

func EndpointHeader(method, endpoint string) string {
	return fmt.Sprintf("%s %s", cBlue.Sprintf("[%s]", method), cWhite.Sprint(endpoint))
}

func Section(title string) string {
	line := strings.Repeat("─", max(10, len(title)+2))
	return fmt.Sprintf("%s\n%s %s", cDim.Sprint(line), SymbolInfo(), Title(title))
}

func ShowSpinner(label string) *spinner.Spinner {
	s := spinner.New(spinner.CharSets[14], 100*time.Millisecond)
	s.Suffix = " " + cCyan.Sprintf(label)
	s.Color("cyan")
	s.Start()
	return s
}
