// +build !noasm !appengine
// Code generated by asm2asm, DO NOT EDIT.

package sse

import (
	`github.com/bytedance/sonic/loader`
)

const (
    _entry__lookup_small_key = 48
)

const (
    _stack__lookup_small_key = 88
)

const (
    _size__lookup_small_key = 876
)

var (
    _pcsp__lookup_small_key = [][2]uint32{
        {0x1, 0},
        {0x6, 8},
        {0x8, 16},
        {0xa, 24},
        {0xc, 32},
        {0xd, 40},
        {0x11, 48},
        {0x361, 88},
        {0x362, 48},
        {0x364, 40},
        {0x366, 32},
        {0x368, 24},
        {0x36a, 16},
        {0x36b, 8},
        {0x36c, 0},
    }
)

var _cfunc_lookup_small_key = []loader.CFunc{
    {"_lookup_small_key_entry", 0,  _entry__lookup_small_key, 0, nil},
    {"_lookup_small_key", _entry__lookup_small_key, _size__lookup_small_key, _stack__lookup_small_key, _pcsp__lookup_small_key},
}
