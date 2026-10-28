# LDAP 联调环境

该目录提供 Docker Compose 的真实 OpenLDAP 联调配置。LDAP 服务使用 Compose profile，不会影响默认 PostgreSQL、后端和前端启动。

## 启动

在服务器项目目录准备未提交的 `.env`，至少设置：

- `LDAP_ADMIN_PASSWORD`
- `LDAP_DOMAIN`（默认 `example.org`）
- `LDAP_HOST_PORT`（默认 `1389`）

然后执行：

```bash
docker compose --profile ldap up -d ldap
```

默认 LDAP 地址：

```text
ldap://127.0.0.1:1389
Base DN: dc=example,dc=org
管理 DN: cn=admin,dc=example,dc=org
```

## 创建联调用户

不要把用户密码写入 Git 或提交到 `.env`。容器首次初始化后，用服务器上的临时环境变量创建测试用户：

```bash
export LDAP_TEST_PASSWORD='请替换为临时测试密码'
cat > /tmp/cabinet-ldap-user.ldif <<EOF
 dn: uid=ldap-tester,ou=people,dc=example,dc=org
 objectClass: inetOrgPerson
 objectClass: organizationalPerson
 objectClass: person
 objectClass: top
 cn: LDAP Tester
 sn: Tester
 uid: ldap-tester
 displayName: LDAP 测试用户
 mail: ldap-tester@example.org
 userPassword: $LDAP_TEST_PASSWORD

 dn: cn=cabinet-users,ou=groups,dc=example,dc=org
 changetype: modify
 add: member
 member: uid=ldap-tester,ou=people,dc=example,dc=org

 dn: cn=cabinet-admins,ou=groups,dc=example,dc=org
 changetype: modify
 add: member
 member: uid=ldap-tester,ou=people,dc=example,dc=org
EOF
# 上述 LDIF 每行开头不要有空格；执行后立即删除临时文件。
sed -i 's/^ //' /tmp/cabinet-ldap-user.ldif
docker exec -i cabinet-ldap ldapadd -x -H ldap://127.0.0.1:389 \
  -D "cn=admin,dc=example,dc=org" -w "$LDAP_ADMIN_PASSWORD" \
  -f /dev/stdin < /tmp/cabinet-ldap-user.ldif
rm -f /tmp/cabinet-ldap-user.ldif
```

应用配置时建议：

- 用户过滤器：`(|(uid={{username}})(sAMAccountName={{username}}))`
- 组查询 Base DN：`ou=groups,dc=example,dc=org`
- 组过滤器：`(&(objectClass=groupOfNames)(member={{userDN}}))`
- 组属性：`cn`
- `cabinet-users` 映射到普通用户角色；`cabinet-admins` 映射到系统管理员角色。

测试结束后可停用 profile：

```bash
docker compose --profile ldap stop ldap
```

