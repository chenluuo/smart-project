# 局域网访问说明

## 当前访问地址

当前开发机局域网 IPv4 为 `192.168.20.165`，同一局域网内的其他电脑打开：

```text
http://192.168.20.165:5173/
```

前端通过 Vite 代理访问 API，因此其他电脑不需要直接访问 `8080`。

## 服务端口

- 前端：`5173`
- Go API：`8080`，由前端代理使用
- MinIO 文件下载：`9000`

如果其他电脑无法访问，需要用管理员 PowerShell 添加入站规则：

```powershell
netsh advfirewall firewall add rule name="Smart Agriculture Frontend LAN" dir=in action=allow protocol=TCP localport=5173 remoteip=localsubnet profile=any
netsh advfirewall firewall add rule name="Smart Agriculture MinIO LAN" dir=in action=allow protocol=TCP localport=9000 remoteip=localsubnet profile=any
```

两条规则只允许本地子网访问前端和文件下载端口。

## 启动方式

在项目的 `frontend` 目录执行：

```powershell
npm.cmd run dev
```

Vite 已配置为监听 `0.0.0.0`，启动后可使用上面的局域网地址访问。

## 更换局域网 IP

如果开发机的局域网 IP 发生变化，需要同步修改：

1. `backend/.env.example` 中的 `MINIO_PUBLIC_ENDPOINT`
2. 重建或重启 Go API 容器
3. 使用新的 `http://新IP:5173/` 地址访问

MinIO 公开签名地址必须使用其他电脑能够访问的局域网 IP，不能填写 `localhost`。
