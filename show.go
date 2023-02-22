package main

import (
	"bufio"
	"errors"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"time"
)

type File struct {
	Name  string
	Usage uint64
	Time  time.Time
}

type Package struct {
	Name  string
	Usage uint64
	Time  time.Time

	Files []File
}

// the zero value of time.Time is some weird number
var zeroTime = time.Unix(0, 0)
var dataFile = "/var/log/spy.db"

func getPackage(pkgname string) (Package, error) {
	pkg := Package{Name: pkgname, Time: zeroTime}

	cmd := exec.Command("spy_list_package_files", pkgname)
	b, err := cmd.Output()
	if err != nil {
		return pkg, fmt.Errorf("command %v failed for package %s: %w", cmd.Path, pkgname, err)
	}
	files := strings.Split(strings.TrimSpace(string(b)), "\n")
	for _, file := range files {
		pkg.Files = append(pkg.Files, File{Name: file, Time: zeroTime})
	}

	return pkg, nil
}

func packageList() []Package {
	cmd := exec.Command("spy_list_packages")
	cmd.Stderr = os.Stderr
	b, err := cmd.Output()
	if err != nil {
		fmt.Fprintln(os.Stderr, cmd.Args)
		os.Exit(1)
	}
	lines := strings.Split(strings.TrimSpace(string(b)), "\n")

	uncachedPkgs := func() []Package {
		pkgs := []Package{}
		// Possible to do concurrently, but might need to limit to only 2
		for _, line := range lines {
			pkg, err := getPackage(line)
			if err != nil {
				continue
			}
			pkgs = append(pkgs, pkg)
		}
		return pkgs
	}

	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return uncachedPkgs()
	}
	cacheDir = cacheDir + "/spy/"
	path := cacheDir + ".spy.db"

	dbfile, err := os.Stat(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return uncachedPkgs()
	}
	datafile, err := os.Stat(dataFile)
	if err != nil || dbfile.ModTime().Unix() < datafile.ModTime().Unix() {
		return uncachedPkgs()
	}

	pkgs := []Package{}
	f, err := os.Open(path)
	if err != nil {
		return uncachedPkgs()
	}
	defer f.Close()
	s := bufio.NewScanner(f)
	sort.Strings(lines)
	for s.Scan() {
		name, usage, time := scanRow(s.Text())
		_, found := sort.Find(len(lines), func(i int) int {
			return strings.Compare(name, lines[i])
		})
		if !found {
			continue
		}

		file, err := os.Open(cacheDir + name)
		if err != nil {
			pkg, err := getPackage(name)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				continue
			}
			pkgs = append(pkgs, pkg)
			continue
		}
		pkg := Package{Name: name, Usage: usage, Time: time}
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			name, usage, time := scanRow(scanner.Text())
			pkg.Files = append(pkg.Files, File{Name: name, Usage: usage, Time: time})
		}
		if scanner.Err() != nil {
			fmt.Fprintln(os.Stderr, err)
			continue
		}

		pkgs = append(pkgs, pkg)
	}
	if s.Err() != nil {
		fmt.Fprintln(os.Stderr, err)
	}

	return pkgs
}

func scanRow(row string) (string, uint64, time.Time) {
	fields := strings.Split(row, "\t")
	if len(fields) != 3 {
		fmt.Fprintf(os.Stderr, "Expected 3 fields got %d\n", len(fields))
		os.Exit(1)
	}
	usage, err := strconv.ParseUint(fields[0], 10, 64)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	t, err := strconv.ParseUint(fields[1], 10, 64)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	return fields[2], usage, time.Unix(int64(t), 0)
}

func filesList(dataFile string) []File {
	f, err := os.Open(dataFile)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer f.Close()
	s := bufio.NewScanner(f)

	files := []File{}

	for s.Scan() {
		name, usage, time := scanRow(s.Text())

		files = append(files, File{
			Name:  name,
			Usage: usage,
			Time:  time})
	}
	if s.Err() != nil {
		fmt.Fprintln(os.Stderr, s.Err())
		os.Exit(1)
	}

	return files
}

