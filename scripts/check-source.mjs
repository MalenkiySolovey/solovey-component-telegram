import fs from 'node:fs'
import path from 'node:path'
import assert from 'node:assert/strict'
import {createHash} from 'node:crypto'
import {execFileSync} from 'node:child_process'
import {fileURLToPath} from 'node:url'

const root=fileURLToPath(new URL('../',import.meta.url))
assert.equal(process.env.GOWORK,'off','qualification requires GOWORK=off')
const source=JSON.parse(fs.readFileSync(path.join(root,'SOURCE_PROVENANCE.json')))
assert.equal(source.component,'telegram')
assert.equal(source.originalCommit,'95f4673dd349fd3ee4220fb32ac675469d685a0b')
assert.equal(createHash('sha256').update(fs.readFileSync(path.join(root,'LICENSE'))).digest('hex'),source.licenseSHA256,'original license changed')
execFileSync('git',['merge-base','--is-ancestor',source.filteredHistoryCommit,'HEAD'],{cwd:root,stdio:'pipe'})
for(const record of source.files) {
 assert.ok(record.destination&&!record.destination.includes('..')&&!path.isAbsolute(record.destination))
 assert.equal(createHash('sha256').update(fs.readFileSync(path.join(root,record.destination))).digest('hex'),record.destinationSHA256,`mapped source drift: ${record.destination}`)
}
const output=execFileSync('go',['list','-deps','-test','-json','./...'],{cwd:root,encoding:'utf8',maxBuffer:16*1024*1024})
const packages=(output.match(/^{[\s\S]*?^}/gm)??[]).map(JSON.parse)
assert.ok(packages.length>0)
const allowed=new Set(['github.com/MalenkiySolovey/solovey-component-telegram','github.com/MalenkiySolovey/solovey-host','github.com/robfig/cron/v3'])
for(const pkg of packages)if(pkg.Module)assert.ok(allowed.has(pkg.Module.Path),`unapproved source dependency ${pkg.ImportPath}`)
const walk=directory=>fs.readdirSync(directory,{withFileTypes:true}).flatMap(entry=>entry.name==='node_modules'||entry.name.startsWith('.')?[]:entry.isDirectory()?walk(path.join(directory,entry.name)):[path.join(directory,entry.name)])
for(const filename of walk(root).filter(name=>/\.(go|ts|vue)$/.test(name))) {
 const text=fs.readFileSync(filename,'utf8')
 assert.ok(!text.includes('github.com/MalenkiySolovey/solovey-ui/'),`Panel dependency ${filename}`)
 assert.ok(!/from\s+['"]@\//.test(text),`private UI import ${filename}`)
 assert.ok(!/\b(?:M[026][A-C]?|R7|Wave|Phase|Goal)[-_]\w+/.test(text),`development-stage identifier ${filename}`)
}
console.log(JSON.stringify({status:'PASS',packages:packages.length,allowedModules:[...allowed],mappedSources:source.files.length,privatePanelImports:0,filteredHistoryPreserved:true}))
