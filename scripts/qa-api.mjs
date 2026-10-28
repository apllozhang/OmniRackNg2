// Isolated read-only UI fixtures. Never used by the production server or build.
import {createServer} from 'node:http'
const id=(n)=>`00000000-0000-4000-8000-${String(n).padStart(12,'0')}`
const base=(n)=>({id:id(n),version:1,createdAt:'2026-10-19T08:00:00Z',updatedAt:'2026-10-19T08:00:00Z'})
const user={...base(1),username:'preview',displayName:'只读预览用户',authSource:'local',enabled:true,roles:[{id:id(2),code:'system_admin',name:'系统管理员'}]}
const room={...base(10),name:'上海生产机房',code:'ROOM-SH',dataCenterId:id(9),racks:[]}
const sites=[{...base(11),name:'总部办公区 · 8F',region:'上海',kind:'OFFICE',address:'虚构示例位置'},{...base(12),name:'上海生产机房',region:'上海',kind:'ROOM',roomId:room.id},{...base(13),name:'灾备机房',region:'北京',kind:'ROOM'}]
const device=(n,name,category,code,sn,model,profile,warranty,pos)=>({device:{...base(n),name,code,serialNumber:sn,manufacturer:'示例厂商',modelNumber:model,type:{name:category==='PC'?'PC 终端':category==='SERVER'?'服务器':'网络设备',category},lifecycleStatus:pos?'RUNNING':'WAITING_RACK',managementIp:`10.20.1.${n}`,currentPosition:pos?{rackId:id(15),roomId:room.id,startU:4,endU:5,rack:{name:'A01 机柜'}}:undefined},profile:{...base(n+100),deviceId:id(n),usageStatus:'IN_USE',assignee:'示例责任人',department:'信息技术部',siteId:sites[pos?1:0].id,site:sites[pos?1:0],memoryGb:0,...profile},warranty,coverages:[],components:[]})
const devices=[device(21,'研究员办公终端','PC','IT-PC-0001','DEMO-PC-001','ThinkPad T14',{cpu:'Intel Core i7',memoryGb:32,disk:'1TB NVMe SSD',operatingSystem:'Windows 11'},{status:'VALID',label:'在保',endsOn:'2027-10-19T00:00:00+08:00'}),device(22,'交易应用服务器','SERVER','IT-SRV-0001','DEMO-SRV-001','PowerEdge R750',{cpu:'Intel Xeon Gold × 2',memoryGb:256,disk:'960GB SSD × 2 / 3.84TB SSD × 4',operatingSystem:'Rocky Linux 9',raid:'RAID 1 + RAID 10'},{status:'SOON',label:'即将过保',endsOn:'2026-11-06T00:00:00+08:00'},true),device(23,'核心交换机','NETWORK','IT-NET-0001','DEMO-NET-001','S6850',{ports:'48 × 10GE / 6 × 40GE'},{status:'EXPIRED',label:'已过保',endsOn:'2026-09-30T00:00:00+08:00'},true),device(24,'风控分析终端','PC','IT-PC-0002','DEMO-PC-002','OptiPlex 7010',{cpu:'Intel Core i7',memoryGb:64,disk:'2TB SSD',operatingSystem:'Windows 11'},{status:'VALID',label:'在保'})]
devices[0].device.externalQrCode='QR-2026-001'
devices[0].device.externalQrCodeUrl='https://vendor.example.test/qr/001'
const specs=[{...base(31),name:'企业级 SSD',category:'硬盘',modelNumber:'PM893',partNumber:'SSD-960-SATA',specification:'960GB / SATA / 2.5英寸',tracking:'SERIAL',unit:'件',compatibility:'经核验适用于示例服务器'},{...base(32),name:'服务器内存',category:'内存',modelNumber:'DDR4 ECC RDIMM',partNumber:'MEM-32G-ECC',specification:'32GB / DDR4-3200 / ECC',tracking:'SERIAL',unit:'件'},{...base(33),name:'光纤跳线',category:'线缆',modelNumber:'LC-LC OM4',partNumber:'FIBER-3M',specification:'双芯 / OM4 / 3米',tracking:'QUANTITY',unit:'根'}]
const stocks=[{...base(41),specId:specs[0].id,spec:specs[0],siteId:sites[1].id,site:sites[1],bin:'S01 / 1层',serialNumber:'DEMO-SSD-001',quantity:1,state:'AVAILABLE'},{...base(42),specId:specs[1].id,spec:specs[1],siteId:sites[1].id,site:sites[1],bin:'S01 / 2层',serialNumber:'DEMO-MEM-001',quantity:1,state:'AVAILABLE'},{...base(43),specId:specs[2].id,spec:specs[2],siteId:sites[1].id,site:sites[1],bin:'S02 / A盒',serialNumber:'',quantity:12,state:'AVAILABLE'},{...base(44),specId:specs[0].id,spec:specs[0],siteId:sites[2].id,site:sites[2],bin:'B01',serialNumber:'DEMO-SSD-002',quantity:1,state:'TESTING'}]
stocks.push({...base(45),specId:specs[0].id,spec:specs[0],siteId:sites[1].id,site:sites[1],bin:'S01 / 待入库',serialNumber:'DEMO-DRAFT-001',quantity:0,state:'AVAILABLE'})
const checks=[{...base(51),name:'上海机房 · 季度盘点',siteId:sites[1].id,site:sites[1],assigneeId:user.id,state:'ACTIVE',createdBy:user.id}]
const lines=[...devices.slice(1,3).map((a,i)=>({...base(61+i),taskId:checks[0].id,kind:'DEVICE',targetId:a.device.id,code:a.device.code,name:a.device.name,serialNumber:a.device.serialNumber,targetVersion:1,expected:1,result:'UNSCANNED',snapshot:JSON.stringify(a)})),...stocks.slice(0,3).map((s,i)=>({...base(63+i),taskId:checks[0].id,kind:'SPARE',targetId:s.id,code:s.id,name:s.spec.name,serialNumber:s.serialNumber,targetVersion:1,expected:s.quantity,result:'UNSCANNED',snapshot:JSON.stringify(s)}))]
const reminders=[{...base(71),title:'交易应用服务器 · 即将过保',detail:'IT-SRV-0001 / DEMO-SRV-001 / 2026-11-06',severity:'WARNING',kind:'WARRANTY',active:true},{...base(72),title:'核心交换机 · 已过保',detail:'IT-NET-0001 / DEMO-NET-001 / 2026-09-30',severity:'DANGER',kind:'WARRANTY',active:true},{...base(73),title:'企业级 SSD · 库存不足',detail:'上海生产机房：可用 1 / 最低 3 件',severity:'WARNING',kind:'STOCK',active:true}]
createServer(async(req,res)=>{res.setHeader('Content-Type','application/json; charset=utf-8');const path=new URL(req.url,'http://localhost').pathname;const ok=(data)=>res.end(JSON.stringify({code:'SUCCESS',message:'只读界面预览',data}));
  if(path==='/api/v1/auth/login'){let raw='';for await(const chunk of req)raw+=chunk;const d=JSON.parse(raw||'{}');if(d.username==='preview'&&d.password==='preview'){return ok({token:'read-only-ui-fixture',user})}res.statusCode=401;return res.end(JSON.stringify({message:'预览账号：preview / preview，仅用于虚构界面数据'}))}
  if(req.method!=='GET'){res.statusCode=405;return res.end(JSON.stringify({code:'READ_ONLY_PREVIEW',message:'当前为只读界面预览，保存功能请使用实际 PostgreSQL 部署验证。'}))}
  if(path==='/api/v1/auth/me')return ok(user)
  if(path==='/api/v1/asset-ops/access/settings')return ok({version:1,baseUrl:'https://assets.example.test',updatedAt:'2026-10-20T01:00:00Z'})
  if(path==='/api/v1/asset-ops/notifications/approval-settings')return ok({version:1,enabled:true,recipients:['admin@example.test','it-ops@example.test'],baseUrl:'https://it.example.test',emailEnabled:true})
  if(path==='/api/v1/asset-ops/notifications/settings')return ok({version:1,emailEnabled:true,smtpHost:'smtp.example.test',smtpPort:587,smtpSecurity:'STARTTLS',smtpUsername:'it-assets@example.test',emailFrom:'it-assets@example.test',emailRecipients:['operations@example.test'],hasSmtpPassword:true,weComEnabled:true,hasWeComWebhook:true,warrantyEnabled:true,rackUEnabled:true,rackWarningPercent:70,rackCriticalPercent:90,repeatHours:24})
  if(path==='/api/v1/asset-ops/notifications/deliveries')return ok({items:[{...base(95),channel:'EMAIL',status:'SUCCESS',summary:'3 项资产保修 / U 位告警',alertCount:3,test:false,errorMessage:''},{...base(96),channel:'WECOM',status:'FAILED',summary:'管理员发送的渠道测试',test:true,errorMessage:'虚构示例：企微连接失败，请检查网络、代理及证书'}],total:2,page:1,pageSize:20})
  if(path==='/api/v1/admin/users')return ok({items:[user]})
  if(path==='/api/v1/admin/roles')return ok({items:[...user.roles,{id:id(3),code:'asset_manager',name:'资产管理员'},{id:id(4),code:'asset_auditor',name:'资产审计员'}]})
  if(path==='/api/v1/admin/approval-policy')return ok({...base(5),assignApprovalEnabled:false,moveApprovalEnabled:true,approvalMode:'SINGLE',defaultApproverRole:'system_admin'})
  if(path==='/api/v1/admin/approvals')return ok({items:[]})
  if(path==='/api/v1/admin/ldap')return ok({...base(6),enabled:false,url:'',baseDn:'',bindDn:'',hasBindPassword:false,userFilter:'(sAMAccountName={{username}})',defaultRole:'user',allowLocalFallback:true,groupRoleMappings:[]})
  if(path.startsWith('/api/v1/devices/')){const row=devices.find(a=>a.device.id===path.split('/').at(-1));if(row)return ok(row.device)}
  if(path==='/api/v1/devices'){
    const query=new URL(req.url,'http://localhost').searchParams
    const page=Number(query.get('page')||1),pageSize=Number(query.get('pageSize')||20)
    const rows=devices.map(a=>({...a.device,typeId:id(a.device.type.category==='PC'?7:a.device.type.category==='SERVER'?8:14),heightU:1,dualPowerRequired:false})).filter(a=>(!query.get('typeId')||a.typeId===query.get('typeId'))&&(!query.get('lifecycleStatus')||a.lifecycleStatus===query.get('lifecycleStatus'))&&(!query.get('search')||[a.name,a.code,a.serialNumber].some(v=>v.includes(query.get('search')))))
    return ok({items:rows.slice((page-1)*pageSize,page*pageSize),total:rows.length,page,pageSize})
  }
  if(path==='/api/v1/device-types')return ok({items:[{...base(7),name:'PC 终端',code:'PC',category:'PC',status:'ACTIVE',defaultHeightU:1},{...base(8),name:'服务器',code:'SERVER',category:'SERVER',status:'ACTIVE',defaultHeightU:2}]})
  if(path==='/api/v1/rack-templates')return ok({items:[]})
  if(path==='/api/v1/asset-ops/audit')return ok({items:[{...base(91),userId:user.id,action:'UPDATE_PROFILE',resourceType:'device',resourceId:devices[0].device.id,before:'{}',after:JSON.stringify({cpu:'Intel Core i7',memoryGb:32}),requestId:'preview-request'}],total:1,page:1,pageSize:50})
  if(path==='/api/v1/resource-tree')return ok({items:[{...base(9),name:'上海数据中心',code:'DC-SH',rooms:[room]}]})
  if(path.startsWith('/health'))return ok({status:'fixture-preview-only'})
  const key=path.replace('/api/v1/asset-ops/','')
  const data={sites,devices,specs,stocks,checks,reminders,operators:[{id:user.id,name:user.displayName,canCheck:true}],movements:[],minimums:[{...base(81),specId:specs[0].id,spec:specs[0],siteId:sites[1].id,site:sites[1],minimum:3}]}
  if(data[key])return ok({items:data[key]})
  if(key===`checks/${checks[0].id}/lines`)return ok({items:lines})
  if(key.endsWith('/discoveries'))return ok({items:[]})
  res.statusCode=404;res.end(JSON.stringify({message:'此页面不在扩展界面预览范围，请查看资产运营、机房备件或扫码盘点。'}))
}).listen(18080,'127.0.0.1',()=>console.log('Read-only QA fixture API: http://127.0.0.1:18080'))
