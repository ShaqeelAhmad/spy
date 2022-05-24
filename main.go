package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"git.sr.ht/~emersion/go-scfg"
)

// Use this type to possibly change it in case a larger value is required
type filesCount = int64

type config struct {
	configFile    string
	dbFile        string
	interval      int
	procDir       string
	dataDir       string
	ignoredPrefix []string
}

var debug = true

// Use interface{} because go1.18 is still new
func debugLog(format string, a ...interface{}) {
	if debug {
		t := time.Now()
		fmt.Fprintf(os.Stderr, "[%d/%d/%d  %d:%d:%d] %s\n", t.Year(), t.Month(),
			t.Day(), t.Hour(), t.Minute(), t.Second(),
			fmt.Sprintf(format, a...))
	}
}

func isDigits(str string) bool {
	for _, s := range str {
		if s < '0' || s > '9' {
			return false
		}
	}
	return true
}

var defaultIgnoredPrefix = []string{
	"anon_inode",
	"/memfd",
	"/root",
	"/home",
	"/proc",
	"/dev",
	"/tmp/go-build",
}

func stringPrefixIgnored(s string, ignoredPrefix []string) bool {
	for _, prefix := range ignoredPrefix {
		if strings.HasPrefix(s, prefix) {
			return true
		}
	}
	return false
}

func getFiles(path string, ignoredPrefix []string) []string {
	var filesMap []string
	dir := path + "/map_files"
	files, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	for _, f := range files {
		path := filepath.Join(dir, f.Name())
		path, err := os.Readlink(path)
		if err != nil {
			debugLog("%s: %v", path, err)
			continue
		}
		if stringPrefixIgnored(path, ignoredPrefix) {
			debugLog("Ignoring %s ", path)
			continue
		}
		filesMap = append(filesMap, path)
	}
	return filesMap
}

func fullPath(s string) string {
	if s == "" {
		return ""
	} else if s[0] == '/' {
		return s
	}

	for _, path := range strings.Split(os.Getenv("PATH"), ":") {
		fullpath := filepath.Join(path, s)
		_, err := os.Stat(fullpath)
		if err == nil {
			return fullpath
		}
	}

	return ""
}

func procCommand(s string, ignoredPrefix []string) []string {
	var commands []string

	b, err := os.ReadFile(s + "/cmdline")
	if err != nil {
		return nil
	}
	if len(b) == 0 {
		return nil
	}
	args := strings.Split(strings.TrimSuffix(string(b), "\000"), "\000")
	if len(args) <= 1 {
		return nil
	}
	cmd := fullPath(args[0])
	cmd = filepath.Clean(cmd)
	if cmd != "." && cmd != "/" && !stringPrefixIgnored(cmd, ignoredPrefix) {
		commands = append(commands, cmd)
	}

	// Handle shebang scripts
	switch filepath.Base(args[0]) {
	case "python", "dash", "sh", "bash", "zsh", "perl", "awk":
		for i := 1; i < len(args); i++ {
			path := fullPath(args[i])
			if path == "" || stringPrefixIgnored(path, ignoredPrefix) {
				continue
			}
			path = filepath.Clean(path)
			if path == "." || path == "/" {
				continue
			}
			commands = append(commands, path)
		}
	}

	return commands
}

func updateData(filesMap map[string]filesCount, ignoredPrefix []string, procDir string) map[string]filesCount {
	procFiles, err := os.ReadDir(procDir)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	for _, f := range procFiles {
		if f.IsDir() && isDigits(f.Name()) {
			processDir := procDir + "/" + f.Name()
			for _, s := range procCommand(processDir, ignoredPrefix) {
				s = filepath.Clean(s)
				if s == "." || s == "/" {
					continue
				}
				filesMap[s]++
			}
			for _, s := range getFiles(processDir, ignoredPrefix) {
				s = filepath.Clean(s)
				if s == "." || s == "/" {
					continue
				}
				filesMap[s]++
			}
		}
	}
	return filesMap
}

