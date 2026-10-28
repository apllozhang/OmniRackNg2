import {expect,it,vi} from 'vitest'
import {submissionId} from './submissionId'
it('supports HTTP browser crypto without randomUUID and returns v4 submission IDs',()=>{
 const getRandomValues=crypto.getRandomValues.bind(crypto)
 vi.stubGlobal('crypto',{getRandomValues})
 try{const first=submissionId();expect(first).toMatch(/^[a-f0-9]{8}-[a-f0-9]{4}-4[a-f0-9]{3}-[89ab][a-f0-9]{3}-[a-f0-9]{12}$/);expect(submissionId()).not.toBe(first)}finally{vi.unstubAllGlobals()}
})
