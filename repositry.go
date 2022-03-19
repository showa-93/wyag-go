package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

var (
	ErrNotRepositry         = errors.New("not a git repository")
	ErrMissingConfiguration = errors.New("missing a config file")
)

type Repository struct {
	worktree string
	gitdir   string
	conf     *Configure
}

func NewRepository(path string, force bool) (*Repository, error) {
	r := &Repository{
		worktree: path,
		gitdir:   filepath.Join(path, ".git"),
	}

	if dir, err := os.Stat(r.gitdir); !(force || err == nil || dir.IsDir()) {
		if !os.IsNotExist(err) {
			return nil, err
		}
		return nil, fmt.Errorf("%w path=%s", ErrNotRepositry, path)
	}

	conf, err := LoadConfigure(r.Path("config"))
	if !force && err != nil {
		return nil, fmt.Errorf("%w path=%s", ErrMissingConfiguration, r.Path("config"))
	}
	r.conf = conf

	if !force {
		if conf.RepositoryFormatVersion != 0 {
			return nil, fmt.Errorf("Unsupported repositoryformatversion %d", conf.RepositoryFormatVersion)
		}
	}

	return r, nil
}

func CreateRepository(path string) (*Repository, error) {
	r, err := NewRepository(path, true)
	if err != nil {
		return nil, err
	}

	fi, err := existPath(r.worktree, func() error { return nil })
	if err != nil {
		return nil, err
	}
	if fi != nil && !fi.IsDir() {
		return nil, fmt.Errorf("指定のパスがディレクトリではありません path=%s", r.worktree)
	}

	{ // フォルダ作成
		if _, err := r.makeDirectories("", true); err != nil {
			return nil, err
		}
		if _, err := r.makeDirectories("branches", true); err != nil {
			return nil, err
		}
		if _, err := r.makeDirectories("objects", true); err != nil {
			return nil, err
		}
		if _, err := r.makeDirectories("refs/tags", true); err != nil {
			return nil, err
		}
		if _, err := r.makeDirectories("refs/heads", true); err != nil {
			return nil, err
		}
	}

	{ // ファイル作成
		if f, err := r.makeFile("description", true); err != nil {
			return nil, err
		} else if f == nil {
			return nil, errors.New("file already exists")
		} else {
			_, err := fmt.Fprint(f, "Unnamed repository; edit this file 'description' to name the repository.\n")
			if err != nil {
				return nil, err
			}
		}

		if f, err := r.makeFile("HEAD", true); err != nil {
			return nil, err
		} else if f == nil {
			return nil, errors.New("file already exists")
		} else {
			_, err := fmt.Fprint(f, "ref: refs/heads/master\n")
			if err != nil {
				return nil, err
			}
		}

		if f, err := r.makeFile("config", true); err != nil {
			return nil, err
		} else if f == nil {
			return nil, errors.New("file already exists")
		} else {
			if err := DefaultConfigure(f); err != nil {
				return nil, err
			}
		}
	}

	return r, nil
}

func (r *Repository) Path(path string) string {
	return filepath.Join(r.gitdir, path)
}

func (r *Repository) makeFile(path string, mkdir bool) (f *os.File, err error) {
	paths := strings.Split(strings.TrimLeft(path, "/"), "/")
	if _, err := r.makeDirectories(strings.Join(paths[:len(paths)-1], "/"), mkdir); err != nil {
		return nil, err
	}
	p := r.Path(path)
	fi, err := existPath(p, func() error {
		f, err = os.OpenFile(p, os.O_CREATE|os.O_WRONLY, os.FileMode(0644))
		return err
	})

	if err != nil {
		return nil, err
	}
	if fi != nil && fi.IsDir() {
		return nil, fmt.Errorf("指定のパスがファイルでなくディレクトリです path=%s", p)
	}

	return
}

func (r *Repository) makeDirectories(path string, mkdir bool) (string, error) {
	path = strings.Trim(path, "/") + "/"

	var i int
	for {
		if i < 0 || i >= len(path) {
			break
		}
		i += strings.Index(path[i:], "/")
		if err := r.makeDirectory(path[:i], mkdir); err != nil {
			return "", err
		}
		i++
	}

	return r.Path(path), nil
}

func (r *Repository) makeDirectory(path string, mkdir bool) error {
	p := r.Path(path)
	f, err := existPath(p, func() error {
		if mkdir {
			if err := os.Mkdir(p, os.FileMode(0755)); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}

	if f == nil || f.IsDir() {
		return nil
	} else {
		return fmt.Errorf("not a directory path=%s", p)
	}
}

func existPath(path string, notExistFunc func() error) (fs.FileInfo, error) {
	f, err := os.Stat(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
		return nil, notExistFunc()
	}
	return f, nil
}
