// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package escape

import (
	"cmd/compile/internal/base"
	"cmd/compile/internal/ir"
	"cmd/compile/internal/logopt"
	"cmd/compile/internal/types"
	"cmd/internal/src"
	"fmt"
	"strings"
)

// walkAll computes the minimal dereferences between all pairs of
// locations.
func (b *batch) walkAll() {
	// We use a work queue to keep track of locations that we need
	// to visit, and repeatedly walk until we reach a fixed point.
	//
	// We walk once from each location (including the heap), and
	// then re-enqueue each location on its transition from
	// !persists->persists and !escapes->escapes, which can each
	// happen at most once. So we take Θ(len(e.allLocs)) walks.

	// LIFO queue, has enough room for e.allLocs and e.heapLoc.
	todo := make([]*location, 0, len(b.allLocs)+1)
	enqueue := func(loc *location) {
		if !loc.queued {
			todo = append(todo, loc)
			loc.queued = true
		}
	}

	for _, loc := range b.allLocs {
		enqueue(loc)
	}
	enqueue(&b.mutatorLoc)
	enqueue(&b.calleeLoc)
	enqueue(&b.heapLoc)

	var walkgen uint32
	for len(todo) > 0 {
		root := todo[len(todo)-1]
		todo = todo[:len(todo)-1]
		root.queued = false

		walkgen++
		b.walkOne(root, walkgen, enqueue)
	}
}

// walkOne computes the minimal number of dereferences from root to
// all other locations.
func (b *batch) walkOne(root *location, walkgen uint32, enqueue func(*location)) {
	// The data flow graph has negative edges (from addressing
	// operations), so we use the Bellman-Ford algorithm. However,
	// we don't have to worry about infinite negative cycles since
	// we bound intermediate dereference counts to 0.

	root.walkgen = walkgen
	root.derefs = 0
	root.dst = nil

	if root.hasAttr(attrCalls) {
		if clo, ok := root.n.(*ir.ClosureExpr); ok {
			if fn := clo.Func; b.inMutualBatch(fn.Nname) && !fn.ClosureResultsLost() {
				fn.SetClosureResultsLost(true)

				// Re-flow from the closure's results, now that we're aware
				// we lost track of them.
				for _, result := range fn.Type().Results() {
					enqueue(b.oldLoc(result.Nname.(*ir.Name)))
				}
			}
		}
	}

	todo := []*location{root} // LIFO queue
	for len(todo) > 0 {
		l := todo[len(todo)-1]
		todo = todo[:len(todo)-1]

		derefs := l.derefs
		var newAttrs locAttr

		// If l.derefs < 0, then l's address flows to root.
		addressOf := derefs < 0
		if addressOf {
			// For a flow path like "root = &l; l = x",
			// l's address flows to root, but x's does
			// not. We recognize this by lower bounding
			// derefs at 0.
			derefs = 0

			// If l's address flows somewhere that
			// outlives it, then l needs to be heap
			// allocated.
			if b.outlives(root, l) {
				if !l.hasAttr(attrEscapes) && (logopt.Enabled() || base.Flag.LowerM >= 2) {
					if base.Flag.LowerM >= 2 {
						fmt.Printf("%s: %v escapes to heap:\n", base.FmtPos(l.n.Pos()), l.n)
					}
					explanation := b.explainPath(root, l) //root<--l (root=&l)
					if logopt.Enabled() {
						var e_curfn *ir.Func // TODO(mdempsky): Fix.
						logopt.LogOpt(l.n.Pos(), "escape", "escape", ir.FuncName(e_curfn), fmt.Sprintf("%v escapes to heap", l.n), explanation)
					}
				}
				newAttrs |= attrEscapes | attrPersists | attrMutates | attrCalls
			} else
			// If l's address flows to a persistent location, then l needs
			// to persist too.
			if root.hasAttr(attrPersists) {
				newAttrs |= attrPersists
			}
		}

		if derefs == 0 {
			newAttrs |= root.attrs & (attrMutates | attrCalls)
		}

		// l's value flows to root. If l is a function
		// parameter and root is the heap or a
		// corresponding result parameter, then record
		// that value flow for tagging the function
		// later.
		if l.isName(ir.PPARAM) {
			if b.outlives(root, l) {
				if !l.hasAttr(attrEscapes) && (logopt.Enabled() || base.Flag.LowerM >= 2) {
					if base.Flag.LowerM >= 2 {
						fmt.Printf("%s: parameter %v leaks to %s with derefs=%d:\n", base.FmtPos(l.n.Pos()), l.n, b.explainLoc(root), derefs)
					}
					is_parameter_leaks = true
					EscReason = E_UNKNOWN
					explanation := b.explainPath(root, l)
					is_parameter_leaks = false
					if logopt.Enabled() {
						var e_curfn *ir.Func // TODO(mdempsky): Fix.
						logopt.LogOpt(l.n.Pos(), "leak", "escape", ir.FuncName(e_curfn),
							fmt.Sprintf("parameter %v leaks to %s with derefs=%d", l.n, b.explainLoc(root), derefs), explanation)
					}
				}
				l.leakTo(root, derefs, EscReason)
			}
			if root.hasAttr(attrMutates) {
				l.paramEsc.AddMutator(derefs, EscReason)
			}
			if root.hasAttr(attrCalls) {
				l.paramEsc.AddCallee(derefs, EscReason)
			}
		}

		if newAttrs&^l.attrs != 0 {
			l.attrs |= newAttrs
			enqueue(l)
			if l.attrs&attrEscapes != 0 {
				continue
			}
		}

		for i, edge := range l.edges {
			if edge.src.hasAttr(attrEscapes) {
				continue
			}
			d := derefs + edge.derefs
			if edge.src.walkgen != walkgen || edge.src.derefs > d {
				edge.src.walkgen = walkgen
				edge.src.derefs = d
				edge.src.dst = l
				edge.src.dstEdgeIdx = i
				todo = append(todo, edge.src)
			}
		}
	}
}

