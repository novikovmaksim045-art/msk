//go:build windows

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"syscall"
	"unicode/utf16"
	"unsafe"
)

var pfWinspool = syscall.NewLazyDLL("winspool.drv")
var pfEnumPrintersW = pfWinspool.NewProc("EnumPrintersW")

const (
	pfPrinterEnumLocal       = 0x00000002
	pfPrinterEnumConnections = 0x00000004
)

type pfPrinterInfo4 struct {
	PrinterName *uint16
	ServerName  *uint16
	Attributes  uint32
}

func pfUTF16String(p *uint16) string {
	if p == nil {
		return ""
	}
	a := make([]uint16, 0, 64)
	for i := uintptr(0); ; i++ {
		v := *(*uint16)(unsafe.Pointer(uintptr(unsafe.Pointer(p)) + i*2))
		if v == 0 {
			break
		}
		a = append(a, v)
	}
	return string(utf16.Decode(a))
}

func main() {
	flags := uintptr(pfPrinterEnumLocal | pfPrinterEnumConnections)
	var needed, returned uint32
	pfEnumPrintersW.Call(flags, 0, 4, 0, 0, uintptr(unsafe.Pointer(&needed)), uintptr(unsafe.Pointer(&returned)))
	if needed == 0 {
		_ = json.NewEncoder(os.Stdout).Encode([]string{})
		return
	}
	buf := make([]byte, needed)
	r, _, e := pfEnumPrintersW.Call(flags, 0, 4, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)), uintptr(unsafe.Pointer(&needed)), uintptr(unsafe.Pointer(&returned)))
	if r == 0 {
		fmt.Fprintln(os.Stderr, "EnumPrinters:", e)
		os.Exit(1)
	}
	names := make([]string, 0)
	if returned > 0 {
		arr := unsafe.Slice((*pfPrinterInfo4)(unsafe.Pointer(&buf[0])), int(returned))
		seen := map[string]bool{}
		for _, pi := range arr {
			n := strings.TrimSpace(pfUTF16String(pi.PrinterName))
			l := strings.ToLower(n)
			if n == "" || (!strings.Contains(l, "zebra") && !strings.Contains(l, "zdesigner")) {
				continue
			}
			if !seen[n] {
				seen[n] = true
				names = append(names, n)
			}
		}
	}
	if err := json.NewEncoder(os.Stdout).Encode(names); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}