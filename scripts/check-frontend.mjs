import fs from 'node:fs'
import path from 'node:path'
import assert from 'node:assert/strict'
import {createHash} from 'node:crypto'
import {execFileSync,spawnSync} from 'node:child_process'
import {fileURLToPath} from 'node:url'

const root=fileURLToPath(new URL('../',import.meta.url)),frontend=path.join(root,'frontend')
const sdk=fs.realpathSync.native(path.resolve(process.argv[2]??path.join(root,'.host-ui/frontend')))
const pin=JSON.parse(fs.readFileSync(path.join(root,'HOST_UI_PIN.json')))
assert.equal(pin.repository,'MalenkiySolovey/solovey-ui')
assert.equal(pin.apiVersion,'1.0')
assert.match(pin.sourceCommit,/^[a-f0-9]{40}$/)
assert.equal(execFileSync('git',['rev-parse','HEAD'],{cwd:sdk,encoding:'utf8'}).trim(),pin.sourceCommit,'UI SDK source commit mismatch')
execFileSync('git',['diff','--exit-code',pin.sourceCommit,'--','frontend'],{cwd:path.resolve(sdk,'..'),stdio:'pipe'})
for(const [name,expected] of Object.entries(pin.files)) {
 assert.ok(!name.includes('..')&&!path.isAbsolute(name))
 assert.equal(createHash('sha256').update(fs.readFileSync(path.join(sdk,name))).digest('hex'),expected,`UI SDK drift: ${name}`)
}
const modules=path.join(frontend,'node_modules'),sdkModules=fs.realpathSync.native(path.join(sdk,'node_modules'))
if(!fs.existsSync(modules))fs.symlinkSync(sdkModules,modules,process.platform==='win32'?'junction':'dir')
assert.equal(fs.realpathSync.native(modules),sdkModules,'UI tests must use the accepted SDK lock/toolchain')
const generated=path.join(root,'.frontend-check')
fs.mkdirSync(generated,{recursive:true})
const tsconfig={compilerOptions:{target:'ESNext',module:'ESNext',moduleResolution:'bundler',strict:true,jsx:'preserve',resolveJsonModule:true,isolatedModules:true,esModuleInterop:true,ignoreDeprecations:'6.0',lib:['ESNext','DOM'],skipLibCheck:true,noEmit:true,baseUrl:frontend,
 paths:{'@solovey/component-ui':[path.join(sdk,'src/componentSystem/sdk.ts')],'@/*':[path.join(sdk,'src/*')],...Object.fromEntries(['vue','vue-i18n','vue-router','notivue','vitest'].map(name=>[name,[path.join(sdkModules,name)]]))},types:['node'],typeRoots:[path.join(sdkModules,'@types')]},include:[path.join(frontend,'**/*.ts'),path.join(frontend,'**/*.vue'),path.join(sdk,'src/vite-env.d.ts')],exclude:[modules]}
fs.writeFileSync(path.join(generated,'tsconfig.json'),JSON.stringify(tsconfig,null,2)+'\n')
const env={...process.env,SOLOVEY_COMPONENT_UI_SDK:sdk}
for(const args of [[path.join(sdkModules,'vue-tsc/bin/vue-tsc.js'),'--noEmit','-p',path.join(generated,'tsconfig.json')],[path.join(sdkModules,'vitest/vitest.mjs'),'run','--config',path.join(frontend,'vitest.config.mts')]]) {
 const result=spawnSync(process.execPath,args,{cwd:frontend,env,stdio:'inherit'})
 if(result.error)throw result.error
 assert.equal(result.status,0,'component frontend qualification failed')
}
console.log(JSON.stringify({status:'PASS',sdkSource:pin.sourceCommit,apiVersion:pin.apiVersion}))
