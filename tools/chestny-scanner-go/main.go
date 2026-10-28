package main

import (
    "bufio"
    "bytes"
    "encoding/hex"
    "fmt"
    "os"
    "os/exec"
    "path/filepath"
    "sort"
    "strings"
    "sync"
    "syscall"

    "github.com/lxn/walk"
    . "github.com/lxn/walk/declarative"
)

const gs = byte(29)

type codeItem struct {
    Raw, File, GTIN, Serial, Key91, Crypto92, Err string
    Page int
}

var (
    mw *walk.MainWindow
    outTE *walk.TextEdit
    statusLbl *walk.Label
    countLbl *walk.Label
    exportBtn *walk.PushButton
    items []codeItem
    mu sync.Mutex
    busy bool
)

func main() {
    MainWindow{
        AssignTo: &mw,
        Title: "Честный знак — сканер кодов маркировки 1.3.2",
        MinSize: Size{900, 560},
        Size: Size{1120, 700},
        Layout: VBox{},
        OnDropFiles: func(files []string) { startScan(files) },
        Children: []Widget{
            Label{Text: "PDF / PNG / JPG → полный код маркировки → CSV для отчёта о нанесении", Font: Font{Family:"Segoe UI Semibold", PointSize:13}},
            Label{Text: "Перетащите файлы в окно или нажмите «Открыть файлы». GS в CSV сохраняется как реальный ASCII 29."},
            Composite{Layout: HBox{}, Children: []Widget{
                PushButton{Text:"Открыть файлы", OnClicked: openFiles},
                PushButton{AssignTo:&exportBtn, Text:"Сохранить CSV", Enabled:false, OnClicked: exportCSV},
                PushButton{Text:"Очистить", OnClicked: clearAll},
            }},
            Label{AssignTo:&countLbl, Text:"Кодов: 0"},
            TextEdit{AssignTo:&outTE, ReadOnly:true, VScroll:true, HScroll:true, Font:Font{Family:"Consolas", PointSize:9}},
            Label{AssignTo:&statusLbl, Text:"Готово"},
        },
    }.Run()
}

func openFiles() {
    if busy { return }
    dlg := new(walk.FileDialog)
    dlg.Title = "Выберите PDF или изображения"
    dlg.Filter = "PDF и изображения (*.pdf;*.png;*.jpg;*.jpeg;*.bmp;*.tif;*.tiff)|*.pdf;*.png;*.jpg;*.jpeg;*.bmp;*.tif;*.tiff|Все файлы (*.*)|*.*"
    ok, err := dlg.ShowOpenMultiple(mw)
    if err != nil { walk.MsgBox(mw,"Ошибка",err.Error(),walk.MsgBoxIconError); return }
    if ok { startScan(dlg.FilePaths) }
}

func startScan(paths []string) {
    if busy || len(paths)==0 { return }
    busy=true
    exportBtn.SetEnabled(false)
    statusLbl.SetText("Сканирование...")
    go func(){
        var errs []string
        for _, p := range paths {
            if _, err := os.Stat(p); err != nil { continue }
            mw.Synchronize(func(){ statusLbl.SetText("Сканирование: "+filepath.Base(p)) })
            found, err := scanFile(p)
            if err != nil { errs=append(errs, filepath.Base(p)+": "+err.Error()); continue }
            mu.Lock(); mergeItems(found); mu.Unlock()
            mw.Synchronize(refreshUI)
        }
        mw.Synchronize(func(){
            busy=false
            refreshUI()
            if len(errs)>0 { statusLbl.SetText("Готово, есть ошибки"); walk.MsgBox(mw,"Часть файлов не обработана",strings.Join(errs,"\\r\\n"),walk.MsgBoxIconWarning) } else { statusLbl.SetText("Готово") }
        })
    }()
}

func scanFile(path string) ([]codeItem,error) {
    ext:=strings.ToLower(filepath.Ext(path))
    if ext==".pdf" { return scanPDF(path) }
    switch ext {case ".png", ".jpg", ".jpeg", ".bmp", ".tif", ".tiff": return scanImage(path,1)}
    return nil, fmt.Errorf("неподдерживаемый формат %s",ext)
}

func appDir() string { p,_:=os.Executable(); return filepath.Dir(p) }
func hiddenCmd(name string,args ...string)*exec.Cmd{ c:=exec.Command(name,args...); c.SysProcAttr=&syscall.SysProcAttr{HideWindow:true,CreationFlags:0x08000000}; return c }

func scanPDF(path string)([]codeItem,error){
    tool:=filepath.Join(appDir(),"tools","poppler","Library","bin","pdftoppm.exe")
    if _,err:=os.Stat(tool); err!=nil { return nil,fmt.Errorf("не найден встроенный PDF-движок") }
    tmp,err:=os.MkdirTemp("","czscan-"); if err!=nil{return nil,err}; defer os.RemoveAll(tmp)
    prefix:=filepath.Join(tmp,"page")
    cmd:=hiddenCmd(tool,"-png","-r","300",path,prefix)
    var se bytes.Buffer; cmd.Stderr=&se
    if err:=cmd.Run(); err!=nil{return nil,fmt.Errorf("ошибка PDF: %v %s",err,se.String())}
    files,_:=filepath.Glob(prefix+"-*.png"); sort.Strings(files)
    var all []codeItem
    for i,f:=range files{ a,e:=scanImage(f,i+1); if e==nil { for k:=range a {a[k].File=filepath.Base(path)}; all=append(all,a...) } }
    if len(all)==0{return nil,fmt.Errorf("DataMatrix не найден")}
    return all,nil
}

