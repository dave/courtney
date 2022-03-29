package patsy

import (
	"path"
	"path/filepath"
	"strings"
	"sync"

	"github.com/dave/courtney/patsy/vos"
	"github.com/pkg/errors"
)

// NewCache returns a new *Cache, allowing cached access to patsy utility
// functions.
func NewCache(env vos.Env) *Cache {
	return &Cache{
		env:       env,
		dirm:      new(sync.RWMutex),
		dirsm:     new(sync.RWMutex),
		pathm:     new(sync.RWMutex),
		namem:     new(sync.RWMutex),
		dirCache:  make(map[keyWithDir]string),
		dirsCache: make(map[keyWithDir]map[string]string),
		pathCache: make(map[keyWithDir]string),
		nameCache: make(map[keyWithDir]string),
	}
}

// depending on the cwdir results can vary, so we include the directory in the cache keys
type keyWithDir struct {
	key string
	dir  string
}

// Cache supports patsy.Dir and patsy.Path, but cached so they can be used in
// tight loops without hammering the filesystem.
type Cache struct {
	env       vos.Env
	dirm      *sync.RWMutex
	dirsm     *sync.RWMutex
	pathm     *sync.RWMutex
	namem     *sync.RWMutex
	dirCache  map[keyWithDir]string
	dirsCache map[keyWithDir]map[string]string
	pathCache map[keyWithDir]string
	nameCache map[keyWithDir]string
}

// Name does the same as patsy.Name but cached.
func (c *Cache) Name(packagePath, srcDir string) (string, error) {
	// check the cache first
	if n, ok := c.getName(keyWithDir{key: packagePath, dir: srcDir}); ok {
		return n, nil
	}
	n, err := Name(c.env, packagePath, srcDir)
	if err != nil {
		return "", err
	}
	c.setName(keyWithDir{key: packagePath, dir: srcDir}, n)
	return n, nil
}

// Path does the same as patsy.Path but cached.
func (c *Cache) Path(dir string) (string, error) {
	// check the cache first
	if ppath, ok := c.getPath(dir); ok {
		return ppath, nil
	}
	ppath, err := Path(c.env, dir)
	if err != nil {
		return "", err
	}
	c.setDir(ppath, dir)
	c.setPath(dir, ppath)
	return ppath, nil
}

// Dir does the same as patsy.Dir but cached.
func (c *Cache) Dir(ppath string) (string, error) {
	// check the cache first
	if dir, ok := c.getDir(ppath); ok {
		return dir, nil
	}
	dirs, err := c.Dirs(ppath)
	if err != nil {
		return "", err
	}
	dir, ok := dirs[ppath]
	if !ok {
		return "", errors.Errorf("Dir not found for %s", ppath)
	}
	return dir, nil
}

// Dirs does the same as patsy.Dirs but cached.
func (c *Cache) Dirs(ppath string) (map[string]string, error) {
	// check the cache first
	if dirs, ok := c.getDirs(ppath); ok {
		return dirs, nil
	}
	dirs, err := Dirs(c.env, ppath)
	if err != nil {
		return nil, err
	}
	c.setDirs(ppath, dirs)

	for importPath, dir := range dirs {
		c.setDir(importPath, dir)
		c.setPath(dir, importPath)
	}
	return dirs, nil
}

// GoName converts a full filepath to a package path and filename:
//     /Users/dave/go/src/github.com/dave/foo.go -> github.com/dave/foo.go
func (c *Cache) GoName(fpath string) (string, error) {
	fdir, fname := filepath.Split(fpath)
	ppath, err := c.Path(fdir)
	if err != nil {
		return "", err
	}
	return path.Join(ppath, fname), nil
}

// FilePath converts a package path and filename to a full filepath:
//     github.com/dave/foo.go -> /Users/dave/go/src/github.com/dave/foo.go
func (c *Cache) FilePath(gpath string) (string, error) {
	ppath, fname := path.Split(gpath)
	ppath = strings.TrimSuffix(ppath, "/")

	fdirs, err := c.Dirs(ppath)
	if err != nil {
		return "", err
	}
	fdir, ok := fdirs[ppath]
	if !ok {
		return "", errors.Errorf("Dir not found for %s", gpath)
	}

	return filepath.Join(fdir, fname), nil
}

func (c *Cache) getDir(key string) (string, bool) {
	c.dirm.RLock()
	defer c.dirm.RUnlock()
	wd, _ := c.env.Getwd()
	v, ok := c.dirCache[keyWithDir{key: wd, dir: key}]
	return v, ok
}

func (c *Cache) getDirs(key string) (map[string]string, bool) {
	c.dirsm.RLock()
	defer c.dirsm.RUnlock()
	wd, _ := c.env.Getwd()
	v, ok := c.dirsCache[keyWithDir{dir: wd, key: key}]
	return v, ok
}

func (c *Cache) getPath(key string) (string, bool) {
	c.pathm.RLock()
	defer c.pathm.RUnlock()
	wd, _ := c.env.Getwd()
	v, ok := c.pathCache[keyWithDir{dir: wd, key: key}]
	return v, ok
}

func (c *Cache) getName(key keyWithDir) (string, bool) {
	c.namem.RLock()
	defer c.namem.RUnlock()
	v, ok := c.nameCache[key]
	return v, ok
}

func (c *Cache) setDir(key, value string) {
	c.dirm.Lock()
	defer c.dirm.Unlock()
	wd, _ := c.env.Getwd()
	c.dirCache[keyWithDir{dir: wd, key: key}] = value
}

func (c *Cache) setDirs(key string, value map[string]string) {
	c.dirsm.Lock()
	defer c.dirsm.Unlock()
	wd, _ := c.env.Getwd()
	c.dirsCache[keyWithDir{dir: wd, key: key}] = value
}

func (c *Cache) setPath(key, value string) {
	c.pathm.Lock()
	defer c.pathm.Unlock()
	wd, _ := c.env.Getwd()
	c.pathCache[keyWithDir{dir: wd, key: key}] = value
}

func (c *Cache) setName(key keyWithDir, value string) {
	c.namem.Lock()
	defer c.namem.Unlock()
	c.nameCache[key] = value
}
