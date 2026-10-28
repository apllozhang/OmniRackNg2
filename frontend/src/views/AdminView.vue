<template>
  <div class="admin-page">
    <div class="page-header">
      <div><h2>账号与认证</h2><p>统一管理用户账号、角色及 LDAP 身份认证。</p></div>
      <el-button :loading="loading" @click="refresh">刷新</el-button>
    </div>
    <el-alert title="仅系统管理员可管理用户账号、角色和 LDAP 认证配置。" type="info" :closable="false" show-icon class="notice" />
    <el-card shadow="never">
      <el-tabs v-model="tab" @tab-change="tabChanged">
        <el-tab-pane label="用户管理" name="users">
          <div class="toolbar"><el-input v-model="userSearch" clearable placeholder="搜索用户名或显示名称" style="width:280px" @keyup.enter="loadUsers" /><el-select v-model="userSource" clearable placeholder="账号来源" style="width:150px"><el-option label="本地账号" value="local" /><el-option label="LDAP 账号" value="ldap" /></el-select><el-button type="primary" @click="loadUsers">查询</el-button><el-button @click="openUser()">新增用户</el-button></div>
          <el-table v-loading="userLoading" :data="users" stripe>
            <el-table-column prop="username" label="用户名" min-width="150" />
            <el-table-column prop="displayName" label="显示名称" min-width="150" />
            <el-table-column prop="email" label="邮箱" min-width="180" />
            <el-table-column label="来源" width="110"><template #default="{row}"><el-tag>{{ row.authSource === 'ldap' ? 'LDAP' : '本地' }}</el-tag></template></el-table-column>
            <el-table-column label="角色" min-width="180"><template #default="{row}"><el-tag v-for="role in row.roles ?? []" :key="role.code" size="small" class="role-tag">{{ role.name }}</el-tag></template></el-table-column>
            <el-table-column label="状态" width="90"><template #default="{row}"><el-tag :type="row.enabled ? 'success' : 'info'">{{ row.enabled ? '启用' : '停用' }}</el-tag></template></el-table-column>
            <el-table-column label="最后登录" width="180"><template #default="{row}">{{ formatTime(row.lastLoginAt) }}</template></el-table-column>
            <el-table-column label="操作" width="300" fixed="right"><template #default="{row}"><el-button link type="primary" @click="openUser(row)">编辑</el-button><el-button link @click="toggleUser(row)">{{ row.enabled ? '停用' : '启用' }}</el-button><el-button link @click="resetPassword(row)">重置密码</el-button><el-button link type="danger" @click="removeUser(row)">删除</el-button></template></el-table-column>
          </el-table>
        </el-tab-pane>
        <el-tab-pane label="LDAP 配置" name="ldap">
          <el-alert title="Bind 密码只用于服务器端连接，不会回显；留空保存时表示保留原密码。建议生产环境使用 LDAPS 或启用 TLS。" type="warning" :closable="false" show-icon />
          <el-form v-if="ldap" :model="ldap" label-width="150px" class="ldap-form"><el-row :gutter="20"><el-col :span="12"><el-form-item label="启用 LDAP"><el-switch v-model="ldap.enabled" /></el-form-item><el-form-item label="LDAP 地址" required><el-input v-model="ldap.url" placeholder="ldap://ldap.example.com:389 或 ldaps://..." /></el-form-item><el-form-item label="使用 TLS"><el-switch v-model="ldap.useTls" /></el-form-item><el-form-item label="跳过证书校验"><el-switch v-model="ldap.skipTlsVerify" /></el-form-item><el-form-item label="Bind DN"><el-input v-model="ldap.bindDn" placeholder="可选；匿名查询可留空" /></el-form-item><el-form-item label="Bind 密码"><el-input v-model="ldap.bindPassword" type="password" show-password :placeholder="ldap.hasBindPassword ? '已配置，留空保持不变' : '请输入服务账号密码'" /></el-form-item><el-form-item label="Base DN" required><el-input v-model="ldap.baseDn" placeholder="dc=example,dc=com" /></el-form-item><el-form-item label="组查询 Base DN"><el-input v-model="ldap.groupSearchBaseDn" placeholder="留空则使用 Base DN" /></el-form-item></el-col><el-col :span="12"><el-form-item label="用户过滤器"><el-input v-model="ldap.userFilter" type="textarea" :rows="2" /></el-form-item><el-form-item label="用户名属性"><el-input v-model="ldap.usernameAttribute" /></el-form-item><el-form-item label="显示名称属性"><el-input v-model="ldap.displayNameAttribute" /></el-form-item><el-form-item label="邮箱属性"><el-input v-model="ldap.emailAttribute" /></el-form-item><el-form-item label="LDAP 默认角色"><el-select v-model="ldap.defaultRole" style="width:100%"><el-option v-for="role in roles" :key="role.code" :label="role.name" :value="role.code" /></el-select></el-form-item><el-form-item label="允许本地账号登录"><el-switch v-model="ldap.allowLocalFallback" /></el-form-item><el-form-item label="组过滤器"><el-input v-model="ldap.groupFilter" type="textarea" :rows="2" placeholder="支持 {{userDN}} 和 {{username}}" /></el-form-item><el-form-item label="组属性"><el-input v-model="ldap.groupAttribute" placeholder="cn" /></el-form-item></el-col></el-row><el-divider content-position="left">LDAP 组到系统角色映射</el-divider><div class="mapping-help">登录用户命中下方 LDAP 组时，将同步对应系统角色；未命中时使用默认角色。组名匹配不区分大小写。</div><div v-for="(mapping,index) in ldap.groupRoleMappings" :key="index" class="mapping-row"><el-input v-model="mapping.group" placeholder="LDAP 组名，例如 cabinet-users" /><el-select v-model="mapping.roleCode" placeholder="选择系统角色" style="width:260px"><el-option v-for="role in roles" :key="role.code" :label="role.name" :value="role.code" /></el-select><el-button type="danger" link @click="removeLDAPMapping(index)">删除</el-button></div><el-button plain @click="addLDAPMapping">新增组映射</el-button><div class="form-actions"><el-button :loading="ldapTesting" @click="testLDAP">测试连接</el-button><el-button type="primary" :loading="ldapSaving" @click="saveLDAP">保存配置</el-button></div></el-form>
        </el-tab-pane>
      </el-tabs>
    </el-card>

    <el-dialog v-model="userDialog" :title="editingUser ? '编辑用户' : '新增用户'" width="560px"><el-form :model="userForm" label-width="100px"><el-form-item v-if="!editingUser" label="用户名" required><el-input v-model="userForm.username" autocomplete="off" /></el-form-item><el-form-item label="显示名称" required><el-input v-model="userForm.displayName" /></el-form-item><el-form-item label="邮箱"><el-input v-model="userForm.email" type="email" /></el-form-item><el-form-item label="账号来源"><el-radio-group v-model="userForm.authSource"><el-radio value="local">本地账号</el-radio><el-radio value="ldap">LDAP 账号</el-radio></el-radio-group></el-form-item><el-form-item label="密码" :required="!editingUser && userForm.authSource === 'local'"><el-input v-model="userForm.password" type="password" show-password :placeholder="editingUser ? '留空表示不修改' : '本地账号至少 8 位'" /></el-form-item><el-form-item label="角色" required><el-checkbox-group v-model="userForm.roleCodes"><el-checkbox v-for="role in roles" :key="role.code" :value="role.code">{{ role.name }}</el-checkbox></el-checkbox-group></el-form-item><el-form-item label="状态"><el-switch v-model="userForm.enabled" /></el-form-item></el-form><template #footer><el-button @click="userDialog=false">取消</el-button><el-button type="primary" :loading="userSaving" @click="saveUser">保存</el-button></template></el-dialog>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { createAdminUser, deleteAdminUser, getLDAPConfig, listAdminRoles, listAdminUsers, resetAdminPassword, testLDAPConfig, updateAdminUser, updateLDAPConfig } from '@/api/admin'
