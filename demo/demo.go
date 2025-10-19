package demo

import "fmt"

var sink interface{}

var p1 *int
var p2 **int

/*
var sink interface{}

func f() {
	i := 0
	p0 := &i
	p1 := &p0
	p2 := &p1
	sink = &p2
}

*/
/*
func mapIndex() {
	map_test := make(map[string]int, 10)
	x := "11"
	x = x + "test"
	map_test[x] = 19
}

/*
var sink *int

func callee(p *int){
	sink = p
}

func call(){
	x := 10
	p := &x
	callee(p)
}

/*
func referencedByGlobal() {
	i := 10
	p := &i
	sink, p2 = p, &p
}

func returnAddress() **int {
	i := 10
	p1 := &i
	p2 := &p1
	return p2
}

func variableSize() {
	x := 10
	// 切片的大小在运行时确定，可能会逃逸到堆上
	slice := make([]int, x)
	_ = slice
}
/*
var sink interface{}


// E_LAREG: 大对象逃逸（通过参数传递大切片指针）
func callee4(p *[]int) {
	_ = (*p)[0] // 仅访问，不修改
}

func caller4() {
	// 分配一个超大切片（1<<20 = 1048576 个元素 ≈ 8MB）
	large := make([]int, 1<<20) // ERROR "moved to heap: large"
	callee4(&large)
}


// E_DYNAMIC: 变长对象逃逸（通过参数传递切片并扩容）
func callee5(p *[]int) {
	*p = append(*p, 42) // append 可能导致扩容
}

func caller5() {
	s := make([]int, 2) // 初始容量较小
	callee5(&s)         // ERROR "moved to heap: s"
}
*/
//******************************以下是caller-callee逃逸案例

func print() {
	fmt.Print("bug")
	fmt.Printf("bug,%s", "test")
}

/*
type buffer []byte

type kkk struct {
	buf *buffer

	wid  int // width
	prec int // precision

	// intbuf is large enough to store %b of an int64 with a sign and
	// avoids padding at the end of the struct on 32 bit architectures.
	intbuf [68]byte
}

// writePadding generates n bytes of padding.
func (f *kkk) writePadding(n int) {
	if n <= 0 { // No padding bytes needed.
		return
	}
	buf := *f.buf
	oldLen := len(buf)
	newLen := oldLen + n
	// Make enough room for padding.
	if newLen > cap(buf) {
		buf = make(buffer, cap(buf)*2+n)
		copy(buf, *f.buf)
	}
	// Decide which byte the padding should be filled with.
	padByte := byte(' ')
	// Zero padding is allowed only to the left.
	padByte = byte('0')
	// Fill padding with padByte.
	padding := buf[oldLen:newLen]
	for i := range padding {
		padding[i] = padByte
	}
	*f.buf = buf[:newLen]
}

func (f *kkk) pad(b []byte) {

	width := 10

	f.writePadding(width)

}

/*
func callee(p *int) **int { //Rtn
	return &p
}

func caller() {
	i := 0 // ERROR "moved to heap: i$"
	_ = callee(&i)
}
*/
/*
type Person struct {
	Name string  // 姓名
	Age  int     // 年龄
	Tall float64 // 身高
}

func main() {

	var p Person
	more := make([]Person, 0, 20)
	more = append(more, Person{
		Name: "Alice" + p.Name + "sdfgdsfgds",
		Age:  25,
		Tall: 1.68,
	})

}
*/
/*
var g *int
func callee3(p *int) {//Glb
	g = p
}

func caller3() {
	i := 0 // ERROR "moved to heap: i"
	callee3(&i)
}

// E_COROUTINE: 参数传入协程闭包
func callee6(p *int) {
	go func() {
		_ = *p
	}()
}
func caller6() {
	x := 0 // ERROR "moved to heap: x"
	callee6(&x)

	panic("arena double free")
}
*/

// func callee6(stackBuf []uintptr) {
// 	//panic("arena double free")
// 	_ = stackBuf
// 	//stackBuf := make([]uintptr, traceStackSize)
// 	//_ = stackBuf
// }
// const traceStackSize = 128
// func caller6() {
// 	//panic("arena double free")

// 	stackBuf := make([]uintptr, traceStackSize)
// 	callee6(stackBuf)         // ERROR "moved to heap: stackBuf"
// }

