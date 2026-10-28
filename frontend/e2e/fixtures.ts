const timestamp = '2026-08-31T08:00:00Z'

function rack(id: string, code: string, name: string, status: string, sortOrder: number, xCoordinate: number, yCoordinate: number) {
  return {
    id, createdAt: timestamp, updatedAt: timestamp, version: 3, code, name, status, sortOrder,
    dataCenterId: 'dc-e2e', roomId: 'room-e2e', type: 'STANDARD', uHeight: 12,
    widthMm: 600, depthMm: 1200, heightMm: 2000, rotation: 0, dualPower: true,
    inputCircuits: 2, pduCount: 2, xCoordinate, yCoordinate, zone: 'A区',
    row: sortOrder < 3 ? 'A排' : 'B排', column: String((sortOrder % 3) + 1).padStart(2, '0'),
    aisle: '冷通道-01', manufacturer: '测试厂商', manager: '值班管理员', purpose: 'UI 自动化回归',
  }
}

export const racks = [
  rack('rack-e2e-1', 'RACK-A01', '01号机柜', 'PARTIAL', 0, 38, 72),
  rack('rack-e2e-2', 'RACK-A02', '02号机柜', 'AVAILABLE', 1, 284, 72),
  rack('rack-e2e-3', 'RACK-A03', '03号机柜', 'MAINTENANCE', 2, 530, 72),
  rack('rack-e2e-4', 'RACK-B01', '04号机柜', 'FULL', 3, 38, 364),
]

export const device = {
  id: 'device-e2e-1', createdAt: timestamp, updatedAt: timestamp, version: 5,
  typeId: 'type-server', code: 'DEV-E2E-001', name: '核心业务服务器01', lifecycleStatus: 'RUNNING',
  heightU: 2, dualPowerRequired: true, managementIp: '10.20.0.11', businessSystem: '自动化测试系统',
  type: {
    id: 'type-server', createdAt: timestamp, updatedAt: timestamp, version: 1, code: 'SERVER',
    name: '服务器', category: 'SERVER', status: 'ACTIVE', defaultHeightU: 2,
    defaultDualPower: true, sortOrder: 0,
  },
}

const position = {
  id: 'position-e2e-1', createdAt: timestamp, updatedAt: timestamp, version: 1,
  deviceId: device.id, rackId: racks[0].id, roomId: 'room-e2e', dataCenterId: 'dc-e2e',
  face: 'BOTH', startU: 5, endU: 6, heightU: 2, orientation: 'NORMAL', installedAt: timestamp,
  device, rack: racks[0],
}

export const resourceTree = {
  items: [{
    id: 'dc-e2e', createdAt: timestamp, updatedAt: timestamp, version: 2,
    code: 'DC-E2E', name: '自动化测试数据中心', status: 'OPERATING', sortOrder: 0,
    province: '测试省', city: '测试市', address: '自动化测试园区',
    rooms: [{
      id: 'room-e2e', createdAt: timestamp, updatedAt: timestamp, version: 7,
      code: 'ROOM-E2E', name: 'UI 回归机房', status: 'OPERATING', sortOrder: 0, dataCenterId: 'dc-e2e',
      building: '测试楼', floor: '2层', areaSquareMeters: 120, designPowerKw: 360, manager: '值班管理员',
      floorPlanEnabled: true, floorPlanFormat: 'GRID', racksPerRow: 3, racks,
    }],
  }],
}

export const rackLayouts = Object.fromEntries(racks.map((item) => [item.id, {
  rack: item,
  positions: item.id === racks[0].id ? [position] : [],
  frontFree: [],
  rearFree: [],
}]))

export const currentUser = {
  id: 'user-e2e', username: 'admin', displayName: 'Playwright 回归用户',
  authSource: 'local', enabled: true, roles: [{ id: 'role-admin', code: 'system_admin', name: '管理员' }],
}
