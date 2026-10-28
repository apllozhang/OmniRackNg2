import ElementPlus, {ElSelect} from 'element-plus'
import {mount,flushPromises} from '@vue/test-utils'
import {afterEach,expect,it,vi} from 'vitest'
import AdminView from './AdminView.vue'
const api=vi.hoisted(()=>({list:vi.fn(),update:vi.fn()}))
vi.mock('@/api/admin',()=>({listAdminUsers:api.list,updateAdminUser:api.update,listAdminRoles:async()=>({items:[{code:'user',name:'用户'}]}),createAdminUser:vi.fn(),deleteAdminUser:vi.fn(),getLDAPConfig:vi.fn(),resetAdminPassword:vi.fn(),testLDAPConfig:vi.fn(),updateLDAPConfig:vi.fn()}))
const wrappers:ReturnType<typeof mount>[]=[]
afterEach(()=>{wrappers.splice(0).forEach(w=>w.unmount());document.body.innerHTML='';vi.clearAllMocks()})
it('sorts and pages complete user results while mutations keep the exact displayed ID/version',async()=>{
 const users=Array.from({length:61},(_,i)=>{const n=61-i;return {id:`id-${n}`,version:n,username:`u${n}`,displayName:`用户${n}`,enabled:true,authSource:n%2?'local':'ldap',roles:[{code:'user',name:'用户'}]}})
 api.list.mockImplementation(async query=>({items:users.filter(u=>(!query?.search||u.username.includes(query.search))&&(!query?.authSource||u.authSource===query.authSource))}))
 api.update.mockResolvedValue({})
 const w=mount(AdminView,{attachTo:document.body,global:{plugins:[ElementPlus]}});wrappers.push(w);await flushPromises()
 const rows=()=>w.findAll('#pane-users .el-table__body tbody tr')
 expect(rows()).toHaveLength(20)
 await w.get('button[aria-label="按用户名排序"]').trigger('click');await flushPromises()
 expect(rows()[0]!.text()).toContain('u1')
 await w.get('nav[aria-label="账号列表分页"] .btn-next').trigger('click');await flushPromises()
 expect(rows()[0]!.text()).toContain('u21')
 await rows()[0]!.findAll('button').find(b=>b.text()==='停用')!.trigger('click');await flushPromises()
 expect(api.update).toHaveBeenCalledWith('id-21',21,expect.objectContaining({username:'u21',enabled:false,roleCodes:['user']}))
 await w.get('input[placeholder="搜索用户名或显示名称"]').setValue('u61')
 await new Promise(resolve=>setTimeout(resolve,330));await flushPromises()
 expect(rows()).toHaveLength(1);expect(rows()[0]!.text()).toContain('u61')
 w.findAllComponents(ElSelect).find(select=>select.props('placeholder')==='账号来源')!.vm.$emit('update:modelValue','ldap');await flushPromises()
 expect(rows()).toHaveLength(0)
 await w.findAll('button').find(b=>b.text()==='清除筛选')!.trigger('click');await flushPromises()
 expect(rows()).toHaveLength(20)
 expect(w.get('nav[aria-label="账号列表分页"] .el-pager .is-active').text()).toBe('1')
})

it('keeps the editor version captured when opened even if a delayed list refresh brings a newer version',async()=>{
 const original={id:'u1',version:1,username:'u1',displayName:'原名称',authSource:'local',enabled:true,roles:[{code:'user',name:'用户'}]}
 api.list.mockResolvedValueOnce({items:[original]}).mockResolvedValue({items:[{...original,version:2,displayName:'并发新名称'}]})
 api.update.mockRejectedValue(new Error('版本冲突'))
 const w=mount(AdminView,{attachTo:document.body,global:{plugins:[ElementPlus]}});wrappers.push(w);await flushPromises()
 await w.get('input[placeholder="搜索用户名或显示名称"]').setValue('u1')
 await w.findAll('button').find(b=>b.text()==='编辑')!.trigger('click');await flushPromises()
 await new Promise(resolve=>setTimeout(resolve,330));await flushPromises()
 expect(w.text()).toContain('并发新名称')
 await [...document.querySelectorAll('button')].find(b=>b.textContent?.trim()==='保存')!.click();await flushPromises()
 expect(api.update).toHaveBeenCalledWith('u1',1,expect.objectContaining({displayName:'原名称',roleCodes:['user']}))
 expect(w.findComponent({name:'ElDialog'}).props('modelValue')).toBe(true)
})
