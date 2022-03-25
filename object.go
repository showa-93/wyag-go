package main

import (
	"bytes"
	"compress/zlib"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strconv"
)

type ObjectType string

const (
	Commit = ObjectType("commit")
	Tree   = ObjectType("treee")
	Tag    = ObjectType("tag")
	Blob   = ObjectType("blob")
)

var (
	ObjectTypes = []ObjectType{Commit, Tree, Tag, Blob}
)

func ConvertObjectType(target string) (ObjectType, bool) {
	for _, t := range ObjectTypes {
		if string(t) == target {
			return ObjectType(target), true
		}
	}
	return ObjectType(""), false
}

type Object interface {
	Serialize() ([]byte, error)
	DeSerialize(data []byte) error
	TypeHeader() string
}

func NewObject(typeHeader ObjectType, raw []byte) Object {
	switch typeHeader {
	case Commit:
	case Tree:
	case Tag:
	case Blob:
		return NewBlobObject(raw)
	}
	return nil
}

func WriteObject(repo *Repository, o Object, acctually bool) (string, error) {
	data, err := o.Serialize()
	if err != nil {
		return "", err
	}

	result := fmt.Sprintf("%s %d\x00%s", o.TypeHeader(), len(data), data)
	s := sha1.New()
	io.WriteString(s, result)
	sha := hex.EncodeToString(s.Sum(nil))

	if acctually {
		path := "objects/" + sha[0:2] + "/" + sha[2:]
		f, err := repo.makeFile(path, acctually)
		if err != nil {
			return "", err
		}
		if f == nil {
			f, err = os.OpenFile(repo.Path(path), os.O_WRONLY, os.FileMode(0644))
			if err != nil {
				return "", err
			}
		}

		zw := zlib.NewWriter(f)
		defer zw.Close()
		if _, err := zw.Write([]byte(result)); err != nil {
			return "", err
		}
	}

	return sha, nil
}

func ReadObject(r *Repository, sha string) (Object, error) {
	path := "objects/" + sha[0:2] + "/" + sha[2:]
	f, err := os.Open(r.Path(path))
	if err != nil {
		return nil, err
	}

	zr, err := zlib.NewReader(f)
	if err != nil {
		return nil, err
	}
	raw, err := io.ReadAll(zr)
	if err != nil {
		return nil, err
	}

	// 00000000  63 6f 6d 6d 69 74 20 31  30 38 36 00 74 72 65 65  |commit 1086.tree|
	// 00000010  20 32 39 66 66 31 36 63  39 63 31 34 65 32 36 35  | 29ff16c9c14e265|
	// 00000020  32 62 32 32 66 38 62 37  38 62 62 30 38 61 35 61  |2b22f8b78bb08a5a|

	// 最初の'commit'の位置を探す
	x := bytes.Index(raw, []byte(" "))
	typeHeader, ok := ConvertObjectType(string(raw[:x]))
	if !ok {
		return nil, fmt.Errorf("unknown type tag=%s sha=%s", typeHeader, sha)
	}

	// オブジェクトのサイズを読み込む
	y := bytes.Index(raw[x:], []byte("\x00"))
	size, err := strconv.Atoi(string(raw[x+1 : x+y]))
	if err != nil {
		return nil, err
	}
	if size != len(raw)-x-y-1 {
		return nil, fmt.Errorf("malformed object: bad length sha=%s", sha)
	}

	return NewObject(typeHeader, raw[x+y+1:]), nil
}

func FindObject(r *Repository, name, typeHeader string, follow bool) string {
	return name
}

func HashObject(f *os.File, t ObjectType, repo *Repository, write bool) (string, error) {
	raw, err := io.ReadAll(f)
	if err != nil {
		return "", err
	}
	o := NewObject(t, raw)
	if o == nil {
		return "", fmt.Errorf("unknown type tag=%s", t)
	}

	return WriteObject(repo, o, write)
}

type BlobObject struct {
	blobdata []byte
}

func NewBlobObject(raw []byte) *BlobObject {
	bo := &BlobObject{}
	bo.DeSerialize(raw)
	return bo
}

func (o *BlobObject) Serialize() ([]byte, error) {
	return o.blobdata, nil
}

func (o *BlobObject) DeSerialize(data []byte) error {
	o.blobdata = data
	return nil
}

func (o *BlobObject) TypeHeader() string {
	return "blob"
}
