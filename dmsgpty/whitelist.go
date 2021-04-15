package dmsgpty

import (
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"

	jsoniter "github.com/json-iterator/go"

	"github.com/skycoin/dmsg/cipher"
)

var json = jsoniter.ConfigFastest

// Whitelist represents a whitelist of public keys.
type Whitelist interface {
	Get(pk cipher.PubKey) (bool, error)
	All() (map[cipher.PubKey]bool, error)
	Add(pks ...cipher.PubKey) error
	Remove(pks ...cipher.PubKey) error
}

// NewJSONFileWhiteList creates a JSON file implementation of a whitelist.
func NewJSONFileWhiteList(fileName string) (Whitelist, error) {
	fileName, err := filepath.Abs(fileName)
	if err != nil {
		return nil, err
	}

	if err := os.MkdirAll(filepath.Dir(fileName), 0750); err != nil {
		return nil, err
	}

	return &jsonFileWhitelist{fileName: fileName}, nil
}

type jsonFileWhitelist struct {
	fileName string
}

func (w *jsonFileWhitelist) Get(pk cipher.PubKey) (bool, error) {
	var ok bool
	err := w.open(os.O_RDONLY|os.O_CREATE, func(pkMap map[cipher.PubKey]bool, _ *os.File) error {
		ok = pkMap[pk]
		return nil
	})
	return ok, jsonFileErr(err)
}

func (w *jsonFileWhitelist) All() (map[cipher.PubKey]bool, error) {
	var out map[cipher.PubKey]bool
	err := w.open(os.O_RDONLY|os.O_CREATE, func(pkMap map[cipher.PubKey]bool, _ *os.File) error {
		out = pkMap
		return nil
	})
	return out, jsonFileErr(err)
}

func (w *jsonFileWhitelist) Add(pks ...cipher.PubKey) error {
	return jsonFileErr(w.open(os.O_RDWR|os.O_CREATE, func(pkMap map[cipher.PubKey]bool, f *os.File) error {
		for _, pk := range pks {
			pkMap[pk] = true
		}
		return json.NewEncoder(f).Encode(pkMap)
	}))
}

func (w *jsonFileWhitelist) Remove(pks ...cipher.PubKey) error {
	return jsonFileErr(w.open(os.O_RDWR|os.O_CREATE, func(pkMap map[cipher.PubKey]bool, f *os.File) error {
		for _, pk := range pks {
			delete(pkMap, pk)
		}
		return json.NewEncoder(f).Encode(pkMap)
	}))
}

func (w *jsonFileWhitelist) open(perm int, fn func(pkMap map[cipher.PubKey]bool, f *os.File) error) error {
	f, err := os.OpenFile(w.fileName, perm, 0600) // nolint:gosec
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }() //nolint:errcheck

	// get file size
	info, err := f.Stat()
	if err != nil {
		return err
	}

	// read public key map from file
	pks := make(map[cipher.PubKey]bool)
	if info.Size() > 0 {
		if err := json.NewDecoder(f).Decode(&pks); err != nil {
			return err
		}
	}

	// seek back to start of file
	if _, err := f.Seek(0, 0); err != nil {
		return err
	}

	return fn(pks, f)
}

func jsonFileErr(err error) error {
	if err != nil {
		return fmt.Errorf("json file whitelist: %v", err)
	}
	return nil
}

// NewMemoryWhitelist creates a memory implementation of a whitelist.
func NewMemoryWhitelist() Whitelist {
	return &memoryWhitelist{
		m: make(map[cipher.PubKey]struct{}),
	}
}

type memoryWhitelist struct {
	m   map[cipher.PubKey]struct{}
	mux sync.RWMutex
}

func (w *memoryWhitelist) Get(pk cipher.PubKey) (bool, error) {
	w.mux.RLock()
	_, ok := w.m[pk]
	w.mux.RUnlock()
	return ok, nil
}

func (w *memoryWhitelist) All() (map[cipher.PubKey]bool, error) {
	out := make(map[cipher.PubKey]bool)
	w.mux.RLock()
	for k := range w.m {
		out[k] = true
	}
	w.mux.RUnlock()
	return out, nil
}

func (w *memoryWhitelist) Add(pks ...cipher.PubKey) error {
	w.mux.Lock()
	for _, pk := range pks {
		w.m[pk] = struct{}{}
	}
	w.mux.Unlock()
	return nil
}

func (w *memoryWhitelist) Remove(pks ...cipher.PubKey) error {
	w.mux.Lock()
	for _, pk := range pks {
		delete(w.m, pk)
	}
	w.mux.Unlock()
	return nil
}

// NewCombinedWhitelist returns a combined whitelist.
// 'modI' defines the index of the internal whitelist in which Add and Remove operations are performed on.
// If 'modI < 0', Add and Remove operations are performed on all internal whitelists.
func NewCombinedWhitelist(modI int, lists ...Whitelist) Whitelist {
	if modI >= len(lists) {
		panic(fmt.Errorf("NewCombinedWhitelist: modI > len(lists)"))
	}
	return &combinedWhitelist{
		modI:  modI,
		lists: lists,
	}
}

