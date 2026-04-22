package cmd

import "flag"

type Flags struct {
	Auth     bool
	Headless bool
	All      bool
}

func ParseFlags() Flags {
	auth := flag.Bool("auth", false, "Check AWS auth and exit")
	headless := flag.Bool("headless", false, "Skip TUI, run scan and print results")
	all := flag.Bool("all", false, "Scan all accessible regions with P/G series, save all formats")
	flag.Parse()
	return Flags{Auth: *auth, Headless: *headless, All: *all}
}
