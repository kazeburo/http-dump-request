package main

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"os"
	"path/filepath"
	"strconv"
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

// version by Makefile
var version string

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

func getTemplate(fileName string) (*template.Template, error) {
	indexHTML, err := getFile("index.html")
	if err != nil {
		return nil, err
	}
	return template.New("index").Parse(indexHTML)
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

func colorHTML(name, code string) (*dumpData, error) {
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

func formatHTML(w http.ResponseWriter, r *http.Request, name, code, title string) {
	if strings.Contains(r.URL.RawQuery, "plain") || strings.Index(r.UserAgent(), "curl/") == 0 {
		w.Write([]byte(code))
		return
	}
	dumpMsg, err := colorHTML(name, code)
	if err != nil {
		w.WriteHeader(500)
		w.Write([]byte(err.Error()))
		return
	}
	dumpMsg.Title = title
	indexTmpl, err := getTemplate("index.html")
	if err != nil {
		w.WriteHeader(500)
		w.Write([]byte(err.Error()))
		return
	}
	indexTmpl.Execute(w, dumpMsg)
}

func handleSource(code string) func(w http.ResponseWriter, r *http.Request) {
	dumpMsg, err := colorHTML("Source Code", code)
	if err != nil {
		panic(err)
	}
	dumpMsg.Title = "Source Code"
	indexTmpl, err := getTemplate("index.html")
	if err != nil {
		panic(err)
	}
	var b bytes.Buffer
	indexTmpl.Execute(&b, dumpMsg)

	return func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.RawQuery, "plain") || strings.Index(r.UserAgent(), "curl/") == 0 {
			w.Write([]byte(code))
			return
		}
		w.Write(b.Bytes())
	}
}

func handleDump(w http.ResponseWriter, r *http.Request) {
	dump, err := dumpRequest(r)
	if err != nil {
		w.WriteHeader(500)
		w.Write([]byte(err.Error()))
		return
	}
	formatHTML(w, r, "HTTP", dump, "HTTP request")
}

func handleBasic(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	dump, err := dumpRequest(r)
	if err != nil {
		w.WriteHeader(500)
		w.Write([]byte(err.Error()))
		return
	}

	dumpMsg, err := colorHTML("HTTP", dump)
	if err != nil {
		w.WriteHeader(500)
		w.Write([]byte(err.Error()))
		return
	}
	dumpMsg.Title = "HTTP request for restricted area"

	indexTmpl, err := getTemplate("index.html")
	if err != nil {
		w.WriteHeader(500)
		w.Write([]byte(err.Error()))
		return
	}

	if user, pass, ok := r.BasicAuth(); !ok || user != vars["id"] || pass != vars["pw"] {
		w.Header().Add("WWW-Authenticate", `Basic realm="restricted area"`)
		w.WriteHeader(http.StatusUnauthorized)
	}

	if strings.Contains(r.URL.RawQuery, "plain") || strings.Index(r.UserAgent(), "curl/") == 0 {
		w.Write([]byte(dump))
		return
	}

	indexTmpl.Execute(w, dumpMsg)

}

func handleHello(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("OK\n"))
}

func handleVersion(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("VERSION:" + version + "\n"))
}

func handleWhoami(w http.ResponseWriter, r *http.Request) {
	hostname, err := os.Hostname()
	if err != nil {
		w.WriteHeader(500)
		w.Write([]byte(err.Error()))
		return
	}
	w.Write([]byte(hostname + "\n"))
}

func handleStatus(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	code, err := strconv.Atoi(vars["code"])
	if err != nil {
		w.WriteHeader(500)
		w.Write([]byte(err.Error()))
		return
	}
	msg := http.StatusText(code)
	if msg == "" {
		w.WriteHeader(400)
		w.Write([]byte("Status " + vars["code"] + " is not supported\n"))
		return
	}
	w.WriteHeader(code)
	w.Write([]byte(fmt.Sprintf("%03d %s\n", code, msg)))
}

func handleContentType(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	ct := vars["major"]
	if vars["minor"] != "" {
		ct = ct + "/"
		ct = ct + vars["minor"]
	}
	w.Header().Set("Content-Type", ct)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(fmt.Sprintf("dummy content for content-type: %s\n", ct)))

}

func handleFizzBuzz(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		w.WriteHeader(500)
		w.Write([]byte("expected http.ResponseWriter to be an http.Flusher"))
		return
	}
	for i := 1; i <= 15; i++ {
		p := fmt.Sprintf("#%03d ", i)
		if i%3 == 0 {
			p += "Fizz"
		}
		if i%5 == 0 {
			p += "Buzz"
		}
		p = strings.TrimSpace(p)
		p += "\n"
		w.Write([]byte(p))
		flusher.Flush()
		time.Sleep(300 * time.Millisecond)
	}
}

func dumpRequest(r *http.Request) (string, error) {
	r2 := r.Clone(context.Background())
	if deletedAE := r.Header.Get("Hdr-Accept-Encoding"); deletedAE != "" {
		r2.Header.Set("Accept-Encoding", deletedAE)
		r2.Header.Del("Hdr-Accept-Encoding")
	}
	dump, err := httputil.DumpRequest(r2, true)
	if err != nil {
		return "", err
	}
	return string(dump), nil
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

	g, _ := gziphandler.NewGzipLevelAndMinSize(gzip.DefaultCompression, 5)

	m := mux.NewRouter()
	m.Handle("/live", g(http.HandlerFunc(handleHello)))
	m.Handle("/nogzip/live", http.HandlerFunc(handleHello))

	m.Handle("/version", g((http.HandlerFunc(handleVersion))))
	m.Handle("/nogzip/version", http.HandlerFunc(handleVersion))

	m.Handle("/source", g(http.HandlerFunc(handleSource(source))))
	m.Handle("/nogzip/source", http.HandlerFunc(handleSource(source)))

	m.Handle(`/whoami{dummy:(?:|\.txt)}`, g((http.HandlerFunc(handleWhoami))))
	m.Handle(`/nogzip/whoami{dummy:(?:|\.txt)}`, http.HandlerFunc(handleWhoami))

	m.Handle("/demo/fizzbuzz{dummy:(?:|_stream)}", g(http.HandlerFunc(handleFizzBuzz)))
	m.Handle("/nogzip/demo/fizzbuzz{dummy:(?:|_stream)}", http.HandlerFunc(handleFizzBuzz))

	m.Handle("/demo/basic/{id}/{pw}", g(http.HandlerFunc(handleBasic)))
	m.Handle("/nogzip/demo/basic/{id}/{pw}", http.HandlerFunc(handleBasic))

	m.Handle(`/demo/status/{code:\d{3}}`, g(http.HandlerFunc(handleStatus)))
	m.Handle(`/nogzip/demo/status/{code:\d{3}}`, http.HandlerFunc(handleStatus))

	m.Handle(`/demo/type/{major}`, g(http.HandlerFunc(handleContentType)))
	m.Handle(`/nogzip/demo/type/{major}`, http.HandlerFunc(handleContentType))
	m.Handle(`/demo/type/{major}/{minor}`, g(http.HandlerFunc(handleContentType)))
	m.Handle(`/nogzip/demo/type/{major}/{minor}`, http.HandlerFunc(handleContentType))

	m.Handle("/favicon.ico", g(http.FileServer(statikFS)))
	m.PathPrefix("/nogzip/").Handler(http.HandlerFunc(handleDump))
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
