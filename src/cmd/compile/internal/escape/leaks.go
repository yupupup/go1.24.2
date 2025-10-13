// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package escape

import (
	"cmd/compile/internal/base"
	//"math"
	"strings"
)

// A leaks represents a set of assignment flows from a parameter to
// the heap, mutator, callee, or to any of its function's (first
// numEscResults) result parameters.
type leaks [8]uint8
//原uint 8位  xxxxxxxx均表示defers，但defers一般不会超过255
//新uint8 xxxx xxxx 前四位给defers，后四位给EscReason进行编码

const (
	leakHeap = iota
	leakMutator
	leakCallee
	leakResult0
)

const numEscResults = len(leaks{}) - leakResult0

// Heap returns the minimum deref count of any assignment flow from l
// to the heap. If no such flows exist, Heap returns -1.
func (l leaks) Heap() int { return l.get(leakHeap) }

// Mutator returns the minimum deref count of any assignment flow from
// l to the pointer operand of an indirect assignment statement. If no
// such flows exist, Mutator returns -1.
func (l leaks) Mutator() int { return l.get(leakMutator) }

// Callee returns the minimum deref count of any assignment flow from
// l to the callee operand of call expression. If no such flows exist,
// Callee returns -1.
func (l leaks) Callee() int { return l.get(leakCallee) }

// Result returns the minimum deref count of any assignment flow from
// l to its function's i'th result parameter. If no such flows exist,
// Result returns -1.
func (l leaks) Result(i int) int { return l.get(leakResult0 + i) }

// AddHeap adds an assignment flow from l to the heap.
func (l *leaks) AddHeap(derefs int, whyesc ESCAPE_TYPE) { l.add(leakHeap, derefs, whyesc) }

// AddMutator adds a flow from l to the mutator (i.e., a pointer
// operand of an indirect assignment statement).
func (l *leaks) AddMutator(derefs int, whyesc ESCAPE_TYPE) { l.add(leakMutator, derefs, whyesc) }

// AddCallee adds an assignment flow from l to the callee operand of a
// call expression.
func (l *leaks) AddCallee(derefs int, whyesc ESCAPE_TYPE) { l.add(leakCallee, derefs, whyesc) }

// AddResult adds an assignment flow from l to its function's i'th
// result parameter.
func (l *leaks) AddResult(i, derefs int, whyesc ESCAPE_TYPE) { l.add(leakResult0+i, derefs, whyesc) }

//get直接取出value-1，代表derefs
//func (l leaks) get(i int) int { return int(l[i] >> 4) - 1 }

// 取 derefs
func (l leaks) get(i int) int {
	d, _ := decode(l[i])
	return d
}

// 取原因
func (l leaks) getReason(i int) ESCAPE_TYPE {
	_, w := decode(l[i])
	return w
}

// 解码
func decode(v uint8) (derefs int, whyesc ESCAPE_TYPE) {
	if v == 0 {
		return -1, EscReason // -1 表示无效
	}
	derefs = int(v>>4) - 1          // 高四位 → derefs
	whyesc = ESCAPE_TYPE(v & 0x0F) // 低四位 → 原因
	return
}

func (l *leaks) add(i, derefs int, whyesc ESCAPE_TYPE) {
	oldD, _ := decode(l[i])
	if oldD < 0 || derefs < oldD {
		l.set(i, derefs, whyesc)
	}
}

//set()存进去时是 derefs+1
//get()取出来时是 value-1

func (l *leaks) set(i, derefs int, whyesc ESCAPE_TYPE) {
	l[i] = encode(derefs, whyesc)
	/*旧版
	v := derefs + 1
	if v < 0 {
		base.Fatalf("invalid derefs count: %v", derefs)
	}
	if v > 15 {//压缩高四位编码derefs
		v = 15
	}
	if int(whyesc) > 15 {
    base.Fatalf("invalid escape reason: %v", whyesc)
}


	// 高 4 位存 derefs，低 4 位存逃逸原因
	l[i] = uint8((v << 4) | (int(whyesc) & 0x0F))
	*/

	//l[i] = uint8(v)
}

// 编码
func encode(derefs int, whyesc ESCAPE_TYPE) uint8 {
	v := derefs + 1
	if v < 0 {
		base.Fatalf("invalid derefs: %v", derefs)
	}
	if v > 15 { // 高 4 位最大 15
		v = 15
	}
	if int(whyesc) > 15 {
		base.Fatalf("invalid escape reason: %v", whyesc)
	}
	return uint8((v << 4) | (int(whyesc) & 0x0F))
}

// Optimize removes result flow paths that are equal in length or
// longer than the shortest heap flow path.
func (l *leaks) Optimize() {
	// If we have a path to the heap, then there's no use in
	// keeping equal or longer paths elsewhere.
	if x := l.Heap(); x >= 0 {
		for i := 1; i < len(*l); i++ {
			if l.get(i) >= x {
				l.set(i, -1, EscReason)
			}
		}
	}
}

var leakTagCache = map[leaks]string{}

// Encode converts l into a binary string for export data.
func (l leaks) Encode() string {
	if l.Heap() == 0 {
		// Space optimization: empty string encodes more
		// efficiently in export data.
		return ""
	}
	if s, ok := leakTagCache[l]; ok {
		return s
	}

	n := len(l)
	for n > 0 && l[n-1] == 0 {
		n--
	}
	s := "esc:" + string(l[:n])
	leakTagCache[l] = s
	return s
}

// parseLeaks parses a binary string representing a leaks.
//Encode()的逆过程
func parseLeaks(s string) leaks {
	var l leaks
	if !strings.HasPrefix(s, "esc:") {
		l.AddHeap(0, EscReason)
		return l
	}

	// copy(l[:], s[4:])
	// return l

	data := []byte(s[4:]) // 原始字节
	for i, v := range data {
		if v == 0 {
			continue
		}
		d, w := decode(v)
		l.set(i, d, w)
	}
	return l
}
