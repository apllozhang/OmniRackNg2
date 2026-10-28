<script setup lang="ts">
import {onMounted,ref} from 'vue'
import {ElMessage} from 'element-plus'
import {getNotificationDeliveries,type NotificationDelivery} from '@/api/notifications'
import {errorMessage} from '@/api/asset-ops'
import '@/styles/asset-ops.css'
const deliveries=ref<NotificationDelivery[]>([]),total=ref(0),page=ref(1),historyLoading=ref(false)
async function history(){historyLoading.value=true;try{const r=await getNotificationDeliveries(page.value);deliveries.value=r.items;total.value=r.total}catch(e){ElMessage.error(errorMessage(e))}finally{historyLoading.value=false}}
const formatTime=(v:string)=>new Date(v).toLocaleString('zh-CN',{hour12:false})
onMounted(history)
</script>
<template><div class="asset-ops"><div class="ops-heading"><div><h1>通知发送记录</h1><p>统一追踪告警、审批邮件和渠道测试的发送结果；通道接收成功不代表收件人已阅读。</p></div></div>    <el-card shadow="never" >
      <template #header><div class="ops-toolbar between"><span class="ops-muted">按渠道追踪告警、审批和测试消息的处理结果</span><el-button :loading="historyLoading" @click="history">刷新记录</el-button></div></template>
      <el-table :data="deliveries" v-loading="historyLoading"><el-table-column label="发送时间" width="185"><template #default="{row}">{{ formatTime(row.createdAt) }}</template></el-table-column><el-table-column label="渠道" width="120"><template #default="{row}">{{ row.channel === 'EMAIL' ? '邮件' : '企业微信' }}</template></el-table-column><el-table-column label="来源" width="100"><template #default="{row}">{{ row.test ? '渠道测试' : row.summary?.startsWith('待审批') ? '审批通知' : '告警通知' }}</template></el-table-column><el-table-column label="收件邮箱 / 消息编号" min-width="240"><template #default="{row}"><span>{{row.recipients?.join('；')||'—'}}</span><small v-if="row.messageId" class="ops-block ops-mono">{{row.messageId}}</small></template></el-table-column><el-table-column prop="summary" label="通知摘要" min-width="210"/><el-table-column label="结果" width="125"><template #default="{row}"><el-tag :type="row.status === 'SUCCESS' ? 'success' : 'danger'" effect="light">{{ row.status === 'SUCCESS' ? '通道已接收' : '发送失败' }}</el-tag></template></el-table-column><el-table-column prop="errorMessage" label="失败原因" min-width="230"/></el-table>
      <div class="ops-toolbar between"><span class="ops-muted">共 {{ total }} 条记录</span><el-pagination v-model:current-page="page" :total="total" :page-size="20" layout="prev, pager, next" @current-change="history"/></div>
    </el-card></div></template>
