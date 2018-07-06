// Copyright 2017 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build darwin dragonfly freebsd linux netbsd openbsd solaris

package gocore

import (
	"fmt"
	"reflect"
	"testing"

	"golang.org/x/debug/internal/core"
)

// loadTest loads a simple core file which resulted from running the
// following program on linux/amd64 with go 1.9.0 (the earliest supported runtime):
// package main
// func main() {
//         _ = *(*int)(nil)
// }
func loadExample(t *testing.T) *Process {
	c, err := core.Core("testdata/core", "testdata", "")
	if err != nil {
		t.Fatalf("can't load test core file: %s", err)
	}
	p, err := Core(c)
	if err != nil {
		t.Fatalf("can't parse Go core: %s", err)
	}
	return p
}

func loadExampleVersion(t *testing.T, version string) *Process {
	if version == "1.9" {
		version = ""
	}
	c, err := core.Core(fmt.Sprintf("testdata/core%s", version), "testdata", "")
	if err != nil {
		t.Fatalf("can't load test core file: %s", err)
	}
	p, err := Core(c)
	if err != nil {
		t.Fatalf("can't parse Go core: %s", err)
	}
	return p
}

func TestObjects(t *testing.T) {
	p := loadExample(t)
	n := 0
	p.ForEachObject(func(x Object) bool {
		n++
		return true
	})
	if n != 104 {
		t.Errorf("#objects = %d, want 104", n)
	}
}

func TestRoots(t *testing.T) {
	p := loadExample(t)
	n := 0
	p.ForEachRoot(func(r *Root) bool {
		n++
		return true
	})
	if n != 257 {
		t.Errorf("#roots = %d, want 257", n)
	}
}

// TestConfig checks the configuration accessors.
func TestConfig(t *testing.T) {
	p := loadExample(t)
	if v := p.BuildVersion(); v != "go1.9" {
		t.Errorf("version=%s, wanted go1.9", v)
	}
	if n := p.Stats().Size; n != 2732032 {
		t.Errorf("all stats=%d, want 2732032", n)
	}
}

func TestFindFunc(t *testing.T) {
	p := loadExample(t)
	a := core.Address(0x404000)
	f := p.FindFunc(a)
	if f == nil {
		t.Errorf("can't find function at %x", a)
		return
	}
	if n := f.Name(); n != "runtime.recvDirect" {
		t.Errorf("funcname(%x)=%s, want runtime.recvDirect", a, n)
	}
}

func TestTypes(t *testing.T) {
	p := loadExample(t)
	// Check the type of a few objects.
	for _, s := range [...]struct {
		addr   core.Address
		size   int64
		kind   Kind
		name   string
		repeat int64
	}{
		{0xc420000480, 384, KindStruct, "runtime.g", 1},
		{0xc42000a020, 32, KindPtr, "*runtime.g", 4},
		{0xc420082000, 96, KindStruct, "hchan<bool>", 1},
		{0xc420062000, 64, KindStruct, "runtime._defer", 1},
	} {
		x, i := p.FindObject(s.addr)
		if x == 0 {
			t.Errorf("can't find object at %x", s.addr)
			continue
		}
		if i != 0 {
			t.Errorf("offset(%x)=%d, want 0", s.addr, i)
		}
		if p.Size(x) != s.size {
			t.Errorf("size(%x)=%d, want %d", s.addr, p.Size(x), s.size)
		}
		typ, repeat := p.Type(x)
		if typ.Kind != s.kind {
			t.Errorf("kind(%x)=%s, want %s", s.addr, typ.Kind, s.kind)
		}
		if typ.Name != s.name {
			t.Errorf("name(%x)=%s, want %s", s.addr, typ.Name, s.name)
		}
		if repeat != s.repeat {
			t.Errorf("repeat(%x)=%d, want %d", s.addr, repeat, s.repeat)
		}

		y, i := p.FindObject(s.addr + 1)
		if y != x {
			t.Errorf("can't find object at %x", s.addr+1)
		}
		if i != 1 {
			t.Errorf("offset(%x)=%d, want i", s.addr, i)
		}
	}
}

func TestReverse(t *testing.T) {
	p := loadExample(t)

	// Build the pointer map.
	// m[x]=y means address x has a pointer to address y.
	m1 := map[core.Address]core.Address{}
	p.ForEachObject(func(x Object) bool {
		p.ForEachPtr(x, func(i int64, y Object, j int64) bool {
			m1[p.Addr(x).Add(i)] = p.Addr(y).Add(j)
			return true
		})
		return true
	})
	p.ForEachRoot(func(r *Root) bool {
		p.ForEachRootPtr(r, func(i int64, y Object, j int64) bool {
			m1[r.Addr.Add(i)] = p.Addr(y).Add(j)
			return true
		})
		return true
	})

	// Build the same, with reverse entries.
	m2 := map[core.Address]core.Address{}
	p.ForEachObject(func(y Object) bool {
		p.ForEachReversePtr(y, func(x Object, r *Root, i, j int64) bool {
			if r != nil {
				m2[r.Addr.Add(i)] = p.Addr(y).Add(j)
			} else {
				m2[p.Addr(x).Add(i)] = p.Addr(y).Add(j)
			}
			return true
		})
		return true
	})

	if !reflect.DeepEqual(m1, m2) {
		t.Errorf("forward and reverse edges don't match")
	}
}

func TestDynamicType(t *testing.T) {
	p := loadExample(t)
	for _, g := range p.Globals() {
		if g.Name == "runtime.indexError" {
			d := p.DynamicType(g.Type, g.Addr)
			if d.Name != "runtime.errorString" {
				t.Errorf("dynamic type wrong: got %s want runtime.errorString", d.Name)
			}
		}
	}
}

func TestVersions(t *testing.T) {
	loadExampleVersion(t, "1.10")
}
