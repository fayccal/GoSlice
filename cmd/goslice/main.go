package main

import (
	"fmt"
	"github.com/aligator/goslice"
	"github.com/aligator/goslice/data"
	"io"
	"os"

	flag "github.com/spf13/pflag"
)

var Version = "unknown development version"

func main() {
	o := data.ParseFlags()

	if o.GoSlice.PrintVersion {
		printVersion(os.Stdout)
		os.Exit(0)
	}

	if o.GoSlice.InputFilePath == "" {
		_, _ = fmt.Fprintf(os.Stderr, "the STL_FILE path has to be specified\n")
		flag.Usage()
		os.Exit(1)
	}

	p := goslice.NewGoSlice(o)
	err := p.Process()

	if err != nil {
		fmt.Println("error while processing file:", err)
		os.Exit(2)
	}
}

func printVersion(w io.Writer) {
	str := fmt.Sprintf("GoSlice %s", Version)
	_, _ = w.Write([]byte(str))
}
