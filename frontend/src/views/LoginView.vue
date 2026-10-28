<template>
  <div class="login-page">
    <div class="login-intro"><span class="login-eyebrow">IT INFRASTRUCTURE</span><h2>资产有据可查，<br>运维有序可循。</h2><p>统一管理办公终端、服务器、网络设备与机房备件，<br>让设备配置、保修保障与每一次盘点清晰可见。</p><div class="login-topics"><span>统一台账</span><span>保修告警</span><span>扫码盘点</span></div></div>
    <el-card class="login-card" shadow="always">
      <div class="brand">
        <span class="login-mark">IT</span><h1>IT 基础设施管理平台</h1>
        <p>登录企业基础设施管理平台</p>
      </div>
      <el-form :model="form" label-position="top" @submit.prevent="submit">
        <el-form-item label="用户名">
          <el-input v-model="form.username" autocomplete="username" placeholder="请输入用户名" />
        </el-form-item>
        <el-form-item label="密码">
          <el-input v-model="form.password" type="password" show-password autocomplete="current-password" placeholder="请输入密码" @keyup.enter="submit" />
        </el-form-item>
        <el-button type="primary" native-type="submit" :loading="auth.loading" class="submit-button">登录</el-button>
      </el-form>
      <el-alert v-if="errorMessage" type="error" :closable="false" :title="errorMessage" class="error-alert" />
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'

const auth = useAuthStore()
const router = useRouter()
const route = useRoute()
const errorMessage = ref(route.query.reason === 'service-unavailable' ? '认证服务暂时不可用，请稍后重试。当前登录凭证已保留。' : '')
const form = reactive({ username: 'admin', password: '' })

async function submit() {
  errorMessage.value = ''
  try {
    await auth.signIn(form.username, form.password)
    await router.replace(typeof route.query.redirect === 'string' ? route.query.redirect : '/')
  } catch (error: any) {
    errorMessage.value = error?.response?.data?.message ?? '登录失败，请检查账号和密码'
  }
}
</script>

<style scoped>
.login-page { min-height: 100vh; display: flex; gap: 90px; justify-content:center; align-items: center; padding:40px; background:radial-gradient(ellipse at 20% 40%,#e5edf9,transparent 60%),var(--color-canvas); }
.login-card { width: min(400px, calc(100vw - 30px)); flex:none;border:1px solid #e0e8f4;border-radius:16px;padding:20px;box-shadow:0 20px 70px #1d3a6010; }
.login-intro{max-width:510px;color:#314969}.login-eyebrow{font-size:11px;letter-spacing:2.5px;color:#718cad}.login-intro h2{font-size:40px;font-weight:600;line-height:1.5;margin:23px 0;letter-spacing:-1px}.login-intro p{font-size:13px;color:#7a8da8;line-height:2.1}.login-topics{display:flex;gap:20px;margin-top:33px;font-size:11px;color:#7085a1}.login-topics span{border-left:2px solid #9eb9df;padding-left:11px}.login-mark{display:grid;place-items:center;width:44px;height:44px;background:var(--color-action);border-radius:12px;color:white;font-size:21px;font-weight:700;margin:0 auto 20px}.brand h1{font-size:23px}.brand p{font-size:12px}
@media(max-width:1000px){.login-page{gap:40px}.login-intro h2{font-size:32px}.login-intro p br{display:none}}@media(max-width:760px){.login-intro{display:none}.login-page{padding:20px}.login-card{padding:12px}}
.brand { text-align: center; margin-bottom: 24px; }
.brand h1 { margin: 0 0 8px; color: var(--color-text-primary); }
.brand p { margin: 0; color: var(--color-text-muted); }
.submit-button { width: 100%; }
.error-alert { margin-top: 16px; }
</style>
