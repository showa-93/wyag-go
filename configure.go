package main

import (
	"fmt"
	"io"

	"github.com/go-ini/ini"
)

type Configure struct {
	Core
}

type Core struct {
	RepositoryFormatVersion int
}

func LoadConfigure(path string) (*Configure, error) {
	e := fmt.Errorf("設定ファイルの読み込みに失敗しました path=%s\n", path)

	f, err := ini.Load(path)
	if err != nil {
		return nil, fmt.Errorf("%s error=%w", e.Error(), err)
	}

	conf := &Configure{}

	v, err := f.Section("core").Key("repositoryformatversion").Int()
	if err != nil {
		e = fmt.Errorf("%s\trepositoryformatversionの読み込みに失敗しました error=%w", e, err)
	}
	conf.RepositoryFormatVersion = v

	return conf, nil
}

func DefaultConfigure(w io.Writer) error {
	f := ini.Empty()
	s, err := f.NewSection("core")
	if err != nil {
		return err
	}
	s.NewKey("repositoryformatversion", "0")
	s.NewKey("filemode", "false")
	s.NewKey("bare", "false")
	_, err = f.WriteTo(w)
	return err
}
