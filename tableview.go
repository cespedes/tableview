package tableview

import (
	"fmt"
	"sort"
	"strings"

	"github.com/cespedes/tview"
	"github.com/gdamore/tcell"
)

type tableViewCommand struct {
	ch     rune
	text   string
	action func(row int)
}

// TableView holds a description of the table to be displayed
type TableView struct {
	app        *tview.Application
	flex       *tview.Flex
	table      *tview.Table
	columns    []string
	data       [][]string
	expansions []int
	filter     string // active filter.  Used to regenerate orderRows
	sortBy     int    // column to sort by
	orderRows  []int  // rows to show, and in which order (generated from filter and sortBy)
	orderCols  []int  // columns to show, and in which order
	selectCols bool   // selecting columns instead of rows
	commands   []tableViewCommand
	lastLine   tview.Primitive
}

// NewTableView returns an empty TableView
func NewTableView() *TableView {
	t := new(TableView)
	t.table = tview.NewTable()
	return t
}

// FillTable populates a TableView with the given data
func (t *TableView) FillTable(columns []string, data [][]string) {
	t.columns = columns
	if len(t.expansions) < len(t.columns) {
		t.expansions = append(t.expansions, make([]int, len(t.columns)-len(t.expansions))...)
	}
	t.data = data
}

func (t *TableView) fillTable() {
	if len(t.orderCols) != len(t.columns) {
		t.orderCols = make([]int, len(t.columns))
		for i := 0; i < len(t.columns); i++ {
			t.orderCols[i] = i
		}
		t.filter = ""
	}
	if len(t.orderRows) != len(t.data) {
		t.orderRows = make([]int, len(t.data))
		for i := 0; i < len(t.data); i++ {
			t.orderRows[i] = i
		}
	}

	for i := 0; i < len(t.orderCols); i++ {
		cell := tview.NewTableCell("[yellow]" + t.columns[t.orderCols[i]]).SetBackgroundColor(tcell.ColorBlue)
		cell.SetSelectable(false)
		t.table.SetCell(0, i, cell)
		for j := 0; j < len(t.orderRows); j++ {
			content := t.data[t.orderRows[j]][t.orderCols[i]]
			cell := tview.NewTableCell(content)
			cell.SetMaxWidth(32)
			cell.SetExpansion(t.expansions[t.orderCols[i]])
			t.table.SetCell(j+1, i, cell)
		}
	}
	for i := t.table.GetColumnCount() - 1; i > len(t.orderCols); i-- {
		t.table.RemoveColumn(i)
	}
	for i := t.table.GetRowCount() - 1; i > len(t.orderRows); i-- {
		t.table.RemoveRow(i)
	}
}

func (t *TableView) filterData() {
	t.orderRows = nil
	text := strings.ToLower(t.filter)
	for i := 0; i < len(t.data); i++ {
		for j := 0; j < len(t.columns); j++ {
			cellContent := strings.ToLower(t.data[i][j])
			if strings.Contains(cellContent, text) {
				t.orderRows = append(t.orderRows, i)
				break
			}
		}
	}
	t.fillTable()
}

// SetCell sets the content of a cell in the specified position.
//
// row must be >=0; column must be
func (t *TableView) SetCell(row int, column int, content string) {
	if column >= len(t.columns) {
		return // TODO show return error
	}
	if row < 0 {
		return // TODO show return error
	}
	if row > len(t.data)-1 {
		t.data = append(t.data, make([][]string, row-len(t.data)+1)...)
	}
	if column > len(t.data[row])-1 {
		t.data[row] = append(t.data[row], make([]string, column-len(t.data[row])+1)...)
	}
	t.data[row][column] = content
}

func (t *TableView) SetExpansion(column int, expansion int) {
	if column < 0 || column >= len(t.columns) {
		return // TODO Check errors
	}
	if len(t.expansions) < len(t.columns) {
		t.expansions = append(t.expansions, make([]int, len(t.columns)-len(t.expansions))...)
	}

	t.expansions[column] = expansion
	for i := 0; i < len(t.data); i++ {
		t.table.GetCell(i, column).SetExpansion(expansion)
	}
}

/*
func (t *TableView) NewRow() {
}

func (t *TableView) NewColumn() {
}
*/

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

/*
func (t *TableView) DelRow() {
}

func (t *TableView) DelColumn() {
}
*/

func (t *TableView) updateLastLine() {
	t.flex.RemoveItem(t.lastLine)
	if t.filter != "" {
		t.lastLine = tview.NewTextView().SetText(fmt.Sprintf("Filter: %q", t.filter))
	} else {
		t.lastLine = tview.NewBox()
	}
	t.flex.AddItem(t.lastLine, 1, 0, false)
}

func (t *TableView) search(startRow int, text string) bool {
	text = strings.ToLower(text)
	for i := 0; i < len(t.orderRows); i++ {
		for j := 0; j < len(t.columns); j++ {
			cellContent := strings.ToLower(t.data[t.orderRows[(startRow+i)%len(t.orderRows)]][j])
			if strings.Contains(cellContent, text) {
				t.table.Select(((startRow+i)%len(t.orderRows))+1, 0)
				return true
			}
		}
	}
	return false
}

