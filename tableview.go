// Package tableview provides a way to display a table widget in a
// terminal, using all the width and height and being able to scroll
// vertically or horizontally, search, filter, sort and adding additional
// funcionality
package tableview

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type Command struct {
	table   *TableView
	ch      rune
	text    string
	action  func(row int)
	enabled bool
}

func (c *Command) Disable() {
	c.enabled = false
	c.table.updateLegend()
}

func (c *Command) Enable() {
	c.enabled = true
	c.table.updateLegend()
}

// TableView holds a description of one table to be displayed
type TableView struct {
	app          *Application
	ID           int // index of this table in parent Application's "tables"
	flex         *tview.Flex
	table        *tview.Table
	columns      []string
	data         [][]string
	expansions   []int
	aligns       []int
	filter       string // active filter.  Used to regenerate orderRows
	sortBy       int    // column to sort by
	orderRows    []int  // rows to show, and in which order (generated from filter and sortBy)
	orderCols    []int  // columns to show, and in which order
	selectCols   bool   // selecting columns instead of rows
	commands     []*Command
	legend       *tview.TextView
	lastLine     tview.Primitive
	inputCapture func(k tcell.Key, r rune, row int) bool
}

type Application struct {
	app    *tview.Application
	pages  *tview.Pages
	tables []*TableView
}

var DefaultApplication = NewApplication()

func NewApplication() *Application {
	a := new(Application)
	a.app = tview.NewApplication()
	a.pages = tview.NewPages()
	a.app.SetRoot(a.pages, true)
	return a
}

func (a *Application) Run() {
	if err := a.app.Run(); err != nil {
		panic(err)
	}
}

func (a *Application) NewTable() *TableView {
	t := new(TableView)
	t.app = a
	a.tables = append(a.tables, t)
	t.ID = len(a.tables) - 1

	t.table = tview.NewTable()
	t.table.SetEvaluateAllRows(true)
	t.table.SetSeparator(tview.Borders.Vertical)
	t.table.SetFixed(1, 0)
	t.table.SetSelectable(true, false)
	t.flex = tview.NewFlex()
	t.legend = tview.NewTextView()
	t.legend.SetBackgroundColor(tcell.ColorBlue)
	t.legend.SetDynamicColors(true)

	var lastSearch string

	t.updateLegend()
	//	t.table.SetDoneFunc(func(key tcell.Key) {
	//		t.app.app.Stop()
	//	})

	t.table.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if t.inputCapture != nil && !t.selectCols {
			row, _ := t.table.GetSelection()
			row--
			if row < len(t.orderRows) {
				row = t.orderRows[row]
			}
			res := t.inputCapture(event.Key(), event.Rune(), row)
			if !res {
				return nil
			}
		}
		switch event.Key() {
		case tcell.KeyESC:
			t.app.app.Stop()
			return nil
		case tcell.KeyRune:
			if t.selectCols {
				switch event.Rune() {
				case 'q':
					t.app.app.Stop()
					return nil
				case 'c':
					t.selectCols = false
					t.updateLegend()
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
			case 'q', rune(tcell.KeyESC):
				t.app.app.Stop()
				return nil
			case 'c':
				t.selectCols = true
				columnsMenu := " [yellow]q:quit   c:back   <:left   >:right   s:sort"
				t.legend.SetText(columnsMenu)
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
					t.app.app.SetFocus(t.table)
				})
				t.flex.RemoveItem(t.lastLine)
				t.lastLine = search
				t.flex.AddItem(t.lastLine, 1, 0, false)
				t.app.app.SetFocus(search)
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
					t.filterData()
					t.updateLastLine()
					t.app.app.SetFocus(t.table)
				})
				t.flex.RemoveItem(t.lastLine)
				t.lastLine = line
				t.flex.AddItem(t.lastLine, 1, 0, false)
				t.app.app.SetFocus(line)
			}
			for _, c := range t.commands {
				if c.enabled && event.Rune() == c.ch {
					row, _ := t.table.GetSelection()
					c.action(t.orderRows[row-1])
					t.fillTable()
				}
			}
		}
		return event
	})

	t.flex.SetBackgroundColor(tcell.ColorRed)
	t.flex.SetDirection(tview.FlexRow)
	t.flex.AddItem(t.table, 0, 1, true)
	t.flex.AddItem(t.legend, 1, 0, false)
	t.lastLine = tview.NewBox()
	t.flex.AddItem(t.lastLine, 1, 0, false)

	a.pages.AddPage(strconv.Itoa(t.ID), a.tables[t.ID].flex, true, false)
	a.SetActiveTable(t)
	return t
}

func (a *Application) SetActiveTable(t *TableView) {
	a.pages.SwitchToPage(strconv.Itoa(t.ID))
	a.app.SetFocus(t.table)
}

// NewTableView returns an empty TableView
func NewTableView() *TableView {
	a := DefaultApplication
	return a.NewTable()
}

