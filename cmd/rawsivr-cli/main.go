// rawsivr-cli exercises the core engine without the UI.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"rawsivr/internal/execute"
	"rawsivr/internal/plan"
	"rawsivr/internal/scan"
	"rawsivr/internal/store"
	"rawsivr/internal/trash"
)

const usage = `usage:
  rawsivr-cli scan   <folder>
  rawsivr-cli mark   <folder> pick|reject|clear <name>...
  rawsivr-cli plan   <folder> [-mode copy|move|cleanup]
  rawsivr-cli apply  <folder> [-mode copy|move|cleanup] -yes
  rawsivr-cli undo   <folder>`

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, usage)
		os.Exit(2)
	}
	cmd, root, rest := os.Args[1], os.Args[2], os.Args[3:]
	if err := run(cmd, root, rest); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(cmd, root string, rest []string) error {
	if cmd == "undo" {
		j, recycled, err := execute.Undo(root)
		if j != nil {
			fmt.Printf("undid %s run from %s\n", j.Mode, j.Time.Format("2006-01-02 15:04:05"))
		}
		if len(recycled) > 0 {
			fmt.Printf("%d files were sent to the Recycle Bin; restore them from there:\n", len(recycled))
			for _, r := range recycled {
				fmt.Println("  ", r)
			}
		}
		return err
	}

	res, err := scan.Scan(root, scan.DefaultLayout)
	if err != nil {
		return err
	}
	st, err := store.Load(res.Root)
	if err != nil {
		return err
	}

	switch cmd {
	case "scan":
		var photos, videos, rawOnly, jpgOnly int
		for _, it := range res.Items {
			mark := string(st.Decisions[it.ID])
			if mark == "" {
				mark = "-"
			}
			var parts []string
			for _, f := range it.Files() {
				parts = append(parts, strings.TrimPrefix(filepath.Ext(f), "."))
			}
			fmt.Printf("%-7s %-6s %-24s %s\n", mark, it.Kind, it.ID, strings.Join(parts, "+"))
			switch {
			case it.Kind == scan.Video:
				videos++
			default:
				photos++
				if it.Preview == "" {
					rawOnly++
				} else if it.Raw == "" {
					jpgOnly++
				}
			}
		}
		fmt.Printf("\n%d photos (%d RAW only, %d JPG only), %d videos\n", photos, rawOnly, jpgOnly, videos)
		for _, s := range res.Strays {
			fmt.Println("stray sidecar:", s)
		}
		return nil

	case "mark":
		if len(rest) < 2 {
			return fmt.Errorf("mark needs pick|reject|clear and at least one name")
		}
		var d store.Decision
		switch rest[0] {
		case "pick":
			d = store.Pick
		case "reject":
			d = store.Reject
		case "clear":
		default:
			return fmt.Errorf("unknown mark %q", rest[0])
		}
		for _, name := range rest[1:] {
			found := false
			for _, it := range res.Items {
				if strings.EqualFold(it.Name, name) || strings.EqualFold(it.ID, name) {
					st.Set(it.ID, d)
					found = true
				}
			}
			if !found {
				return fmt.Errorf("no item named %q", name)
			}
		}
		return st.Save()

	case "plan", "apply":
		fs := flag.NewFlagSet(cmd, flag.ContinueOnError)
		mode := fs.String("mode", "copy", "copy, move or cleanup")
		yes := fs.Bool("yes", false, "confirm apply")
		if err := fs.Parse(rest); err != nil {
			return err
		}
		switch plan.Mode(*mode) {
		case plan.ModeCopy, plan.ModeMove, plan.ModeCleanup:
		default:
			return fmt.Errorf("unknown mode %q", *mode)
		}
		p := plan.Build(res, st.Decisions, plan.Mode(*mode), scan.DefaultLayout)
		for _, op := range p.Ops {
			if op.Dst != "" {
				fmt.Printf("%-8s %s -> %s\n", op.Action, op.Src, op.Dst)
			} else {
				fmt.Printf("%-8s %s\n", op.Action, op.Src)
			}
		}
		fmt.Printf("\nmode %s: %d picks, %d rejects, %d unmarked; %d operations, %.1f MB to %s\n",
			p.Mode, p.Picks, p.Rejects, p.Unmarked, len(p.Ops), float64(p.Bytes)/(1<<20), p.Mode)
		for _, d := range p.Done {
			fmt.Println("already there:", d)
		}
		for _, c := range p.Conflicts {
			fmt.Println("CONFLICT (skipped):", c)
		}
		if cmd == "plan" {
			return nil
		}
		if !*yes {
			return fmt.Errorf("nothing done; add -yes to apply")
		}
		j, err := execute.Run(res.Root, p, trash.Recycle, nil)
		if err != nil {
			return err
		}
		for _, e := range j.Entries {
			if !e.OK {
				fmt.Println("FAILED:", e.Src, "-", e.Err)
			}
		}
		fmt.Printf("done: %d ok, %d failed\n", len(j.Entries)-j.Failed(), j.Failed())
		if j.Failed() > 0 {
			return fmt.Errorf("%d operations failed", j.Failed())
		}
		return nil
	}
	return fmt.Errorf("unknown command %q\n%s", cmd, usage)
}
