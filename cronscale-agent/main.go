package main

import (
	"context"
	"fmt"
	"github.com/containerd/containerd"
	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
	"github.com/robfig/cron/v3"
	"go.uber.org/zap"
	"os"
	"strings"
)

var db *sqlx.DB

func main() {
	if err := InitMySQL(); err != nil {
		fmt.Println("err:", err)
		return
	}
	// agent 直接访问宿主机 containerd socket，用于读取当前节点已有镜像。
	client, err := containerd.New("/run/containerd/containerd.sock", containerd.WithDefaultNamespace("k8s.io"))
	if err != nil {
		fmt.Println("err:", err)
	}
	c := cron.New(cron.WithSeconds())
	spec := "*/5 * * * * *" // 每隔5s执行一次，cron格式（秒，分，时，天，月，周）
	// 定时上报节点镜像清单，供 Operator 判断哪些节点还需要预热镜像。
	c.AddFunc(spec, func() {
		node := os.Getenv("nodeName")
		var image string
		images, err := client.ListImages(context.Background(), "")
		test, _ := client.ImageService().List(context.Background(), "")
		fmt.Println("test:", test)
		if err != nil {
			fmt.Println("err:", err)
		}
		for i := 0; i < len(images); i++ {
			if !strings.Contains(images[i].Name(), "sha") || !strings.Contains(images[i].Name(), "ccr.ccs") {
				image = images[i].Name()
				insertRowDemo(node, image)
			}
		}
		deleteExpireData()

	})
	c.Start()
	defer client.Close()
	defer Close()
	defer c.Stop()
	select {}
}
func InitMySQL() (err error) {
	// public 仓库不提交真实 DSN；部署时建议从 Secret 或环境变量注入。
	dsn := ""
	// 也可以使用MustConnect连接不成功就panic
	db, err = sqlx.Connect("mysql", dsn)
	if err != nil {
		zap.L().Error("connect DB failed, err:%v\n", zap.Error(err))
		return err
	}
	db.SetMaxOpenConns(20)
	db.SetMaxIdleConns(20)
	return
}

func insertRowDemo(node string, image string) {
	// replace into 保证同一节点、同一镜像重复上报时刷新记录而不是无限追加。
	sqlStr := "replace into cronscale_agent(node_ip, image_name) values (?,?)"
	ret, err := db.Exec(sqlStr, node, image)
	if err != nil {
		fmt.Printf("insert failed, err:%v\n", err)
		return
	}
	theID, err := ret.LastInsertId() // 新插入数据的id
	if err != nil {
		fmt.Printf("get lastinsert ID failed, err:%v\n", err)
		return
	}
	fmt.Printf("insert success, the id is %d.\n", theID)
}
func deleteExpireData() {
	// 清理短时间内未刷新的记录，避免 Operator 使用过期的节点镜像信息。
	sqlStr := "DELETE FROM cronscale_agent WHERE node_ip='' OR image_name='' OR update_time < DATE_SUB(NOW(),INTERVAL 5 SECOND) ;"
	ret, err := db.Exec(sqlStr)
	if err != nil {
		fmt.Printf("insert failed, err:%v\n", err)
		return
	}
	rows, err := ret.RowsAffected() // 新插入数据的id
	if err != nil {
		fmt.Printf("get rows failed, err:%v\n", err)
		return
	}
	fmt.Printf("delete rows count is %d.\n", rows)
}
func Close() {
	_ = db.Close()
}