func scanImage(path string,page int)([]codeItem,error){
    tool:=filepath.Join(appDir(),"tools","zxing","ZXingReader.exe")
    if _,err:=os.Stat(tool); err!=nil{return nil,fmt.Errorf("не найден встроенный сканер DataMatrix")}
    cmd:=hiddenCmd(tool,"-formats","datamatrix",path)
    var out,se bytes.Buffer; cmd.Stdout=&out; cmd.Stderr=&se
    _=cmd.Run()
    var found []codeItem
    sc:=bufio.NewScanner(bytes.NewReader(out.Bytes()))
    for sc.Scan(){
        line:=strings.TrimSpace(sc.Text())
        if !strings.HasPrefix(line,"Bytes:"){continue}
        hx:=strings.TrimSpace(strings.TrimPrefix(line,"Bytes:")); hx=strings.ReplaceAll(hx," ",""); hx=strings.ReplaceAll(hx,"0x","")
        raw,err:=hex.DecodeString(hx); if err!=nil || len(raw)==0 {continue}
        found=append(found,parseCode(string(raw),filepath.Base(path),page))
    }
    if len(found)==0 { return nil,fmt.Errorf("DataMatrix не найден или не прочитан: %s", strings.TrimSpace(se.String())) }
    return found,nil
}

func parseCode(raw,file string,page int) codeItem{
    c:=codeItem{Raw:raw,File:file,Page:page}
    b:=[]byte(raw)
    if len(b)>=3 && string(b[:3])=="]d2" {b=b[3:]; c.Raw=string(b)}
    if len(b)>0 && b[0]==232 {b=b[1:]; c.Raw=string(b)}
    if len(b)<18 || string(b[:2])!="01" {c.Err="нет AI 01"; return c}
    c.GTIN=string(b[2:16]); for _,x:=range []byte(c.GTIN){if x<'0'||x>'9'{c.Err="GTIN должен быть 14 цифр";return c}}
    if string(b[16:18])!="21"{c.Err="нет AI 21";return c}
    p:=bytes.IndexByte(b[18:],gs); if p<0{c.Err="нет GS после 21";return c}; p+=18
    c.Serial=string(b[18:p]); pos:=p+1
    if len(b)<pos+2 || string(b[pos:pos+2])!="91"{c.Err="нет AI 91";return c}; pos+=2
    q:=bytes.IndexByte(b[pos:],gs); if q<0{c.Err="нет второго GS";return c}; q+=pos
    c.Key91=string(b[pos:q]); if len(c.Key91)!=4{c.Err="AI 91 не 4 символа";return c}; pos=q+1
    if len(b)<pos+2 || string(b[pos:pos+2])!="92"{c.Err="нет AI 92";return c}; pos+=2
    c.Crypto92=string(b[pos:]); if len([]byte(c.Crypto92))!=44{c.Err=fmt.Sprintf("AI 92: %d вместо 44 символов",len([]byte(c.Crypto92)));return c}
    return c
}

func mergeItems(in []codeItem){
    seen:=map[string]bool{}; for _,x:=range items{seen[x.Raw]=true}
    for _,x:=range in{if !seen[x.Raw]{items=append(items,x);seen[x.Raw]=true}}
}

func refreshUI(){
    mu.Lock(); defer mu.Unlock()
    var sb strings.Builder; ok:=0; bad:=0
    for i,x:=range items{
        st:="OK"; if x.Err!=""{st="ОШИБКА";bad++}else{ok++}
        fmt.Fprintf(&sb,"%03d  %-6s  %s  стр.%d\\r\\n",i+1,st,x.File,x.Page)
        fmt.Fprintf(&sb,"     GTIN: %s   21: %s   91: %s\\r\\n",x.GTIN,x.Serial,x.Key91)
        if x.Err!=""{fmt.Fprintf(&sb,"     %s\\r\\n",x.Err)} else {fmt.Fprintf(&sb,"     92: %s\\r\\n",x.Crypto92)}
        sb.WriteString("\\r\\n")
    }
    outTE.SetText(sb.String()); countLbl.SetText(fmt.Sprintf("Кодов: %d    корректных: %d    ошибок: %d",len(items),ok,bad)); exportBtn.SetEnabled(!busy && ok>0)
}

func clearAll(){ if busy{return}; mu.Lock(); items=nil; mu.Unlock(); refreshUI(); statusLbl.SetText("Готово") }

func exportCSV(){
    mu.Lock(); defer mu.Unlock()
    dlg:=new(walk.FileDialog); dlg.Title="Сохранить CSV"; dlg.Filter="CSV (*.csv)|*.csv"; dlg.FilePath="КМ_для_отчета_о_нанесении.csv"
    ok,err:=dlg.ShowSave(mw); if err!=nil||!ok{return}
    if !strings.EqualFold(filepath.Ext(dlg.FilePath), ".csv") { dlg.FilePath += ".csv" }
    var buf bytes.Buffer; n:=0
    for _,x:=range items{if x.Err==""{buf.WriteString(x.Raw);buf.WriteByte(13);buf.WriteByte(10);n++}}
    if err:=os.WriteFile(dlg.FilePath,buf.Bytes(),0644);err!=nil{walk.MsgBox(mw,"Ошибка",err.Error(),walk.MsgBoxIconError);return}
    statusLbl.SetText(fmt.Sprintf("CSV сохранён: %d кодов",n))
}