// explainPath prints an explanation of how src flows to the walk root.
func (b *batch) explainPath(root, src *location) []*logopt.LoggedOpt {

	// 进入这个函数，可能有for循环的边
	is_not_1_edge = true
	whys = []one_why{}
	escape_paths = []*location{}

	visited := make(map[*location]bool)
	pos := base.FmtPos(src.n.Pos())
	var explanation []*logopt.LoggedOpt
	for {
		// Prevent infinite loop.
		if visited[src] {
			if base.Flag.LowerM >= 2 {
				fmt.Printf("%s:   warning: truncated explanation due to assignment cycle; see golang.org/issue/35518\n", pos)
			}
			break
		}
		visited[src] = true
		dst := src.dst
		edge := &dst.edges[src.dstEdgeIdx]
		if edge.src != src {
			base.Fatalf("path inconsistency: %v != %v", edge.src, src)
		}

		explanation = b.explainFlow(pos, dst, src, edge.derefs, edge.notes, explanation)
		if is_not_1_edge { // 修改1
			b.recordInfo(dst, src, edge.notes)
		}

		if dst == root {
			break
		}
		src = dst
	}

	b.countAll() // 修改2
	return explanation
}

var this_stmt_is_go_defer bool = false // 用于记录当前分析的变量是不是go语句或者defer语句，这个在stmt.go的192行修改
var is_not_1_edge bool = false         // 是不是一条边，如果explainPath0函数由graph.go调用则只有一条边，否则两条边
var whys = []one_why{}                 // 用于记录所有原因
var escape_paths = []*location{}       // 按顺序记录逃逸路径，[0]是逃逸节点,[len-1]是逃逸的去向
var one_escape_func = []one_escape{}   // 用于记录所有的逃逸节点以及其逃逸原因，每次只记录一个函数的所有节点
var output_flow bool = true            // 是否输出详细的flow

var is_parameter_leaks bool = false // 当前变量是不是不用统计的函数参数类型
var lvalue_is_map bool = false      //左边变量是一个

var EscReason ESCAPE_TYPE //用于向leaks.go传递逃逸类型

type one_why struct {
	why     string    // 一个why
	where   *ir.Node  // 哪个语句发生了逃逸
	srcLoc  *location // 有向边的起初节点
	dstLoc  *location // 有向边的终止节点
	srcName string
	dstName string
}

type one_escape struct {
	srcLoc  *location   // 逃逸节点
	dstLoc  *location   // 逃逸目的节点
	why     ESCAPE_TYPE // 逃逸原因
	srcName string
	dstName string
	//a--> b--> c,在one_why（存单独每一跳）中记录为a--> b, b--> c ,在one_escape中记录为a--> c
}

type ESCAPE_TYPE int

