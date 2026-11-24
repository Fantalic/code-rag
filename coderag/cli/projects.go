package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/coderag/projects"
	"github.com/coderag/registry"
	"github.com/urfave/cli/v2"
)

// ── projects subcommand ───────────────────────────────────────────────────

func projectsListCmd(c *cli.Context) error {
	reg := registry.New(registry.DefaultPath())
	focused, _ := reg.Focused()
	items := reg.List()
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tNAME\tKIND\tSCANS\tLAST STATUS\tLAST SCAN\tPATH")
	for _, p := range items {
		last := time.Unix(p.LastScanAt, 0).Format("2006-01-02 15:04")
		if p.LastScanAt == 0 {
			last = "-"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%s\t%s\t%s\n",
			p.ID, p.Name, p.Kind, p.ScanCount, p.LastStatus, last, p.Path)
	}
	w.Flush()
	if focused.ID != "" {
		fmt.Printf("\n* focused: %s (%s)\n", focused.Name, focused.ID)
	} else {
		fmt.Println("\n(no focused project; run `coderag projects focus <path|id>`)")
	}
	return nil
}

func projectsFocusCmd(c *cli.Context) error {
	target := strings.TrimSpace(c.Args().First())
	if target == "" {
		return fmt.Errorf("usage: coderag projects focus <path-or-id>")
	}
	reg := registry.New(registry.DefaultPath())

	// Resolve as path (derive id) or as existing id.
	var id string
	if existing, ok := reg.Get(target); ok {
		id = existing.ID
	} else if proj := findProjectByPath(reg, target); proj.ID != "" {
		id = proj.ID
	} else {
		// Treat as path: derive id and create an unsaved entry only if needed.
		id = registry.NewID(target)
	}
	if err := reg.SetFocused(id); err != nil {
		return err
	}
	// Best-effort mirror push.
	base := c.String("url")
	if base == "" {
		base = os.Getenv("LIGHTRAG_URL")
	}
	if base != "" {
		pc := projects.New(base)
		defer pc.Close()
		if err := pc.FocusPOST(id); err != nil {
			fmt.Printf("Warning: could not sync focus to server: %v\n", err)
		}
	}
	proj, _ := reg.Get(id)
	fmt.Printf("focused: %s (%s)\n", proj.Name, proj.ID)
	return nil
}

func projectsForgetCmd(c *cli.Context) error {
	target := strings.TrimSpace(c.Args().First())
	if target == "" {
		return fmt.Errorf("usage: coderag projects forget <path-or-id>")
	}
	reg := registry.New(registry.DefaultPath())
	var id string
	if proj, ok := reg.Get(target); ok {
		id = proj.ID
	} else if proj := findProjectByPath(reg, target); proj.ID != "" {
		id = proj.ID
	} else {
		id = registry.NewID(target)
	}
	if err := reg.Forget(id); err != nil {
		return err
	}
	fmt.Printf("forgot project %s\n", id)
	return nil
}

func findProjectByPath(reg *registry.Registry, path string) registry.Project {
	abs, _ := filepath.Abs(path)
	want := registry.NewID(abs)
	p, ok := reg.Get(want)
	if ok {
		return p
	}
	return registry.Project{}
}
