package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

var (
	BasePath = "."
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
	Type   ObjectType
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
	cf.Object = cf.Args()[1]

	t, ok := ConvertObjectType(cf.Args()[0])
	if !ok {
		fmt.Printf("unknown object type %s\n", cf.Type)
		os.Exit(1)
	}
	cf.Type = t

	return cf
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

type HashObjectCommand struct {
	*flag.FlagSet
	Write bool
	Type  ObjectType
	Path  string
}

func NewHashObjectCommand(args []string) *HashObjectCommand {
	ho := &HashObjectCommand{}
	ho.FlagSet = flag.NewFlagSet("init", flag.ExitOnError)
	ho.FlagSet.BoolVar(&ho.Write, "w", false, "Actually write the object into the database")
	t := ho.FlagSet.String("t", "blob", "Specify the type")

	ho.Usage = func() {
		o := flag.CommandLine.Output()
		fmt.Fprint(o, "Usage: wyag-go hash-object [-w] [-t TYPE] FILE\n")
		fmt.Fprint(o, "\tCompute object ID and optionally creates a blob from a file\n")
	}

	ho.Parse(args)
	if len(ho.Args()) != 1 {
		fmt.Printf("expected 1 arguments count=%d\n", len(ho.Args()))
		os.Exit(1)
	}
	ho.Path = ho.Args()[0]

	if !filepath.IsAbs(ho.Path) {
		p, err := filepath.Abs(ho.Path)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		ho.Path = p
	}

	ot, ok := ConvertObjectType(*t)
	if !ok {
		fmt.Printf("unknown object type %s\n", ot)
		os.Exit(1)
	}
	ho.Type = ot

	return ho
}

func (ho *HashObjectCommand) Run() error {
	var (
		repo *Repository
		err  error
	)
	if ho.Write {
		if repo, err = NewRepository(BasePath, false); err != nil {
			return err
		}
	}

	f, err := os.OpenFile(ho.Path, os.O_RDONLY, os.FileMode(0644))
	if err != nil {
		return err
	}
	sha, err := HashObject(f, ho.Type, repo, ho.Write)
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stdout, "%s\n", sha)
	return nil
}

type LogCommand struct {
	*flag.FlagSet
	sha string
}

func NewLogCommand(args []string) *LogCommand {
	lc := &LogCommand{}
	lc.FlagSet = flag.NewFlagSet("log", flag.ExitOnError)

	lc.Usage = func() {
		o := flag.CommandLine.Output()
		fmt.Fprint(o, "Usage: wyag-go log COMMIT\n")
		fmt.Fprint(o, "\tDisplay history of a given commit.\n")
	}

	lc.Parse(args)
	if len(lc.Args()) != 1 {
		fmt.Printf("expected 1 arguments count=%d\n", len(lc.Args()))
		os.Exit(1)
	}
	lc.sha = lc.Args()[0]

	return lc
}

func (lc *LogCommand) Run() error {
	repo, err := FindRepository(BasePath, false)
	if err != nil {
		return err
	}

	fmt.Fprintln(os.Stdout, "digraph wyaglog{")
	sha := FindObject(repo, lc.sha, string(Commit), false)
	LogGraphviz(repo, sha, make(map[string]struct{}))
	fmt.Fprintln(os.Stdout, "}")
	return nil
}

func LogGraphviz(repo *Repository, sha string, exist map[string]struct{}) error {
	if _, ok := exist[sha]; ok {
		return nil
	}
	exist[sha] = struct{}{}

	c, err := ReadObject(repo, sha)
	if err != nil {
		return err
	}
	if c.TypeHeader() != Commit {
		return fmt.Errorf("unexpected type: %s", c.TypeHeader())
	}
	commit := c.(*CommitObject)
	parents, ok := commit.kvlm.Get("parent")
	if !ok {
		// 最初のコミットだと存在しない
		return nil
	}

	for _, p := range parents {
		fmt.Fprintf(os.Stdout, "c_%s -> c_%s\n", sha, p)
		if err := LogGraphviz(repo, p, exist); err != nil {
			return err
		}
	}

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

	var cmd Command
	switch os.Args[1] {
	case "init":
		cmd = NewInit(os.Args[2:])
	case "cat-file":
		cmd = NewCatFile(os.Args[2:])
	case "hash-object":
		cmd = NewHashObjectCommand(os.Args[2:])
	case "log":
		cmd = NewLogCommand(os.Args[2:])
	default:
		fmt.Printf("unknown subcommand %s\n", os.Args[1])
		os.Exit(1)
	}

	if err := cmd.Run(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