const (
	E_NOT       ESCAPE_TYPE = iota // 没找到逃逸
	E_RETURN                       // 返回值指针
	E_LAREG                        // 过大的make
	E_DYNAMIC                      // make动态赋值
	E_GLOBAL                       // 全局变量引用
	E_INDIRECT                     // 间接
	E_OUTERLOOP                    // 外层循环
	//E_FUNCPARAM                     // 函数调用
	E_COROUTINE // 协程
	E_MAPINDEX  // MapIndex类型的
	E_CALLPARAM // 被调用导致逃逸

	E_CLOSURE    // 闭包
	E_CO_CLOSURE // 协程调用所需要的闭包
	E_UNKNOWN
)

// 定义的所有种类的个数
type all_count struct {
	c_retrun        int // return型的个数
	c_too_large     int // 超出栈空间
	c_dynamic_alloc int // 动态分配空间
	c_global_ref    int // 被全局变量引用
	c_indirect_ref  int // 被间接引用
	c_outerloop_ref int // 被外层循环使用
	c_func_param    int // 函数参数引用
	c_callparam     int // 被函数调用
	c_closure       int // 闭包类型逃逸
	c_coroutine     int // 协程类型逃逸
	c_co_closure    int // 由于协程调用需要的闭包
	c_mapindex      int // MapIndex类型

	c_unknown int // 未知
}

// 全局变量，输出个数
var ac = all_count{
	c_retrun:        0,
	c_too_large:     0,
	c_dynamic_alloc: 0,
	c_global_ref:    0,
	c_indirect_ref:  0,
	c_outerloop_ref: 0,
	c_func_param:    0,
	c_callparam:     0,
	c_closure:       0,
	c_coroutine:     0,
	c_co_closure:    0,
	c_mapindex:      0,

	c_unknown: 0,
}

// 记录whys信息，并摘出逃逸节点到escape_paths
func (b *batch) recordInfo(dstLoc, srcLoc *location, notes *note) {
	for n := notes; n != nil; n = n.next {
		clonedWhy := strings.Clone(n.why) // 完全复制字节，不再共享内存

		clonedSrcName := b.explainLoc(srcLoc)
		clonedDstName := b.explainLoc(dstLoc)

		// 构造要追加的 one_why 实例
		entry := one_why{
			srcLoc:  srcLoc,
			dstLoc:  dstLoc,
			why:     clonedWhy,
			where:   &n.where,
			srcName: clonedSrcName,
			dstName: clonedDstName,
		}
		// 追加到切片末尾
		whys = append(whys, entry)
	}

	//whys记录a->b,b->c，escape_paths把每次whys的目的节点记录下来，即a,b,c

	// 记录逃逸的节点
	if len(escape_paths) == 0 {
		escape_paths = append(escape_paths, srcLoc, dstLoc)
	} else {
		escape_paths = append(escape_paths, dstLoc)
	}

}

