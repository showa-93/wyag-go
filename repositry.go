package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var (
	ErrNotRepositry         = errors.New("not a git repository")
	ErrMissingConfiguration = errors.New("missing a config file")
	ErrNotExist             = errors.New("not exist such file or directory")
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

	if dir, err := os.Stat(r.gitdir); !(force || err == nil || (dir != nil && dir.IsDir())) {
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

	fi, err := os.Stat(r.worktree)
	if err != nil {
		return nil, err
	}

	if !fi.IsDir() {
		return nil, fmt.Errorf("指定のパスがディレクトリではありません path=%s", r.worktree)
	}

	{ // フォルダ作成
		if _, err := r.MakeDirectories("", true); err != nil {
			return nil, err
		}
		if _, err := r.MakeDirectories("branches", true); err != nil {
			return nil, err
		}
		if _, err := r.MakeDirectories("objects", true); err != nil {
			return nil, err
		}
		if _, err := r.MakeDirectories("refs/tags", true); err != nil {
			return nil, err
		}
		if _, err := r.MakeDirectories("refs/heads", true); err != nil {
			return nil, err
		}
	}

	{ // ファイル作成
		if f, err := r.MakeFile("description", true); err != nil {
			return nil, err
		} else if f == nil {
			return nil, errors.New("file already exists")
		} else {
			_, err := fmt.Fprint(f, "Unnamed repository; edit this file 'description' to name the repository.\n")
			if err != nil {
				return nil, err
			}
		}

		if f, err := r.MakeFile("HEAD", true); err != nil {
			return nil, err
		} else if f == nil {
			return nil, errors.New("file already exists")
		} else {
			_, err := fmt.Fprint(f, "ref: refs/heads/master\n")
			if err != nil {
				return nil, err
			}
		}

		if f, err := r.MakeFile("config", true); err != nil {
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

func (r *Repository) MakeFile(path string, mkdir bool) (f *os.File, err error) {
	if _, err := r.MakeDirectories(filepath.Dir(path), mkdir); err != nil {
		return nil, err
	}

	flag := os.O_RDWR
	if mkdir {
		flag = flag | os.O_CREATE
	}

	path = r.Path(path)
	f, err = os.OpenFile(path, flag, os.FileMode(0644))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNotExist
		}
		return nil, err
	}

	fi, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if fi.IsDir() {
		return nil, fmt.Errorf("指定のパスがファイルでなくディレクトリです path=%s", path)
	}

	return
}

func (r *Repository) MakeDirectories(path string, mkdir bool) (string, error) {
	path = strings.Trim(path, string(os.PathSeparator)) + string(os.PathSeparator)

	var i int
	for {
		if i < 0 || i >= len(path) {
			break
		}
		i += strings.Index(path[i:], string(os.PathSeparator))
		if err := r.makeDirectory(path[:i], mkdir); err != nil {
			return "", err
		}
		i++
	}

	return r.Path(path), nil
}

func (r *Repository) makeDirectory(path string, mkdir bool) error {
	path = r.Path(path)
	fi, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			if mkdir {
				return os.Mkdir(path, os.FileMode(0755))
			}
			return nil
		}
		return err
	}

	if !fi.IsDir() {
		return fmt.Errorf("not a directory path=%s", path)
	}

	return nil
}

func FindRepository(path string, requred bool) (*Repository, error) {
	var err error
	if !filepath.IsAbs(path) {
		path, err = filepath.Abs(path)
		if err != nil {
			return nil, err
		}
	}
	r, err := NewRepository(path, false)
	if err == nil {
		return r, nil
	}
	if !errors.Is(err, ErrNotRepositry) {
		return nil, err
	}

	parenet := filepath.Join(path, "..")
	if parenet == path {
		if requred {
			return nil, fmt.Errorf("%w path=%s", ErrNotRepositry, path)
		}
		return nil, nil
	}

	return FindRepository(parenet, requred)
}
