package tableview

import (
	"fmt"
	"strings"

	"github.com/cespedes/tview"
	"github.com/gdamore/tcell"
)

type tableViewCommand struct {
	ch     rune
	text   string
	action func(row int)
}

type TableView struct {
	columns    []string
	data       [][]string
	commands   []tableViewCommand
	app        *tview.Application
	table      *tview.Table
	expansions []int
}

func NewTableView() *TableView {
	t := new(TableView)
	t.table = tview.NewTable()
	return t
}

func (t *TableView) FillTable(columns []string, data [][]string) {
	t.columns = columns[:]
	t.data = data[:]
	if len(t.expansions) < len(columns) {
		t.expansions = append(t.expansions, make([]int, len(columns)-len(t.expansions))...)
	}
	for i := 0; i < len(columns); i++ {
		cell := tview.NewTableCell("[yellow]" + columns[i]).SetBackgroundColor(tcell.ColorBlue)
		cell.SetSelectable(false)
		t.table.SetCell(0, i, cell)
		for j := 0; j < len(data); j++ {
			content := data[j][i]
			cell := tview.NewTableCell(content)
			cell.SetMaxWidth(32)
			if t.expansions[i] > 0 {
				cell.SetExpansion(t.expansions[i])
			}
			t.table.SetCell(j+1, i, cell)
		}
	}
}

func (t *TableView) SetCell(row int, column int, content string) {
	// TODO Check column < len(t.column)
	if row > len(t.data)-1 {
		t.data = append(t.data, make([][]string, row-len(t.data)+1)...)
	}
	if column > len(t.data[row])-1 {
		t.data[row] = append(t.data[row], make([]string, column-len(t.data[row])+1)...)
	}
	t.data[row][column] = content
	cell := tview.NewTableCell(content)
	cell.SetMaxWidth(32)
	if t.expansions[column] > 0 {
		cell.SetExpansion(t.expansions[column])
	}
	t.table.SetCell(row+1, column, cell)
}

func (t *TableView) SetExpansion(column int, expansion int) {
	// TODO Check errors
	t.expansions[column] = expansion
	for i := 0; i < len(t.data); i++ {
		t.table.GetCell(i, column).SetExpansion(expansion)
	}
}

func (t *TableView) NewRow() {
}

func (t *TableView) NewColumn() {
}

func (t *TableView) NewCommand(ch rune, text string, action func(row int)) {
	t.commands = append(t.commands, tableViewCommand{ch, text, action})
}

func (t *TableView) SetSelectedFunc(action func(row int)) {
	t.table.SetSelectedFunc(func(row int, col int) {
		t.app.Suspend(func() {
			action(row)
		})
	})
}

func (t *TableView) DelRow() {
}

func (t *TableView) DelColumn() {
}

func (t *TableView) Run() {
	t.app = tview.NewApplication()
	text := tview.NewTextView()
	flex := tview.NewFlex()
	var lastLine tview.Primitive
	var lastSearch string

	tviewSearch := func(row int, text string) bool {
		text = strings.ToLower(text)
		for i := 0; i < len(t.data); i++ {
			for j := 0; j < len(t.columns); j++ {
				cellContent := strings.ToLower(t.data[(row+i)%len(t.data)][j])
				if strings.Contains(cellContent, text) {
					t.table.Select(((row+i)%len(t.data))+1, 0)
					return true
				}
			}
		}
		return false
	}

	// t.table.SetBorder(true)
	t.table.SetTitle(" LDAP ")
	t.table.SetFixedColumnsWidth(true)
	// t.table.SetBorders(true)
	t.table.SetSeparator(tview.Borders.Vertical)
	t.table.SetFixed(1, 0)
	t.table.SetSelectable(true, false)
	t.FillTable(t.columns, t.data)
	t.table.SetDoneFunc(func(key tcell.Key) {
		t.app.Stop()
	})
	t.table.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyRune:
			switch event.Rune() {
			case 'q':
				t.app.Stop()
				return nil
			case '<':
				row, col := t.table.GetOffset()
				if row > 0 {
					t.table.SetOffset(row-1, col)
				}
			case '>':
				row, col := t.table.GetOffset()
				t.table.SetOffset(row+1, col)
			case '/':
				row, _ := t.table.GetSelection()
				row--
				search := tview.NewInputField()
				search.SetLabel("Search: ")
				search.SetFieldBackgroundColor((tcell.ColorBlack))
				search.SetChangedFunc(func(text string) {
					if tviewSearch(row, text) {
						search.SetFieldTextColor(tcell.ColorWhite)
					} else {
						search.SetFieldTextColor((tcell.ColorRed))
					}
				})
				search.SetDoneFunc(func(key tcell.Key) {
					lastSearch = search.GetText()
					flex.RemoveItem(lastLine)
					lastLine = tview.NewTextView().SetText(fmt.Sprintf("Last search: %q from line %d", lastSearch, row))
					flex.AddItem(lastLine, 1, 0, false)
					t.app.SetFocus(t.table)
				})
				flex.RemoveItem(lastLine)
				lastLine = search
				flex.AddItem(lastLine, 1, 0, false)
				t.app.SetFocus(search)

			case 'n':
				row, _ := t.table.GetSelection()
				tviewSearch(row, lastSearch)
				flex.RemoveItem(lastLine)
				lastLine = tview.NewTextView().SetText(fmt.Sprintf("Searching again: %q from line %d", lastSearch, row))
				flex.AddItem(lastLine, 1, 0, false)
			}
			for _, c := range t.commands {
				if event.Rune() == c.ch {
					row, _ := t.table.GetSelection()
					t.app.Suspend(func() {
						c.action(row)
					})
				}
			}
		}
		return event
	})
	text.SetBackgroundColor(tcell.ColorBlue)
	text.SetDynamicColors(true)
	innerText := " [yellow]q:quit   /:search   n:next"
	for _, c := range t.commands {
		innerText = fmt.Sprintf("%s   %c:%s", innerText, c.ch, c.text)
	}
	text.SetText(innerText)
	flex.SetBackgroundColor(tcell.ColorRed)
	flex.SetDirection(tview.FlexRow)
	flex.AddItem(t.table, 0, 1, true)
	flex.AddItem(text, 1, 0, false)
	lastLine = tview.NewBox()
	flex.AddItem(lastLine, 1, 0, false)
	t.app.SetRoot(flex, true)
	if err := t.app.Run(); err != nil {
		panic(err)
	}
}
