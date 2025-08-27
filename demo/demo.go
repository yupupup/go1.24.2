package demo

//var sink interface{}

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
*/
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

func callee(p *int) {
	sink = p
}

func caller() {
	i := 0 // ERROR "moved to heap: i$"
	callee(&i)
}

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
