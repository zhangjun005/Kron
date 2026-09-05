# Kron

> A modern, lightweight personal development tool with graphical interface.

## 📖 项目简介

**Kron** 是一款为个人开发者设计的开源开发工具软件。它提供直观的图形界面，帮助开发者更高效地管理项目、任务和开发流程。

## 🎯 核心特性

- **现代化图形界面** - 简洁直观的用户界面设计
- **跨平台支持** - 支持 Windows、macOS、Linux
- **轻量高效** - 基于 Rust + Web 技术栈，运行速度快，占用资源少
- **开源免费** - 采用 MIT 开源许可证

## 🛠 技术栈

### 核心技术
- **Tauri 2.x** - 桌面应用框架 (Rust 后端)
- **React 18** - 前端 UI 框架
- **TypeScript 5.x** - 类型安全的 JavaScript
- **Vite** - 快速构建工具
- **Tailwind CSS** - 现代化 CSS 框架

### 开发工具
- **ESLint** - 代码质量检查
- **Prettier** - 代码格式化
- **Husky** - Git hooks 管理
- **lint-staged** - 暂存区代码检查

## 📦 安装与运行

### 环境要求
- Node.js >= 18.x
- Rust >= 1.70
- pnpm (推荐) 或 npm

### 安装依赖
```bash
pnpm install
```

### 开发模式
```bash
pnpm dev
```

### 构建应用
```bash
pnpm build
```

### 运行生产版本
```bash
pnpm run:prod
```

## 📁 项目结构

```
kron/
├── src/                    # 前端源代码
│   ├── components/         # React 组件
│   ├── pages/              # 页面组件
│   ├── hooks/              # 自定义 Hooks
│   ├── stores/             # 状态管理
│   ├── services/           # 业务逻辑服务
│   ├── types/              # TypeScript 类型定义
│   ├── utils/              # 工具函数
│   ├── styles/             # 全局样式
│   ├── App.tsx             # 应用主组件
│   └── main.tsx            # 应用入口
├── src-tauri/              # Tauri 后端源代码 (Rust)
│   ├── src/
│   │   └── main.rs         # Rust 主入口
│   ├── Cargo.toml          # Rust 依赖配置
│   ├── tauri.conf.json     # Tauri 配置
│   └── icons/              # 应用图标
├── public/                 # 静态资源
├── package.json            # 项目配置
├── tsconfig.json           # TypeScript 配置
├── vite.config.ts          # Vite 配置
├── tailwind.config.js      # Tailwind CSS 配置
└── README.md               # 项目文档
```

## 📄 开源许可证

本项目基于 **MIT License** 开源。

```
MIT License

Copyright (c) 2026 Kron

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
```

## 🤝 贡献指南

欢迎提交 Issue 和 Pull Request！

## 📝 后续开发计划

项目初始化完成，需求分析阶段即将开始。

---

**作者**: zhangjun005  
**邮箱**: zhangjv233@163.com  
**许可证**: MIT
