# 酒店管理系统（连锁多门店）

面向连锁酒店的集团化管控平台：总部统一管理会员、房价、渠道、协议客户与品牌标准，门店独立运营日常接待业务，总部实时汇总全集团经营数据。

## 技术栈

| 层 | 技术 |
|----|------|
| 前端 | Vue 3 + Element Plus + Vite |
| 后端 | Go |
| 数据库 | PostgreSQL |
| 部署 | Docker 容器化 + 云端 SaaS |

## 目录结构

```
hotel-management/
├── server/               # Go 后端
│   ├── go.mod
│   ├── main.go           # 入口
│   └── internal/
│       ├── config/       # 配置
│       ├── router/       # 路由
│       └── handler/      # HTTP 处理器
├── web/                  # Vue3 前端
│   ├── package.json
│   ├── vite.config.js
│   ├── index.html
│   └── src/
│       ├── main.js
│       ├── App.vue
│       └── router/
├── docker-compose.yml    # 数据库 + 前后端编排
└── README.md
```

## 快速启动

### 后端
```bash
cd server
go run .
```

### 前端
```bash
cd web
npm install
npm run dev
```

### Docker（PostgreSQL）
```bash
docker compose up -d
```

## 第一期范围

纯客房核心闭环：预订 → 入住/退房 → 房态 → 房价 → 客户 → 收银 → 会员 → 报表 → 权限。

OTA 直连、断网离线同步、餐饮/会议等放二期。

## 需求与设计文档

详见项目知识库（`.pair/project-info/`）：
- 概览 / 需求-功能模块 / 需求-连锁架构 / 需求-角色权限 / 需求-非功能需求
- 设计-数据库表结构 / 设计-页面原型
