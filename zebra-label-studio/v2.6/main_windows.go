//go:build windows

package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
	"unicode/utf16"
	"unsafe"
)

var (
	user32   = syscall.NewLazyDLL("user32.dll")
	gdi32    = syscall.NewLazyDLL("gdi32.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")
	comdlg32 = syscall.NewLazyDLL("comdlg32.dll")
	winspool = syscall.NewLazyDLL("winspool.drv")
	shell32  = syscall.NewLazyDLL("shell32.dll")

	pRegisterClassExW    = user32.NewProc("RegisterClassExW")
	pCreateWindowExW     = user32.NewProc("CreateWindowExW")
	pDefWindowProcW      = user32.NewProc("DefWindowProcW")
	pShowWindow          = user32.NewProc("ShowWindow")
	pUpdateWindow        = user32.NewProc("UpdateWindow")
	pGetMessageW         = user32.NewProc("GetMessageW")
	pTranslateMessage    = user32.NewProc("TranslateMessage")
	pDispatchMessageW    = user32.NewProc("DispatchMessageW")
	pPostQuitMessage     = user32.NewProc("PostQuitMessage")
	pPostMessageW        = user32.NewProc("PostMessageW")
	pSetWindowTextW      = user32.NewProc("SetWindowTextW")
	pGetWindowTextW      = user32.NewProc("GetWindowTextW")
	pGetWindowTextLen    = user32.NewProc("GetWindowTextLengthW")
	pSendMessageW        = user32.NewProc("SendMessageW")
	pMessageBoxW         = user32.NewProc("MessageBoxW")
	pGetDC               = user32.NewProc("GetDC")
	pReleaseDC           = user32.NewProc("ReleaseDC")
	pInvalidateRect      = user32.NewProc("InvalidateRect")
	pBeginPaint          = user32.NewProc("BeginPaint")
	pEndPaint            = user32.NewProc("EndPaint")
	pGetClientRect       = user32.NewProc("GetClientRect")
	pSetCapture          = user32.NewProc("SetCapture")
	pReleaseCapture      = user32.NewProc("ReleaseCapture")
	pSetFocus            = user32.NewProc("SetFocus")
	pFindWindowW         = user32.NewProc("FindWindowW")
	pSetForegroundWindow = user32.NewProc("SetForegroundWindow")
	pBringWindowToTop    = user32.NewProc("BringWindowToTop")
	pSetWindowPos        = user32.NewProc("SetWindowPos")

	pCreateSolidBrush = gdi32.NewProc("CreateSolidBrush")
	pDeleteObject     = gdi32.NewProc("DeleteObject")
	pFillRect         = user32.NewProc("FillRect")
	pRectangle        = gdi32.NewProc("Rectangle")
	pMoveToEx         = gdi32.NewProc("MoveToEx")
	pLineTo           = gdi32.NewProc("LineTo")
	pSetBkMode        = gdi32.NewProc("SetBkMode")
	pTextOutW         = gdi32.NewProc("TextOutW")
	pSelectObject     = gdi32.NewProc("SelectObject")
	pCreateFontW      = gdi32.NewProc("CreateFontW")
	pCreatePen        = gdi32.NewProc("CreatePen")
	pSetTextColor     = gdi32.NewProc("SetTextColor")

	pGetModuleHandleW = kernel32.NewProc("GetModuleHandleW")
	pOpenFileNameW    = comdlg32.NewProc("GetOpenFileNameW")

	pEnumPrintersW    = winspool.NewProc("EnumPrintersW")
	pOpenPrinterW     = winspool.NewProc("OpenPrinterW")
	pClosePrinter     = winspool.NewProc("ClosePrinter")
	pStartDocPrinterW = winspool.NewProc("StartDocPrinterW")
	pEndDocPrinter    = winspool.NewProc("EndDocPrinter")
	pStartPagePrinter = winspool.NewProc("StartPagePrinter")
	pEndPagePrinter   = winspool.NewProc("EndPagePrinter")
	pWritePrinter     = winspool.NewProc("WritePrinter")
	pShellExecuteW    = shell32.NewProc("ShellExecuteW")
)

const (
	WS_OVERLAPPEDWINDOW      = 0x00CF0000
	WS_CHILD                 = 0x40000000
	WS_VISIBLE               = 0x10000000
	WS_BORDER                = 0x00800000
	WS_TABSTOP               = 0x00010000
	BS_PUSHBUTTON            = 0x00000000
	BS_AUTOCHECKBOX          = 0x00000003
	CBS_DROPDOWN             = 0x0002
	CBS_DROPDOWNLIST         = 0x0003
	ES_AUTOHSCROLL           = 0x0080
	ES_READONLY              = 0x0800
	LBS_NOTIFY               = 0x0001
	LBS_OWNERDRAWFIXED       = 0x0010
	LBS_HASSTRINGS           = 0x0040
	SW_SHOW                  = 5
	SW_RESTORE               = 9
	HWND_TOP                 = 0
	SWP_NOSIZE               = 0x0001
	SWP_NOMOVE               = 0x0002
	SWP_SHOWWINDOW           = 0x0040
	WM_DESTROY               = 0x0002
	WM_COMMAND               = 0x0111
	WM_PAINT                 = 0x000F
	WM_LBUTTONDOWN           = 0x0201
	WM_LBUTTONUP             = 0x0202
	WM_MOUSEMOVE             = 0x0200
	WM_SIZE                  = 0x0005
	WM_DRAWITEM              = 0x002B
	WM_APP                   = 0x8000
	WM_PDF_EVENT             = WM_APP + 21
	WM_PRINT_EVENT           = WM_APP + 22
	WM_PRINTER_EVENT         = WM_APP + 23
	WM_CSV_EVENT             = WM_APP + 24
	WM_TEXT_EVENT            = WM_APP + 25
	WM_DIAG_HEARTBEAT        = WM_APP + 26
	BM_GETCHECK              = 0x00F0
	BM_SETCHECK              = 0x00F1
	BST_CHECKED              = 1
	CB_ADDSTRING             = 0x0143
	CB_GETCURSEL             = 0x0147
	CB_RESETCONTENT          = 0x014B
	CB_SETCURSEL             = 0x014E
	CBN_SELCHANGE            = 1
	BN_CLICKED               = 0
	EN_CHANGE                = 0x0300
	EN_KILLFOCUS             = 0x0200
	LB_ADDSTRING             = 0x0180
	LB_RESETCONTENT          = 0x0184
	LB_GETTEXT               = 0x0189
	LB_GETTEXTLEN            = 0x018A
	LB_SETITEMHEIGHT         = 0x01A0
	ODS_SELECTED             = 0x0001
	MB_OK                    = 0x00000000
	MB_ICONERROR             = 0x00000010
	MB_ICONINFORMATION       = 0x00000040
	OFN_FILEMUSTEXIST        = 0x00001000
	OFN_PATHMUSTEXIST        = 0x00000800
	TRANSPARENT              = 1
	PRINTER_ENUM_LOCAL       = 0x00000002
	PRINTER_ENUM_CONNECTIONS = 0x00000004
	PBM_SETPOS               = 0x0402
	PBM_SETRANGE32           = 0x0406
)