// 输出打印逃逸信息，记录逃逸信息到one_escape_func
func (b *batch) recordEscapeInfo(srcLoc, dstLoc *location, whyx ESCAPE_TYPE, bytesize int64, escapetype string) {

	// 然后对不同种类进行记录和输出
	switch whyx {
	case E_RETURN:
		ac.c_retrun++
		fmt.Printf("\033[32mmy return escape count %d , escape size: %d , escape type: %s\033[0m\n", ac.c_retrun, bytesize, escapetype)
	case E_LAREG:
		ac.c_too_large++
		fmt.Printf("\033[32mmy too large escape count %d , escape size: %d , escape type: %s\033[0m\n", ac.c_too_large, bytesize, escapetype)
	case E_DYNAMIC:
		ac.c_dynamic_alloc++
		fmt.Printf("\033[32mmy dynamic alloc escape count %d , escape size: %d , escape type: %s\033[0m\n", ac.c_dynamic_alloc, bytesize, escapetype)
	case E_GLOBAL:
		ac.c_global_ref++
		fmt.Printf("\033[32mmy global ref escape count %d , escape size: %d , escape type: %s\033[0m\n", ac.c_global_ref, bytesize, escapetype)
	case E_OUTERLOOP:
		ac.c_outerloop_ref++
		fmt.Printf("\033[32mmy outerloop ref escape count %d , escape size: %d , escape type: %s\033[0m\n", ac.c_outerloop_ref, bytesize, escapetype)
	case E_INDIRECT:
		ac.c_indirect_ref++
		fmt.Printf("\033[32mmy indirect ref escape count %d , escape size: %d , escape type: %s\033[0m\n", ac.c_indirect_ref, bytesize, escapetype)
	case E_CLOSURE:
		// 这里就是普通的closure逃逸
		ac.c_closure++
		fmt.Printf("\033[32mmy clousure ref escape count %d , escape size: %d , escape type: %s\033[0m\n", ac.c_closure, bytesize, escapetype)
	case E_COROUTINE:
		ac.c_coroutine++
		fmt.Printf("\033[32mmy coroutine ref escape count %d , escape size: %d , escape type: %s\033[0m\n", ac.c_coroutine, bytesize, escapetype)
	case E_CO_CLOSURE:
		// 这里的闭包是因为协程调用导致的，所有后面对这个逃逸的，一定是协程导致的
		ac.c_co_closure++
	//	fmt.Printf("my coroutine_closure ref escape count %d , escape size: %d , escape type: %s\n", ac.c_co_closure, bytesize, escapetype)
	// case E_FUNCPARAM:
	// 	ac.c_func_param++
	// 	fmt.Printf("my func_param ref escape count %d , escape size: %d , escape type: %s\n", ac.c_func_param, bytesize, escapetype)
	// case E_CALLPARAM:
	// 	ac.c_callparam++
	// 	fmt.Printf("\033[32mmy callparam ref escape count %d , escape size: %d , escape type: %s\n", ac.c_callparam, bytesize, escapetype)
	case E_MAPINDEX:
		ac.c_mapindex++
		fmt.Printf("\033[32mmy mapindex ref escape count %d , escape size: %d , escape type: %s\033[0m\n", ac.c_mapindex, bytesize, escapetype)

	case E_UNKNOWN:
		// 未知类型
		ac.c_unknown++
		fmt.Printf("\033[32mmy unKnown escape count %d , escape size: %d , escape type: %s\033[0m\n", ac.c_unknown, bytesize, escapetype)
	default:
		fmt.Printf("\033[32mswitch defalut:%d\033[0m\n", whyx)
	}

	clonedSrcName := b.explainLoc(srcLoc)
	clonedDstName := b.explainLoc(dstLoc)
	// 构造要追加的 one_why 实例
	entry := one_escape{
		srcLoc:  srcLoc,
		dstLoc:  dstLoc,
		why:     whyx,
		srcName: clonedSrcName,
		dstName: clonedDstName,
	}
	// 追加到切片末尾
	one_escape_func = append(one_escape_func, entry)
}

// 找已经知道的节点的逃逸情况，如果没有，输出空""，如果有则输出节点和逃逸的类型
func (b *batch) find_escape(dstLoc *location) one_escape {
	var i one_escape
	is_find := false

	// 如果不是一个符号而是堆或者空等等
	// if dstLoc.n == nil || dstLoc == &b.heapLoc {
	// 	return one_escape{
	// 		why: E_NOT,
	// 	}
	// }
	// A -> B -> C -> D
	// 第一次 A -> B -> C  存 A --> C  C逃逸
	// 第二次 C -> D 去找C的逃逸情况  找到了C的逃逸情况，那么D的逃逸情况就是C的逃逸情况
	// 只有我找的节点是一个正常的变量，再去找
	for _, i = range one_escape_func {
		// 直接找节点指向的是不是同一个而不是找名字
		if dstLoc.n == i.srcLoc.n {
			// 找新的节点的逃逸的节点等于现在这个节点未逃逸的，则找到
			is_find = true
			break
		}
	}
	if is_find {
		return i
	} else {
		return one_escape{
			why: E_NOT,
		}
	}
}

func (b *batch) lhs_is_oname(n *ir.Node) bool {
	_, ok1 := (*n).(*ir.Name)
	if ok1 {
		return true
	} else {
		return false
	}
}

func (b *batch) node_is_indirect_lvalue(n *ir.Node) bool {
	_, ok1 := (*n).(*ir.StarExpr)
	_, ok2 := (*n).(*ir.SelectorExpr)
	_, ok3 := (*n).(*ir.IndexExpr)

	if ok1 || ok2 || ok3 {
		return true
	} else {
		return false
	}
}

// 返回一个任意lhs种类的ir.Node的ir.Name字段
// 间接访存用到，与lhs_is_oname函数一起判断左值是否是合法的，递归找左值变量名
func (b *batch) find_Node_Name(n *ir.Node) *ir.Name {
	v1, ok1 := (*n).(*ir.Name)
	if ok1 {
		return v1
	}

	v2, ok2 := (*n).(*ir.StarExpr)
	if ok2 {
		return b.find_Node_Name(&v2.X)
	}

	v3, ok3 := (*n).(*ir.SelectorExpr)
	if ok3 {
		return b.find_Node_Name(&v3.X)
	}

	v4, ok4 := (*n).(*ir.IndexExpr)
	if ok4 {
		return b.find_Node_Name(&v4.X)
	}

	v5, ok5 := (*n).(*ir.ConvExpr)
	if ok5 {
		return b.find_Node_Name(&v5.X)
	}

	v6, ok6 := (*n).(*ir.TypeAssertExpr)
	if ok6 {
		return b.find_Node_Name(&v6.X)
	}

	return nil
}

