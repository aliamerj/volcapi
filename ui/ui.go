package ui

import (
	"time"

	"github.com/briandowns/spinner"
	"github.com/fatih/color"
)

var (
	cGreen = color.New(color.FgHiGreen)
	cRed   = color.New(color.FgHiRed)
	cCyan  = color.New(color.FgHiCyan)
)

func SymbolPass() string {
	return cGreen.Sprint("✔")
}

func SymbolFail() string {
	return cRed.Sprint("✖")
}

func ShowSpinner(label string) *spinner.Spinner {
	s := spinner.New(spinner.CharSets[14], 100*time.Millisecond)
	s.Suffix = " " + cCyan.Sprintf(label)
	s.Color("cyan")
	s.Start()
	return s
}