const (
	ID_PRINTER       = 101
	ID_REFRESH       = 102
	ID_DPI           = 103
	ID_LABEL_W       = 104
	ID_LABEL_H       = 105
	ID_MEDIA_W       = 106
	ID_GAP           = 107
	ID_DM_SIZE       = 108
	ID_X             = 109
	ID_Y             = 110
	ID_OPEN_CSV      = 111
	ID_OPEN_PDF      = 112
	ID_PRINT_ONE     = 113
	ID_PRINT_ALL     = 114
	ID_TEST          = 115
	ID_TEXT1_CHECK   = 121
	ID_TEXT1         = 122
	ID_TEXT2_CHECK   = 123
	ID_TEXT2         = 124
	ID_TEXT3_CHECK   = 125
	ID_TEXT3         = 126
	ID_TEXT_SIZE     = 127
	ID_LABEL_OFFSET  = 128
	ID_CENTER_LABEL  = 129
	ID_EXPORT_ZPL    = 130
	ID_REPRINT_N     = 131
	ID_REPRINT_LAST  = 132
	ID_FIND_PRINTER  = 133
	ID_DPI_300       = 134
	ID_DPI_600       = 135
	ID_DPI_VALUE     = 136
	ID_CANCEL_PDF    = 137
	ID_PDF_PROGRESS  = 138
	ID_PDF_TEXT      = 139
	ID_STATUS        = 140
	ID_CODELIST      = 141
	ID_QUEUE_STATUS  = 142
	ID_TEXT1_SIZE    = 143
	ID_TEXT1_X       = 144
	ID_TEXT1_Y       = 145
	ID_TEXT2_SIZE    = 146
	ID_TEXT2_X       = 147
	ID_TEXT2_Y       = 148
	ID_TEXT3_SIZE    = 149
	ID_TEXT3_X       = 150
	ID_TEXT3_Y       = 151
	ID_TEXT1_LINES   = 152
	ID_TEXT2_LINES   = 153
	ID_TEXT3_LINES   = 154
	ID_RANGE_FROM    = 155
	ID_RANGE_TO      = 156
	ID_PRINT_RANGE   = 157
	ID_TEXT1_EDITBTN = 158
	ID_TEXT2_EDITBTN = 159
	ID_TEXT3_EDITBTN = 160
	ID_OPEN_LOGS     = 161
)

type WNDCLASSEX struct {
	cbSize        uint32
	style         uint32
	lpfnWndProc   uintptr
	cbClsExtra    int32
	cbWndExtra    int32
	hInstance     syscall.Handle
	hIcon         syscall.Handle
	hCursor       syscall.Handle
	hbrBackground syscall.Handle
	lpszMenuName  *uint16
	lpszClassName *uint16
	hIconSm       syscall.Handle
}
type POINT struct{ X, Y int32 }
type MSG struct {
	HWnd           syscall.Handle
	Message        uint32
	WParam, LParam uintptr
	Time           uint32
	Pt             POINT
	LPrivate       uint32
}
type RECT struct{ Left, Top, Right, Bottom int32 }
type PAINTSTRUCT struct {
	Hdc         syscall.Handle
	FErase      int32
	RcPaint     RECT
	FRestore    int32
	FIncUpdate  int32
	RgbReserved [32]byte
}
type DRAWITEMSTRUCT struct {
	CtlType    uint32
	CtlID      uint32
	ItemID     uint32
	ItemAction uint32
	ItemState  uint32
	HwndItem   syscall.Handle
	HDC        syscall.Handle
	RcItem     RECT
	ItemData   uintptr
}
type OPENFILENAME struct {
	lStructSize       uint32
	hwndOwner         syscall.Handle
	hInstance         syscall.Handle
	lpstrFilter       *uint16
	lpstrCustomFilter *uint16
	nMaxCustFilter    uint32
	nFilterIndex      uint32
	lpstrFile         *uint16
	nMaxFile          uint32
	lpstrFileTitle    *uint16
	nMaxFileTitle     uint32
	lpstrInitialDir   *uint16
	lpstrTitle        *uint16
	Flags             uint32
	nFileOffset       uint16
	nFileExtension    uint16
	lpstrDefExt       *uint16
	lCustData         uintptr
	lpfnHook          uintptr
	lpTemplateName    *uint16
	pvReserved        uintptr
	dwReserved        uint32
	FlagsEx           uint32
}
type DOC_INFO_1 struct {
	pDocName    *uint16
	pOutputFile *uint16
	pDatatype   *uint16
}
type PRINTER_INFO_4 struct {
	pPrinterName *uint16
	pServerName  *uint16
	Attributes   uint32
}

type Element struct {
	kind       int
	x, y, w, h float64
}