const styleCSS = `
table, th, td {
	border: 1px solid #aaa;
	border-collapse: collapse;
	padding: 5px;
}
@media (prefers-color-scheme: dark) {
	body {
		background: #121212;
		color: #fff;
	}
	a {
		color: #1bf;
	}
	a:visited {
		color: #08b;
	}
	a:hover {
		color: #2ef;
		text-decoration: none;
	}
}
table {
	width: 100%;
}
body {
	margin-left: 1em;
	margin-right: 1em;
}
`

var indexJs = `
let colSorted = [false, false, false];
function sortTable(idx) {
	let table = document.querySelector("table");
	let rows = [];
	for (let y = 1; y < table.rows.length; y++) {
		let cols = [];
		for (let x = 0; x < table.rows[y].children.length; x++) {
			cols.push({
				text: table.rows[y].children[x].textContent,
				html: table.rows[y].children[x].innerHTML
			});
		}
		rows.push(cols);
	}
	rows.sort((a, b) => {
		switch (idx) {
		case 0:
			if (colSorted[idx]) {
				return b[idx].text.localeCompare(a[idx].text);
			}
			return a[idx].text.localeCompare(b[idx].text);
		case 1:
			if (colSorted[idx]) {
				return a[idx].text - b[idx].text;
			}
			return b[idx].text - a[idx].text;
		case 2:
			let t1 = 0;
			if (a[idx].text !== '0') {
				t1 = Date.parse(a[idx].text)
			}
			let t2 = 0;
			if (b[idx].text !== '0') {
				t2 = Date.parse(b[idx].text)
			}
			if (colSorted[idx]) {
				return t1 - t2;
			}
			return t2 - t1;
		};
	})
	colSorted[idx] = !colSorted[idx];
	for (let y = 1; y < table.rows.length; y++) {
		for (let x = 0; x < table.rows[y].children.length; x++) {
			table.rows[y].children[x].innerHTML = rows[y-1][x].html;
		}
	}
}
document.getElementById("head1").onclick = function() {
	sortTable(0);
	let div = document.getElementById("sortStatus1");
	if (colSorted[0]) {
		div.textContent = "↑";
	} else {
		div.textContent = "↓";
	}
	colSorted[1] = false;
	document.getElementById("sortStatus2").textContent = "↔";
	colSorted[2] = false;
	document.getElementById("sortStatus3").textContent = "↔";
}
document.getElementById("head2").onclick = function() {
	sortTable(1);
	let div = document.getElementById("sortStatus2")
	if (colSorted[1]) {
		div.textContent = "↑";
	} else {
		div.textContent = "↓";
	}
	colSorted[0] = false;
	document.getElementById("sortStatus1").textContent = "↔";
	colSorted[2] = false;
	document.getElementById("sortStatus3").textContent = "↔";
}
document.getElementById("head3").onclick = function() {
	sortTable(2);
	let div = document.getElementById("sortStatus3")
	if (colSorted[2]) {
		div.textContent = "↑";
	} else {
		div.textContent = "↓";
	}
	colSorted[0] = false;
	document.getElementById("sortStatus1").textContent = "↔";
	colSorted[1] = false;
	document.getElementById("sortStatus2").textContent = "↔";
}
`

const htmlTemplate = `
<!DOCTYPE html>
<html>
	<head>
		<script type="text/javascript" src="/index.js" defer></script>
		<title>{{ .Name }}</title>
		<meta charset="utf-8">
		<link rel="stylesheet" type="text/css" href="/style.css">
	</head>
<body>
{{ if .File }}
<h4><a href="/">index</a></h4>
{{ end }}
<h3> {{ .Name }} </h3>
<table>
<tr><th id="head1">
{{ if .File }}
Files
{{ else }}
Packages
{{ end }}
<div id="sortStatus1">↔</div>
	</th><th id="head2">Times Used<div id="sortStatus2">↔</div></th>
	<th id="head3">Last used<div id="sortStatus3">↔</div></th></tr>
	{{ $File := .File }}
	{{ range .Entries }}
		<tr>
		{{ if $File }}
		<td>{{.Name}}</td>
		{{ else }}
		<td><a href="/{{ .Name }}">{{.Name}}</a></td>
		{{ end}}
		{{ if .Usage }}
		<td>{{ .Usage }}</td><td>{{ .Time.Format "` + time.ANSIC + `" }}</td>
		{{ else }}
		<td>0</td><td>0</td>
		{{ end }}
		</tr>
	{{ end }}
</table>
</body>
</html>
`

