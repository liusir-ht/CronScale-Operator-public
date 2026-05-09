/*
Copyright 2024 liuchong.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package pkg

import (
	"fmt"

	"github.com/containerd/containerd"
	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
	"go.uber.org/zap"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"liuchong.cn/m/pkg/cron"
)

var db *sqlx.DB

// KubectlClient 初始化 Kubectl 客户端
func KubectlClient() *kubernetes.Clientset {
	// public 仓库不提交真实 kubeconfig；本地调试时再通过变量填入路径。
	var kubeconfig = ""
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		panic(err.Error())
	}

	// create the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}
	return clientset
}

// CronClient 初始化cron客户端
func CronClient() *cron.Cron {
	return cron.New(cron.WithSeconds())
}

// CtrClient 初始化containerd 客户端
func CtrClient() (*containerd.Client, error) {
	client, err := containerd.New("/run/containerd/containerd.sock", containerd.WithDefaultNamespace("k8s.io"))
	if err != nil {
		fmt.Println("ctr New Client:", err)
		return nil, err
	}
	return client, nil
}

func InitMySQL() *sqlx.DB {
	var err error
	// public 仓库不提交真实 DSN；部署时建议从 Secret 或环境变量注入。
	dsn := ""
	// 也可以使用MustConnect连接不成功就panic
	db, err = sqlx.Connect("mysql", dsn)
	if err != nil {
		zap.L().Error("connect DB failed, err:%v\n", zap.Error(err))
		panic(err)
	}
	db.SetMaxOpenConns(20)
	db.SetMaxIdleConns(20)
	return db
}
