// Package gui is the native Host app window with a tray icon.
package gui

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/rodrigocfd/windigo/co"
	"github.com/rodrigocfd/windigo/ui"
	"github.com/rodrigocfd/windigo/win"
	"github.com/rodrigocfd/windigo/x/cosh"
	"github.com/rodrigocfd/windigo/x/winsh"
	"rsc.io/qr"

	"github.com/Wuxriff/hwinfo-lan-monitor/internal/app"
	"github.com/Wuxriff/hwinfo-lan-monitor/internal/config"
	"github.com/Wuxriff/hwinfo-lan-monitor/internal/hwinfo"
	"github.com/Wuxriff/hwinfo-lan-monitor/internal/source"
)

const (
	className  = "HWiNFOLanMonitor"
	wmTray     = co.WM_APP + 1
	wmShowMe   = co.WM_APP + 2
	timerWatch = 1
	cmdOpen    = 1001
	cmdDash    = 1002
	cmdExit    = 1003
	maxLog     = 300
	appIconID  = 1
)

type palette struct {
	bg, edit, text win.COLORREF
}

var (
	lightPal = palette{win.RGB(243, 243, 243), win.RGB(255, 255, 255), win.RGB(0, 0, 0)}
	darkPal  = palette{win.RGB(32, 32, 32), win.RGB(45, 45, 45), win.RGB(235, 235, 235)}
)

type window struct {
	app     *app.App
	opts    Options
	cfg     config.Config
	cfgPath string
	lang    string

	wnd     *ui.Main
	status  *ui.Static
	path    *ui.Edit
	browse  *ui.Button
	launch  *ui.Button
	port    *ui.Edit
	access  *ui.ComboBox
	apply   *ui.Button
	tray    *ui.CheckBox
	toTray  *ui.CheckBox
	langSel *ui.ComboBox
	urls    *ui.ComboBox
	open    *ui.Button
	scan    *ui.Static
	logBox  *ui.Edit
	labels  map[*ui.Static]string

	nid       winsh.NOTIFYICONDATA
	dark      bool
	themeSet  bool
	bgBrush   win.HBRUSH
	editBrush win.HBRUSH
	pal       palette
	qrCode    *qr.Code
	qrRect    win.RECT
	logLines  []string
	lastSt    source.State
	exiting   bool
}

// Options changes behaviour for demo screenshots: no HWiNFO detection, no
// config writes, and a fixed color mode.
type Options struct {
	Demo bool
	Dark bool
}

// Run shows the Host app and blocks until the user exits from the tray menu.
func Run(a *app.App, cfg config.Config, cfgPath string, opts Options) int {
	runtime.LockOSThread()
	if _, err := win.CoInitializeEx(co.COINIT_APARTMENTTHREADED | co.COINIT_DISABLE_OLE1DDE); err == nil {
		defer win.CoUninitialize()
	}
	if !singleInstance() {
		// UIPI blocks posting to an elevated instance, so tell the user instead of exiting silently.
		if h := findWindow(className); h == 0 || h.PostMessage(wmShowMe, 0, 0) != nil {
			lang := cfg.Language
			if lang == "" {
				lang = windowsLanguage()
			}
			win.HWND(0).MessageBox(texts[lang]["alreadyRunning"], texts[lang]["title"], co.MB_ICONINFORMATION)
		}
		return 0
	}
	if opts.Demo {
		cfg.HWiNFOPath, cfg.Window, cfg.StartInTray = `C:\Program Files\HWiNFO64\HWiNFO64.exe`, nil, false
	}
	w := &window{app: a, cfg: cfg, cfgPath: cfgPath, opts: opts, labels: map[*ui.Static]string{}}
	w.lang = cfg.Language
	if w.lang == "" {
		w.lang = windowsLanguage()
	}
	w.build()
	return w.wnd.RunAsMain()
}

func (w *window) t(key string) string { return texts[w.lang][key] }