func writeData(filesMap map[string]filesCount, filename string) {
	var f *os.File
	var err error
	if filename == "-" {
		f = os.Stdout
	} else {
		f, err = os.Create(filename)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		defer f.Close()
	}

	for i, j := range filesMap {
		i = strings.ReplaceAll(i, `\`, `\\`)
		i = strings.ReplaceAll(i, `"`, `\"`)
		i = strings.ReplaceAll(i, "\n", `\n`)
		i = strings.ReplaceAll(i, "\t", `\t`)
		fmt.Fprintf(f, "\"%s\" %d\n", i, j)
	}
}

func collect(conf config) {
	os.MkdirAll(filepath.Dir(conf.dbFile), 0755)

	quit := make(chan os.Signal, 1)
	reload := make(chan os.Signal)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	signal.Notify(reload, syscall.SIGHUP)

	locked := false // avoid quitting when writing to files

	go func() {
		for range reload {
			conf = getConfig(conf.configFile)
			debugLog("Reloaded config from %s", conf.configFile)
		}
	}()
	go func() {
		<-quit
		fmt.Println("Interrupted... Exiting.")
		for locked {
			// wait until we have finished writing to files
		}
		os.Exit(1)
	}()

	var filesMap map[string]filesCount
	if _, err := os.Stat(conf.dbFile); errors.Is(err, os.ErrNotExist) ||
		conf.dbFile == "-" {
		filesMap = make(map[string]filesCount)
	} else {
		var err error
		filesMap, err = parseScfgDBFile(conf.dbFile)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Failed to parse db file ", err)
			os.Exit(1)
		}
	}

	if conf.interval < 0 {
		writeData(updateData(filesMap, conf.ignoredPrefix, conf.procDir), conf.dbFile)
		os.Exit(0)
	}

	for {
		filesMap = updateData(filesMap, conf.ignoredPrefix, conf.procDir)

		locked = true
		writeData(filesMap, conf.dbFile)
		locked = false

		time.Sleep(time.Second * time.Duration(conf.interval))
	}
}

func listPackages() []string {
	cmd := exec.Command("spy-list_packages")
	list, err := cmd.Output()
	if err != nil {
		fmt.Fprintln(os.Stderr, cmd.Args, err)
		return nil
	}

	return strings.Split(strings.TrimSpace(string(list)), "\n")
}

func listFilesForPackage(pkg string) []string {
	cmd := exec.Command("spy-list_package_files", pkg)

	list, err := cmd.Output()
	if err != nil {
		fmt.Fprintf(os.Stderr, "command `%v` failed for package: %s : %v\n",
			cmd.Args[0], pkg, err)
		return nil
	}

	return strings.Split(strings.TrimSpace(string(list)), "\n")
}

func updatePkgData(dir string, dbFile string) {
	os.MkdirAll(dir, 0755)

	pkgList := listPackages()
	filesMap, err := parseScfgDBFile(dbFile)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Failed to parse db file ", err)
		os.Exit(1)
	}

	for _, pkg := range pkgList {
		files := listFilesForPackage(pkg)
		f, err := os.Create(filepath.Join(dir, pkg))
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			continue
		}
		for _, file := range files {
			fmt.Fprintln(f, filesMap[file], file)
		}
		f.Close()
	}
}

func parseWhileInt(s string) (filesCount, error) {
	i := 0
	var r rune
	for i, r = range s {
		if r < '0' || r > '9' {
			break
		}
		i++
	}
	return strconv.ParseInt(s[:i], 10, 64)
}

func showData(dir string) {
	files, err := os.ReadDir(dir)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	var freqs []filesCount
	var pkgs []string
	for _, file := range files {
		var freq filesCount
		var pkg string

		pkg = file.Name()
		f, err := os.Open(filepath.Join(dir, file.Name()))
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			continue
		}
		s := bufio.NewScanner(f)
		for s.Scan() {
			v, err := parseWhileInt(s.Text())
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				continue
			}
			freq += v
		}
		if s.Err() != nil {
			fmt.Fprintln(os.Stderr, err)
			continue
		}
		pkgs = append(pkgs, pkg)
		freqs = append(freqs, freq)
	}
	if len(pkgs) != len(freqs) {
		panic("Unreachable")
	}
	for i, pkgname := range pkgs {
		fmt.Printf("%-8d %s\n", freqs[i], pkgname)
	}
}

func userDataDir() string {
	var dir string
	switch runtime.GOOS {
	case "windows", "darwin", "ios":
		fmt.Fprintln(os.Stderr,
			"Error: windows, darwin and ios are not supported")
		os.Exit(1)
	case "plan9":
		dir = os.Getenv("home")
		if dir == "" {
			fmt.Fprintln(os.Stderr, "Error: $home is not defined")
		}
		dir += "/lib"
	default:
		dir = os.Getenv("XDG_DATA_HOME")
		if dir == "" {
			dir = os.Getenv("HOME")
			if dir == "" {
				fmt.Fprintln(os.Stderr,
					"Error: Both $XDG_DATA_HOME and $HOME are not set")
				os.Exit(1)
			}
			dir += "/.local/share"
		}
	}
	return dir
}

