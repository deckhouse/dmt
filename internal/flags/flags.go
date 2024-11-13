package flags

import (
	"fmt"
	"os"

	"github.com/spf13/pflag"
)

var (
	LintersLimit int
	LogLevel     string
)

const (
	numThreads = 10
)

func ParseFlags() []string {
	var (
		printHelp    bool
		printVersion bool
		version      = "HEAD"
	)

	flagSet := pflag.NewFlagSet("dmt", pflag.ContinueOnError)

	flagSet.BoolVarP(&printHelp, "help", "h", false, "help message")
	flagSet.BoolVarP(&printVersion, "version", "v", false, "version message")
	flagSet.IntVarP(&LintersLimit, "parallel", "p", numThreads, "number of threads for parallel processing")
	flagSet.StringVarP(&LogLevel, "log-level", "l", "INFO", "log-level [DEBUG | INFO | WARN | ERROR]")

	flagSet.Usage = func() {
		_, _ = fmt.Fprintln(os.Stderr, "Usage: dmt [OPTIONS] [dirs...]")
		flagSet.PrintDefaults()
	}

	if err := flagSet.Parse(os.Args[1:]); err != nil {
		flagSet.Usage()
		return nil
	}

	if printHelp {
		flagSet.Usage()
		return nil
	}

	if printVersion {
		fmt.Println("dmt version: ", version)
		return nil
	}

	var dirs = flagSet.Args()
	if len(dirs) == 0 {
		dirs = []string{"."}
	}

	return dirs
}