// 确定右值是不是取地址类型的

// storesAddress 判断 v 的底层种类是否为引用或指针类型
func (b *batch) storesAddress(v *ir.Node) bool {
	//t := reflect.TypeOf(v)
	// if t == nil {
	// 	return false // nil 接口
	// }
	switch (*v).Type().Kind() {
	case types.TPTR, types.TUNSAFEPTR,
		types.TUINTPTR, types.TMAP,
		types.TCHAN, types.TSLICE,
		types.TSTRING,
		types.TINTER, types.TFUNC:
		return true
	// case reflect.Ptr, reflect.UnsafePointer,
	// 	reflect.Slice, reflect.Map,
	// 	reflect.Chan, reflect.Func,
	// 	reflect.Interface:
	//	return true
	default:
		return false
	}
}

// 用于输出逃逸变量的三种类型，map为1，slice为2，其余为3
func (b *batch) judgeType(v ir.Node) string {

	switch v.Type().Kind() {
	case types.TMAP:
		return "map"
	case types.TSLICE:
		return "slice"
	default:
		return "oname"
	}
}

func (b *batch) rvalue_is_addr(n *ir.Node) bool {
	_, ok1 := (*n).(*ir.AddrExpr)
	if ok1 {
		return true
	}

	v2, ok2 := (*n).(*ir.StarExpr)
	if ok2 {
		return b.rvalue_is_addr(&v2.X)
	}

	v3, ok3 := (*n).(*ir.SelectorExpr)
	if ok3 {
		return b.rvalue_is_addr(&v3.X)
	}

	v4, ok4 := (*n).(*ir.IndexExpr)
	if ok4 {
		return b.rvalue_is_addr(&v4.X)
	}

	v5, ok5 := (*n).(*ir.ConvExpr)
	if ok5 {
		return b.rvalue_is_addr(&v5.X)
	}

	v6, ok6 := (*n).(*ir.SliceExpr)
	if ok6 {
		return b.rvalue_is_addr(&v6.X)
	}

	v7, ok7 := (*n).(*ir.CompLitExpr)
	if ok7 {
		return b.rvalue_is_addr(&v7.RType)
	}

	return false
}

var strToEscType = map[string]ESCAPE_TYPE{
	"E_NOT":        E_NOT,
	"E_RETURN":     E_RETURN,
	"E_LAREG":      E_LAREG,
	"E_DYNAMIC":    E_DYNAMIC,
	"E_GLOBAL":     E_GLOBAL,
	"E_INDIRECT":   E_INDIRECT,
	"E_OUTERLOOP":  E_OUTERLOOP,
	"E_COROUTINE":  E_COROUTINE,
	"E_MAPINDEX":   E_MAPINDEX,
	"E_CLOSURE":    E_CLOSURE,
	"E_CO_CLOSURE": E_CO_CLOSURE,
	"E_UNKNOWN":    E_UNKNOWN,
}