var (
	hMain, hCanvas      syscall.Handle
	controls            = map[int]syscall.Handle{}
	codes               [][]byte
	currentFile         string
	currentPDF          string
	codeSources         []string
	pdfBusy             atomic.Bool
	selected            = 0
	dragging            bool
	dragDX, dragDY      float64
	suppressControlSync bool
	printerScanRunning  atomic.Bool
	printerMu           sync.Mutex
	printerFound        string
	currentDPI          atomic.Int64
	pdfCancelMu         sync.Mutex
	pdfCancel           context.CancelFunc
	elems               = []Element{{kind: 0, x: 2, y: 2, w: 12, h: 12}, {kind: 1, x: 2, y: 15, w: 8, h: 2.2}, {kind: 2, x: 2, y: 19, w: 8, h: 2.2}, {kind: 3, x: 2, y: 23, w: 8, h: 2.2}}
	textCache           = [3]string{"GTIN: {GTIN}", "SN: {SERIAL}", ""}
	lastUIAck           atomic.Int64
	hangDumped          atomic.Bool
)

func ptr(s string) *uint16 { p, _ := syscall.UTF16PtrFromString(s); return p }
func utf16String(p *uint16) string {
	if p == nil {
		return ""
	}
	a := []uint16{}
	for i := uintptr(0); ; i++ {
		v := *(*uint16)(unsafe.Pointer(uintptr(unsafe.Pointer(p)) + i*2))
		if v == 0 {
			break
		}
		a = append(a, v)
	}
	return string(utf16.Decode(a))
}
func loword(v uintptr) int { return int(v & 0xffff) }
func hiword(v uintptr) int { return int((v >> 16) & 0xffff) }

type pdfWorkerResult struct {
	Codes   [][]byte `json:"codes"`
	Sources []string `json:"sources"`
	Error   string   `json:"error,omitempty"`
}