/*

// E_LAREG: 大对象逃逸（通过参数传入大切片指针）
func callee1(p *[]int) {
	_ = *p
}

func caller1() {
	// 超大切片
	large := make([]int, 1<<20) // ERROR "moved to heap: large"
	callee1(&large)
}
*/

/*
func calleeVar(s *[]int) *[]int {
	*s = append(*s, 42) // append 可能触发扩容
	return s
}

func callerVar() {
	s := make([]int, 2) // 初始容量较小
	_ = calleeVar(&s)   // ERROR "moved to heap: s$"
}

/*
func calleeDynamic(s []int) {
	// 被调函数内部 append 导致可能扩容
	s = append(s, 1<<20)
	_ = s
}

func callerDynamic() {
	// 初始切片较小
	small := make([]int, 2)
	calleeDynamic(small) // ERROR "moved to heap: small"
}

/*
func callee1(p *[]int) {
	_ = *p
}

func caller1() {
	// 超大切片
	large := make([]int, 1<<20) // ERROR "moved to heap: large"
	callee1(&large)
}

/*
// E_DYNAMIC: 动态大小逃逸（参数决定 make 的大小）
func callee2(n int) *[]int {
	s := make([]int, n)
	return &s
}
func caller2() {
	x := 10
	_ = callee2(x) // ERROR "moved to heap: x"
}

// E_GLOBAL: 参数赋值给全局变量
var g *int
func callee3(p *int) {
	g = p
}
func caller3() {
	i := 0 // ERROR "moved to heap: i"
	callee3(&i)
}

// E_INDIRECT: 参数通过间接赋值导致逃逸
func callee4(p **int) {
	j := 1
	*p = &j
}
func caller4() {
	var q *int
	callee4(&q) // ERROR "moved to heap: q"
}

// E_OUTERLOOP: 循环变量通过参数传递到闭包
func callee5(p *int) func() int {
	return func() int { return *p }
}
func caller5() {
	for i := 0; i < 3; i++ {
		_ = callee5(&i) // ERROR "moved to heap: i"
	}
}
*/

/*
// too_large：分配的局部数组太大，栈放不下 -> 逃逸
func callee_too_large(p *[1 << 20]int) []int {
	return *p
}

func caller_too_large() {
	var big [1 << 20]int // ERROR "moved to heap: big"
	callee_too_large(&big)
}

/*
func callee6(p int[]) {
	p = [1000000]int{}
	_ = p
}

func caller6() {
	var x int[]// ERROR "moved to heap: x"
	callee6(x)
}

*/

