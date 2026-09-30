package main

import (
	"fmt"
	"os"
	"path/filepath"

	"ccs/internal/session"
	"ccs/internal/tui"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "ccs:", err)
		os.Exit(1)
	}
}

func run() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	root := filepath.Join(home, ".claude", "projects")
	current := session.Load([]string{filepath.Join(root, session.EncodeDir(cwd))})
	loadAll := func() []session.Session { return session.Load(session.AllDirs(root)) }

	hidden, err := session.LoadHidden(filepath.Join(home, ".config", "ccs", "hidden"))
	if err != nil {
		return err
	}
	res, err := tui.Run(current, filepath.Base(cwd), loadAll, hidden)
	if err := hidden.Save(); err != nil {
		fmt.Fprintln(os.Stderr, "ccs:", err)
	}
	if err != nil {
		return err
	}
	if res != nil {
		fmt.Printf("%s\t%s\n", res.Cwd, res.ID)
	}
	return nil
}
