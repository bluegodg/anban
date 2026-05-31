package main

import (
	"log"

	"github.com/bluegodg/anban/server/internal/childapi"
	"github.com/bluegodg/anban/server/internal/config"
	"github.com/bluegodg/anban/server/internal/scheduler"
	"github.com/bluegodg/anban/server/internal/store"
	"github.com/bluegodg/anban/server/internal/xiaozhiclient"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("配置加载失败: %v", err)
	}

	st, err := store.Open(cfg.DBDSN)
	if err != nil {
		log.Fatalf("数据库打开失败: %v", err)
	}
	_ = st // 地基期各域尚未注册模型；域接入时在此 st.AutoMigrate(...)

	xc := xiaozhiclient.NewHTTPClient(cfg.ManagerBaseURL, cfg.ManagerAPIToken)
	_ = xc // 地基期各域尚未接入；域接入时把 xc 注入各域 service

	sch := scheduler.New()
	sch.Start()
	defer sch.Stop()

	r := childapi.NewRouter(childapi.Deps{AccessCode: cfg.AccessCode})

	log.Printf("anban 启动，监听 %s（manager=%s）", cfg.ListenAddr, cfg.ManagerBaseURL)
	if err := r.Run(cfg.ListenAddr); err != nil {
		log.Fatalf("HTTP 服务退出: %v", err)
	}
}