// 遍历whys，计算一个变量逃逸的情况
func (b *batch) countAll() {
	//fmt.Printf("Start Count\n")

	var haven_find_escape bool = false // 表示找到了逃逸的原因
	var escape_reason ESCAPE_TYPE      // 当前变量的逃逸原因

	whys_len := len(whys) - 1 // whys的长度减一，用于索引
	if whys_len < 0 {
		//b.recordEscapeInfo(whys[0].srcLoc, whys[len(whys)-1].dstLoc, E_UNKNOWN)
		is_not_1_edge = false
		return
	}

	// is_parameter_leaks之=指示现在是参数逃逸，但是我现在需要记录参数逃逸，因为之后的节点可能用到现在这个参数节点的信息
	//if !is_parameter_leaks && whys_len > 0 && whys[whys_len-1].why == "call parameter" {
	if whys_len > 0 && whys[whys_len-1].why == "call parameter" {
		//b.recordEscapeInfo(escape_paths[0], escape_paths[len(escape_paths)-1], strToEscType[whys[whys_len].why], escape_paths[0].n.Type().Size(), b.judgeType(escape_paths[0].n))
		is_not_1_edge = false
		escape_reason = strToEscType[whys[whys_len].why]
		haven_find_escape = true
		//return
	}

	if is_parameter_leaks {
		// 如果这个是还是函数参数的，直接不管
		is_not_1_edge = false
		//return
	}

	// 当前节点是不是闭包
	_, escape_is_closure := (whys[0].srcLoc.n).(*ir.ClosureExpr) // 表示当前逃逸的变量是闭包

	// 先找是不是有其他已知的逃逸节点
	ss := b.find_escape(escape_paths[len(escape_paths)-1])
	if ss.why != E_NOT && !haven_find_escape {
		// 不为空，则能找到
		haven_find_escape = true
		escape_reason = ss.why

		// 如果找到是E_CO_CLOSURE而当前逃逸的不是CLOSURE，说明当前节点是因为协程而逃逸的
		if escape_reason == E_CO_CLOSURE && !escape_is_closure {
			escape_reason = E_COROUTINE
		}

	} else {
		/*	第一步确定左值的类型
			   	合法左值有6种类型：
				标识符		k   Name
				指针解引用  *p  StarExpr
				结构体字段选择	X.p SelectorExpr
				数组或切片索引  X[2] IndexExpr
				map索引		    X["index"]	IndexExpr
				括号表达式  编译后没有括号了

				除了Name以外的其他表达式都有一个节点X Node，因此只需要判断出是否是ir.Name即可
		*/

		// 判断是不是闭包类型的
		if !haven_find_escape && escape_is_closure {
			// 认为是闭包类型的，记录
			if this_stmt_is_go_defer {
				// 协程调用导致的闭包
				// 如果当前分析的语句是go或者defer语句，且这个语句对于closure逃逸
				// 由于 go func() 这样的语句，func会全部转化为闭包，因此后面只要对这个闭包逃逸的就是coroutine造成的逃逸
				escape_reason = E_CO_CLOSURE
				haven_find_escape = true

				this_stmt_is_go_defer = false
			} else {
				// 普通闭包
				escape_reason = E_CLOSURE
				haven_find_escape = true
			}
		}

		// 判断是不是堆溢出类型的，如果是，那么则可能是引用全局变量导致的
		// 如果后面找不到其他的溢出可能，那么就是heap
		var haven_heap_escape bool = false // 表示有堆溢出的情况
		if whys[whys_len].dstLoc == &b.heapLoc {
			haven_heap_escape = true
		}

		//panic导致全局
		// if !haven_find_escape && whys[whys_len].why == "E_GLOBAL" {
		// 	escape_reason = E_GLOBAL
		// 	haven_find_escape = true
		// }

		// 堆逃逸的情形分为全局引用和间接引用两种，合在一起判断，均为赋值语句。
		// 赋值目前可能存在2种，一种是AssignStmt，一种是assignListStmt
		if !haven_find_escape && haven_heap_escape {
			ass1, ok1 := (*whys[whys_len].where).(*ir.AssignStmt)
			ass2, ok2 := (*whys[whys_len].where).(*ir.AssignListStmt)
			ass3, ok3 := (*whys[whys_len].where).(*ir.AssignOpStmt)
			if ok1 || ok2 || ok3 {
				var alvalues []ir.Node
				if ok1 {
					alvalues = append(alvalues, ass1.X)
				} else if ok2 {
					alvalues = ass2.Lhs
				} else {
					alvalues = append(alvalues, ass3.X)
				}

				for _, alvalue := range alvalues {
					lhs_name := b.find_Node_Name(&alvalue) // 递归取变量名 x[0].i 取 x
					if lhs_name == nil {
						_, ok := (alvalue).(*ir.LinksymOffsetExpr) // 全局变量偏移量
						if ok {
							escape_reason = E_GLOBAL
							haven_find_escape = true
							break
						} else {
							// 左边是一个map类型的变量，如果是，那么是indirect类型的
							if lvalue_is_map {
								escape_reason = E_INDIRECT
								haven_find_escape = true
								break
							} else {
								escape_reason = E_UNKNOWN
								haven_find_escape = true
							}
						}
					} else if lhs_name.Class != ir.PEXTERN && b.node_is_indirect_lvalue(&alvalue) {
						// 如果是对应的逃逸节点，再去判断是不是全局类型的
						// 判断间接引用
						// 如果左边不是Name类型的，而且左边不是PEXTERN，那么可能是间接引用的，也可能是函数参数的
						escape_reason = E_INDIRECT
						haven_find_escape = true
						break
					} else if lhs_name.Class == ir.PEXTERN {
						// 一定是全局变量类型的
						// 如果有多个变量，我只运行一次，因为一次这个函数调用只会调用一个逃逸的变量
						escape_reason = E_GLOBAL
						haven_find_escape = true
						break
					}
				}
			} else {
				// 还有一种情况，如果节点类型是CompLitExpr，说明是全局变量类型的组合值，一定是全局类型的
				_, ok3 := (*whys[whys_len].where).(*ir.CompLitExpr)
				if ok3 {
					escape_reason = E_GLOBAL
					haven_find_escape = true
				}
			}
		}

		// 如果是返回值类型的
		ass, ok1 := (*whys[whys_len].where).(*ir.ReturnStmt)
		// 看是不是赋值给PPARAMOUT类型的
		ok2 := whys[whys_len].dstLoc.isName(ir.PPARAMOUT)
		if !haven_find_escape && (ok1 || ok2) {
			var arvalues []ir.Node
			if ok1 {
				arvalues = ass.Results
				if whys[whys_len].why == "return" {
					escape_reason = E_RETURN
					haven_find_escape = true
				}
				// for _, arvalue := range ass.Results{
				// 	//arvalues[i].n = arvalue
				// 	arvalues = append(arvalues, &location{n: arvalue})
				// }
			} else { //如果不是return类型，判断是以下三种类型（以下三种语句右值为取地址时，归结为return）
				ar1, rok1 := (*whys[whys_len].where).(*ir.AssignStmt)
				ar2, rok2 := (*whys[whys_len].where).(*ir.AssignListStmt)
				ar3, rok3 := (*whys[whys_len].where).(*ir.AssignOpStmt)

				//var loc *location//临时location
				if rok1 { //arvalues存右值
					arvalues = append(arvalues, ar1.Y)
				} else if rok2 {
					arvalues = ar2.Rhs
					// for _, ar2R := range ar2.Rhs{
					// 	//loc.n = ar2R
					// 	arvalues = append(arvalues, &location{n: ar2R})
					// }
				} else if rok3 {
					//loc.n = ar3.Y
					arvalues = append(arvalues, ar3.Y)
				}

				// 找右值是不是取地址的
				for _, arvalue := range arvalues {
					if b.storesAddress(&arvalue) {
						escape_reason = E_RETURN
						haven_find_escape = true
						break
					}
				}
			}

		}

		// 其他RETURN类型的，这里是call param的
		if whys[whys_len].why == "call parameter" &&
			((whys[0].srcLoc.n != nil && b.storesAddress(&(whys[0].srcLoc.n))) || whys[0].why == "address-of") { // 2种取地址的操作
			escape_reason = E_RETURN
			haven_find_escape = true
		}

		if !haven_find_escape && whys_len >= 1 && whys[whys_len].why == "reference" {
			if whys[whys_len-1].why == "captured by a closure" {
				escape_reason = E_CLOSURE
				haven_find_escape = true
			}
		}

		// send多线程
		if !haven_find_escape && whys[whys_len].why == "send" {
			escape_reason = E_COROUTINE
			haven_find_escape = true
		}

		// 过大的数组
		if !haven_find_escape && whys[whys_len].why == "too large for stack" {
			escape_reason = E_LAREG
			haven_find_escape = true
		}

		// 非常量make
		if !haven_find_escape && whys[whys_len].why == "non-constant size" || whys[whys_len].why == "appendee slice" ||
			whys[whys_len].why == "appended slice..." {
			escape_reason = E_DYNAMIC
			haven_find_escape = true
		}

		// MapIndex
		if !haven_find_escape && whys[whys_len].why == "key of map put" {
			escape_reason = E_MAPINDEX
			haven_find_escape = true
		}

		// 这里用于判断外层循环的，堆泄露节点的loopDepth=0
		if !haven_find_escape && whys[whys_len].dstLoc != &b.heapLoc && (whys[0].srcLoc.loopDepth > whys[whys_len].dstLoc.loopDepth) {
			// 表示循环
			escape_reason = E_OUTERLOOP
			haven_find_escape = true
		}

		// 没找到
		if !haven_find_escape {
			escape_reason = E_UNKNOWN
			haven_find_escape = true
		}
	}

	EscReason = escape_reason //更新逃逸原因，传递给leaks.go
	// 记录逃逸原因
	if !is_parameter_leaks {
		b.recordEscapeInfo(escape_paths[0], escape_paths[len(escape_paths)-1], escape_reason, escape_paths[0].n.Type().Size(), b.judgeType(escape_paths[0].n))
	}

	is_not_1_edge = false
}