func (t *TableView) Run() {
	t.app = tview.NewApplication()
	text := tview.NewTextView()
	t.flex = tview.NewFlex()
	var lastSearch string

	text.SetBackgroundColor(tcell.ColorBlue)
	text.SetDynamicColors(true)
	defaultMenu := " [yellow]q:quit   /:search   n:next   f:filter   c:columns"
	for _, c := range t.commands {
		defaultMenu = fmt.Sprintf("%s   %c:%s", defaultMenu, c.ch, c.text)
	}
	columnsMenu := " [yellow]q:quit   c:back   <:left   >:right   s:sort"
	text.SetText(defaultMenu)
	t.table.SetFixedColumnsWidth(true)
	t.table.SetSeparator(tview.Borders.Vertical)
	t.table.SetFixed(1, 0)
	t.table.SetSelectable(true, false)
	t.fillTable()
	t.table.SetDoneFunc(func(key tcell.Key) {
		t.app.Stop()
	})
	t.table.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyRune:
			if t.selectCols {
				switch event.Rune() {
				case 'q':
					t.app.Stop()
					return nil
				case 'c':
					t.selectCols = false
					text.SetText(defaultMenu)
					t.table.SetSelectable(true, false)
				case '<':
					row, col := t.table.GetSelection()
					if col == 0 {
						break
					}
					t.orderCols[col-1], t.orderCols[col] = t.orderCols[col], t.orderCols[col-1]
					t.table.Select(row, col-1)
					t.fillTable()
				case '>':
					row, col := t.table.GetSelection()
					if col == len(t.orderCols)-1 {
						break
					}
					t.orderCols[col], t.orderCols[col+1] = t.orderCols[col+1], t.orderCols[col]
					t.table.Select(row, col+1)
					t.fillTable()
				case 's':
					_, col := t.table.GetSelection()
					sort.Slice(t.orderRows, func(a, b int) bool {
						return t.data[t.orderRows[a]][t.orderCols[col]] < t.data[t.orderRows[b]][t.orderCols[col]]
					})
					t.fillTable()
				}
				return event
			}
			switch event.Rune() {
			case 'q':
				t.app.Stop()
				return nil
			case 'c':
				t.selectCols = true
				text.SetText(columnsMenu)
				t.table.SetSelectable(false, true)
			case '=':
				size := len(t.orderRows)
				_, _, wid, hei := t.table.GetInnerRect()
				sel, _ := t.table.GetSelection()
				off, _ := t.table.GetOffset()
				t.flex.RemoveItem(t.lastLine)
				t.lastLine = tview.NewTextView().SetText(fmt.Sprintf("size=%d wid=%d hei=%d sel=%d off=%d", size, wid, hei, sel, off))
				t.flex.AddItem(t.lastLine, 1, 0, false)
			case '<':
				row, col := t.table.GetSelection()
				offh, offv := t.table.GetOffset()
				_, _, _, hei := t.table.GetInnerRect()
				if offh > 0 {
					if row-offh+1 == hei {
						t.table.Select(row-1, col)
					}
					t.table.SetOffset(offh-1, offv)
				}
			case '>':
				row, col := t.table.GetSelection()
				offh, offv := t.table.GetOffset()
				_, _, _, hei := t.table.GetInnerRect()
				if (row == offh+1) && (offh+hei <= len(t.data)) {
					t.table.Select(row+1, col)
				}
				t.table.SetOffset(offh+1, offv)
			case '/':
				row, _ := t.table.GetSelection()
				row--
				search := tview.NewInputField()
				search.SetLabel("Search: ")
				search.SetFieldBackgroundColor((tcell.ColorBlack))
				search.SetChangedFunc(func(text string) {
					if t.search(row, text) {
						search.SetFieldTextColor(tcell.ColorWhite)
					} else {
						search.SetFieldTextColor((tcell.ColorRed))
					}
				})
				search.SetDoneFunc(func(key tcell.Key) {
					lastSearch = search.GetText()
					t.updateLastLine()
					t.app.SetFocus(t.table)
				})
				t.flex.RemoveItem(t.lastLine)
				t.lastLine = search
				t.flex.AddItem(t.lastLine, 1, 0, false)
				t.app.SetFocus(search)
			case 'n':
				row, _ := t.table.GetSelection()
				t.search(row, lastSearch)
			case 'f':
				row, _ := t.table.GetSelection()
				row--
				line := tview.NewInputField()
				line.SetLabel("Filter: ")
				t.filter = ""
				t.filterData()
				line.SetFieldBackgroundColor((tcell.ColorBlack))
				line.SetChangedFunc(func(text string) {
					t.filter = text
					t.filterData()
				})
				line.SetDoneFunc(func(key tcell.Key) {
					t.filter = line.GetText()
					t.updateLastLine()
					t.app.SetFocus(t.table)
				})
				t.flex.RemoveItem(t.lastLine)
				t.lastLine = line
				t.flex.AddItem(t.lastLine, 1, 0, false)
				t.app.SetFocus(line)
			}
			for _, c := range t.commands {
				if event.Rune() == c.ch {
					row, _ := t.table.GetSelection()
					t.app.Suspend(func() {
						c.action(t.orderRows[row-1])
						t.fillTable()
					})
				}
			}
		}
		return event
	})
	t.flex.SetBackgroundColor(tcell.ColorRed)
	t.flex.SetDirection(tview.FlexRow)
	t.flex.AddItem(t.table, 0, 1, true)
	t.flex.AddItem(text, 1, 0, false)
	t.lastLine = tview.NewBox()
	t.flex.AddItem(t.lastLine, 1, 0, false)
	t.app.SetRoot(t.flex, true)
	if err := t.app.Run(); err != nil {
		panic(err)
	}
}