type handler struct {
	pkgs  []Package
	files []File
}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var (
		isFile      = false
		name        = "Packages"
		entries any = nil
	)
	switch r.URL.Path {
	case "/style.css":
		w.Header().Set("Content-type", "text/css")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, styleCSS)
		return
	case "/index.js":
		w.Header().Set("Content-type", "text/javascript")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, indexJs)
		return
	case "/", "/index.html":
		sort.Slice(h.pkgs, func(i, j int) bool {
			return h.pkgs[i].Name < h.pkgs[j].Name
		})
		entries = h.pkgs
	default:
		sort.Slice(h.pkgs, func(i, j int) bool {
			return h.pkgs[i].Name < h.pkgs[j].Name
		})
		path := strings.Trim(r.URL.Path, "/")
		i, found := sort.Find(len(h.pkgs), func(i int) int {
			return strings.Compare(path, h.pkgs[i].Name)
		})
		if !found {
			w.WriteHeader(404)
			fmt.Fprintln(w, "not found")
			return
		}
		files := h.pkgs[i].Files
		entries = files
		name = h.pkgs[i].Name
		isFile = true
	}

	t := template.New("spy")
	t, err := t.Parse(htmlTemplate)
	if err != nil {
		log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintln(w, "Internal error")
		return
	}
	err = t.Execute(w, struct {
		Path    string
		Name    string
		File    bool
		Entries any
	}{
		Path:    r.URL.Path,
		Name:    name,
		File:    isFile,
		Entries: entries,
	})
	if err != nil {
		log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintln(w, "Internal error")
		return
	}
}

func writeCacheFiles(pkgs []Package) {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to get cache directory: %v\n", err)
		return
	}
	cacheDir = cacheDir + "/spy/"
	err = os.Mkdir(cacheDir, 0755)
	if err != nil && !errors.Is(err, os.ErrExist) {
		fmt.Fprintln(os.Stderr, err)
		return
	}
	pkgFile, err := os.Create(cacheDir + ".spy.db")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return
	}
	defer pkgFile.Close()
	for _, pkg := range pkgs {
		fmt.Fprintf(pkgFile, "%v\t%v\t%s\n", pkg.Usage,
			pkg.Time.Unix(), pkg.Name)
		f, err := os.Create(cacheDir + pkg.Name)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			continue
		}
		for _, file := range pkg.Files {
			fmt.Fprintf(f, "%v\t%v\t%s\n", file.Usage,
				file.Time.Unix(), file.Name)
		}
		f.Close()
	}
}

func main() {
	if len(os.Args) > 1 {
		dataFile = os.Args[1]
	} else {
		_, err := os.Stat(dataFile)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			fmt.Fprintf(os.Stderr, "usage: %s [datafile]\n", os.Args[0])
			os.Exit(1)
		}
	}
	ch := make(chan []Package)
	go func() {
		ch <- packageList()
	}()

	files := filesList(dataFile)

	sort.Slice(files, func(i, j int) bool { return files[i].Name < files[j].Name })

	pkgs := <-ch

	for _, file := range files {
		for i, pkg := range pkgs {
			j, found := sort.Find(len(pkg.Files), func(i int) int {
				return strings.Compare(file.Name, pkg.Files[i].Name)
			})
			if found {
				pkgs[i].Files[j] = file
				pkgs[i].Usage += file.Usage
				if pkgs[i].Time.Unix() < file.Time.Unix() {
					pkgs[i].Time = file.Time
				}
			}
		}
	}

	writeCacheFiles(pkgs)

	sort.Slice(pkgs, func(i, j int) bool { return pkgs[i].Usage < pkgs[j].Usage })

	http.Handle("/", &handler{pkgs: pkgs, files: files})

	log.Println("Starting host at http://localhost:8000")
	log.Fatal(http.ListenAndServe(":8000", nil))
}