func (b *batch) explainFlow(pos string, dst, srcloc *location, derefs int, notes *note, explanation []*logopt.LoggedOpt) []*logopt.LoggedOpt {
	ops := "&"
	if derefs >= 0 {
		ops = strings.Repeat("*", derefs)
	}
	print := base.Flag.LowerM >= 2 && output_flow

	flow := fmt.Sprintf("   flow: %s = %s%v:", b.explainLoc(dst), ops, b.explainLoc(srcloc))
	if print {
		fmt.Printf("%s:%s\n", pos, flow)
	}
	if logopt.Enabled() {
		var epos src.XPos
		if notes != nil {
			epos = notes.where.Pos()
		} else if srcloc != nil && srcloc.n != nil {
			epos = srcloc.n.Pos()
		}
		var e_curfn *ir.Func // TODO(mdempsky): Fix.
		explanation = append(explanation, logopt.NewLoggedOpt(epos, epos, "escflow", "escape", ir.FuncName(e_curfn), flow))
	}

	// 如果我定义的Z小于2，则输出
	if output_flow {
		for note := notes; note != nil; note = note.next {
			if print {
				fmt.Printf("%s:     from %v (%v) at %s\n", pos, note.where, note.why, base.FmtPos(note.where.Pos()))
			}
			if logopt.Enabled() {
				var e_curfn *ir.Func // TODO(mdempsky): Fix.
				notePos := note.where.Pos()
				explanation = append(explanation, logopt.NewLoggedOpt(notePos, notePos, "escflow", "escape", ir.FuncName(e_curfn),
					fmt.Sprintf("     from %v (%v)", note.where, note.why)))
			}
		}
	}

	// 判断种类
	if !is_not_1_edge {
		whys = []one_why{} // 清空
		escape_paths = []*location{}
		// 是一条边，说明在graph.go里调用，记录一组即可，然后直接count
		b.recordInfo(dst, srcloc, notes)
		b.countAll()
	}

	return explanation
}