/*
func outerLoopReference() {
	var outerRef *int

	for i := 0; i < 10; i++ {
		i_testOuter := i
		outerRef = &i_testOuter // 循环内部的变量 num 被外部引用，num 会逃逸到堆上
	}

	_ = outerRef
}

func bigVariable() {
	bigArray := [1000000]int{}
	_ = bigArray
}

func indirect() {
	var p **int
	var i int
	*p = &i
}
/*
func f(d *int) {
	*d = *d + 1
	_ = d
}

func GoRoutine() {
	x := 1
	go f(&x)
}

/*
func addr() {
	i := new(int)
	p1 = i
}


func foo(p *int) {
	p1 = p
}
func main() {
	i := 10
	foo(&i)
}

// func makeSlice3() {
// 	s := make([]int, 10) // 不逃逸
// 	sink = s[0] + s[1]
// }
/*

*/
/*
func f(d *int) {
	*d = *d + 1
	_ = d
}

func main() {
	x := 1
	// go func(data *int) {
	// 	*data = *data + 1
	// 	_ = data
	// }(&x)

	go f(&x)
}

*/
/*
func main() {
	var result int
	ch := make(chan int)
	ch <- 1
	go func() { result = <-ch }()

	_ = result
	//fmt.Println(result)
}
*/
/*
func directorySet(pp **int, nt *int) {
	*(**int)(pp) = nt
}

/*
func newCounter() func() int {
	count := 0
	return func() int {
		count++
		return count
	}
}
*/
/*
type X struct {
	i *int
	j int
}

type Y struct {
	i *int
}

func indirect() {
	// i_testAddr := 10
	// p := &i_testAddr
	// sink = &p // 局部变量 i 的地址被返回，i 会逃逸到堆上
	var p **int
	var i int
	*p = &i

	//q := 10
	// x := []X{}
	// x[0].i = &q

	// y := []Y{}
	// y1 := Y{&q}
	// y[0] = y1

	//*p2 = &q
}

/*
func referencedByGlobal() {
	//i_testGlobal := 30
	//b := 20 // 局部变量 i 被全局变量引用，i 会逃逸到堆上
	var b *int
	t := 10
	// for i := 0; i < 1; i++ {
	// 	q := 10
	// 	b = &q // 局部变量 i 被全局变量引用，i 会逃逸到堆上
	// }
	//q := &b
	//sink = &p
	//sink = &i_testGlobal // 局部变量 i 被全局变量引用，i 会逃逸到堆上
	//sink, p = &q, &b
	sink, p1 = &b, &t

}
*/
/*
type Data struct {
	value int
}

// 处理数据的函数
func processData(d *Data, wg *sync.WaitGroup) {
	defer wg.Done()
	fmt.Println("Processing data:", d.value)
}

func main() {
	var wg sync.WaitGroup

	// 创建一个共享的数据对象
	data := &Data{value: 42}

	// 启动多个goroutine，这些goroutine会共享同一个数据对象
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go processData(data, &wg) // 启动goroutine并传递共享数据
	}

	wg.Wait() // 等待所有goroutine完成
}
*/
/*
//select channel(send recv)

// type Node struct {
// 	p *Node
// }

// func g(x *Node) *Node { // ERROR "leaking param content: x"
// 	return &Node{x.p} // ERROR "&Node{...} escapes to heap"
// }

// func returnAddress1() *int {
// 	i := 10
// 	//y := &i_testAddr
// 	return i // 局部变量 i 的地址被返回，i 会逃逸到堆上
// }
*/
/*


/*


// func paramPointee(p *int) {
// 	sink = p // 参数 p 指向的变量被全局变量引用，该变量会逃逸到堆上
// }

// func paramCall() {
// 	i_testParam := 20
// 	paramPointee(&i_testParam)
// }

// func param14a(x [4]*int) interface{} { // ERROR "leaking param: x$"

//		return x // ERROR "x escapes to heap"
//	}
// func param14b(x int) interface{} { //接口转换

// 	return x // ERROR "x escapes to heap"
// }

/************************************************************

var whys = []one_why{}               // 用于记录一个变量逃逸的所有why标签
var one_escape_func = []one_escape{} // 用于记录所有的逃逸节点以及其逃逸原因，每次只记录一个函数的所有节点

type one_why struct {//one_why存入每个编译输出的from数据流信息（标签）
	why    string    // 一个why
	srcLoc *location // 有向边的起初节点
	dstLoc *location // 有向边的终止节点
}

a--> b--> c,在one_why（存单独每一跳）中记录为a--> b, b--> c ,在one_escape中记录为a--> c

type one_escape struct {//one_escape解决p2-->p1-->i_testAddr,编译器首先会判断p1逃逸，标签有return add,但在i逃逸时，仅有add
	srcLoc *location   // 逃逸节点
	dstLoc *location   // 逃逸目的节点
	why    ESCAPE_TYPE // 逃逸原因，枚举
}

if whys[0].srcLoc.loopDepth > whys[len(whys)-1].dstLoc.loopDepth {//外层循环
	haven_loop_escape = true
}

 if explainLoc(whys[len(whys)-1].dstLoc) == "{heap}"//全局变量{
	haven_global_escape = true//相对原因，若后面未找到则判定为该原因
}

 if	whys[0].srcLoc.n.Name().Class == ir.PPARAM（?）

 if whys[0].why == "too large for stack"{return "large"}//绝对原因，直接返回
 if whys[0].why == "non-constant size" {return "[]make"}

 if !haven_find_reason
 {
	if haven_loop_escape = true
	{
		return "loop"
	}
 }

Referenced by outer loop 	least
Too big to stack-allocated	least
变长大小的(make(Iint,x))	least
Referenced by pointee of parameter =addr
Address is returned <全局 <多线程
Referenced by global variable
Referenced by multi-threads
记录最大的原因
*/
