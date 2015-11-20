package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

const appName = "logias"

func main() {
	var optCfgFile string
	var optGenSysvInit bool
	flag.Usage = func() {
		fmt.Printf(`%s [gen-sysvinit-script] -c FILE
Options of logias:
    -c                 : lua configuration file path.
    gen-sysvinit-script: generate init script for the Sysv.
`, os.Args[0])
	}
	if len(os.Args) > 2 {
		switch os.Args[1] {
		case "gen-sysvinit-script":
			optGenSysvInit = true
			os.Args = os.Args[1:]
		}
	}

	flag.StringVar(&optCfgFile, "c", "", "config file")
	flag.Parse()
	if len(optCfgFile) == 0 {
		flag.Usage()
		os.Exit(1)
	}
	if !isFile(optCfgFile) {
		fmt.Fprintf(os.Stderr, "%s does not exist or is not a regular file.\n", optCfgFile)
		os.Exit(1)
	}
	if optGenSysvInit {
		fmt.Println(genSysvInitScript(optCfgFile))
		os.Exit(0)
	}

	dp := newDispathcer(optCfgFile)
	sigs := make(chan os.Signal, 1)
	sigUSR1 := syscall.Signal(0xa)
	signal.Notify(sigs, os.Interrupt, sigUSR1)
	go func() {
		for {
			s := <-sigs
			switch s {
			case os.Interrupt:
				dp.exitc <- 1
			case sigUSR1:
				dp.shared.logger.reloadFile()
			}
		}
	}()
	dp.run()
}
