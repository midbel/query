package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Entry struct {
	Name  string
	Size  int64
	Files []*Entry
}

func (e *Entry) MarshalJSON() ([]byte, error) {
	if len(e.Files) == 0 {
		return e.asFile()
	}
	return e.asDir()
}

func (e *Entry) asFile() ([]byte, error) {
	x := struct {
		Name string `json:"name"`
		Size int64  `json:"size"`
	}{
		Name: e.Name,
		Size: e.Size,
	}
	return json.Marshal(x)
}

func (e *Entry) asDir() ([]byte, error) {
	x := struct {
		Name      string   `json:"name"`
		Directory bool     `json:"directory"`
		Files     []*Entry `json:"files,omitempty"`
	}{
		Name:      e.Name,
		Directory: true,
		Files:     e.Files,
	}
	return json.Marshal(x)
}

func keep(dir , excludes string) bool {
	return !strings.Contains(excludes, dir)
}

func main() {
	names := flag.String("x", "", "list of directories to exclude")
	flag.Parse()
	e, err := walk(flag.Arg(0), "", *names)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if err := json.NewEncoder(os.Stdout).Encode(e); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func walk(dir, parent string, excludes string) (*Entry, error) {
	list, err := os.ReadDir(filepath.Join(parent, dir))
	if err != nil {
		return nil, err
	}
	e := Entry{
		Name: filepath.Base(dir),
	}
	for i := range list {
		if strings.HasPrefix(list[i].Name(), ".") {
			continue
		}
		if list[i].IsDir() {
			if !keep(list[i].Name(), excludes) {
				continue
			}
			n, err := walk(list[i].Name(), filepath.Join(parent, dir), excludes)
			if err != nil {
				return nil, err
			}
			e.Files = append(e.Files, n)
		} else {
			info, err := list[i].Info()
			if err != nil {
				return nil, err
			}
			n := Entry{
				Name: list[i].Name(),
				Size: info.Size(),
			}
			if n.Size > 0 {
				e.Files = append(e.Files, &n)
			}
		}
	}
	return &e, nil
}