func runPDFHelperMode() bool {
	var pdfPath, outPath, progressPath string
	if len(os.Args) >= 4 && os.Args[1] == "--pdf-helper" {
		pdfPath, outPath = os.Args[2], os.Args[3]
		if len(os.Args) >= 5 {
			progressPath = os.Args[4]
		}
	} else if (len(os.Args) == 3 || len(os.Args) == 4) && strings.EqualFold(filepath.Base(os.Args[0]), "PDFDecoder.exe") {
		pdfPath, outPath = os.Args[1], os.Args[2]
		if len(os.Args) == 4 {
			progressPath = os.Args[3]
		}
	} else {
		return false
	}
	res := pdfWorkerResult{}
	defer func() {
		if r := recover(); r != nil {
			res.Error = fmt.Sprintf("внутренняя ошибка PDF-сканера: %v", r)
		}
		b, _ := json.Marshal(res)
		_ = os.WriteFile(outPath, b, 0600)
	}()
	progress := func(msg string) {
		if progressPath == "" {
			return
		}
		f, err := os.OpenFile(progressPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
		if err != nil {
			return
		}
		_, _ = fmt.Fprintln(f, msg)
		_ = f.Close()
	}
	codes, sources, err := scanPDFCodes(pdfPath, progress)
	if err != nil {
		res.Error = err.Error()
		return true
	}
	res.Codes = codes
	res.Sources = sources
	return true
}

func diagnosticsDir() string {
	base := os.Getenv("LOCALAPPDATA")
	if base == "" {
		base = os.TempDir()
	}
	return filepath.Join(base, "MSK", "Zebra Label Studio", "logs")
}

func diagLog(format string, args ...any) {
	dir := diagnosticsDir()
	_ = os.MkdirAll(dir, 0755)
	f, err := os.OpenFile(filepath.Join(dir, "app.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "%s  %s\r\n", time.Now().Format("2006-01-02 15:04:05.000"), fmt.Sprintf(format, args...))
}

func writeStackDump(prefix string) string {
	dir := diagnosticsDir()
	_ = os.MkdirAll(dir, 0755)
	path := filepath.Join(dir, fmt.Sprintf("%s-%s.log", prefix, time.Now().Format("20060102-150405")))
	buf := make([]byte, 4<<20)
	n := runtime.Stack(buf, true)
	body := fmt.Sprintf("MSK Zebra Label Studio v2.6\r\nTime: %s\r\nGo: %s\r\nOS/Arch: %s/%s\r\n\r\n%s", time.Now().Format(time.RFC3339), runtime.Version(), runtime.GOOS, runtime.GOARCH, string(buf[:n]))
	_ = os.WriteFile(path, []byte(body), 0644)
	return path
}

func startUIWatchdog() {
	lastUIAck.Store(time.Now().UnixNano())
	go func() {
		t := time.NewTicker(time.Second)
		defer t.Stop()
		for range t.C {
			if hMain == 0 {
				continue
			}
			pPostMessageW.Call(uintptr(hMain), WM_DIAG_HEARTBEAT, 0, 0)
			last := time.Unix(0, lastUIAck.Load())
			if time.Since(last) > 5*time.Second {
				if hangDumped.CompareAndSwap(false, true) {
					path := writeStackDump("hang")
					diagLog("WATCHDOG: UI не отвечает >5с, stack=%s", path)
				}
			}
		}
	}()
}

func logCrash(where string, v any) {
	dir := diagnosticsDir()
	_ = os.MkdirAll(dir, 0755)
	path := filepath.Join(dir, fmt.Sprintf("crash-%s.log", time.Now().Format("20060102-150405")))
	body := fmt.Sprintf("MSK Zebra Label Studio v2.6\r\nWhere: %s\r\nPanic: %v\r\nTime: %s\r\n\r\n%s", where, v, time.Now().Format(time.RFC3339), string(debug.Stack()))
	_ = os.WriteFile(path, []byte(body), 0644)
	diagLog("CRASH %s: %v, file=%s", where, v, path)
}

func crashGuard(where string) {
	if r := recover(); r != nil {
		logCrash(where, r)
		msgbox(fmt.Sprintf("Внутренняя ошибка %s: %v\n\nЛог: %%LOCALAPPDATA%%\\MSK\\Zebra Label Studio\\error.log", where, r), true)
	}
}

func main() {
	defer crashGuard("main")
	diagLog("START v2.6")
	// PDF decoding runs in a separate process. A native decoder crash must never
	// be able to terminate the main GUI process.
	if runPDFHelperMode() {
		return
	}
	className := ptr("MSK_ZebraLabelStudio")
	// Single-instance behavior: if a previous copy is already running,
	// restore and bring its window to the foreground instead of starting
	// an invisible duplicate process.
	if existing, _, _ := pFindWindowW.Call(uintptr(unsafe.Pointer(className)), 0); existing != 0 {
		pShowWindow.Call(existing, SW_RESTORE)
		pBringWindowToTop.Call(existing)
		pSetWindowPos.Call(existing, HWND_TOP, 0, 0, 0, 0, SWP_NOMOVE|SWP_NOSIZE|SWP_SHOWWINDOW)
		pSetForegroundWindow.Call(existing)
		return
	}
	hInst, _, _ := pGetModuleHandleW.Call(0)
	wc := WNDCLASSEX{cbSize: uint32(unsafe.Sizeof(WNDCLASSEX{})), lpfnWndProc: syscall.NewCallback(wndProc), hInstance: syscall.Handle(hInst), hbrBackground: syscall.Handle(6), lpszClassName: className}
	pRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))
	h, _, _ := pCreateWindowExW.Call(0, uintptr(unsafe.Pointer(className)), uintptr(unsafe.Pointer(ptr("MSK Zebra Label Studio v2.6 — Честный Знак"))), WS_OVERLAPPEDWINDOW, 45, 25, 1540, 940, 0, 0, hInst, 0)
	hMain = syscall.Handle(h)
	startUIWatchdog()
	buildUI(hMain, syscall.Handle(hInst))
	tryRecoverAfterUI()
	pShowWindow.Call(h, SW_SHOW)
	pUpdateWindow.Call(h)
	// Make the main window visible even when launched by an installer or
	// from a stale shortcut. This avoids the "process is running but I
	// cannot see the program" situation.
	pShowWindow.Call(h, SW_RESTORE)
	pBringWindowToTop.Call(h)
	pSetWindowPos.Call(h, HWND_TOP, 0, 0, 0, 0, SWP_NOMOVE|SWP_NOSIZE|SWP_SHOWWINDOW)
	pSetForegroundWindow.Call(h)
	var msg MSG
	for {
		r, _, _ := pGetMessageW.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if int32(r) <= 0 {
			break
		}
		pTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
		pDispatchMessageW.Call(uintptr(unsafe.Pointer(&msg)))
	}
}

func create(class, text string, style uint32, x, y, w, h int, parent syscall.Handle, id int) syscall.Handle {
	r, _, _ := pCreateWindowExW.Call(0, uintptr(unsafe.Pointer(ptr(class))), uintptr(unsafe.Pointer(ptr(text))), uintptr(style), uintptr(x), uintptr(y), uintptr(w), uintptr(h), uintptr(parent), uintptr(id), 0, 0)
	hw := syscall.Handle(r)
	if id != 0 {
		controls[id] = hw
	}
	return hw
}
func label(text string, x, y, w int, parent syscall.Handle) {
	create("STATIC", text, WS_CHILD|WS_VISIBLE, x, y, w, 20, parent, 0)
}

type pdfUIEvent struct {
	kind    string
	status  string
	percent int
	found   [][]byte
	sources []string
	err     error
}

var (
	pdfEventMu  sync.Mutex
	pdfEvents   = map[uintptr]pdfUIEvent{}
	pdfEventSeq atomic.Uint64
)

func postPDFEvent(ev pdfUIEvent) {
	id := uintptr(pdfEventSeq.Add(1))
	pdfEventMu.Lock()
	pdfEvents[id] = ev
	pdfEventMu.Unlock()
	pPostMessageW.Call(uintptr(hMain), WM_PDF_EVENT, id, 0)
}

func takePDFEvent(id uintptr) (pdfUIEvent, bool) {
	pdfEventMu.Lock()
	defer pdfEventMu.Unlock()
	ev, ok := pdfEvents[id]
	if ok {
		delete(pdfEvents, id)
	}
	return ev, ok
}

func buildUI(parent syscall.Handle, hInst syscall.Handle) {
	label("Принтер Zebra / ZDesigner", 18, 18, 165, parent)
	create("COMBOBOX", "", WS_CHILD|WS_VISIBLE|WS_TABSTOP|WS_BORDER|CBS_DROPDOWN, 185, 14, 320, 240, parent, ID_PRINTER)
	create("BUTTON", "Найти Zebra", WS_CHILD|WS_VISIBLE|BS_PUSHBUTTON, 515, 14, 110, 28, parent, ID_FIND_PRINTER)
	label("DPI", 640, 18, 28, parent)
	create("BUTTON", "203", WS_CHILD|WS_VISIBLE|BS_PUSHBUTTON, 672, 14, 48, 28, parent, ID_DPI)
	create("BUTTON", "300", WS_CHILD|WS_VISIBLE|BS_PUSHBUTTON, 724, 14, 48, 28, parent, ID_DPI_300)
	create("BUTTON", "600", WS_CHILD|WS_VISIBLE|BS_PUSHBUTTON, 776, 14, 48, 28, parent, ID_DPI_600)
	create("STATIC", "выбрано: 203", WS_CHILD|WS_VISIBLE, 835, 18, 105, 20, parent, ID_DPI_VALUE)
	create("BUTTON", "Диагностика", WS_CHILD|WS_VISIBLE|BS_PUSHBUTTON, 950, 14, 115, 28, parent, ID_OPEN_LOGS)

	label("Наклейка, мм", 18, 58, 100, parent)
	label("Ш", 120, 58, 18, parent)
	create("EDIT", "15", WS_CHILD|WS_VISIBLE|WS_BORDER|ES_AUTOHSCROLL, 140, 54, 55, 24, parent, ID_LABEL_W)
	label("В", 205, 58, 18, parent)
	create("EDIT", "15", WS_CHILD|WS_VISIBLE|WS_BORDER|ES_AUTOHSCROLL, 225, 54, 55, 24, parent, ID_LABEL_H)
	label("Физ. ширина", 300, 58, 90, parent)
	create("EDIT", "26", WS_CHILD|WS_VISIBLE|WS_BORDER|ES_AUTOHSCROLL, 395, 54, 55, 24, parent, ID_MEDIA_W)
	label("Зазор", 465, 58, 45, parent)
	create("EDIT", "2", WS_CHILD|WS_VISIBLE|WS_BORDER|ES_AUTOHSCROLL, 510, 54, 55, 24, parent, ID_GAP)
	label("Смещение X", 580, 58, 80, parent)
	create("EDIT", "0", WS_CHILD|WS_VISIBLE|WS_BORDER|ES_AUTOHSCROLL, 665, 54, 55, 24, parent, ID_LABEL_OFFSET)
	center := create("BUTTON", "Центрировать", WS_CHILD|WS_VISIBLE|BS_AUTOCHECKBOX, 735, 54, 125, 24, parent, ID_CENTER_LABEL)
	pSendMessageW.Call(uintptr(center), BM_SETCHECK, BST_CHECKED, 0)

	label("Data Matrix, мм", 18, 96, 110, parent)
	create("EDIT", "12", WS_CHILD|WS_VISIBLE|WS_BORDER|ES_AUTOHSCROLL, 130, 92, 55, 24, parent, ID_DM_SIZE)
	label("X", 200, 96, 15, parent)
	create("EDIT", "2", WS_CHILD|WS_VISIBLE|WS_BORDER|ES_AUTOHSCROLL, 218, 92, 55, 24, parent, ID_X)
	label("Y", 285, 96, 15, parent)
	create("EDIT", "2", WS_CHILD|WS_VISIBLE|WS_BORDER|ES_AUTOHSCROLL, 303, 92, 55, 24, parent, ID_Y)
	label("Текст", 88, 122, 100, parent)
	label("Размер", 535, 122, 55, parent)
	label("X", 612, 122, 20, parent)
	label("Y", 682, 122, 20, parent)

	create("BUTTON", "Показывать", WS_CHILD|WS_VISIBLE|BS_AUTOCHECKBOX, 18, 145, 90, 24, parent, ID_TEXT1_CHECK)
	label("GTIN", 115, 148, 45, parent)
	create("STATIC", "GTIN: {GTIN}", WS_CHILD|WS_VISIBLE, 165, 147, 235, 24, parent, ID_TEXT1)
	create("BUTTON", "Редактировать...", WS_CHILD|WS_VISIBLE|BS_PUSHBUTTON, 405, 143, 120, 26, parent, ID_TEXT1_EDITBTN)
	create("EDIT", "2.2", WS_CHILD|WS_VISIBLE|WS_BORDER|ES_AUTOHSCROLL, 675, 143, 55, 26, parent, ID_TEXT1_SIZE)
	create("EDIT", "2", WS_CHILD|WS_VISIBLE|WS_BORDER|ES_AUTOHSCROLL, 605, 143, 55, 26, parent, ID_TEXT1_X)
	create("EDIT", "15", WS_CHILD|WS_VISIBLE|WS_BORDER|ES_AUTOHSCROLL, 675, 143, 55, 26, parent, ID_TEXT1_Y)

	create("BUTTON", "Показывать", WS_CHILD|WS_VISIBLE|BS_AUTOCHECKBOX, 18, 178, 90, 24, parent, ID_TEXT2_CHECK)
	label("Серийный", 115, 181, 60, parent)
	create("STATIC", "SN: {SERIAL}", WS_CHILD|WS_VISIBLE, 180, 180, 220, 24, parent, ID_TEXT2)
	create("BUTTON", "Редактировать...", WS_CHILD|WS_VISIBLE|BS_PUSHBUTTON, 405, 176, 120, 26, parent, ID_TEXT2_EDITBTN)
	create("EDIT", "2.2", WS_CHILD|WS_VISIBLE|WS_BORDER|ES_AUTOHSCROLL, 675, 176, 55, 26, parent, ID_TEXT2_SIZE)
	create("EDIT", "2", WS_CHILD|WS_VISIBLE|WS_BORDER|ES_AUTOHSCROLL, 605, 176, 55, 26, parent, ID_TEXT2_X)
	create("EDIT", "19", WS_CHILD|WS_VISIBLE|WS_BORDER|ES_AUTOHSCROLL, 675, 176, 55, 26, parent, ID_TEXT2_Y)

	create("BUTTON", "Показывать", WS_CHILD|WS_VISIBLE|BS_AUTOCHECKBOX, 18, 211, 90, 24, parent, ID_TEXT3_CHECK)
	label("Доп. текст", 115, 214, 75, parent)
	create("STATIC", "—", WS_CHILD|WS_VISIBLE, 195, 213, 205, 24, parent, ID_TEXT3)
	create("BUTTON", "Редактировать...", WS_CHILD|WS_VISIBLE|BS_PUSHBUTTON, 405, 209, 120, 26, parent, ID_TEXT3_EDITBTN)
	label("Редактор показывает явно: 1 строка или 2 строки. Enter = вторая строка.", 115, 238, 570, parent)
	create("EDIT", "2.2", WS_CHILD|WS_VISIBLE|WS_BORDER|ES_AUTOHSCROLL, 675, 209, 55, 26, parent, ID_TEXT3_SIZE)
	create("EDIT", "2", WS_CHILD|WS_VISIBLE|WS_BORDER|ES_AUTOHSCROLL, 605, 209, 55, 26, parent, ID_TEXT3_X)
	create("EDIT", "23", WS_CHILD|WS_VISIBLE|WS_BORDER|ES_AUTOHSCROLL, 675, 209, 55, 26, parent, ID_TEXT3_Y)

	create("BUTTON", "Открыть CSV", WS_CHILD|WS_VISIBLE|BS_PUSHBUTTON, 18, 270, 110, 32, parent, ID_OPEN_CSV)
	create("BUTTON", "Открыть PDF", WS_CHILD|WS_VISIBLE|BS_PUSHBUTTON, 138, 270, 110, 32, parent, ID_OPEN_PDF)
	create("BUTTON", "Сохранить ZPL", WS_CHILD|WS_VISIBLE|BS_PUSHBUTTON, 258, 270, 110, 32, parent, ID_EXPORT_ZPL)
	create("BUTTON", "Тест выбранным КМ", WS_CHILD|WS_VISIBLE|BS_PUSHBUTTON, 378, 270, 140, 32, parent, ID_TEST)
	create("BUTTON", "Печать 1", WS_CHILD|WS_VISIBLE|BS_PUSHBUTTON, 528, 270, 92, 32, parent, ID_PRINT_ONE)
	create("BUTTON", "Печать всех", WS_CHILD|WS_VISIBLE|BS_PUSHBUTTON, 18, 309, 110, 30, parent, ID_PRINT_ALL)
	label("Диапазон №", 145, 315, 78, parent)
	create("EDIT", "1", WS_CHILD|WS_VISIBLE|WS_BORDER|ES_AUTOHSCROLL, 225, 309, 55, 30, parent, ID_RANGE_FROM)
	label("—", 286, 315, 18, parent)
	create("EDIT", "100", WS_CHILD|WS_VISIBLE|WS_BORDER|ES_AUTOHSCROLL, 306, 309, 55, 30, parent, ID_RANGE_TO)
	create("BUTTON", "Печать диапазона", WS_CHILD|WS_VISIBLE|BS_PUSHBUTTON, 372, 309, 150, 30, parent, ID_PRINT_RANGE)

	label("Перепечатать последние", 18, 354, 145, parent)
	create("EDIT", "1", WS_CHILD|WS_VISIBLE|WS_BORDER|ES_AUTOHSCROLL, 165, 348, 45, 30, parent, ID_REPRINT_N)
	create("BUTTON", "Перепечатать", WS_CHILD|WS_VISIBLE|BS_PUSHBUTTON, 220, 348, 115, 30, parent, ID_REPRINT_LAST)

	lb := create("LISTBOX", "", WS_CHILD|WS_VISIBLE|WS_BORDER|LBS_NOTIFY|LBS_OWNERDRAWFIXED|LBS_HASSTRINGS, 18, 390, 650, 380, parent, ID_CODELIST)
	pSendMessageW.Call(uintptr(lb), LB_SETITEMHEIGHT, 0, 21)
	create("STATIC", "PDF/CSV: ожидание", WS_CHILD|WS_VISIBLE, 18, 782, 480, 22, parent, ID_PDF_TEXT)
	pb := create("msctls_progress32", "", WS_CHILD|WS_VISIBLE, 18, 808, 480, 22, parent, ID_PDF_PROGRESS)
	pSendMessageW.Call(uintptr(pb), PBM_SETRANGE32, 0, 100)
	create("BUTTON", "Отменить PDF", WS_CHILD|WS_VISIBLE|BS_PUSHBUTTON, 510, 780, 140, 52, parent, ID_CANCEL_PDF)

	canvasClass := ptr("MSK_LabelCanvas")
	wc := WNDCLASSEX{cbSize: uint32(unsafe.Sizeof(WNDCLASSEX{})), lpfnWndProc: syscall.NewCallback(canvasProc), hInstance: hInst, hbrBackground: syscall.Handle(6), lpszClassName: canvasClass}
	pRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))
	r, _, _ := pCreateWindowExW.Call(0, uintptr(unsafe.Pointer(canvasClass)), 0, WS_CHILD|WS_VISIBLE|WS_BORDER, 700, 52, 790, 760, uintptr(parent), 0, uintptr(hInst), 0)
	hCanvas = syscall.Handle(r)
	create("STATIC", "Очередь: новая сессия не создана", WS_CHILD|WS_VISIBLE, 700, 820, 790, 24, parent, ID_QUEUE_STATUS)
	create("STATIC", "Готово. Напечатанные КМ подсвечиваются зелёным. PDF обрабатывается быстрым параллельным режимом.", WS_CHILD|WS_VISIBLE, 18, 855, 1470, 30, parent, ID_STATUS)
}

