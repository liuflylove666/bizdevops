# 📱 应用管理模块 (Application Module)

## 功能概述
负责应用配置管理、交付记录查询和发布锁控制。

## 文件结构
```
application/
├── handler/                        # HTTP处理器
│   ├── application_handler.go     # 应用管理接口
│   ├── deploy_check_handler.go    # 交付前检查接口
│   └── deploy_lock_handler.go     # 发布锁接口
└── repository/                     # 数据访问层
    ├── application_repo.go        # 应用数据操作
    └── deploy_lock_repo.go        # 发布锁数据操作
```

## 主要功能
- **应用管理**: 应用CRUD、环境配置、团队管理
- **交付记录**: CI/GitOps 交付流水账、版本和状态查询
- **发布锁**: 防止并发部署、锁定管理

## API接口
- `GET /applications` - 获取应用列表
- `POST /applications` - 创建应用
- `PUT /applications/:id` - 更新应用
- `DELETE /applications/:id` - 删除应用
- `GET /applications/:id/delivery-records` - 查询应用交付记录
- `GET /app/delivery-records` - 查询全局交付记录
- `GET /deploy/locks` - 获取发布锁

## 相关Service
- `internal/service/pipeline/` - 流水线与 GitOps 交付交接
