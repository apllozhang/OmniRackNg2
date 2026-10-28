#!/usr/bin/env bash
set -Eeuo pipefail

readonly SSH_USER="developer"
readonly SSH_HOME="/home/${SSH_USER}"
readonly AUTHORIZED_KEYS="${SSH_HOME}/.ssh/authorized_keys"

if [[ "$(id -u)" -ne 0 ]]; then
  echo "此开发容器必须以 root 启动，以便初始化 SSH 配置。" >&2
  exit 1
fi

mkdir -p /run/sshd "${SSH_HOME}/.ssh"
chmod 700 "${SSH_HOME}/.ssh"

# 首次启动时生成 host key；host key 不写入镜像，避免所有容器复用同一密钥。
ssh-keygen -A

password="${DEV_SSH_PASSWORD:-}"
public_key="${DEV_SSH_PUBLIC_KEY:-}"

if [[ -z "${password}" && -z "${public_key}" ]]; then
  echo "请至少设置 DEV_SSH_PASSWORD 或 DEV_SSH_PUBLIC_KEY 后再启动容器。" >&2
  exit 1
fi

if [[ -n "${password}" ]]; then
  printf '%s:%s\n' "${SSH_USER}" "${password}" | chpasswd
else
  # 仅使用公钥登录时锁定密码，防止意外启用空/未知密码登录。
  passwd --lock "${SSH_USER}" >/dev/null
fi

: > "${AUTHORIZED_KEYS}"
if [[ -n "${public_key}" ]]; then
  printf '%s\n' "${public_key}" > "${AUTHORIZED_KEYS}"
fi

# Compose named volumes 覆盖镜像目录后通常由 root 创建，需在启动时修正权限，避免 developer 无法下载 Go 依赖。
mkdir -p "${SSH_HOME}/go/pkg/mod" "${SSH_HOME}/.cache/go-build"
chown -R "${SSH_USER}:${SSH_USER}" "${SSH_HOME}/go" "${SSH_HOME}/.cache"

chown -R "${SSH_USER}:${SSH_USER}" "${SSH_HOME}/.ssh"
chmod 600 "${AUTHORIZED_KEYS}"

/usr/sbin/sshd -t -f /etc/ssh/sshd_config
exec /usr/sbin/sshd -D -e -f /etc/ssh/sshd_config