import type { LDAPConfig, ManagedUser, UserPayload } from '@/types/admin'
import type { Role } from '@/types/auth'

const tab = ref('users'), loading = ref(false), userLoading = ref(false), userSaving = ref(false), ldapSaving = ref(false), ldapTesting = ref(false)
const users = ref<ManagedUser[]>([]), roles = ref<Role[]>([])
const userSearch = ref(''), userSource = ref('')
const ldap = ref<LDAPConfig>(newLDAP()), userDialog = ref(false), editingUser = ref('')
const userForm = reactive<UserPayload>({ username:'', displayName:'', email:'', password:'', authSource:'local', enabled:true, roleCodes:['user'] })
function newLDAP(): LDAPConfig { return { enabled:false,url:'',useTls:false,skipTlsVerify:false,bindDn:'',bindPassword:'',baseDn:'',userFilter:'(|(uid={{username}})(sAMAccountName={{username}}))',usernameAttribute:'uid',displayNameAttribute:'displayName',emailAttribute:'mail',defaultRole:'user',allowLocalFallback:true,groupSearchBaseDn:'',groupFilter:'(&(objectClass=groupOfNames)(member={{userDN}}))',groupAttribute:'cn',groupRoleMappings:[] } }
function resetUser(){ Object.assign(userForm,{username:'',displayName:'',email:'',password:'',authSource:'local',enabled:true,roleCodes:['user']}) }
async function refresh(){ loading.value=true; try { await Promise.all([loadUsers(),loadRoles()]); if(tab.value==='ldap') await loadLDAP() } finally { loading.value=false } }
async function loadUsers(){userLoading.value=true;try{users.value=(await listAdminUsers({search:userSearch.value||undefined,authSource:userSource.value||undefined})).items}catch(e){fail(e)}finally{userLoading.value=false}}
async function loadRoles(){try{roles.value=(await listAdminRoles()).items}catch(e){fail(e)}}
async function tabChanged(value:string|number){if(value==='ldap')await loadLDAP()}
function openUser(item?:ManagedUser){editingUser.value=item?.id??'';if(item){Object.assign(userForm,{username:item.username,displayName:item.displayName,email:item.email ?? '',password:'',authSource:item.authSource,enabled:item.enabled,roleCodes:(item.roles??[]).map(i=>i.code)})}else resetUser();userDialog.value=true}
async function saveUser(){if(!userForm.displayName.trim()||(!editingUser.value&&!userForm.username?.trim())||!userForm.roleCodes.length){ElMessage.warning('请填写显示名称、用户名并至少选择一个角色');return}userSaving.value=true;try{if(editingUser.value){const item=users.value.find(i=>i.id===editingUser.value);await updateAdminUser(editingUser.value,item?.version??0,userForm)}else await createAdminUser(userForm);ElMessage.success('用户保存成功');userDialog.value=false;await loadUsers()}catch(e){fail(e)}finally{userSaving.value=false}}
async function toggleUser(item:ManagedUser){const next=!item.enabled;try{await updateAdminUser(item.id,item.version,{username:item.username,displayName:item.displayName,email:item.email ?? '',authSource:item.authSource,enabled:next,roleCodes:(item.roles??[]).map(i=>i.code)});ElMessage.success(next?'用户已启用':'用户已停用');await loadUsers()}catch(e){fail(e)}}
async function resetPassword(item:ManagedUser){try{const result=await ElMessageBox.prompt('请输入新密码（至少 8 位）','重置密码',{inputType:'password',inputPlaceholder:'新密码'});await resetAdminPassword(item.id,item.version,result.value);ElMessage.success('密码已重置');await loadUsers()}catch(e){cancelOrFail(e)}}
async function removeUser(item:ManagedUser){try{await ElMessageBox.confirm(`确认删除用户“${item.displayName}”吗？`,'删除用户',{type:'warning'});await deleteAdminUser(item.id,item.version);ElMessage.success('用户已删除');await loadUsers()}catch(e){cancelOrFail(e)}}
async function loadLDAP(){try{ldap.value=await getLDAPConfig()}catch(e){fail(e)}}
async function saveLDAP(){ldapSaving.value=true;try{ldap.value=await updateLDAPConfig(ldap.value);ElMessage.success('LDAP 配置已保存')}catch(e){fail(e)}finally{ldapSaving.value=false}}
async function testLDAP(){ldapTesting.value=true;try{await testLDAPConfig(ldap.value);ElMessage.success('LDAP 连接测试成功')}catch(e){fail(e)}finally{ldapTesting.value=false}}
function addLDAPMapping(){ldap.value.groupRoleMappings.push({group:'',roleCode:''})}
function removeLDAPMapping(index:number){ldap.value.groupRoleMappings.splice(index,1)}
function formatTime(v?:string){return v?new Date(v).toLocaleString('zh-CN',{hour12:false}):'-'}
function message(e:unknown){const x=e as {response?:{data?:{message?:string}};message?:string};return x.response?.data?.message??x.message??'操作失败'}
function fail(e:unknown){ElMessage.error(message(e))}function cancelOrFail(e:unknown){if(e!=='cancel'&&e!=='close')fail(e)}
onMounted(refresh)
</script>
<style scoped>
.admin-page{max-width:1560px;margin:0 auto}.page-header{display:flex;justify-content:space-between;align-items:flex-start;margin-bottom:18px}.page-header h2{margin:0 0 8px}.page-header p{margin:0;color:#6b7280}.notice{margin-bottom:16px}.toolbar,.form-actions{display:flex;gap:10px;align-items:center;margin-bottom:16px}.role-tag{margin-right:5px}.policy-form,.ldap-form{margin-top:20px}.form-actions{justify-content:flex-end}.muted{color:#9ca3af}.mapping-help{color:#6b7280;font-size:13px;margin:-6px 0 14px 150px}.mapping-row{display:flex;gap:10px;align-items:center;margin:0 0 10px 150px}.mapping-row .el-input{max-width:420px;flex:1}
</style>
