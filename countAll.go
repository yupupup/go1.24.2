package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/360EntSecGroup-Skylar/excelize"
)

// ==== 配置区（请根据需要修改） ====
const (
	//logFile = "/home/rennny/dev/go/dataset/photoprism/photoprism.log" // 日志文件路径
	//logFile = "/home/rennny/dev/go/dataset/kubernetes/k8s.log" // 日志文件路径
	//logFile = "/home/rennny/dev/go/dataset/moby/moby.log" // 日志文件路径
	//logFile = "/home/rennny/dev/go/dataset/dgraph/dgraph.log" // 日志文件路径
	//logFile = "/home/rennny/dev/go/dataset/cli/cli.log" // 日志文件路径
	//logFile = "/home/rennny/dev/go/dataset/hugo/hugo.log" // 日志文件路径
	//logFile = "/home/rennny/dev/go/dataset/gogs/gogs.log" // 日志文件路径
	//logFile = "/home/rennny/dev/go/dataset/v2ray-core/v2ray.log" // 日志文件路径
	//logFile = "/home/rennny/dev/go/dataset/etcd/etcd.log" // 日志文件路径
	logFile = "/home/rennny/dev/go/dataset/rclone/rclone.log" // 日志文件路径
	count   = 10                              // 每组数据的整数个数
)

// ===================================

func main() {
	// 打开日志文件
	f, err := os.Open(logFile)
	if err != nil {
		log.Fatalf("无法打开日志文件 %s: %v", logFile, err)
	}
	defer f.Close()

	// 初始化累加切片
	sums := make([]int64, count)

	// 构造正则：^*+#(\d+)#\s+([\d\s]+)\s+#$
	// 组1 捕获数字个数，组2 捕获数据序列
	pattern := regexp.MustCompile(`^\*+#(\d+)#\s+([\d\s]+)\s+#$`)

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		// 正则匹配
		matches := pattern.FindStringSubmatch(line)
		if matches == nil {
			continue
		}
		// 解析捕获到的组长度
		num, err := strconv.Atoi(matches[1])
		if err != nil {
			log.Printf("警告：无法解析组长度 %q: %v", matches[1], err)
			continue
		}
		if num != count {
			log.Printf("警告：解析到的组长度 %d 与期望 %d 不符", num, count)
		}

		// 拆分数据并累加
		parts := strings.Fields(matches[2])
		if len(parts) < count {
			log.Printf("警告：数据个数 %d 少于期望 %d，跳过该行", len(parts), count)
			continue
		}
		for i := 0; i < count; i++ {
			v, err := strconv.ParseInt(parts[i], 10, 64)
			if err != nil {
				log.Printf("警告：无法解析数据 %q 为整数: %v", parts[i], err)
				v = 0
			}
			sums[i] += v
		}
	}
	if err := scanner.Err(); err != nil {
		log.Fatalf("读取文件时出错: %v", err)
	}

	// 定义每一列的名称
	// 这里数组长度必须和 count 保持一致
	var labels = [count]string{
		"返回地址",
		"空间过大",
		"动态大小",
		"全局变量",
		"外层循环",
		"间接访存",
		"多线程访问",
		"函数参数",
		"映射索引",
		"unknown",
	}

	// 输出累加结果
	var sum int64 = 0
	fmt.Println("统计结果：")
	for i := 0; i< len(sums) - 1; i++ {
		sum += sums[i]
	}

	for i := 0; i< len(sums) - 1; i++ {
		s := sums[i]
		ratio := float64(s) / float64(sum) * 100
		fmt.Printf("  %s:  \t %d \t %.2f%%\n", labels[i], s, ratio)
	}

	fmt.Printf("  总计: %d\n", sum)

		// 创建一个新的 Excel 文件
	fx := excelize.NewFile()
	// 获取默认工作表
	sheetName := fx.GetSheetName(fx.GetActiveSheetIndex())

	// 设置表头
	fx.SetCellValue(sheetName, "A1", "类别")
	fx.SetCellValue(sheetName, "B1", "数量")
	fx.SetCellValue(sheetName, "C1", "比例")

	// 写入统计结果
	for i := 0; i < len(sums)-1; i++ {
		s := sums[i]
		ratio := float64(s) / float64(sum) * 100
		row := i + 2
		fx.SetCellValue(sheetName, fmt.Sprintf("A%d", row), labels[i])
		fx.SetCellValue(sheetName, fmt.Sprintf("B%d", row), s)
		fx.SetCellValue(sheetName, fmt.Sprintf("C%d", row), fmt.Sprintf("%.2f%%", ratio))
	}

	// 写入总和
	lastRow := len(sums) + 1
	fx.SetCellValue(sheetName, fmt.Sprintf("A%d", lastRow), "Sum")
	fx.SetCellValue(sheetName, fmt.Sprintf("B%d", lastRow), sum)

	// 保存 Excel 文件
	if err := fx.SaveAs("统计结果.xlsx"); err != nil {
		log.Fatalf("保存 Excel 文件时出错: %v", err)
	}

	fmt.Println("统计结果已保存到 统计结果.xlsx")
}
