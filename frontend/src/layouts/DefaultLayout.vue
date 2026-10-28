<script setup lang="ts">
import webuiVersion from '@/styles/webui/UPSTREAM.json'
import { darkTheme, toggleTheme } from '@/theme'
import { computed, ref, watch, nextTick } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
const router = useRouter(), route = useRoute(), auth = useAuthStore(), mobileOpen = ref(false)
const canUseAssets = computed(() => auth.user?.roles?.some(r => ['system_admin', 'asset_manager', 'asset_auditor'].includes(r.code)))
const groups = computed(() => [
  { title: '工作台', items: [
    { path: '/', label: '运行概览', icon: 'M3 3h7v7H3zM14 3h7v7h-7zM3 14h7v7H3zM14 14h7v7h-7z' },
    ...(canUseAssets.value ? [{ path: '/asset-reminders', label: '提醒中心', icon: 'M5 17h14l-2-4V9a5 5 0 0 0-10 0v4l-2 4zM10 21h4' }] : []),
    ...(auth.isAdmin ? [{ path: '/approvals', label: '审批中心', icon: 'M5 3h14v18H5zM8 12l3 3 5-6' }] : []),
  ] },
  ...(canUseAssets.value ? [{ title: '资产管理', items: [
    { path: '/terminal-assets', label: '终端台账', icon: 'M3 4h18v13H3zM8 21h8M12 17v4' },
    { path: '/spare-parts', label: '机房备件', icon: 'm12 3 9 5-9 5-9-5 9-5zM3 8v9l9 5 9-5V8M12 13v9' },
    { path: '/asset-operations', label: '配置与保修中心', icon: 'M3 5h18v12H3zM8 21h8M12 17v4' },
    { path: '/asset-checks', label: '扫码盘点', icon: 'M8 3H3v5M16 3h5v5M21 16v5h-5M8 21H3v-5M7 12l3 3 7-7' },
    { path: '/asset-scan', label: '扫码查询', icon: 'M3 3h6v6H3zM15 3h6v6h-6zM3 15h6v6H3zM15 15h2v2h-2zM21 15v6h-6' },
  ] }] : []),
  { title: '机房资源', items: [
    { path: '/data-centers', label: '区域与机房', icon: 'M4 21V5h10v16M14 10h6v11M8 9h2M8 13h2M8 17h2M2 21h20' },
    { path: '/racks', label: '机柜管理', icon: 'M5 3h14v18H5zM5 9h14M5 15h14M8 6h1M8 12h1M8 18h1' },
    { path: '/room-devices', label: '机房设备台账', icon: 'M3 4h18v5H3zM3 15h18v5H3zM7 6v1M7 17v1' },
    { path: '/room-screen', label: '机房大屏', icon: 'M3 4h18v14H3zM8 22h8M12 18v4M6 14l4-4 4 2 4-5' },
  ] },
  { title: '基础资料', items: [
    ...(canUseAssets.value ? [{ path: '/asset-locations', label: '区域与资产位置', icon: 'M12 22s8-8 8-14a8 8 0 0 0-16 0c0 6 8 14 8 14zM9 8h6' }] : []),
    { path: '/rack-templates', label: '机柜模板', icon: 'M4 4h10v16H4zM10 4V2h10v16h-6M7 8h4M7 12h4' },
  ] },
  ...(canUseAssets.value || auth.isAdmin ? [{ title: '系统管理', items: [
    ...(canUseAssets.value ? [
      { path: '/asset-audit', label: '资产操作记录', icon: 'M5 3h14v18H5zM8 8h8M8 12h8M8 16h6' },
      { path: '/asset-data', label: '数据导出与备份', icon: 'M12 3v12m-5-5 5 5 5-5M4 17v4h16v-4' },
    ] : []),
    ...(auth.isAdmin ? [
      { path: '/notification-deliveries', label: '通知发送记录', icon: 'M3 5h18v14H3zM3 5l9 7 9-7' },
      { path: '/notification-settings', label: '告警通知', icon: 'M5 17h14l-2-4V9a5 5 0 0 0-10 0v4l-2 4zM10 21h4' },
      { path: '/approval-rules', label: '审批规则', icon: 'M5 3h14v18H5zM8 8h8M8 12h8M8 16h5' },
      { path: '/scan-access-settings', label: '扫码访问设置', icon: 'M3 3h6v6H3zM15 3h6v6h-6zM3 15h6v6H3zM15 15h6v6h-6z' },
      { path: '/admin', label: '账号与认证', icon: 'M16 21v-2a4 4 0 0 0-4-4H6a4 4 0 0 0-4 4v2M9 3a4 4 0 1 0 0 8 4 4 0 0 0 0-8M19 8v6M16 11h6' },
    ] : []),
  ] }] : []),
])
const mobileButton = ref<HTMLButtonElement | null>(null)
watch(mobileOpen, (open, previous) => { if (!open && previous) nextTick(() => mobileButton.value?.focus()) })
const activePath = computed(() => route.path === '/pc-assets' ? '/terminal-assets' : route.path)
const current = computed(() => groups.value.flatMap(g => g.items).find(i => i.path === activePath.value)?.label || 'IT 基础设施管理平台')
watch(() => route.path, () => { mobileOpen.value = false })
async function logout() { await auth.signOut(); await router.replace('/login') }
</script>
<template>
  <a class="skip-link" href="#main-content">跳到主要内容</a>
  <el-container class="app-shell" @keydown.esc="mobileOpen = false">
    <button v-if="mobileOpen" class="sidebar-backdrop" aria-label="关闭导航" @click="mobileOpen = false" />
    <el-aside width="236px" id="workspace-navigation" class="sidebar" :class="{ 'mobile-open': mobileOpen }">
      <RouterLink to="/" class="brand-link"><span class="brand-wordmark">OmniRack</span><span><strong>IT 基础设施管理平台</strong><small>IT INFRASTRUCTURE</small></span></RouterLink>
      <el-menu :default-active="activePath" router background-color="transparent" class="workspace-menu">
        <template v-for="group in groups" :key="group.title"><div class="nav-section">{{ group.title }}</div><el-menu-item v-for="item in group.items" :key="item.path" :index="item.path" :aria-current="activePath === item.path ? 'page' : undefined"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path :d="item.icon"/></svg><span>{{ item.label }}</span></el-menu-item></template>
      </el-menu>
      <div class="sidebar-footer"><span class="deployment-dot"/><span>企业内部资产工作空间<small>设备 · 机房 · 备件 · 盘点</small></span></div>
    </el-aside>
    <el-container class="content-shell">
      <el-header class="topbar">
        <div class="topbar-left"><button ref="mobileButton" class="mobile-nav-button" aria-label="打开导航" :aria-expanded="mobileOpen" aria-controls="workspace-navigation" @click="mobileOpen = !mobileOpen">☰</button><span class="breadcrumb-root">工作空间</span><span class="breadcrumb-slash">/</span><strong aria-current="page">{{ current }}</strong></div>
        <div class="topbar-right"><el-button text aria-label="切换亮暗主题" :aria-pressed="darkTheme" @click="toggleTheme">{{ darkTheme ? '亮色' : '暗色' }}</el-button><span class="webui-version">WebUI {{ webuiVersion.design_version }}</span><el-button v-if="canUseAssets" text class="reminder-shortcut" @click="router.push('/asset-reminders')">提醒中心</el-button><span class="topbar-divider"/><el-dropdown @command="logout"><span class="user-menu"><span class="user-avatar">{{ (auth.user?.displayName || 'U').slice(0, 1) }}</span><span>{{ auth.user?.displayName || '当前用户' }}</span><span class="menu-arrow">⌄</span></span><template #dropdown><el-dropdown-menu><el-dropdown-item command="logout">退出登录</el-dropdown-item></el-dropdown-menu></template></el-dropdown></div>
      </el-header>
      <el-main id="main-content" tabindex="-1" class="main-content"><router-view /></el-main>
    </el-container>
  </el-container>
