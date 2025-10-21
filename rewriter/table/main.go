package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/olekukonko/tablewriter"
	"github.com/olekukonko/tablewriter/renderer"
	"github.com/olekukonko/tablewriter/tw"
)

func main() {
	flag.Parse()
	args := flag.Args()
	if len(args) == 0 {
		log.Fatal("no input")
	}
	file, err := os.ReadFile(args[0])
	if err != nil {
		log.Fatal(err)
	}

	lines := strings.Split(string(file), "\n")

	data := [][]any{}
	row := make([]any, 0)

	symbols := tw.NewSymbolCustom("Table").
		WithRow("-").
		WithColumn("|").
		WithTopLeft("+").
		WithTopMid("+").
		WithTopRight("+").
		WithMidLeft("+").
		WithCenter("+").
		WithMidRight("+").
		WithBottomLeft("+").
		WithBottomMid("+").
		WithBottomRight("+")

	blockBase := ".. code-block::\n\n"

	block := blockBase
	inTable := false
	for _, l := range lines {
		if l == "------" {
			inTable = !inTable
			if !inTable {
				row = append(row, block)
				data = append(data, row)
				table := tablewriter.NewTable(os.Stdout,
					tablewriter.WithRenderer(renderer.NewBlueprint(tw.Rendition{
						Symbols:  symbols,
						Settings: tw.Settings{Separators: tw.Separators{BetweenRows: tw.On}},
					})),
					tablewriter.WithConfig(tablewriter.Config{
						Row: tw.CellConfig{
							Formatting:   tw.CellFormatting{AutoWrap: tw.WrapNone},
							Alignment:    tw.CellAlignment{Global: tw.AlignLeft},
							ColMaxWidths: tw.CellWidth{Global: 500},
						},
						Footer: tw.CellConfig{
							Alignment: tw.CellAlignment{Global: tw.AlignRight},
						},
					}),
					tablewriter.WithTrimLine(tw.Off),
					tablewriter.WithHeaderAutoFormat(tw.Off),
				)
				table.Header("Original", "Rewritten")
				table.Bulk(data)
				table.Render()
				block = blockBase
				data = [][]any{}
				row = make([]any, 0)
			}
			continue
		} else if l == "---" {
			row = append(row, block)
			data = append(data, row)
			row = make([]any, 0)
			block = blockBase
			continue
		} else if l == ">>>" {
			row = append(row, block)
			block = blockBase
			continue
		}
		if inTable {
			block += "   " + l + "\n"
		} else {
			fmt.Println(l)
		}
	}
}
