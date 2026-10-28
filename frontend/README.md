# 前端工程

## 技术栈

- Vue 3 + TypeScript + Vite
- Element Plus + Pinia + Vue Router
- SVG 作为机房平面图默认底图和图元格式

## 启动

```powershell
npm install
npm run dev
```

默认开发地址：`http://localhost:5174`

前端通过 Vite 将 `/api` 和 `/health` 代理到 `http://127.0.0.1:18085`。可通过 `VITE_API_BASE_URL` 覆盖 API 地址。

通过 `API_PROXY_TARGET=http://127.0.0.1:其他端口 npm run dev` 指向自定义后端。
