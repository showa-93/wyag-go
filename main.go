package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

type Command interface {
	Run() error
}
type Init struct {
	*flag.FlagSet
	Path string
}

func NewInit(args []string) *Init {
	i := &Init{}
	i.FlagSet = flag.NewFlagSet("init", flag.ExitOnError)
	i.Usage = func() {
		o := flag.CommandLine.Output()
		fmt.Fprint(o, "Usage: init [path]\n")
		fmt.Fprint(o, "\tInitialize a new, empty repository.\n")
	}

	i.Parse(args)
	if len(i.Args()) != 1 {
		fmt.Printf("expected 1 arguments count=%d\n", len(i.Args()))
		os.Exit(1)
	}
	i.Path = i.Args()[0]

	if !filepath.IsAbs(i.Path) {
		p, err := filepath.Abs(i.Path)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		i.Path = p
	}

	return i
}

func (i *Init) Run() error {
	_, err := CreateRepository(i.Path)
	return err
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("expected subcommands")
		os.Exit(1)
	}

	switch os.Args[1] {
	case "init":
		i := NewInit(os.Args[2:])
		if err := i.Run(); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	default:
		fmt.Printf("unknown subcommand %s\n", os.Args[1])
	}

}
