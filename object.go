package main

import (
	"bytes"
	"compress/zlib"
	"crypto/sha1"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

type ObjectType string

const (
	Commit = ObjectType("commit")
	Tree   = ObjectType("tree")
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

// loose objects
// Gitではpackfileと呼ばれるloose objectsを
// コンパイルしたような保存メカニズムがある
// 複雑な処理のため、実装は省く
type Object interface {
	Serialize() ([]byte, error)
	DeSerialize(data []byte) error
	TypeHeader() ObjectType
}

func NewObject(typeHeader ObjectType, raw []byte) (Object, error) {
	switch typeHeader {
	case Commit:
		return NewCommitObject(raw), nil
	case Tree:
		return NewTreeObject(raw)
	case Tag:
	case Blob:
		return NewBlobObject(raw), nil
	}
	return nil, errors.New("unknown object type")
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

	return NewObject(typeHeader, raw[x+y+1:])
}

func FindObject(r *Repository, name, typeHeader string, follow bool) string {
	return name
}

func HashObject(f *os.File, t ObjectType, repo *Repository, write bool) (string, error) {
	raw, err := io.ReadAll(f)
	if err != nil {
		return "", err
	}
	o, err := NewObject(t, raw)
	if err == nil {
		return "", fmt.Errorf("unknown type tag=%s %w", t, err)
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

func (o *BlobObject) TypeHeader() ObjectType {
	return Blob
}

// Key Value List With Message
type Kvlm struct {
	m    map[string][]string
	keys []string
}

func (k *Kvlm) Add(key string, value string) {
	if _, ok := k.m[key]; !ok {
		k.keys = append(k.keys, key)
		k.m[key] = make([]string, 0, 1)
	}
	k.m[key] = append(k.m[key], value)
}

func (k *Kvlm) Get(key string) ([]string, bool) {
	v, ok := k.m[key]
	return v, ok
}

func (k *Kvlm) Serialize() []byte {
	var sb strings.Builder
	for _, key := range k.keys {
		if key == "" {
			continue
		}
		values := k.m[key]
		for _, v := range values {
			line := fmt.Sprintf("%s %s\n", key, strings.Replace(v, "\n", "\n ", -1))
			sb.WriteString(line)
		}
	}

	message := k.m[""]
	sb.WriteString("\n" + strings.Join(message, ""))

	return []byte(sb.String())
}

// コミットオブジェクトのフォーマットにパースする
func ParseKvlm(raw []byte, start int, kvlm *Kvlm) *Kvlm {
	if kvlm == nil {
		kvlm = &Kvlm{
			m:    make(map[string][]string),
			keys: []string{},
		}
	}

	spc := bytes.Index(raw[start:], []byte(" ")) + start
	n1 := bytes.Index(raw[start:], []byte("\n")) + start

	// スペーズまたは改行がないことを意味するため、残りはメッセージである
	if spc < start+1 || n1 < start+1 {
		kvlm.Add("", string(raw[start+1:]))
		return kvlm
	}

	// 各フィールドのキー
	key := string(raw[start:spc])

	// 行の終わりを見つけるため、スペースの伴わない改行（\n）探します
	end := start
	for true {
		end = bytes.Index(raw[end+1:], []byte("\n")) + end + 1
		if raw[end+1] != byte(' ') {
			break
		}
	}

	// 先頭行のスペースを削除する
	value := bytes.Replace(raw[spc+1:end], []byte("\n "), []byte("\n"), -1)
	kvlm.Add(key, string(value))

	return ParseKvlm(raw, end+1, kvlm)
}

type CommitObject struct {
	kvlm *Kvlm
}

func NewCommitObject(raw []byte) *CommitObject {
	o := &CommitObject{}
	o.DeSerialize(raw)
	return o
}

func (o *CommitObject) Serialize() ([]byte, error) {
	return o.kvlm.Serialize(), nil
}

func (o *CommitObject) DeSerialize(data []byte) error {
	o.kvlm = ParseKvlm(data, 0, nil)
	return nil
}

func (o *CommitObject) TypeHeader() ObjectType {
	return Commit
}

// 複数のファイルをまとめて格納するオブジェクト
type TreeObject struct {
	items []*TreeLeafObject
}

func NewTreeObject(raw []byte) (*TreeObject, error) {
	o := &TreeObject{}
	return o, o.DeSerialize(raw)
}

func (o *TreeObject) Serialize() ([]byte, error) {
	var sb strings.Builder
	for _, i := range o.items {
		sb.WriteString(i.mode)
		sb.WriteString(" ")
		sb.WriteString(i.path)
		sb.WriteString("\x00")
		x, err := strconv.ParseUint(i.sha, 16, 64)
		if err != nil {
			return nil, err
		}
		buf := make([]byte, 20)
		if n := binary.PutUvarint(buf, x); len(buf) != n {
			return nil, errors.New("invalid sha")
		}
		sb.Write(buf)
	}
	return []byte(sb.String()), nil
}

func (o *TreeObject) DeSerialize(data []byte) error {
	items, err := ParseTree(data)
	if err != nil {
		return err
	}
	o.items = items
	return nil
}

func (o *TreeObject) TypeHeader() ObjectType {
	return Tree
}

type TreeLeafObject struct {
	mode string // ファイルモード
	path string // ファイルのパス
	sha  string // 20byteのバイナリエンコーディングされたobjectのsha1
}

func NewTreeLeafObject(mode, path, sha string) *TreeLeafObject {
	return &TreeLeafObject{
		mode: mode,
		path: path,
		sha:  sha,
	}
}

func ParseTree(raw []byte) (list []*TreeLeafObject, err error) {
	var (
		pos  = 0
		max  = len(raw)
		leaf *TreeLeafObject
	)

	for pos < max {
		pos, leaf, err = ParseLeaf(raw, pos)
		if err != nil {
			return nil, err
		}
		list = append(list, leaf)
	}

	return list, nil
}

func ParseLeaf(raw []byte, start int) (int, *TreeLeafObject, error) {
	// modeの位置を取得
	x := bytes.Index(raw[start:], []byte(" ")) + start
	if !(x-start == 5 || x-start == 6) {
		return -1, nil, errors.New("invalid leaf")
	}
	mode := string(raw[start:x])

	y := bytes.Index(raw[x:], []byte("\x00")) + x
	path := string(raw[x+1 : y])

	return y + 21, NewTreeLeafObject(mode, path, fmt.Sprintf("%x", raw[y+1:y+21])), nil
}