func (b *batch) explainLoc(l *location) string {
	if l == &b.heapLoc {
		return "{heap}"
	}
	if l.n == nil {
		// TODO(mdempsky): Omit entirely.
		return "{temp}"
	}
	if l.n.Op() == ir.ONAME {
		return fmt.Sprintf("%v", l.n)
	}
	return fmt.Sprintf("{storage for %v}", l.n)
}

// outlives reports whether values stored in l may survive beyond
// other's lifetime if stack allocated.
func (b *batch) outlives(l, other *location) bool {
	// The heap outlives everything.
	if l.hasAttr(attrEscapes) {
		return true
	}

	// Pseudo-locations that don't really exist.
	if l == &b.mutatorLoc || l == &b.calleeLoc {
		return false
	}

	// We don't know what callers do with returned values, so
	// pessimistically we need to assume they flow to the heap and
	// outlive everything too.
	if l.isName(ir.PPARAMOUT) {
		// Exception: Closures can return locations allocated outside of
		// them without forcing them to the heap, if we can statically
		// identify all call sites. For example:
		//
		//	var u int  // okay to stack allocate
		//	fn := func() *int { return &u }()
		//	*fn() = 42
		if containsClosure(other.curfn, l.curfn) && !l.curfn.ClosureResultsLost() {
			return false
		}

		return true
	}

	// If l and other are within the same function, then l
	// outlives other if it was declared outside other's loop
	// scope. For example:
	//
	//	var l *int
	//	for {
	//		l = new(int) // must heap allocate: outlives for loop
	//	}
	if l.curfn == other.curfn && l.loopDepth < other.loopDepth {
		return true
	}

	// If other is declared within a child closure of where l is
	// declared, then l outlives it. For example:
	//
	//	var l *int
	//	func() {
	//		l = new(int) // must heap allocate: outlives call frame (if not inlined)
	//	}()
	if containsClosure(l.curfn, other.curfn) {
		return true
	}

	return false
}

// containsClosure reports whether c is a closure contained within f.
func containsClosure(f, c *ir.Func) bool {
	// Common cases.
	if f == c || c.OClosure == nil {
		return false
	}

	for p := c.ClosureParent; p != nil; p = p.ClosureParent {
		if p == f {
			return true
		}
	}
	return false
}