</template>
<style scoped>
.app-shell{min-height:100vh;align-items:flex-start}.sidebar{position:sticky;top:0;height:100dvh;display:flex;flex-direction:column;background:#14243b;color:var(--color-surface);z-index:100;flex-shrink:0}.brand-link{display:flex;gap:12px;align-items:center;padding:27px 22px 24px;text-decoration:none;color:var(--color-surface)}.brand-mark{width:38px;height:40px;border-radius:10px;background:#4d82df;display:grid;place-items:center;font-size:18px;font-weight:750;letter-spacing:-1px;flex:none}.brand-link strong{display:block;font-size:15px;font-weight:600;letter-spacing:.3px}.brand-link small{display:block;font-size:9px;letter-spacing:1.6px;color:#92a8c5;margin-top:7px}.workspace-menu{border:0;padding:0 12px 20px;overflow:auto;flex:1}.nav-section{font-size:10px;font-weight:500;letter-spacing:1.5px;color:#7c91ae;padding:20px 14px 10px}.workspace-menu .el-menu-item{height:43px;line-height:43px;border-radius:7px;margin:3px 0;font-size:13px;gap:12px;padding-left:14px!important}.workspace-menu .el-menu-item svg{width:18px;height:18px;flex:none}.workspace-menu .el-menu-item:hover{background:#203651}.workspace-menu .el-menu-item.is-active{background:#2d4a70;color:var(--color-surface)}.sidebar-footer{display:flex;gap:10px;align-items:flex-start;padding:20px 23px;border-top:1px solid #2a3b51;font-size:11px;color:#b0bed0;line-height:1.6}.sidebar-footer small{display:block;margin-top:5px;color:#7b91af;font-size:10px}.deployment-dot{height:6px;width:6px;background:#7795bf;border-radius:50%;margin-top:6px;flex:none}.content-shell{min-width:0;min-height:100vh;flex:1}.topbar{position:sticky;top:0;z-index:90;background:var(--color-surface);border-bottom:1px solid #e6ebf2;display:flex;align-items:center;justify-content:space-between;padding:0 30px;height:68px;gap:16px}.topbar-left,.topbar-right{display:flex;align-items:center;gap:17px;font-size:12px;min-width:0}.topbar-left strong{font-weight:550;color:#384b64;white-space:nowrap}.breadcrumb-root,.breadcrumb-slash{color:#8a99ad}.topbar-divider{height:23px;width:1px;background:#e8edf4}.user-menu{display:inline-flex;align-items:center;gap:9px;font-size:12px;color:#43556b;cursor:pointer;white-space:nowrap}.user-avatar{width:30px;height:30px;display:grid;place-items:center;background:#e9f0fc;border-radius:50%;color:#4f73aa;font-size:12px}.menu-arrow{color:#8291a5;margin-left:4px}.main-content{padding:30px;background:var(--color-canvas);min-width:0;width:100%}.mobile-nav-button,.sidebar-backdrop{display:none}
@media(min-width:1800px){.main-content{padding:36px 44px}}@media(max-width:1000px){.sidebar{width:206px!important}.topbar{padding:0 20px}.main-content{padding:22px}}@media(max-width:760px){.sidebar{position:fixed;left:-250px;top:0;width:236px!important;transition:left .2s;z-index:1001}.sidebar.mobile-open{left:0}.sidebar-backdrop{display:block;position:fixed;inset:0;background:#0e1c3280;border:0;z-index:1000}.mobile-nav-button{display:block;border:0;background:transparent;color:#42566f;font-size:18px;padding:4px;cursor:pointer}.topbar{height:60px;padding:0 14px}.topbar-left,.topbar-right{gap:10px}.breadcrumb-root,.breadcrumb-slash,.reminder-shortcut,.topbar-divider{display:none}.topbar-left strong{font-size:12px}.main-content{padding:18px 14px}.user-menu>span:nth-child(2){max-width:75px;overflow:hidden;text-overflow:ellipsis}}
</style>
