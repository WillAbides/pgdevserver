// package bdcache is largely copied from github.com/willabides/bindown with
// some modifications for this repo.

package bdcache

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/rogpeppe/go-internal/lockedfile"
)

type (
	populateFunc func(string) error
	validateFunc func(string) error
)

type Cache struct {
	Root string
}

type WalkFunc func(cacheDir string) error

// Walk calls walkFn for each key in the cache. If walkFn returns an error, Walk returns that error
// and stops walking the cache.
func (c *Cache) Walk(walkFn WalkFunc) (errOut error) {
	rootLock, err := c.rLockRoot()
	if err != nil {
		return err
	}
	defer func() { errOut = errors.Join(errOut, rootLock.Close()) }()
	dir, err := os.ReadDir(c.Root)
	if err != nil {
		return err
	}
	for _, entry := range dir {
		err = c.walkEntry(entry, walkFn)
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *Cache) walkEntry(entry os.DirEntry, walkFn WalkFunc) (errOut error) {
	key, err := parseKey(entry.Name())
	if err != nil {
		return nil
	}
	lock, err := c.rLock(key)
	if err != nil {
		return err
	}
	defer func() { errOut = errors.Join(errOut, lock.Close()) }()
	return walkFn(filepath.Join(c.Root, key))
}

// Dir returns a fs.FS for the given key, populating the cache if necessary.
// The returned fs.FS is valid until unlock is called. After that the contents may change unexpectedly.
func (c *Cache) Dir(key string, validate validateFunc, populate populateFunc) (_ string, unlock func() error, _ error) {
	var err error
	key, err = parseKey(key)
	if err != nil {
		return "", nil, err
	}
	lock, err := c.rLock(key)
	if err != nil {
		return "", nil, err
	}
	dir := filepath.Join(c.Root, key)
	validateErr := validateDir(dir, validate)
	if validateErr == nil {
		return dir, lock.Close, nil
	}
	if populate == nil {
		return "", nil, errors.Join(validateErr, lock.Close())
	}
	err = lock.Close()
	if err != nil {
		return "", nil, err
	}
	err = c.populate(key, validate, populate)
	if err != nil {
		return "", nil, err
	}
	lock, err = c.rLock(key)
	if err != nil {
		return "", nil, err
	}
	err = validateDir(dir, validate)
	if err != nil {
		return "", nil, errors.Join(err, lock.Close())
	}
	return dir, lock.Close, nil
}

// Evict removes acquires a write lock and removes the cache entry for the given key.
func (c *Cache) Evict(key string) (errOut error) {
	var err error
	key, err = parseKey(key)
	if err != nil {
		return err
	}
	lock, err := c.lock(key)
	if err != nil {
		return err
	}
	unlocked := false
	defer func() {
		if unlocked {
			return
		}
		errOut = errors.Join(errOut, lock.Close())
	}()
	dir := filepath.Join(c.Root, key)
	info, err := os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if !info.IsDir() {
		return errors.New("not a directory")
	}
	err = os.RemoveAll(dir)
	if err != nil {
		return err
	}
	// Unlock early to get around a Windows issue where you can't delete a locked file.
	unlocked = true
	err = lock.Close()
	if err != nil {
		return err
	}
	return os.Remove(c.lockfile(key))
}

func (c *Cache) lockfile(key string) string {
	return filepath.Join(c.locksDir(), key)
}

func (c *Cache) locksDir() string {
	return filepath.Join(c.Root, ".locks")
}

func (c *Cache) rLockRoot() (io.Closer, error) {
	return acquireRLock(c.lockfile(".root"))
}

func (c *Cache) lockRoot() (io.Closer, error) {
	return acquireLock(c.lockfile(".root"))
}

func (c *Cache) lock(key string) (io.Closer, error) {
	rootLock, err := c.rLockRoot()
	if err != nil {
		return nil, err
	}
	file, err := lockedfile.Create(c.lockfile(key))
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(c.Root, key)
	return &writeLock{
		rootLock: rootLock,
		lock:     file,
		dir:      dir,
	}, nil
}

func (c *Cache) rLock(key string) (io.Closer, error) {
	rootLock, err := c.rLockRoot()
	if err != nil {
		return nil, err
	}
	rLock, err := acquireRLock(c.lockfile(key))
	if err != nil {
		return nil, err
	}
	return &readLock{
		rootLock: rootLock,
		lock:     rLock,
	}, nil
}

func acquireRLock(lockfile string) (io.Closer, error) {
	var rLock io.Closer
	for i := 0; i < 8; i++ {
		err := os.MkdirAll(filepath.Dir(lockfile), 0o777)
		if err != nil {
			return nil, err
		}
		rLock, err = lockedfile.Open(lockfile)
		if err == nil {
			break
		}
		if !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
		rLock = nil
		var wl io.Closer
		wl, err = lockedfile.Create(lockfile)
		if err != nil {
			return nil, err
		}
		err = wl.Close()
		if err != nil {
			return nil, err
		}
	}
	if rLock == nil {
		return nil, errors.New("failed to acquire lock")
	}
	return rLock, nil
}

func acquireLock(lockfile string) (io.Closer, error) {
	err := os.MkdirAll(filepath.Dir(lockfile), 0o777)
	if err != nil {
		return nil, err
	}
	return lockedfile.Create(lockfile)
}

func (c *Cache) populate(key string, validate validateFunc, populate populateFunc) (errOut error) {
	lock, err := c.lock(key)
	if err != nil {
		return err
	}
	defer func() {
		errOut = errors.Join(errOut, lock.Close())
	}()
	dir := filepath.Join(c.Root, key)
	info, err := os.Stat(dir)
	if os.IsNotExist(err) {
		err = os.MkdirAll(dir, 0o777)
		if err != nil {
			return err
		}
		return populate(dir)
	}
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return errors.New("not a directory")
	}
	if validateDir(dir, validate) == nil {
		return nil
	}
	err = os.RemoveAll(dir)
	if err != nil {
		return err
	}
	err = os.MkdirAll(dir, 0o777)
	if err != nil {
		return err
	}
	return populate(dir)
}

