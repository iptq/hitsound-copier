package main

import (
	"github.com/andlabs/ui"
	_ "github.com/andlabs/ui/winmanifest"
)

var (
	win  *ui.Window
	form *ui.Form

	copyFrom, copyTo       *ui.Entry
	copyFromBtn, copyToBtn *ui.Button

	saveBackup *ui.Checkbox
)

func connectBtn(btn *ui.Button, entry *ui.Entry) {
	btn.OnClicked(func(b *ui.Button) {
		p := ui.OpenFile(win)
		entry.SetText(p)
	})
}

func setupUI() {
	win = ui.NewWindow("hitsound copier", 480, 240, false)
	win.SetMargined(true)
	win.OnClosing(func(*ui.Window) bool {
		ui.Quit()
		return true
	})
	ui.OnShouldQuit(func() bool {
		win.Destroy()
		return true
	})

	form = ui.NewForm()
	form.SetPadded(true)
	win.SetChild(form)

	copyFromBox := ui.NewHorizontalBox()
	copyFrom = ui.NewEntry()
	copyFrom.Disable()
	copyFromBox.Append(copyFrom, false)
	copyFromBtn = ui.NewButton("browse")
	copyFromBox.Append(copyFromBtn, false)
	connectBtn(copyFromBtn, copyFrom)
	form.Append("copy hitsounds from", copyFromBox, false)

	copyToBox := ui.NewHorizontalBox()
	copyTo = ui.NewEntry()
	copyTo.Disable()
	copyToBox.Append(copyTo, false)
	copyToBtn = ui.NewButton("browse")
	copyToBox.Append(copyToBtn, false)
	connectBtn(copyToBtn, copyTo)
	form.Append("copy hitsounds to", copyToBox, false)

	copyVolumes := ui.NewCheckbox("")
	copyVolumes.SetChecked(true)
	form.Append("copy volume sections", copyVolumes, false)

	saveBackup = ui.NewCheckbox("")
	saveBackup.SetChecked(true)
	form.Append("save backups", saveBackup, false)

	copyBtn := ui.NewButton("copy")
	copyBtn.OnClicked(func(*ui.Button) {
		fromFile := copyFrom.Text()
		toFile := copyTo.Text()
		backup := saveBackup.Checked()
		err := copyHitsounds(fromFile, toFile, backup)
		if err == nil {
			ui.MsgBox(win, "done", "copied")
		} else {
			ui.MsgBoxError(win, "error", err.Error())
		}
	})
	form.Append("", copyBtn, false)

	win.Show()
}

func main() {
	// ui.Main(setupUI)
	copyHitsounds("input", "output", true)
}