func wndProc(hwnd syscall.Handle, msg uint32, wparam, lparam uintptr) (ret uintptr) {
	defer func() {
		if r := recover(); r != nil {
			logCrash("wndProc", r)
			ret = 0
		}
	}()
	switch msg {
	case WM_DIAG_HEARTBEAT:
		lastUIAck.Store(time.Now().UnixNano())
		hangDumped.Store(false)
		return 0
	case WM_DESTROY:
		clearRecoveryMarker()
		pPostQuitMessage.Call(0)
		return 0
	case WM_PRINTER_EVENT:
		if ev, ok := takePrinterEvent(wparam); ok {
			printerScanRunning.Store(false)
			if ev.err != nil {
				setStatus("Поиск Zebra: " + ev.err.Error())
				msgbox("Не удалось получить список принтеров:\n\n"+ev.err.Error()+"\n\nОсновная программа продолжает работать. Имя Zebra можно ввести вручную.", true)
				return 0
			}
			if len(ev.names) == 0 {
				setStatus("Zebra / ZDesigner не найдена среди установленных очередей Windows.")
				msgbox("Zebra / ZDesigner не найдена.\n\nПроверьте, что драйвер принтера установлен в Windows и принтер виден в 'Принтеры и сканеры'.", false)
				return 0
			}
			// Заполняем выпадающий список всеми найденными очередями Zebra/ZDesigner.
			hp := controls[ID_PRINTER]
			pSendMessageW.Call(uintptr(hp), CB_RESETCONTENT, 0, 0)
			for _, name := range ev.names {
				pSendMessageW.Call(uintptr(hp), CB_ADDSTRING, 0, uintptr(unsafe.Pointer(ptr(name))))
			}
			pSendMessageW.Call(uintptr(hp), CB_SETCURSEL, 0, 0)
			setText(ID_PRINTER, ev.names[0])
			if len(ev.names) == 1 {
				setStatus("Найдена Zebra: " + ev.names[0])
			} else {
				setStatus(fmt.Sprintf("Найдено Zebra/ZDesigner: %d. Выберите нужную очередь в списке принтеров.", len(ev.names)))
			}
		}
		return 0
	case WM_PDF_EVENT:
		if ev, ok := takePDFEvent(wparam); ok {
			switch ev.kind {
			case "progress":
				setStatus(ev.status)
				setText(ID_PDF_TEXT, ev.status)
				if hp := controls[ID_PDF_PROGRESS]; hp != 0 {
					pSendMessageW.Call(uintptr(hp), PBM_SETPOS, uintptr(ev.percent), 0)
				}
			case "done":
				pdfBusy.Store(false)
				pdfCancelMu.Lock()
				pdfCancel = nil
				pdfCancelMu.Unlock()
				if hp := controls[ID_PDF_PROGRESS]; hp != 0 {
					pSendMessageW.Call(uintptr(hp), PBM_SETPOS, 100, 0)
				}
				if ev.err != nil {
					setStatus("Ошибка распознавания PDF: " + ev.err.Error())
					msgbox("Не удалось распознать PDF:\n\n"+ev.err.Error(), true)
					return 0
				}
				codeSources = ev.sources
				codes = ev.found
				activateSessionJournal(currentPDF)
				refreshCodeList()
				pInvalidateRect.Call(uintptr(hCanvas), 0, 1)
				if len(ev.found) == 0 {
					setStatus("PDF обработан, но Data Matrix не найден. Попробуйте другой PDF или более качественный исходник.")
				} else {
					setStatus(fmt.Sprintf("PDF: %s — найдено полных кодов: %d. Готово к печати.", filepath.Base(currentPDF), len(ev.found)))
				}
			}
		}
		return 0
	case WM_PRINT_EVENT:
		if ev, ok := takePrintEvent(wparam); ok {
			printBusy.Store(false)
			confirmed := readProgressIndices(ev.progressPath)
			if len(confirmed) > 0 {
				applyConfirmedPrints(confirmed, ev.isReprint)
			}
			if ev.progressPath != "" && ev.isReprint {
				_ = os.Remove(ev.progressPath)
			}
			if ev.err != nil {
				setStatus(fmt.Sprintf("Печать остановлена: подтверждено %d из %d. %s", len(confirmed), len(ev.requested), ev.err.Error()))
				msgbox(fmt.Sprintf("Печать остановлена.\n\nПодтверждено передачей в очередь Zebra: %d из %d.\nСледующая обычная печать продолжит с первого неподтверждённого кода.\n\n%s", len(confirmed), len(ev.requested), ev.err.Error()), true)
			} else {
				setStatus(ev.status)
			}
			updateQueueStatus()
			refreshCodeList()
		}
		return 0
	case WM_CSV_EVENT:
		if ev, ok := takeCSVEvent(wparam); ok {
			if ev.kind == "progress" {
				pSendMessageW.Call(uintptr(controls[ID_PDF_PROGRESS]), PBM_SETPOS, uintptr(ev.percent), 0)
				setStatus(ev.status)
				return 0
			}
			csvBusy.Store(false)
			if ev.err != nil {
				setStatus("Ошибка CSV: " + ev.err.Error())
				msgbox("Ошибка чтения CSV:\n\n"+ev.err.Error(), true)
				return 0
			}
			codes = ev.codes
			// For huge CSV files do not allocate one source string per code.
			// The UI renders only the first 200 rows and the full code array is used
			// directly by the print queue.
			codeSources = nil
			currentFile = ev.path
			currentPDF = ""
			activateSessionJournal(ev.path)
			pSendMessageW.Call(uintptr(controls[ID_PDF_PROGRESS]), PBM_SETPOS, 100, 0)
			setStatus(fmt.Sprintf("CSV: %s — загружено кодов: %d", filepath.Base(ev.path), len(codes)))
			refreshCodeList()
			pInvalidateRect.Call(uintptr(hCanvas), 0, 1)
		}
		return 0
	case WM_DRAWITEM:
		if loword(wparam) == ID_CODELIST {
			drawCodeListItem(lparam)
			return 1
		}
		return 0
	case WM_TEXT_EVENT:
		if ev, ok := takeTextEvent(wparam); ok {
			if ev.err != nil {
				setStatus("Редактор текста: " + ev.err.Error())
				return 0
			}
			if ev.index >= 0 && ev.index < 3 {
				textCache[ev.index] = ev.text
				setText(textEditID(ev.index), ev.text)
				updateRecoveryTexts()
				pInvalidateRect.Call(uintptr(hCanvas), 0, 1)
				diagLog("TEXT: applied index=%d lines=%d len=%d", ev.index, len(strings.Split(ev.text, "\n")), len(ev.text))
				setStatus("Текст обновлён")
			}
		}
		return 0
	case WM_COMMAND:
		id := loword(wparam)
		notify := hiword(wparam)
		// IMPORTANT: route by notification code. A ComboBox selection is not a
		// button click and an Edit EN_CHANGE is not a generic command. Keeping
		// these paths separate prevents re-entrant Win32 processing/freezes.
		switch id {
		case ID_TEXT1, ID_TEXT2, ID_TEXT3:
			// Read-only in the main window. Editing is isolated in TextEditor.exe.
		case ID_TEXT1_EDITBTN, ID_TEXT2_EDITBTN, ID_TEXT3_EDITBTN:
			if notify == BN_CLICKED {
				i := 0
				if id == ID_TEXT2_EDITBTN {
					i = 1
				} else if id == ID_TEXT3_EDITBTN {
					i = 2
				}
				startExternalTextEditor(i)
			}
		case ID_PRINTER:
			// ComboBox локальный: выбор/ввод только меняет текст. Никаких обращений к spooler.
			// Найденные Zebra добавляются кнопкой «Найти Zebra».
		case ID_OPEN_LOGS:
			if notify == BN_CLICKED {
				diagLog("UI: открыть папку диагностики")
				dir := diagnosticsDir()
				_ = os.MkdirAll(dir, 0755)
				pShellExecuteW.Call(0, uintptr(unsafe.Pointer(ptr("open"))), uintptr(unsafe.Pointer(ptr(dir))), 0, 0, SW_SHOW)
			}
		case ID_FIND_PRINTER:
			if notify == BN_CLICKED {
				startPrinterScan()
			}
		case ID_DPI:
			if notify == BN_CLICKED {
				setDPI(203)
			}
		case ID_DPI_300:
			if notify == BN_CLICKED {
				setDPI(300)
			}
		case ID_DPI_600:
			if notify == BN_CLICKED {
				setDPI(600)
			}
		case ID_CANCEL_PDF:
			if notify == BN_CLICKED {
				cancelPDFScan()
			}
		case ID_OPEN_CSV:
			if notify == BN_CLICKED {
				if f := openFile(hwnd, []string{"CSV / TXT", "*.csv;*.txt", "Все файлы", "*.*"}); f != "" {
					loadCSVAsync(f)
				}
			}
		case ID_OPEN_PDF:
			if notify == BN_CLICKED {
				if f := openFile(hwnd, []string{"PDF", "*.pdf"}); f != "" {
					loadPDFAsync(f)
				}
			}
		case ID_EXPORT_ZPL:
			if notify == BN_CLICKED {
				exportZPL()
			}
		case ID_TEST:
			if notify == BN_CLICKED {
				printTest()
			}
		case ID_CENTER_LABEL:
			if notify == BN_CLICKED {
				pInvalidateRect.Call(uintptr(hCanvas), 0, 1)
			}
		case ID_PRINT_ONE:
			if notify == BN_CLICKED {
				printOne()
			}
		case ID_PRINT_ALL:
			if notify == BN_CLICKED {
				printAll()
			}
		case ID_PRINT_RANGE:
			if notify == BN_CLICKED {
				printRange()
			}
		case ID_REPRINT_LAST:
			if notify == BN_CLICKED {
				reprintLast()
			}
		case ID_LABEL_W, ID_LABEL_H, ID_MEDIA_W, ID_GAP, ID_DM_SIZE, ID_X, ID_Y, ID_LABEL_OFFSET, ID_TEXT1_SIZE, ID_TEXT1_X, ID_TEXT1_Y, ID_TEXT2_SIZE, ID_TEXT2_X, ID_TEXT2_Y, ID_TEXT3_SIZE, ID_TEXT3_X, ID_TEXT3_Y:
			// Same rule for numeric edits: no full-state reread on each character.
			if notify == EN_KILLFOCUS && !suppressControlSync {
				syncFromControls()
				pInvalidateRect.Call(uintptr(hCanvas), 0, 1)
			}
		}
		return 0
	}
	r, _, _ := pDefWindowProcW.Call(uintptr(hwnd), uintptr(msg), wparam, lparam)
	return r
}