func (w *window) build() {
	show := co.SW_SHOW
	if w.cfg.StartInTray {
		show = co.SW_HIDE
	}
	w.wnd = ui.NewMain(ui.OptsMain().
		ClassName(className).
		ClassIconId(appIconID).
		Title(w.t("title")).
		Size(ui.DpiX(560), ui.DpiY(646)).
		Center(true).
		CmdShow(show))

	x0, x1, x2 := 12, 104, 412
	label := func(key string, x, y, cx int) *ui.Static {
		s := ui.NewStatic(w.wnd, ui.OptsStatic().Text(w.t(key)).Position(ui.DpiX(x), ui.DpiY(y+3)).Size(ui.DpiX(cx), ui.DpiY(20)))
		w.labels[s] = key
		return s
	}

	w.status = ui.NewStatic(w.wnd, ui.OptsStatic().Position(ui.DpiX(x0), ui.DpiY(12)).Size(ui.DpiX(536), ui.DpiY(20)))

	label("hwinfo", x0, 44, 88)
	w.path = ui.NewEdit(w.wnd, ui.OptsEdit().Text(w.cfg.HWiNFOPath).Position(ui.DpiX(x1), ui.DpiY(44)).Width(ui.DpiX(210)))
	w.browse = ui.NewButton(w.wnd, ui.OptsButton().Text(w.t("browse")).Position(ui.DpiX(x1+218), ui.DpiY(43)).Width(ui.DpiX(82)))
	w.launch = ui.NewButton(w.wnd, ui.OptsButton().Text(w.t("launch")).Position(ui.DpiX(x2), ui.DpiY(43)).Width(ui.DpiX(136)))

	label("port", x0, 78, 88)
	w.port = ui.NewEdit(w.wnd, ui.OptsEdit().Text(strconv.Itoa(w.cfg.Port)).CtrlStyle(co.ES_NUMBER).Position(ui.DpiX(x1), ui.DpiY(78)).Width(ui.DpiX(70)))
	sel := 0
	if !w.cfg.LANAccess {
		sel = 1
	}
	w.access = ui.NewComboBox(w.wnd, ui.OptsComboBox().Texts(w.t("accessLAN"), w.t("accessLocal")).Select(sel).Position(ui.DpiX(x1+80), ui.DpiY(78)).Width(ui.DpiX(220)))
	w.apply = ui.NewButton(w.wnd, ui.OptsButton().Text(w.t("apply")).Position(ui.DpiX(x2), ui.DpiY(77)).Width(ui.DpiX(136)))

	trayState := co.BST_UNCHECKED
	if w.cfg.StartInTray {
		trayState = co.BST_CHECKED
	}
	w.tray = ui.NewCheckBox(w.wnd, ui.OptsCheckBox().Text(w.t("startInTray")).State(trayState).Position(ui.DpiX(x0), ui.DpiY(114)).Size(ui.DpiX(280), ui.DpiY(20)))
	toTrayState := co.BST_UNCHECKED
	if w.cfg.CloseToTray {
		toTrayState = co.BST_CHECKED
	}
	w.toTray = ui.NewCheckBox(w.wnd, ui.OptsCheckBox().Text(w.t("closeToTray")).State(toTrayState).Position(ui.DpiX(x0), ui.DpiY(140)).Size(ui.DpiX(280), ui.DpiY(20)))
	label("language", 300, 112, 100)
	langIdx := 0
	if w.lang == "ru" {
		langIdx = 1
	}
	w.langSel = ui.NewComboBox(w.wnd, ui.OptsComboBox().Texts("English", "Русский").Select(langIdx).Position(ui.DpiX(x2), ui.DpiY(112)).Width(ui.DpiX(136)))

	label("dashboard", x0, 176, 88)
	w.urls = ui.NewComboBox(w.wnd, ui.OptsComboBox().Position(ui.DpiX(x1), ui.DpiY(176)).Width(ui.DpiX(300)))
	w.open = ui.NewButton(w.wnd, ui.OptsButton().Text(w.t("open")).Position(ui.DpiX(x2), ui.DpiY(175)).Width(ui.DpiX(136)))

	w.qrRect = win.RECT{Left: int32(ui.DpiX(x0)), Top: int32(ui.DpiY(212)), Right: int32(ui.DpiX(x0 + 200)), Bottom: int32(ui.DpiY(412))}
	w.scan = ui.NewStatic(w.wnd, ui.OptsStatic().Text(w.t("scan")).Position(ui.DpiX(x0+216), ui.DpiY(216)).Size(ui.DpiX(320), ui.DpiY(60)))

	label("log", x0, 422, 200)
	w.logBox = ui.NewEdit(w.wnd, ui.OptsEdit().
		CtrlStyle(co.ES_MULTILINE|co.ES_READONLY|co.ES_AUTOVSCROLL).
		WndStyle(co.WS_CHILD|co.WS_VISIBLE|co.WS_VSCROLL|co.WS_TABSTOP).
		Position(ui.DpiX(x0), ui.DpiY(446)).Width(ui.DpiX(536)).Height(ui.DpiY(188)))

	w.events()
}

