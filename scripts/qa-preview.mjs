// Serve the already-built production UI with a read-only fixture API for visual QA.
// No production code uses this server. Bind only to loopback.
import {createServer} from 'node:http'
import {readFile,stat} from 'node:fs/promises'
import {resolve,dirname,sep,extname} from 'node:path'
import {fileURLToPath} from 'node:url'
const root=resolve(dirname(fileURLToPath(import.meta.url)),'../frontend/dist')
const mime={'.html':'text/html; charset=utf-8','.js':'text/javascript; charset=utf-8','.css':'text/css; charset=utf-8','.svg':'image/svg+xml','.png':'image/png','.woff2':'font/woff2'}
createServer(async(req,res)=>{try{
  const url=new URL(req.url,'http://localhost')
  if(url.pathname.startsWith('/api/')||url.pathname.startsWith('/health/')){
    let body='';for await(const chunk of req){body+=chunk;if(body.length>1024*1024){res.writeHead(413);return res.end()}}
    const r=await fetch('http://127.0.0.1:18080'+url.pathname+url.search,{method:req.method,headers:{'Content-Type':'application/json'},...(body?{body}:{})})
    res.writeHead(r.status,{'Content-Type':'application/json; charset=utf-8'});return res.end(Buffer.from(await r.arrayBuffer()))
  }
  let path=resolve(root,'.'+decodeURIComponent(url.pathname))
  if(path!==root&&!path.startsWith(root+sep)){res.writeHead(403);return res.end()}
  const exists=await stat(path).catch(()=>null);if(!exists?.isFile())path=resolve(root,'index.html')
  let content=await readFile(path)
  if(extname(path)==='.html')content=Buffer.from(content.toString().replace('<body>','<body><div style="position:fixed;bottom:0;left:0;right:0;z-index:99999;padding:8px;background:#fff2cb;color:#85632c;text-align:center;font:12px sans-serif;">只读界面预览 · 数据均为虚构 · 非生产系统 · 预览账号 preview / preview</div>'))
  res.writeHead(200,{'Content-Type':mime[extname(path)]||'application/octet-stream','Cache-Control':'no-store'});res.end(content)
}catch{res.writeHead(500);res.end('Preview unavailable')}}).listen(5175,'127.0.0.1',()=>console.log('Read-only UI preview: http://127.0.0.1:5175/asset-operations'))
