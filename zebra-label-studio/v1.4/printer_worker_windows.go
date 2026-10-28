//go:build windows

package main

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"strings"
	"syscall"
	"unsafe"
)

var (
	winspool          = syscall.NewLazyDLL("winspool.drv")
	pOpenPrinterW     = winspool.NewProc("OpenPrinterW")
	pClosePrinter     = winspool.NewProc("ClosePrinter")
	pStartDocPrinterW = winspool.NewProc("StartDocPrinterW")
	pEndDocPrinter    = winspool.NewProc("EndDocPrinter")
	pStartPagePrinter = winspool.NewProc("StartPagePrinter")
	pEndPagePrinter   = winspool.NewProc("EndPagePrinter")
	pWritePrinter     = winspool.NewProc("WritePrinter")
)

type DOC_INFO_1 struct{ pDocName, pOutputFile, pDatatype *uint16 }

func w(s string) *uint16 { p, _ := syscall.UTF16PtrFromString(s); return p }

func main() {
	if len(os.Args) != 4 {
		os.Exit(2)
	}
	printer := strings.TrimSpace(os.Args[1])
	queuePath := os.Args[2]
	progressPath := os.Args[3]
	if printer == "" {
		fail("имя принтера не указано")
	}

	qf, err := os.Open(queuePath)
	if err != nil {
		fail(err.Error())
	}
	defer qf.Close()
	pf, err := os.OpenFile(progressPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		fail(err.Error())
	}
	defer pf.Close()
	pw := bufio.NewWriterSize(pf, 4096)

	var hp syscall.Handle
	r, _, e := pOpenPrinterW.Call(uintptr(unsafe.Pointer(w(printer))), uintptr(unsafe.Pointer(&hp)), 0)
	if r == 0 {
		fail(fmt.Sprintf("OpenPrinter: %v", e))
	}
	defer pClosePrinter.Call(uintptr(hp))

	di := DOC_INFO_1{pDocName: w("MSK Zebra Label Studio"), pDatatype: w("RAW")}
	job, _, e := pStartDocPrinterW.Call(uintptr(hp), 1, uintptr(unsafe.Pointer(&di)))
	if job == 0 {
		fail(fmt.Sprintf("StartDocPrinter: %v", e))
	}
	defer pEndDocPrinter.Call(uintptr(hp))
	if rr, _, pe := pStartPagePrinter.Call(uintptr(hp)); rr == 0 {
		fail(fmt.Sprintf("StartPagePrinter: %v", pe))
	}
	defer pEndPagePrinter.Call(uintptr(hp))

	br := bufio.NewReaderSize(qf, 1024*1024)
	hdr := make([]byte, 8)
	for {
		_, er := io.ReadFull(br, hdr)
		if er == io.EOF {
			break
		}
		if er != nil {
			fail("повреждена очередь печати: " + er.Error())
		}
		idx := int(int32(binary.LittleEndian.Uint32(hdr[0:4])))
		n := int(binary.LittleEndian.Uint32(hdr[4:8]))
		if n <= 0 || n > 16*1024*1024 {
			fail(fmt.Sprintf("некорректный размер записи: %d", n))
		}
		payload := make([]byte, n)
		if _, er = io.ReadFull(br, payload); er != nil {
			fail("повреждена запись ZPL: " + er.Error())
		}
		var wr uint32
		rr, _, we := pWritePrinter.Call(uintptr(hp), uintptr(unsafe.Pointer(&payload[0])), uintptr(len(payload)), uintptr(unsafe.Pointer(&wr)))
		if rr == 0 || int(wr) != len(payload) {
			fail(fmt.Sprintf("WritePrinter: %v (%d/%d)", we, wr, len(payload)))
		}
		// Persist every successful label. This is deliberately per-label rather
		// than per-batch: after a crash/restart the GUI can resume from the first
		// index that was not confirmed as accepted by the Windows print queue.
		if _, er = fmt.Fprintf(pw, "%d\n", idx); er != nil {
			fail("журнал прогресса: " + er.Error())
		}
		if er = pw.Flush(); er != nil {
			fail("журнал прогресса: " + er.Error())
		}
		if er = pf.Sync(); er != nil {
			fail("журнал прогресса: " + er.Error())
		}
	}
}

func fail(s string) { fmt.Fprintln(os.Stderr, s); os.Exit(1) }