func (w *window) events() {
	w.wnd.On().WmCreate(func(_ ui.WmCreate) int {
		w.addTrayIcon()
		w.applyTheme(true)
		w.listen()
		w.detectHWiNFO()
		w.restorePosition()
		w.wnd.Hwnd().SetTimer(timerWatch, 2000)
		go w.watchSource()
		return 0
	})
	w.wnd.On().WmTimer(timerWatch, func() {
		w.detectHWiNFO()
		w.applyTheme(false)
	})
	w.wnd.On().WmExitSizeMove(w.savePosition)
	w.wnd.On().WmSize(func(p ui.WmSize) {
		if p.Request() == co.SIZE_REQ_MINIMIZED {
			w.wnd.Hwnd().ShowWindow(co.SW_HIDE)
		}
	})
	w.wnd.On().WmClose(func() {
		if w.exiting || !w.cfg.CloseToTray {
			w.wnd.Hwnd().DestroyWindow()
			return
		}
		w.wnd.Hwnd().ShowWindow(co.SW_HIDE)
	})
	w.wnd.On().WmDestroy(func() {
		winsh.Shell_NotifyIcon(cosh.NIM_DELETE, &w.nid)
		w.app.Close()
	})
	w.wnd.On().Wm(wmShowMe, func(_ ui.Wm) uintptr { w.restore(); return 0 })
	w.wnd.On().Wm(wmTray, func(p ui.Wm) uintptr {
		switch co.WM(p.LParam & 0xFFFF) {
		case co.WM_LBUTTONUP:
			w.restore()
		case co.WM_RBUTTONUP:
			w.trayMenu()
		}
		return 0
	})
	w.wnd.On().WmCommand(cmdOpen, co.CMD_MENU, w.restore)
	w.wnd.On().WmCommand(cmdDash, co.CMD_MENU, w.openDashboard)
	w.wnd.On().WmCommand(cmdExit, co.CMD_MENU, func() {
		w.exiting = true
		w.wnd.Hwnd().PostMessage(co.WM_CLOSE, 0, 0)
	})

	w.wnd.On().WmEraseBkgnd(func(p ui.WmEraseBkgnd) int {
		rc, _ := w.wnd.Hwnd().GetClientRect()
		p.Hdc().FillRect(&rc, w.bgBrush)
		return 1
	})
	w.wnd.On().WmPaint(w.paint)
	colorStatic := func(p ui.WmCtlColor) win.HBRUSH {
		p.Hdc().SetTextColor(w.pal.text)
		p.Hdc().SetBkColor(w.pal.bg)
		if p.HwndControl() == w.logBox.Hwnd() {
			p.Hdc().SetBkColor(w.pal.edit)
			return w.editBrush
		}
		return w.bgBrush
	}
	w.wnd.On().WmCtlColorStatic(colorStatic)
	w.wnd.On().WmCtlColorBtn(colorStatic)
	colorEdit := func(p ui.WmCtlColor) win.HBRUSH {
		p.Hdc().SetTextColor(w.pal.text)
		p.Hdc().SetBkColor(w.pal.edit)
		return w.editBrush
	}
	w.wnd.On().WmCtlColorEdit(colorEdit)
	w.wnd.On().WmCtlColorListBox(colorEdit)

	w.browse.On().BnClicked(w.browseHWiNFO)
	w.launch.On().BnClicked(w.launchHWiNFO)
	w.apply.On().BnClicked(w.applyServer)
	w.open.On().BnClicked(w.openDashboard)
	w.tray.On().BnClicked(func() {
		w.cfg.StartInTray = w.tray.IsChecked()
		w.save()
	})
	w.toTray.On().BnClicked(func() {
		w.cfg.CloseToTray = w.toTray.IsChecked()
		w.save()
	})
	w.langSel.On().CbnSelChange(func() {
		w.lang = []string{"en", "ru"}[w.langSel.SelectedIndex()]
		w.cfg.Language = w.lang
		w.save()
		w.relabel()
	})
	w.urls.On().CbnSelChange(w.updateQR)
}

