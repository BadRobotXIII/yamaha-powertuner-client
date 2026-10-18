package main

import (
	"context"
	"fmt"
	"io"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	pt "yamaha-pt-client"
)

func main() {
	a := app.NewWithID("local.powertuner.fyne")
	w := a.NewWindow("CCU Read Only")
	w.Resize(fyne.NewSize(480, 800))
	w.SetContent(makeUI(w))
	w.ShowAndRun()
}

func makeUI(w fyne.Window) fyne.CanvasObject {
	address := widget.NewEntry()
	address.SetText(pt.DefaultAddress)
	command := widget.NewSelect([]string{"info", "maps", "monitor", "model", "lock", "laps", "eps", "log"}, nil)
	command.SetSelected("info")
	page := widget.NewSelect([]string{"0", "1", "2", "3"}, nil)
	page.SetSelected("0")
	eps := widget.NewCheck("Include EPS with maps", nil)
	eps.Disable()
	page.Disable()
	command.OnChanged = func(name string) {
		if name == "maps" {
			eps.Enable()
		} else {
			eps.SetChecked(false)
			eps.Disable()
		}
		if name == "log" {
			page.Enable()
		} else {
			page.Disable()
		}
	}
	mode := widget.NewSelect([]string{"json", "text", "binary", "hex", "all"}, nil)
	mode.SetSelected("json")
	response := widget.NewSelect([]string{}, nil)
	status := widget.NewLabel("Join CCU Wi-Fi in OS settings. No requests until Read.")
	status.Wrapping = fyne.TextWrapWord
	output := widget.NewMultiLineEntry()
	output.TextStyle = fyne.TextStyle{Monospace: true}
	output.Wrapping = fyne.TextWrapOff
	difference := widget.NewMultiLineEntry()
	difference.TextStyle = fyne.TextStyle{Monospace: true}
	difference.Wrapping = fyne.TextWrapOff
	difference.SetText("Read twice from the same address and endpoint to compare successive snapshots.")
	var current, previous *pt.Snapshot
	var history pt.History
	render := func() {
		if current == nil {
			return
		}
		text, err := pt.FormatSnapshot(current, mode.Selected, response.Selected, 16384)
		if err != nil {
			status.SetText(err.Error())
			return
		}
		output.SetText(text)
	}
	mode.OnChanged = func(string) { render() }
	response.OnChanged = func(string) { render() }
	ctx, cancel := context.WithCancel(context.Background())
	w.SetOnClosed(cancel)
	var read *widget.Button
	read = widget.NewButtonWithIcon("Read snapshot", theme.DownloadIcon(), func() {
		request := pt.Request{Command: command.Selected, IncludeEPS: eps.Checked, Page: 0}
		if command.Selected == "log" {
			request.Page = int(page.Selected[0] - '0')
		}
		if err := request.Validate(); err != nil {
			dialog.ShowError(err, w)
			return
		}
		client, err := pt.New(address.Text, 5*time.Second)
		if err != nil {
			dialog.ShowError(err, w)
			return
		}
		read.Disable()
		status.SetText("Reading... GET only.")
		go func() {
			defer client.Close()
			snapshot, err := client.Capture(ctx, request)
			if ctx.Err() != nil {
				return
			}
			fyne.Do(func() {
				read.Enable()
				if err != nil {
					status.SetText("Read failed; previous successful capture retained: " + err.Error())
					return
				}
				previous, current = history.Add(snapshot)
				names := []string{}
				for _, part := range current.Parts {
					names = append(names, part.Name)
				}
				response.Options = names
				response.Refresh()
				if len(names) > 1 {
					response.SetSelected("ecu")
				} else {
					response.SetSelected(names[0])
				}
				render()
				if previous == nil {
					difference.SetText("First snapshot for this address/endpoint/options. Read again to compare.")
				} else {
					report, e := pt.Compare(previous, current)
					if e != nil {
						difference.SetText(e.Error())
					} else {
						difference.SetText(report)
					}
				}
				status.SetText(fmt.Sprintf("Captured %s. Compare uses previous successful capture of this source.", current.CapturedAt.Local().Format("15:04:05")))
			})
		}()
	})
	copy := widget.NewButtonWithIcon("Copy view", theme.ContentCopyIcon(), func() { w.Clipboard().SetContent(output.Text) })
	export := func(snapshotFile bool) {
		if current == nil {
			dialog.ShowInformation("No snapshot", "Read a snapshot first.", w)
			return
		}
		s := current
		var body []byte
		if !snapshotFile {
			for _, p := range s.Parts {
				if p.Name == response.Selected {
					body = append([]byte(nil), p.Body...)
					break
				}
			}
		}
		d := dialog.NewFileSave(func(writer fyne.URIWriteCloser, err error) {
			if err != nil {
				dialog.ShowError(err, w)
				return
			}
			if writer == nil {
				return
			}
			var e error
			if snapshotFile {
				e = pt.WriteSnapshot(writer, s)
			} else {
				_, e = writer.Write(body)
			}
			ce := writer.Close()
			if e == nil {
				e = ce
			}
			if e != nil {
				dialog.ShowError(e, w)
			}
		}, w)
		if snapshotFile {
			d.SetFileName("ccu-snapshot.json")
		} else {
			d.SetFileName("ccu-response.bin")
		}
		d.Show()
	}
	save := widget.NewButton("Save snapshot", func() { export(true) })
	raw := widget.NewButton("Save raw bytes", func() { export(false) })
	compareFile := widget.NewButton("Compare saved snapshot", func() {
		if current == nil {
			dialog.ShowInformation("No snapshot", "Read a current snapshot first.", w)
			return
		}
		s := current
		dialog.ShowFileOpen(func(reader fyne.URIReadCloser, err error) {
			if err != nil {
				dialog.ShowError(err, w)
				return
			}
			if reader == nil {
				return
			}
			defer reader.Close()
			baseline, e := pt.ReadSnapshot(io.Reader(reader))
			if e != nil {
				dialog.ShowError(e, w)
				return
			}
			report, e := pt.Compare(baseline, s)
			if e != nil {
				dialog.ShowError(e, w)
				return
			}
			difference.SetText(report)
		}, w)
	})
	reset := widget.NewButton("Reset comparisons", func() {
		history = pt.History{}
		previous = nil
		difference.SetText("Comparison history cleared. Next successful read starts a new baseline.")
	})
	controls := container.NewVBox(widget.NewLabelWithStyle("CCU Read Only", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}), address,
		container.NewGridWithColumns(2, command, page), eps, container.NewGridWithColumns(2, response, mode), read,
		container.NewGridWithColumns(3, copy, raw, save), container.NewGridWithColumns(2, compareFile, reset))
	// Scroll controls separately so small mobile windows retain a usable result pane.
	top := container.NewVScroll(controls)
	top.SetMinSize(fyne.NewSize(300, 330))
	tabs := container.NewAppTabs(container.NewTabItem("Response", output), container.NewTabItem("Compare", difference))
	return container.NewBorder(top, status, nil, nil, tabs)
}
