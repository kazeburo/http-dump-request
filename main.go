package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/alecthomas/chroma/formatters/html"
	"github.com/alecthomas/chroma/lexers"
	"github.com/alecthomas/chroma/styles"
	_ "github.com/kazeburo/http-dump-request/statik"

	"github.com/gorilla/mux"
	"github.com/jessevdk/go-flags"
	"github.com/rakyll/statik/fs"
)

type commandOpts struct {
	Listen       string        `short:"l" long:"listen" default:"0.0.0.0" description:"address to bind"`
	Port         string        `short:"p" long:"port" default:"3000" description:"Port number to bind"`
	ReadTimeout  time.Duration `long:"read-timeout" default:"30s" description:"timeout of reading request"`
	WriteTimeout time.Duration `long:"write-timeout" default:"90s" description:"timeout of writing response"`
}

func getTemplate(fileName string) (string, error) {

	filePath := filepath.Join("/", fileName)
	fileSystem, err := fs.New()
	if err != nil {
		return "", err
	}

	file, err := fileSystem.Open(filePath)
	if err != nil {
		return "", err
	}

	defer file.Close()

	fileContent, err := ioutil.ReadAll(file)
	if err != nil {
		return "", err
	}

	return string(fileContent), err
}

type dumpData struct {
	Body  string
	Style string
}

type preWrapper struct {
	styleAttr string
}

func (p *preWrapper) Start(code bool, styleAttr string) string {
	p.styleAttr = styleAttr
	return ""
}

func (p *preWrapper) End(code bool) string {
	return ""
}

func handleDump(w http.ResponseWriter, r *http.Request) {
	dump, err := httputil.DumpRequest(r, true)
	if err != nil {
		w.WriteHeader(500)
		w.Write([]byte(err.Error()))
		return
	}
	if strings.Contains(r.URL.RawQuery, "plain") || strings.Index(r.UserAgent(), "curl/") == 0 {
		w.Write(dump)
		return
	}

	lexer := lexers.Get("HTTP")
	if lexer == nil {
		lexer = lexers.Fallback
	}
	style := styles.Get("monokailight")
	if style == nil {
		style = styles.Fallback
	}
	pwr := &preWrapper{}
	formatter := html.New(html.Standalone(false), html.WithPreWrapper(pwr))

	buf := new(bytes.Buffer)

	it, err := lexer.Tokenise(nil, string(dump))
	if err != nil {
		w.WriteHeader(500)
		w.Write([]byte(err.Error()))
		return
	}
	err = formatter.Format(buf, style, it)
	if err != nil {
		w.WriteHeader(500)
		w.Write([]byte(err.Error()))
		return
	}
	lines := make([]string, 0)
	for k, s := range strings.Split(buf.String(), "\n") {
		lines = append(lines, fmt.Sprintf(`<tr><td>%d</td><td>%s</td></tr>`, k+1, s))
	}

	code := "<pre><table>" + strings.Join(lines, "\n") + "</table></pre>"
	dumpMsg := dumpData{
		Body:  code,
		Style: pwr.styleAttr,
	}
	indexHTML, err := getTemplate("index.html")
	if err != nil {
		w.WriteHeader(500)
		w.Write([]byte(err.Error()))
		return
	}
	indexTmpl, err := template.New("index").Parse(indexHTML)
	if err != nil {
		fmt.Println(err)
		return
	}

	indexTmpl.Execute(w, dumpMsg)
}

func handleHello(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("OK\n"))
}

func main() {
	os.Exit(_main())
}

var opts commandOpts

func _main() int {
	opts = commandOpts{}
	psr := flags.NewParser(&opts, flags.Default)
	_, err := psr.Parse()
	if err != nil {
		os.Exit(1)
	}

	statikFS, err := fs.New()
	if err != nil {
		log.Printf("failed to init fs %v", err)
		return 1
	}

	m := mux.NewRouter()
	m.HandleFunc("/live", handleHello)
	m.Handle("/favicon.ico", http.FileServer(statikFS))
	m.PathPrefix("/").HandlerFunc(handleDump)
	server := http.Server{
		Handler:      m,
		ReadTimeout:  opts.ReadTimeout,
		WriteTimeout: opts.WriteTimeout,
	}
	listen, err := net.Listen("tcp", fmt.Sprintf("%s:%s", opts.Listen, opts.Port))
	if err != nil {
		log.Fatal(err)
	}
	if err := server.Serve(listen); err != http.ErrServerClosed {
		log.Fatal(err)
	}

	return 0
}