type combinedWhitelist struct {
	modI  int
	lists []Whitelist
}

func (w *combinedWhitelist) Get(pk cipher.PubKey) (bool, error) {
	for _, list := range w.lists {
		ok, err := list.Get(pk)
		if err != nil {
			return false, err
		}
		if ok {
			return true, nil
		}
	}
	return false, nil
}

func (w *combinedWhitelist) All() (map[cipher.PubKey]bool, error) {
	all := make(map[cipher.PubKey]bool)
	for _, list := range w.lists {
		pks, err := list.All()
		if err != nil {
			return nil, err
		}
		for pk, ok := range pks {
			if ok {
				all[pk] = ok
			}
		}
	}
	return all, nil
}

func (w *combinedWhitelist) Add(pks ...cipher.PubKey) error {
	// Add to all internal whitelists if modI < 0
	if w.modI < 0 {
		for _, list := range w.lists {
			if err := list.Add(pks...); err != nil {
				return err
			}
		}
		return nil
	}
	// Otherwise, add to the specified internal whitelist at index.
	return w.lists[w.modI].Add(pks...)
}

func (w *combinedWhitelist) Remove(pks ...cipher.PubKey) error {
	// Remove from all internal whitelists if modI < 0
	if w.modI < 0 {
		for _, list := range w.lists {
			if err := list.Remove(pks...); err != nil {
				return err
			}
		}
		return nil
	}
	// Otherwise, remove from the specified internal whitelist at index.
	return w.lists[w.modI].Remove(pks...)
}

type config struct {
	CLIAddr      string         `json:"cliaddr"`
	CLINet       string         `json:"clinet"`
	DmsgDisc     string         `json:"dmsgdisc"`
	DmsgPort     uint16         `json:"dmsgport"`
	DmsgSessions int            `json:"dmsgsessions"`
	SK           cipher.SecKey  `json:"-"`
	SKStr        string         `json:"sk"`
	Wl           cipher.PubKeys `json:"wl"`
}

// // conf to update whitelists
var conf config = config{}

// NewConfigWhitelist creates a config file implementation of a whitelist.
func NewConfigWhitelist(confPath string) (Whitelist, error) {
	confPath, err := filepath.Abs(confPath)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(confPath), 0750); err != nil {
		return nil, err
	}

	return &configWhitelist{confPath: confPath}, nil
}

type configWhitelist struct {
	confPath string
}

func (w *configWhitelist) Get(pk cipher.PubKey) (bool, error) {
	var ok bool

	return ok, nil
}

func (w *configWhitelist) All() (map[cipher.PubKey]bool, error) {
	err := w.open()
	if err != nil {
		return nil, err
	}
	out := make(map[cipher.PubKey]bool)
	for _, k := range conf.Wl {
		out[k] = true
	}
	return out, nil
}

func (w *configWhitelist) Add(pks ...cipher.PubKey) error {
	err := w.open()
	if err != nil {
		return err
	}
	// duplicate flag
	var dFlag bool

	// append new pks to the whitelist slice within the config file
	// for each pk to be added
	var pke []string
	for _, k := range pks {

		dFlag = false
		// check if the pk already exists
		for _, p := range conf.Wl {

			// if it does
			if p == k {
				// flag it
				dFlag = true
				pke = append(pke, p.String())
				fmt.Printf("skipping append for %v. Already exists", k)
				break
			}
		}

		// if pk does already not exist
		if !dFlag {
			// append it
			conf.Wl = append(conf.Wl, k)
		}

	}

	// write the changes back to the config file
	err = updateFile(w.confPath)
	if err != nil {
		log.Println("unable to update config file")
		return err
	}
	if len(pke) != 0 {
		return errors.New("skipping append for " + strings.Join(pke, ",") + ". Already exists")
	}
	return nil
}

func (w *configWhitelist) Remove(pks ...cipher.PubKey) error {
	err := w.open()
	if err != nil {
		return err
	}

	// for each pubkey to be removed
	for _, k := range pks {

		// find occurrence of pubkey in config whitelist
		for i := 0; i < len(conf.Wl); i++ {

			// if an occurrence is found
			if k == conf.Wl[i] {
				// remove element
				conf.Wl = append(conf.Wl[:i], conf.Wl[i+1:]...)
				break
			}
		}
	}

	// write changes back to the config file
	err = updateFile(w.confPath)
	if err != nil {
		log.Println("unable to update config file")
		return err
	}
	return nil
}

func (w *configWhitelist) open() error {
	if _, err := os.Stat(w.confPath); err != nil {
		return err
	}
	// read file using ioutil
	file, err := ioutil.ReadFile(w.confPath)
	if err != nil {
		return err
	}
	// store config.json into conf to manipulate whitelists
	err = json.Unmarshal(file, &conf)
	if err != nil {
		return err
	}
	return nil
}

// updateFile writes changes to config file
func updateFile(confPath string) error {

	// marshal content
	b, err := json.MarshalIndent(conf, "", "  ")
	if err != nil {
		return err
	}

	// write to config file
	err = ioutil.WriteFile(confPath, b, 0600)
	if err != nil {
		return err
	}

	return nil
}
