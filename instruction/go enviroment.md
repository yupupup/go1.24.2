## 1.安装go插件（环境）（解决打断点等问题）

## 2.ctrl+shift+p 命令面板输入go

选择install/update tools，安装gopls等tools，在过程中遇到go代理网络问题，切换代理

`rmy@k8s--master:~/dev/go$ go env -w GO111MODULE=on`
`rmy@k8s--master:~/dev/go$ go env -w GOPROXY=https://goproxy.cn,direct`

仍然失败，需要将报错信息中自动执行的cmd在终端自己手动安装即可

```go
2025-09-16 11:15:05.924 [info] {
 "code": 1,
 "killed": false,
 "signal": null,
 "cmd": "/usr/bin/go install -v honnef.co/go/tools/cmd/staticcheck@latest",//手动执行改行命令
 "stdout": "",
 "stderr": "go: honnef.co/go/tools/cmd/staticcheck@latest: module honnef.co/go/tools/cmd/staticcheck: Get \"https://proxy.golang.org/honnef.co/go/tools/cmd/staticcheck/@v/list\": dial tcp 142.250.73.81:443: i/o timeout\n"
}
```

原因是vscode自动执行的代理未生效

后续：自动安装也不行，在ctrl shift p的命令行里可以自动安装了

## 3.文件可以跳转了，但是红色浪线报错

方案：配置goroot

先./make.bash构建，执行`chmod +x ./make.bash`给makebash加一个执行权限

然后在设置--远程[ssh:128-core-3090]中找到`settings.json`文件中添加`go.goroot`

```go
{
    "go.goroot": "/home/rmy/dev/go/",
    "go.alternateTools": {

    }
}
```

## 4.单步调试发现dlv未识别

- Installing github.com/go-delve/delve/cmd/dlv@latest (/home/rmy/go/bin/dlv) SUCCEEDED但是输入dlv没有信息，说明没安装成功

### 解决办法

1. 先确认 `dlv` 安装位置`ls ~/go/bin/dlv`     如果能看到文件，说明 `dlv` 已经安装了。

2. 查看 `PATH` 中有没有 `~/go/bin`      `echo $PATH`        如果里面没有 `/home/rmy/go/bin`，那就需要把它加进去。

3. 修改 `~/.bashrc` 或 `~/.zshrc`（取决于你用的 shell）      在文件末尾加上：`export PATH=$PATH:$HOME/go/bin`

4. 让配置生效： `source ~/.bashrc`

5. 再试：`dlv version`      能看到 Delve 的版本信息。

###  配置环境变量

`./make.bash`运行不了，源代码坏了

- 解决：

  `git restore .`撤销本地工作区未commit的所有更改

  [(4 条消息) Git 后悔药完全指南：restore / reset / revert 命令详解 - 知乎](https://zhuanlan.zhihu.com/p/1938562402275815984)

然后配置环境变量

`nano ~/.bashrc`

```go
export GOROOT=/home/rmy/dev/go
export GOROOT_BOOTSTRAP=/home/rmy/dev/go-compile
export PATH=$PATH:/home/rmy/go/bin
```

`source ~/.bashrc`



## 5.Go编译器环境坏了

```go
rmy@k8s--master:~/dev/go$ which go
/home/rmy/dev/go/bin/go
rmy@k8s--master:~/dev/go$ rm bin/go
rmy@k8s--master:~/dev/go$ which go
/usr/bin/go
rmy@k8s--master:~/dev/go$ cd src/
rmy@k8s--master:~/dev/go/src$ ./make.bash 
```





