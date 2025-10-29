方法概述：在github仓库跑出编译日志后 ，根据每个未知分类（unknown）的情况，在本地构造小型`demo`示例（[go/demo/demo.go](http://222.195.92.204:1480/vm/golang/work/go1.24.2/-/blob/escape-src/demo/demo.go?ref_type=heads)中），结合调试，修改escape文件夹下的go文件。（判断逻辑集中在[solve.go](http://222.195.92.204:1480/vm/golang/work/go1.24.2/-/blob/escape-src/src/cmd/compile/internal/escape/solve.go?ref_type=heads)的`countAll`函数中）

调试：可通过[go/.vscode/launch.json](http://222.195.92.204:1480/vm/golang/work/go1.24.2/-/blob/escape-src/.vscode/launch.json?ref_type=heads)进行调试，program和GOROOT路径需要根据实际更改。（多线程函数不可调试）

①源码构建：首先使用`go install -a -gcflags "-N -l" cmd/go cmd/compile`在`src`目录下重新构建修改后的`go`工具链。

②仓库编译：在每个仓库根目录下运行`go build -a -gcflags="all=-m=2 -N -l" > rep_name.log 2>&1`命令进行编译，并将日志重定向到指定log文件，其中-m=2表示开启高详模式，以输出详细逃逸信息，-N表示禁止编译优化，-l表示禁止内联。

③脚本收集：统计脚本位于[go/countAll/countAll.go](http://222.195.92.204:1480/vm/golang/work/go1.24.2/-/tree/escape-src/countAll?ref_type=heads)中，运行方法：`go run countAll.go -logFile=/home/rmy/dev/dataset/rclone/rclone.log`，传参路径按实际修改即可。

---

### Go源码构建

- go build命令

```go
go install -a -gcflags "-N -l" cmd/go cmd/compile
```

- 构建命令（用于第一次构建，修改源码时非必要不使用）

```go
./make.bash
```

- 多次连续编译/编译器坏掉（修改源码常用方法）

用以下命令

第一行正常编译

第二行修复坏掉的编译器(存一个好的二进制go编译器)

```go
rmy@k8s--master:~/dev/go/src$ go install -a -gcflags "-N -l" cmd/go cmd/compile
rmy@k8s--master:~/dev/go/src$ /home/rmy/dev/go-compile/bin/go install -a -gcflags "-N -l" cmd/go cmd/compile
```