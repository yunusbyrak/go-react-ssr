package main

import (
	"fmt"

	"github.com/common-nighthawk/go-figure"
	"github.com/fatih/color"
	"github.com/yunusbyrak/go-react-ssr/gossr-cli/cmd"
	_ "github.com/yunusbyrak/go-react-ssr/gossr-cli/cmd/create"
	"github.com/yunusbyrak/go-react-ssr/gossr-cli/cmd/update"
)

func main() {
	art := figure.NewFigure("Go - SSR CLI", "slant", true)
	art.Print()
	fmt.Println()
	if update.CheckNeedsUpdate() {
		color.Magenta("🚨 A new version of gossr-cli is available! Run `gossr-cli update` to update. 🚨\n\n")
	}
	cmd.Execute()
}
