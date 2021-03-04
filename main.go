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

	"github.com/NYTimes/gziphandler"
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

func getFile(fileName string) (string, error) {

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
	Title string
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

func formatHTML(name, code string) (*dumpData, error) {
	lexer := lexers.Get(name)
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

	it, err := lexer.Tokenise(nil, code)
	if err != nil {
		return nil, err
	}
	err = formatter.Format(buf, style, it)
	if err != nil {
		return nil, err
	}
	lines := make([]string, 0)
	for k, s := range strings.Split(buf.String(), "\n") {
		lines = append(lines, fmt.Sprintf(`<tr><td>%d</td><td>%s</td></tr>`, k+1, s))
	}

	body := "<pre><table>" + strings.Join(lines, "\n") + "</table></pre>"
	dumpMsg := &dumpData{
		Body:  body,
		Style: pwr.styleAttr,
	}
	return dumpMsg, nil
}

func handleSource(code string) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.RawQuery, "plain") || strings.Index(r.UserAgent(), "curl/") == 0 {
			w.Write([]byte(code))
			return
		}
		dumpMsg, err := formatHTML("Go", string(code))
		if err != nil {
			w.WriteHeader(500)
			w.Write([]byte(err.Error()))
			return
		}
		dumpMsg.Title = "Souce code"
		indexHTML, err := getFile("index.html")
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

	dumpMsg, err := formatHTML("HTTP", string(dump))
	if err != nil {
		w.WriteHeader(500)
		w.Write([]byte(err.Error()))
		return
	}
	dumpMsg.Title = "HTTP request"

	indexHTML, err := getFile("index.html")
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

func handleChunk(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		w.WriteHeader(500)
		w.Write([]byte("expected http.ResponseWriter to be an http.Flusher"))
		return
	}
	w.WriteHeader(http.StatusOK)
	flusher.Flush()
	for i := 1; i <= 7; i++ {
		w.Write([]byte(fmt.Sprintf("Chunk #%d\n", i)))
		flusher.Flush()
		time.Sleep(500 * time.Millisecond)
	}
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

	source, err := getFile("main.go")
	if err != nil {
		log.Printf("failed to read main.go %v", err)
		return 1
	}

	g, _ := gziphandler.NewGzipLevelAndMinSize(6, 5)

	m := mux.NewRouter()
	m.Handle("/live", g(http.HandlerFunc(handleHello)))
	m.Handle("/source", g(http.HandlerFunc(handleSource(source))))
	m.Handle("/chunk", g(http.HandlerFunc(handleChunk)))
	m.Handle("/favicon.ico", http.FileServer(statikFS))
	m.PathPrefix("/").Handler(g(http.HandlerFunc(handleDump)))
	server := http.Server{
		Handler:      m,
		ReadTimeout:  opts.ReadTimeout,
		WriteTimeout: opts.WriteTimeout,
	}
	// server.SetKeepAlivesEnabled(false)
	listen, err := net.Listen("tcp", fmt.Sprintf("%s:%s", opts.Listen, opts.Port))
	if err != nil {
		log.Fatal(err)
	}
	if err := server.Serve(listen); err != http.ErrServerClosed {
		log.Fatal(err)
	}

	return 0
}
