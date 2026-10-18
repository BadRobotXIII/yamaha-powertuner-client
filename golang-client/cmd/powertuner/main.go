package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"time"

	powertuner "yamaha-pt-client"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run() error {
	address := flag.String("addr", powertuner.DefaultAddress, "CCU base URL")
	timeout := flag.Duration("timeout", 5*time.Second, "timeout per HTTP request")
	eps := flag.Bool("eps", false, "include EPS in maps download (only for supported vehicles)")
	page := flag.Int("page", 0, "activity-log page, 0..3 (log command only)")
	format := flag.String("format", "json", "json, text, binary, hex, all, or raw")
	part := flag.String("part", "", "response name (e.g. ecu for maps); required for raw multi-response output")
	save := flag.String("save", "", "save a lossless snapshot file locally")
	rawOut := flag.String("raw-out", "", "save exact selected response bytes locally (use -part ecu for maps)")
	compare := flag.String("compare", "", "compare this capture against a saved snapshot (report to stderr)")
	flag.Usage = func() {
		fmt.Fprintln(flag.CommandLine.Output(), "Usage: powertuner [flags] <info|maps|eps|monitor|model|lock|laps|log>\n       powertuner compare previous.json current.json\nGET-only; one capture per invocation. Flags precede the command. compare is offline.")
		flag.PrintDefaults()
	}
	flag.Parse()
	if flag.NArg() == 3 && flag.Arg(0) == "compare" {
		a, err := load(flag.Arg(1))
		if err != nil {
			return err
		}
		b, err := load(flag.Arg(2))
		if err != nil {
			return err
		}
		report, err := powertuner.Compare(a, b)
		if err != nil {
			return err
		}
		_, err = fmt.Fprint(os.Stdout, report)
		return err
	}
	if flag.NArg() != 1 {
		flag.Usage()
		return fmt.Errorf("expected one command")
	}
	command := flag.Arg(0)
	if command == "maps" && (*format == "raw" || *rawOut != "") && *part == "" {
		return fmt.Errorf("maps raw export requires -part, e.g. -part ecu")
	}
	if !powertuner.ValidFormat(*format) {
		return fmt.Errorf("unknown format %q", *format)
	}
	switch command {
	case "info", "maps", "eps", "monitor", "model", "lock", "laps", "log":
	default:
		return fmt.Errorf("unknown command %q", command)
	}
	if *eps && command != "maps" {
		return fmt.Errorf("-eps is only valid with maps")
	}
	if *page < 0 || *page > 3 {
		return fmt.Errorf("-page must be between 0 and 3")
	}
	var pageSpecified bool
	flag.Visit(func(f *flag.Flag) {
		if f.Name == "page" {
			pageSpecified = true
		}
	})
	if pageSpecified && command != "log" {
		return fmt.Errorf("-page is only valid with log")
	}
	client, err := powertuner.New(*address, *timeout)
	if err != nil {
		return err
	}
	defer client.Close()
	var previous *powertuner.Snapshot
	if *compare != "" {
		previous, err = load(*compare)
		if err != nil {
			return err
		}
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	snapshot, err := client.Capture(ctx, powertuner.Request{Command: command, IncludeEPS: *eps, Page: *page})
	if err != nil {
		return err
	}
	text, err := powertuner.FormatSnapshot(snapshot, *format, *part, 0)
	if err != nil {
		return err
	}
	if *rawOut != "" {
		var selected []byte
		for _, p := range snapshot.Parts {
			if p.Name == *part || (*part == "" && len(snapshot.Parts) == 1) {
				selected = p.Body
				break
			}
		}
		if err := os.WriteFile(*rawOut, selected, 0600); err != nil {
			return err
		}
	}
	if previous != nil {
		report, e := powertuner.Compare(previous, snapshot)
		if e != nil {
			return e
		}
		if _, e = fmt.Fprint(os.Stderr, report); e != nil {
			return e
		}
	}
	if *save != "" {
		f, e := os.Create(*save)
		if e != nil {
			return e
		}
		e = powertuner.WriteSnapshot(f, snapshot)
		closeErr := f.Close()
		if e != nil {
			return e
		}
		if closeErr != nil {
			return closeErr
		}
	}
	if _, err := os.Stdout.Write([]byte(text)); err != nil {
		return err
	}
	if *format != "raw" && (len(text) == 0 || text[len(text)-1] != '\n') {
		_, err = fmt.Fprintln(os.Stdout)
	}
	return err
}

func load(path string) (*powertuner.Snapshot, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return powertuner.ReadSnapshot(f)
}
