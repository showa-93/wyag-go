package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

var (
	BasePath          = "."
	Types    []string = []string{"blob", "commit", "tag", "tree"}
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

type CatFile struct {
	*flag.FlagSet
	Type   string
	Object string
}

func NewCatFile(args []string) *CatFile {
	cf := &CatFile{}
	cf.FlagSet = flag.NewFlagSet("cat-file", flag.ExitOnError)
	cf.Usage = func() {
		o := flag.CommandLine.Output()
		fmt.Fprint(o, "Usage: cat-file [TYPE] [OBJECT]\n")
		fmt.Fprint(o, "\tProvide content of repository objects.\n")
	}

	cf.Parse(args)
	if len(cf.Args()) != 2 {
		fmt.Printf("expected 1 arguments count=%d\n", len(cf.Args()))
		os.Exit(1)
	}
	cf.Type = cf.Args()[0]
	cf.Object = cf.Args()[1]

	for _, t := range Types {
		if t == cf.Type {
			return cf
		}
	}

	fmt.Printf("unknown object type %s\n", cf.Type)
	os.Exit(1)
	return nil
}

func (cf *CatFile) Run() error {
	repo, err := FindRepository(BasePath, true)
	if err != nil {
		return err
	}
	o, err := ReadObject(repo, cf.Object)
	if err != nil {
		return err
	}
	b, err := o.Serialize()
	if err != nil {
		return err
	}
	fmt.Println(string(b))
	return nil
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("expected subcommands")
		os.Exit(1)
	}

	if path := os.Getenv("WYAG_WORKDIR"); path != "" {
		BasePath = path
	}

	switch os.Args[1] {
	case "init":
		i := NewInit(os.Args[2:])
		if err := i.Run(); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	case "cat-file":
		cf := NewCatFile(os.Args[2:])
		if err := cf.Run(); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	default:
		fmt.Printf("unknown subcommand %s\n", os.Args[1])
	}

}