func getConfig(file string) config {
	oldFile := file
	if file == "" {
		var err error
		file, err = os.UserConfigDir()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		file = file + "/spy/config"
	}
	conf, err := parseScfgConfigFile(file)
	if err != nil {
		// only ignored when using default config path and the file not
		// existing
		if !(oldFile == "" && errors.Is(err, os.ErrNotExist)) {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}

	return conf
}

func parseScfgConfigFile(file string) (config, error) {
	dataDir := userDataDir()

	// default config
	conf := config{
		configFile:    file,
		dbFile:        dataDir + "/db",
		interval:      2,
		procDir:       "/proc",
		dataDir:       dataDir,
		ignoredPrefix: defaultIgnoredPrefix,
	}

	blocks, err := scfg.Load(file)
	if err != nil {
		return conf, fmt.Errorf("scfg: %w", err)
	}
	for _, block := range blocks {
		switch block.Name {
		case "interval":
			if len(block.Params) != 1 {
				fmt.Fprintf(os.Stderr, "Error: expected 1 param got %d\n",
					len(block.Params))
				continue
			}
			val, err := strconv.Atoi(block.Params[0])
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				continue
			}
			conf.interval = val
			debugLog("[CONFIG] Set interval to %v", val)
		case "procDir":
			if len(block.Params) != 1 {
				fmt.Fprintf(os.Stderr, "Error: expected 1 param got %d\n",
					len(block.Params))
				continue
			}
			dirname := block.Params[0]
			stat, err := os.Stat(dirname)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
			if !stat.IsDir() {
				fmt.Fprintf(os.Stderr, "Error: %s is not a directory\n", dirname)
				os.Exit(1)
			}
			conf.procDir = dirname
			debugLog("[CONFIG] Set procDir to %s", dirname)
		case "ignoredPrefix":
			var prefixes []string
			for _, prefix := range block.Children {
				prefixes = append(prefixes, prefix.Name)
			}
			conf.ignoredPrefix = prefixes
			debugLog("[CONFIG] Set ignoredPrefix to %v", prefixes)
		case "dbFile":
			if len(block.Params) != 1 {
				fmt.Fprintf(os.Stderr, "Error: expected 1 parameter got %d\n",
					len(block.Params))
				continue
			}
			conf.dbFile = block.Params[0]
			debugLog("[CONFIG] Set dbFile to %v", conf.dbFile)
		default:
			fmt.Fprintf(os.Stderr, "Ignoring %s, unrecognized directive\n", block.Name)
		}
	}
	return conf, nil
}

func parseScfgDBFile(file string) (map[string]filesCount, error) {
	filesMap := make(map[string]filesCount)

	blocks, err := scfg.Load(file)
	if err != nil {
		return nil, fmt.Errorf("scfg: %w", err)
	}

	for _, block := range blocks {
		if len(block.Params) != 1 {
			fmt.Fprintf(os.Stderr, "Error: expected 1 parameter but got %d for %s\n",
				len(block.Params), block.Name)
			continue
		}
		if block.Children != nil {
			fmt.Fprintf(os.Stderr, "Error: Unexpected children for %s, expected none\n",
				block.Name)
			continue
		}
		file, err := strconv.ParseInt(block.Params[0], 10, 64)
		if err != nil {
			fmt.Fprintln(os.Stderr, block.Name, err)
			continue
		}
		filesMap[block.Name] = file
	}

	return filesMap, nil
}

func usage(f *os.File) {
	fmt.Fprintln(f, "Usage: spy [options] <command>")
	fmt.Fprintln(f, "command: ")
	fmt.Fprintln(f, "\tcollect   collect information for all active processes")
	fmt.Fprintln(f, "\tupdate    update the database")
	fmt.Fprintln(f, "\tshow      show the data for all the packages")
	fmt.Fprintln(f, "\thelp      show this help")
	fmt.Fprintln(f, "")
	fmt.Fprintln(f, "options:")
	flag.PrintDefaults()
	fmt.Fprintln(f, "For more information lookup the manpage with `man spy`")
}

func main() {
	configFile := ""
	flag.StringVar(&configFile, "c", "", "Specify a config file")
	flag.BoolVar(&debug, "d", false, "Enable debugging")
	help := false
	flag.BoolVar(&help, "h", false, "Show this help")

	flag.Parse()
	if help {
		usage(os.Stdout)
		os.Exit(0)
	}

	if flag.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "Error: not enough arguments")
		usage(os.Stderr)
		os.Exit(1)
	}
	args := flag.Args()

	switch args[0] {
	case "collect":
		conf := getConfig(configFile)
		collect(conf)
	case "show":
		conf := getConfig(configFile)
		showData(conf.dataDir + "/data")
	case "update":
		conf := getConfig(configFile)
		updatePkgData(conf.dataDir+"/data", conf.dbFile)
	case "help":
		usage(os.Stdout)
	default:
		usage(os.Stderr)
		os.Exit(1)
	}
}