// FillTable populates a TableView with the given data
func (t *TableView) FillTable(columns []string, data [][]string) {
	t.columns = columns
	if len(t.expansions) < len(t.columns) {
		t.expansions = append(t.expansions, make([]int, len(t.columns)-len(t.expansions))...)
	}
	if len(t.aligns) < len(t.columns) {
		t.aligns = append(t.aligns, make([]int, len(t.columns)-len(t.aligns))...)
	}
	if len(t.orderCols) != len(t.columns) {
		t.orderCols = make([]int, len(t.columns))
		for i := 0; i < len(t.columns); i++ {
			t.orderCols[i] = i
		}
	}
	if len(data) != len(t.data) {
		t.orderRows = make([]int, len(data))
		for i := 0; i < len(data); i++ {
			t.orderRows[i] = i
		}
		t.filter = ""
	}
	t.data = data
	t.fillTable()
	t.table.Select(1, 0)
	t.table.SetOffset(0, 0)
}

func (t *TableView) updateLegend() {
	defaultMenu := " [yellow]q:quit   /:search   n:next   f:filter   c:columns"
	for _, c := range t.commands {
		if c.enabled && c.text != "" {
			defaultMenu = fmt.Sprintf("%s   %c:%s", defaultMenu, c.ch, c.text)
		}
	}
	t.legend.SetText(defaultMenu)
}

func (t *TableView) fillTable() {
	for i := 0; i < len(t.orderCols); i++ {
		cell := tview.NewTableCell("[yellow]" + t.columns[t.orderCols[i]]).SetBackgroundColor(tcell.ColorBlue)
		cell.SetSelectable(false)
		t.table.SetCell(0, i, cell)
		for j := 0; j < len(t.orderRows); j++ {
			content := t.data[t.orderRows[j]][t.orderCols[i]]
			cell := tview.NewTableCell(content)
			cell.SetMaxWidth(32)
			cell.SetExpansion(t.expansions[t.orderCols[i]])
			cell.SetAlign(t.aligns[t.orderCols[i]])
			t.table.SetCell(j+1, i, cell)
		}
	}
	for i := t.table.GetColumnCount() - 1; i >= len(t.orderCols); i-- {
		t.table.RemoveColumn(i)
	}
	for i := t.table.GetRowCount(); i > len(t.orderRows); i-- {
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
func (t *TableView) SetCell(row int, column int, content string) {
	if column >= len(t.columns) {
		return // TODO show return error
	}
	if row < 0 {
		return // TODO show return error
	}
	if row > len(t.data)-1 {
		t.orderRows = append(t.orderRows, make([]int, row-len(t.data)+1)...)
		for i := len(t.data); i < row+1; i++ {
			t.orderRows[i] = i
		}
		t.data = append(t.data, make([][]string, row-len(t.data)+1)...)
	}
	if column > len(t.data[row])-1 {
		t.data[row] = append(t.data[row], make([]string, column-len(t.data[row])+1)...)
	}
	t.data[row][column] = content
}

// SetExpansion sets the value by which the column expands if the
// available width for the table is more than the table width.
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

// Text alignment in each column.
const (
	AlignLeft   = tview.AlignLeft
	AlignCenter = tview.AlignCenter
	AlignRight  = tview.AlignRight
)

// SetAlign sets the alignment in this column.
// This must be either AlignLeft, AlignCenter, or AlignRight.
func (t *TableView) SetAlign(column int, align int) {
	if column < 0 || column >= len(t.columns) {
		return // TODO Check errors
	}
	if len(t.aligns) < len(t.columns) {
		t.aligns = append(t.aligns, make([]int, len(t.columns)-len(t.aligns))...)
	}

	t.aligns[column] = align
	for i := 0; i < len(t.data); i++ {
		t.table.GetCell(i, column).SetAlign(align)
	}
}

/*
func (t *TableView) NewRow() {
}

func (t *TableView) NewColumn() {
}
*/

// NewCommand sets the function to be executed when a given key is
// pressed.  The selected row is passed to the function as an argument.
func (t *TableView) NewCommand(ch rune, text string, action func(row int)) *Command {
	command := Command{}
	command.table = t
	command.ch = ch
	command.text = text
	command.action = action
	command.enabled = true
	t.commands = append(t.commands, &command)
	if !t.selectCols {
		t.updateLegend()
	}
	return &command
}

// SetSelectedFunc sets the function to be executed when the user
// presses ENTER.  The selecred row is passed to the function as an argument.
func (t *TableView) SetSelectedFunc(action func(row int)) {
	t.table.SetSelectedFunc(func(row int, col int) {
		action(row - 1)
	})
}

type Key = tcell.Key

// SetInputCapture sets a function to be executed when the user
// presses any key (and not in column mode).
func (t *TableView) SetInputCapture(f func(k Key, r rune, row int) bool) {
	t.inputCapture = f
}

// Suspend temporarily suspends the application
// by exiting terminal UI mode
// and invoking the provided function "f".
// When "f" returns, terminal UI mode is entered again
// and the application resumes.
func (t *TableView) Suspend(f func()) {
	t.app.app.Suspend(f)
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
		text := fmt.Sprintf("Filter: %q (%d/%d lines)", t.filter, len(t.orderRows), len(t.data))

		t.lastLine = tview.NewTextView().SetText(text)
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

// JoinRows marks several rows to be always together, and with the same visibility.
// This affects the behaviour of t.search(), t.filterData() and t.sort()
func (t *TableView) JoinRows(startRow int, endRow int) error {
	return fmt.Errorf("not implemented")
}

// Run draws the table and starts a loop, waiting for keystrokes
// and redrawing the screen.  It exits when ^C or "q" is pressed.
func (t *TableView) Run() {
	t.app.SetActiveTable(t)
	t.fillTable()
	t.app.Run()
}