// RemoveRoot removes a cache root and all of its contents. This is the nuclear option.
func RemoveRoot(root string) (errOut error) {
	c := &Cache{Root: root}
	rootLock, err := c.lockRoot()
	if err != nil {
		return err
	}
	unlocked := false
	defer func() {
		if unlocked {
			return
		}
		errOut = errors.Join(errOut, rootLock.Close())
	}()
	// Unlock early to get around a Windows issue where you can't delete a locked file.
	unlocked = true
	err = rootLock.Close()
	if err != nil {
		return err
	}
	return os.RemoveAll(root)
}

type writeLock struct {
	rootLock io.Closer
	lock     io.Closer
	dir      string
}

func (l *writeLock) Close() (errOut error) {
	return errors.Join(l.lock.Close(), l.rootLock.Close())
}

type readLock struct {
	rootLock io.Closer
	lock     io.Closer
}

func (l *readLock) Close() (errOut error) {
	return errors.Join(l.lock.Close(), l.rootLock.Close())
}

func validateDir(dir string, validate validateFunc) error {
	info, err := os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return errors.New("entry does not exist")
		}
		return err
	}
	if !info.IsDir() {
		return errors.New("not a directory")
	}
	if validate == nil {
		return nil
	}
	return validate(dir)
}

func parseKey(key string) (string, error) {
	key = filepath.FromSlash(key)
	// key must be a valid file name without path separators
	if key != filepath.Base(key) {
		return "", errors.New("invalid key")
	}
	// reserve dot files for internal use
	if strings.HasPrefix(key, ".") {
		return "", errors.New("invalid key")
	}
	return key, nil
}
