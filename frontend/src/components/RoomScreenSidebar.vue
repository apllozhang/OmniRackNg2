<template>
<aside class="control-panel">
  <div class="brand-block">
    <div class="brand-mark">DC</div>
    <div><h1>机房资源大屏</h1><p>Data Center Visualization</p></div>
  </div>

  <div class="selector-section">
    <label>数据中心</label>
    <el-select v-model="selectedDataCenterId" placeholder="请选择数据中心" size="large" class="screen-select" @change="changeDataCenter">
      <el-option v-for="item in tree" :key="item.id" :label="item.name" :value="item.id" />
    </el-select>
    <label>机房</label>
    <el-select v-model="selectedRoomId" placeholder="请选择机房" size="large" class="screen-select" @change="changeRoom">
      <el-option v-for="item in rooms" :key="item.id" :label="item.name" :value="item.id" />
    </el-select>
  </div>

  <div v-if="selectedRoom" class="room-card">
    <div class="section-title"><span></span>机房信息</div>
    <div class="room-name">{{ selectedRoom.name }}</div>
    <div class="room-code">{{ selectedRoom.code }}</div>
    <div class="room-meta">
      <div><span>所在位置</span><strong>{{ roomLocation }}</strong></div>
      <div><span>机房面积</span><strong>{{ selectedRoom.areaSquareMeters ? `${selectedRoom.areaSquareMeters} ㎡` : '-' }}</strong></div>
      <div><span>设计功率</span><strong>{{ selectedRoom.designPowerKw ? `${selectedRoom.designPowerKw} kW` : '-' }}</strong></div>
      <div><span>负责人</span><strong>{{ selectedRoom.manager || '-' }}</strong></div>
    </div>
    <div class="room-card-actions">
      <button type="button" @click="openCreateRack">新增机柜</button>
      <button type="button" :disabled="!roomRacks.length || rackDiagramExporting" @click="exportRackDiagram">{{ rackDiagramExporting ? '正在导出...' : '导出机柜图' }}</button>
      <button type="button" :disabled="rackDiagramDeviceTypesLoading" @click="openRackDiagramImport">{{ rackDiagramDeviceTypesLoading ? '正在加载...' : '导入机柜图' }}</button>
      <button type="button" @click="loadTree">刷新数据</button>
    </div>
  </div>

  <div class="stats-grid">
    <div class="stat-item cyan"><span>机柜总数</span><strong>{{ roomRacks.length }}</strong><small>RACKS</small></div>
    <div class="stat-item green"><span>空闲</span><strong>{{ statusCount('AVAILABLE') }}</strong><small>AVAILABLE</small></div>
    <div class="stat-item blue"><span>已用 U 位</span><strong>{{ roomUsedU }}</strong><small>/ {{ totalU }}U</small></div>
    <div class="stat-item orange"><span>U 位使用率</span><strong>{{ roomUsageRate }}</strong><small>%</small></div>
  </div>

  <div class="legend-card">
    <div class="section-title"><span></span>机柜状态图例<b v-if="statusFilter !== 'ALL'" class="filter-hint">已筛选</b></div>
    <div class="status-legend-list">
      <button type="button" class="status-legend-item" :class="{ active: statusFilter === 'ALL' }" @click="setStatusFilter('ALL')"><i class="all-dot"></i><span>全部</span><b>{{ roomRacks.length }}</b></button>
      <button v-for="item in legends" :key="item.value" type="button" class="status-legend-item" :class="{ active: statusFilter === item.value }" @click="setStatusFilter(item.value)"><i :style="{ background: item.color }"></i><span>{{ item.label }}</span><b>{{ statusCount(item.value) }}</b></button>
    </div>
    <div class="u-note">点击任意 U 位可打开操作菜单，直接进行上架、查看、迁移或下架操作。</div>
  </div>

  <div class="legend-card">
    <div class="section-title"><span></span>设备类型图例</div>
    <div class="device-legend-list">
      <div v-for="item in deviceLegends" :key="item.value"><i :style="{ background: item.color }"></i><span>{{ item.label }}</span></div>
    </div>
    <div class="u-note">机柜统一展示一套 U 位，所有操作按 U 区间管理。</div>
  </div>

  <div v-if="selectedRack" class="selected-card">
    <div class="selected-header">
      <div><small>当前选中</small><strong>{{ selectedRack.name }}</strong></div>
      <button type="button" class="selected-status-button" :style="{ color: rackColor(selectedRack.status) }" @click.stop="openRackEditor(selectedRack, true)">{{ statusLabel(selectedRack.status) }}</button>
    </div>
    <div class="selected-grid">
      <div><span>编码</span><b>{{ selectedRack.code }}</b></div>
      <div><span>规格</span><b>{{ selectedRack.uHeight }}U</b></div>
      <div><span>已使用</span><b>{{ selectedRackMetrics.used }}U</b></div>
      <div><span>剩余</span><b>{{ selectedRackMetrics.free }}U</b></div>
      <div><span>使用率</span><b>{{ selectedRackMetrics.rate }}%</b></div>
      <div><span>位置</span><b>{{ rackPosition(selectedRack) }}</b></div>
    </div>
    <div class="selected-progress"><i :style="{ width: `${selectedRackMetrics.rate}%` }"></i></div>

    <div v-if="selectedDevicePosition" class="selected-device">
      <small>选中设备</small><strong>{{ deviceName(selectedDevicePosition) }}</strong>
      <div><span>设备编码</span><b>{{ selectedDevicePosition.device?.code || selectedDevicePosition.deviceId }}</b></div>
      <div><span>设备类型</span><b>{{ deviceCategoryLabel(selectedDevicePosition.device?.type?.category) }}</b></div>
      <div><span>占用 U 位</span><b>{{ positionRange(selectedDevicePosition) }}</b></div>
      <div><span>管理地址</span><b>{{ selectedDevicePosition.device?.managementIp || '-' }}</b></div>
    </div>
    <div class="selected-actions">
      <button type="button" @click="openRackDetail(selectedRack)">查看详情</button>
      <button type="button" @click="openRackEditor(selectedRack)">编辑机柜</button>
      <button type="button" class="danger" @click="removeRack(selectedRack)">删除机柜</button>
    </div>
    <button class="u-button" type="button" @click="openUView">查看完整 U 位详情</button>
  </div>

  <div class="panel-actions">
    <button type="button" @click="goBack">返回管理平台</button>
    <button type="button" @click="toggleFullscreen">{{ isFullscreen ? '退出全屏' : '进入全屏' }}</button>
  </div>
</aside>
</template>
<script setup lang="ts">


const props = defineProps<{ screen: Record<string, any> }>()
const {
  tree, selectedDataCenterId, changeDataCenter, rooms, selectedRoomId, changeRoom,
  selectedRoom, roomLocation, openCreateRack, loadTree, exportRackDiagram, rackDiagramExporting, openRackDiagramImport, rackDiagramDeviceTypesLoading, roomRacks, statusCount,
  roomUsedU, totalU, roomUsageRate, statusFilter, legends, setStatusFilter,
  deviceLegends, selectedRack, rackColor, openRackEditor, statusLabel,
  selectedRackMetrics, rackPosition, selectedDevicePosition, deviceName,
  deviceCategoryLabel, positionRange, openRackDetail, removeRack, openUView,
  goBack, isFullscreen, toggleFullscreen,
} = props.screen
</script>
<style>
</style>