// --- tray ---

func (w *window) addTrayIcon() {
	w.nid.SetCbSize()
	w.nid.HWnd = w.wnd.Hwnd()
	w.nid.UID = 1
	w.nid.UFlags = cosh.NIF_MESSAGE | cosh.NIF_ICON | cosh.NIF_TIP
	w.nid.UCallbackMessage = wmTray
	w.nid.HIcon = w.trayIcon()
	w.nid.SetSzTip(w.t("title"))
	winsh.Shell_NotifyIcon(cosh.NIM_ADD, &w.nid)
}

func (w *window) trayIcon() win.HICON {
	if h := smallIcon(appIconID); h != 0 {
		return h
	}
	h, _ := win.HINSTANCE(0).LoadIcon(win.IconResIdi(co.IDI_APPLICATION))
	return h
}

func (w *window) trayMenu() {
	menu, _ := win.CreatePopupMenu()
	appendMenu(menu, cmdOpen, w.t("trayOpen"))
	appendMenu(menu, cmdDash, w.t("trayDash"))
	appendMenu(menu, 0, "")
	appendMenu(menu, cmdExit, w.t("trayExit"))
	pt, _ := win.GetCursorPos()
	w.wnd.Hwnd().SetForegroundWindow()
	menu.TrackPopupMenu(co.TPM_RIGHTBUTTON, int(pt.X), int(pt.Y), w.wnd.Hwnd())
	menu.DestroyMenu()
}

func (w *window) restore() {
	w.wnd.Hwnd().ShowWindow(co.SW_SHOW)
	w.wnd.Hwnd().ShowWindow(co.SW_RESTORE)
	w.wnd.Hwnd().SetForegroundWindow()
}

// --- actions ---

func (w *window) save() {
	if w.opts.Demo {
		return
	}
	if err := config.Save(w.cfgPath, w.cfg); err != nil {
		w.log(w.t("logSaveErr"), err)
	}
}

func (w *window) listen() {
	addr := w.cfg.Addr()
	if err := w.app.Listen(addr); err != nil {
		if errors.Is(err, syscall.Errno(10048)) { // WSAEADDRINUSE
			w.log(w.t("logPortBusy"), w.cfg.Port)
		} else {
			w.log(w.t("logListenErr"), addr, err)
		}
	}
	urls := w.app.URLs()
	w.urls.DeleteAllItems()
	w.urls.AddItem(urls...)
	if len(urls) > 0 {
		w.urls.SelectIndex(0)
		w.log(w.t("logListen"), strings.Join(urls, ", "))
	}
	w.updateQR()
}

func (w *window) applyServer() {
	port, err := strconv.Atoi(strings.TrimSpace(w.port.Text()))
	if err != nil || port < 1 || port > 65535 {
		w.log("%s", w.t("logBadPort"))
		return
	}
	w.cfg.Port = port
	w.cfg.LANAccess = w.access.SelectedIndex() == 0
	w.cfg.HWiNFOPath = strings.TrimSpace(w.path.Text())
	w.save()
	w.listen()
}

func (w *window) openDashboard() {
	if u := w.urls.CurrentText(); u != "" {
		hwinfo.ShellOpen(u, "")
	}
}

func (w *window) browseHWiNFO() {
	p, ok := pickExe(w.wnd.Hwnd(), w.t("browseTitle"), w.t("programs"), strings.TrimSpace(w.path.Text()))
	if !ok {
		return
	}
	w.path.SetText(p)
	w.cfg.HWiNFOPath = p
	w.save()
	w.log(w.t("logPath"), p)
	w.applyTheme(false)
}