func canvasProc(hwnd syscall.Handle, msg uint32, wparam, lparam uintptr) (ret uintptr) {
	defer func() {
		if r := recover(); r != nil {
			logCrash("canvasProc", r)
			ret = 0
		}
	}()
	switch msg {
	case WM_PAINT:
		paintCanvas(hwnd)
		return 0
	case WM_LBUTTONDOWN:
		x := int16(loword(lparam))
		y := int16(hiword(lparam))
		selected = hitElement(float64(x), float64(y))
		if selected >= 0 {
			dragging = true
			sx, sy := canvasToMM(float64(x), float64(y))
			dragDX = sx - elems[selected].x
			dragDY = sy - elems[selected].y
			pSetCapture.Call(uintptr(hwnd))
			pSetFocus.Call(uintptr(hwnd))
		}
		return 0
	case WM_MOUSEMOVE:
		if dragging && selected >= 0 {
			x := int16(loword(lparam))
			y := int16(hiword(lparam))
			mx, my := canvasToMM(float64(x), float64(y))
			e := &elems[selected]
			lw := getFloat(ID_LABEL_W, 15)
			lh := getFloat(ID_LABEL_H, 15)
			mw := maxf(lw, getFloat(ID_MEDIA_W, 26))
			off := labelPhysicalOffset(lw, mw)
			// X хранится относительно левого края наклейки, но двигать можно по всей
			// физической области печати. Поэтому допустимы и отрицательные X.
			minX := -off
			maxX := mw - off - e.w
			maxY := maxf(0, lh-e.h)
			if selected > 0 {
				// Text is positioned by its ^FO anchor. It may extend right/down from
				// that anchor, so do not clamp it by the preview bounding box.
				maxX = mw - off - 0.2
				maxY = lh - 0.2
			}
			e.x = minf(maxX, maxf(minX, mx-dragDX))
			e.y = minf(maxY, maxf(0, my-dragDY))
			suppressControlSync = true
			if selected == 0 {
				setEditFloat(ID_X, e.x)
				setEditFloat(ID_Y, e.y)
			} else {
				setEditFloat(textXID(selected-1), e.x)
				setEditFloat(textYID(selected-1), e.y)
			}
			suppressControlSync = false
			pInvalidateRect.Call(uintptr(hwnd), 0, 1)
		}
		return 0
	case WM_LBUTTONUP:
		if dragging {
			dragging = false
			pReleaseCapture.Call()
		}
		return 0
	}
	r, _, _ := pDefWindowProcW.Call(uintptr(hwnd), uintptr(msg), wparam, lparam)
	return r
}

func labelPhysicalOffset(labelW, mediaW float64) float64 {
	off := getFloat(ID_LABEL_OFFSET, 0)
	if checked(ID_CENTER_LABEL) && mediaW > labelW {
		off = (mediaW - labelW) / 2
	}
	return off
}

func textSizeID(i int) int { return []int{ID_TEXT1_SIZE, ID_TEXT2_SIZE, ID_TEXT3_SIZE}[i] }
func textXID(i int) int    { return []int{ID_TEXT1_X, ID_TEXT2_X, ID_TEXT3_X}[i] }
func textYID(i int) int    { return []int{ID_TEXT1_Y, ID_TEXT2_Y, ID_TEXT3_Y}[i] }
func textEditID(i int) int { return []int{ID_TEXT1, ID_TEXT2, ID_TEXT3}[i] }
func textLineCount(i int) int {
	if i < 0 || i >= len(textCache) {
		return 1
	}
	t := normalizeLabelText(textCache[i])
	if strings.Contains(t, "\n") {
		return 2
	}
	return 1
}
func normalizeLabelText(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	// A trailing newline in Notepad should not create an accidental empty third line.
	s = strings.TrimRight(s, "\n")
	return s
}
func splitTextLines(s string, _ int) []string {