package storage

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"regexp"
)

var validKey = regexp.MustCompile(`^[a-f0-9]{64}$`)

type Local struct{ Root string }

func (l Local) path(key string) (string, error) {
	if !validKey.MatchString(key) {
		return "", errors.New("invalid image key")
	}
	return filepath.Join(l.Root, key+".png"), nil
}
func (l Local) Put(_ context.Context, key string, b []byte) error {
	p, e := l.path(key)
	if e != nil {
		return e
	}
	if e = os.MkdirAll(l.Root, 0700); e != nil {
		return e
	}
	f, e := os.CreateTemp(l.Root, ".upload-*")
	if e != nil {
		return e
	}
	defer os.Remove(f.Name())
	if e = f.Chmod(0600); e != nil {
		f.Close()
		return e
	}
	if _, e = f.Write(b); e != nil {
		f.Close()
		return e
	}
	if e = f.Close(); e != nil {
		return e
	}
	return os.Rename(f.Name(), p)
}
func (l Local) Read(_ context.Context, key string) ([]byte, error) {
	p, e := l.path(key)
	if e != nil {
		return nil, e
	}
	return os.ReadFile(p)
}
func (l Local) Delete(_ context.Context, key string) error {
	p, e := l.path(key)
	if e != nil {
		return e
	}
	e = os.Remove(p)
	if errors.Is(e, os.ErrNotExist) {
		return nil
	}
	return e
}