func (w *window) launchHWiNFO() {
	p := strings.TrimSpace(w.path.Text())
	if p == "" {
		return
	}
	if p != w.cfg.HWiNFOPath {
		w.cfg.HWiNFOPath = p
		w.save()
	}
	w.log(w.t("logLaunch"), p)
	if err := hwinfo.Launch(p); err != nil {
		w.log(w.t("logLaunchErr"), err)
	}
}

func (w *window) detectHWiNFO() {
	if w.opts.Demo {
		w.launch.Hwnd().EnableWindow(false)
		return
	}
	running := hwinfo.IsRunning()
	w.launch.Hwnd().EnableWindow(!running)
	if !running || w.cfg.HWiNFOPath != "" {
		return
	}
	if p := hwinfo.FindRunning(); p != "" {
		w.cfg.HWiNFOPath = p
		w.path.SetText(p)
		w.save()
		w.log(w.t("logFound"), p)
	}
}

// --- source status ---

func (w *window) watchSource() {
	ctx := context.Background()
	for {
		st, changed := w.app.Source.Current()
		w.wnd.UiThread(func() { w.onState(st) })
		select {
		case <-changed:
		case <-ctx.Done():
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
}

func (w *window) onState(st source.State) {
	prev := w.lastSt
	w.lastSt = st
	text := w.statusText(st)
	if text != w.statusText(prev) || w.status.Text() != text {
		w.status.Hwnd().SetWindowText(text)
		if st.Status != prev.Status || st.Err != prev.Err {
			w.log("%s", text)
		}
	}
	if st.Status == source.StatusOK && st.Rev != prev.Rev {
		n := 0
		for _, s := range st.Snapshot.Sensors {
			n += len(s.Readings)
		}
		w.log(w.t("logStructure"), len(st.Snapshot.Sensors), n)
	}
}

func (w *window) statusText(st source.State) string {
	switch st.Status {
	case source.StatusOK:
		return fmt.Sprintf(w.t("stOK"), st.Snapshot.PollingPeriod)
	case source.StatusUnavailable:
		return w.t("stUnavail")
	case source.StatusInactive:
		return w.t("stInactive")
	case source.StatusError:
		return fmt.Sprintf(w.t("stError"), st.Err)
	default:
		return w.t("stStarting")
	}
}

// --- log ---

func (w *window) log(format string, args ...any) {
	line := time.Now().Format("15:04:05") + "  " + fmt.Sprintf(format, args...)
	w.logLines = append(w.logLines, line)
	if len(w.logLines) > maxLog {
		w.logLines = w.logLines[len(w.logLines)-maxLog:]
	}
	w.logBox.SetText(strings.Join(w.logLines, "\r\n"))
	n := len(w.logBox.Text())
	w.logBox.SetSelection(n, n)
	w.logBox.Hwnd().SendMessage(co.WM(0x00B7), 0, 0) // EM_SCROLLCARET
}

// --- language ---

func (w *window) relabel() {
	w.wnd.Hwnd().SetWindowText(w.t("title"))
	for s, key := range w.labels {
		s.Hwnd().SetWindowText(w.t(key))
	}
	w.browse.Hwnd().SetWindowText(w.t("browse"))
	w.launch.Hwnd().SetWindowText(w.t("launch"))
	w.apply.Hwnd().SetWindowText(w.t("apply"))
	w.tray.Hwnd().SetWindowText(w.t("startInTray"))
	w.toTray.Hwnd().SetWindowText(w.t("closeToTray"))
	w.open.Hwnd().SetWindowText(w.t("open"))
	w.scan.Hwnd().SetWindowText(w.t("scan"))
	sel := w.access.SelectedIndex()
	w.access.DeleteAllItems()
	w.access.AddItem(w.t("accessLAN"), w.t("accessLocal"))
	w.access.SelectIndex(sel)
	w.status.Hwnd().SetWindowText(w.statusText(w.lastSt))
	w.nid.SetSzTip(w.t("title"))
	winsh.Shell_NotifyIcon(cosh.NIM_MODIFY, &w.nid)
}

// --- theme ---

func (w *window) applyTheme(force bool) {
	dark := w.opts.Dark
	if p := w.cfg.HWiNFOPath; p != "" && !w.opts.Demo {
		if d, ok := hwinfo.DarkMode(p); ok {
			dark = d
		}
	}
	if !force && w.themeSet && dark == w.dark {
		return
	}
	w.dark, w.themeSet = dark, true
	w.pal = lightPal
	if dark {
		w.pal = darkPal
	}
	if w.bgBrush != 0 {
		w.bgBrush.DeleteObject()
		w.editBrush.DeleteObject()
	}
	w.bgBrush, w.editBrush = solidBrush(w.pal.bg), solidBrush(w.pal.edit)

	h := w.wnd.Hwnd()
	setTitleBarDark(h, dark)
	setMenusDark(dark)
	explorer, cfd := "Explorer", "CFD"
	if dark {
		explorer, cfd = "DarkMode_Explorer", "DarkMode_CFD"
	}
	for _, c := range []win.HWND{w.browse.Hwnd(), w.launch.Hwnd(), w.apply.Hwnd(), w.open.Hwnd(), w.logBox.Hwnd(), w.path.Hwnd(), w.port.Hwnd()} {
		setTheme(c, explorer, false)
	}
	for _, c := range []win.HWND{w.access.Hwnd(), w.langSel.Hwnd(), w.urls.Hwnd()} {
		setTheme(c, cfd, false)
	}
	// Themed checkboxes ignore WM_CTLCOLORSTATIC text color, so dark mode uses the classic look.
	setTheme(w.tray.Hwnd(), explorer, dark)
	setTheme(w.toTray.Hwnd(), explorer, dark)
	h.RedrawWindow(nil, 0, co.RDW_ERASE|co.RDW_INVALIDATE|co.RDW_ALLCHILDREN|co.RDW_FRAME)
}

// --- QR ---

func (w *window) updateQR() {
	w.qrCode = nil
	if u := w.urls.CurrentText(); u != "" {
		w.qrCode, _ = qr.Encode(u, qr.M)
	}
	w.wnd.Hwnd().InvalidateRect(&w.qrRect, true)
}

func (w *window) paint() {
	var ps win.PAINTSTRUCT
	hdc, err := w.wnd.Hwnd().BeginPaint(&ps)
	if err != nil {
		return
	}
	defer w.wnd.Hwnd().EndPaint(&ps)
	if w.qrCode == nil {
		return
	}
	white, black := solidBrush(win.RGB(255, 255, 255)), solidBrush(win.RGB(0, 0, 0))
	defer white.DeleteObject()
	defer black.DeleteObject()
	hdc.FillRect(&w.qrRect, white)
	const quiet = 4
	n := w.qrCode.Size
	side := int(w.qrRect.Right - w.qrRect.Left)
	cell := side / (n + 2*quiet)
	off := (side - cell*n) / 2
	for y := 0; y < n; y++ {
		for x := 0; x < n; x++ {
			if !w.qrCode.Black(x, y) {
				continue
			}
			rc := win.RECT{
				Left: w.qrRect.Left + int32(off+x*cell), Top: w.qrRect.Top + int32(off+y*cell),
				Right: w.qrRect.Left + int32(off+(x+1)*cell), Bottom: w.qrRect.Top + int32(off+(y+1)*cell),
			}
			hdc.FillRect(&rc, black)
		}
	}
}

// --- window position ---

// restorePosition moves the window to where the user left it, unless that spot
// is no longer on any monitor; otherwise the window stays centered.
func (w *window) restorePosition() {
	p := w.cfg.Window
	if p == nil {
		return
	}
	grip := win.POINT{X: int32(p.X + ui.DpiX(40)), Y: int32(p.Y + ui.DpiY(10))}
	if win.MonitorFromPoint(grip, co.MONITOR_DEFAULTTONULL) == 0 {
		return
	}
	w.wnd.Hwnd().SetWindowPos(0, win.POINT{X: int32(p.X), Y: int32(p.Y)}, win.SIZE{}, co.SWP_NOSIZE|co.SWP_NOZORDER|co.SWP_NOACTIVATE)
}

func (w *window) savePosition() {
	rc, err := w.wnd.Hwnd().GetWindowRect()
	if err != nil {
		return
	}
	p := &config.Point{X: int(rc.Left), Y: int(rc.Top)}
	if w.cfg.Window != nil && *w.cfg.Window == *p {
		return
	}
	w.cfg.Window = p
	w.save()
}
