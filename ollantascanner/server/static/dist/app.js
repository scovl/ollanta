"use strict";(()=>{
const SEV_ORDER=['blocker','critical','major','minor','info'];
const SEV_COLOR={blocker:'#e53e3e',critical:'#dd6b20',major:'#d69e2e',minor:'#38a169',info:'#718096'};
const SEV_BG={blocker:'rgba(229,62,62,.10)',critical:'rgba(221,107,32,.09)',major:'rgba(214,158,46,.08)',minor:'rgba(56,161,105,.08)',info:'rgba(113,128,150,.08)'};
var f=[],l=[],m='all',p='all',y='all',g='',d='severity',o=true,v={blocker:0,critical:1,major:2,minor:3,info:4};

function sevCounts(arr){const c={blocker:0,critical:0,major:0,minor:0,info:0};for(const i of arr)if(i.severity in c)c[i.severity]++;return c;}

async function $(){try{let t=await fetch('/report.json');if(!t.ok)throw new Error(`HTTP ${t.status}`);let n=await t.json();h(n),S(n),f=n.issues??[],T(),i()}catch(t){document.getElementById('app').innerHTML=`<div class="error">Failed to load report: ${String(t)}</div>`;}}
document.addEventListener('DOMContentLoaded',$);

function h(t){let n=new Date(t.metadata.analysis_date).toLocaleString();document.getElementById('project-key').textContent=t.metadata.project_key,document.getElementById('scan-date').textContent=n,document.getElementById('scan-version').textContent=`v${t.metadata.version}`,document.getElementById('elapsed').textContent=`${t.metadata.elapsed_ms}ms`;}

function S(t){let n=t.measures,e=[['Files',n.files,'\u{1F4C1}'],['Lines',n.lines.toLocaleString(),'\u{1F4C4}'],['NCLOC',n.ncloc.toLocaleString(),'\u{1F4DD}'],['Bugs',n.bugs,'\u{1F41B}'],['Code Smells',n.code_smells,'\u{1F33F}'],['Vulnerabilities',n.vulnerabilities,'\u{1F512}']],s=document.getElementById('measures');s.innerHTML=e.map(([r,a,I])=>`<div class="card"><span class="card-icon">${I}</span><span class="card-value">${a}</span><span class="card-label">${r}</span></div>`).join('');let u=Object.entries(n.by_language).sort((r,a)=>a[1]-r[1]);u.length&&(document.getElementById('by-lang').innerHTML=u.map(([r,a])=>`<span class="tag">${r}: ${a}</span>`).join(' '));}

function buildSevBar(){const counts=sevCounts(l.length?l:f);document.getElementById('sev-bar').innerHTML=SEV_ORDER.map(sev=>{const n=counts[sev],active=m===sev;return`<button class="sev-chip${active?' active':''}" data-sev="${sev}" style="--chip-color:${SEV_COLOR[sev]};--chip-bg:${SEV_BG[sev]}"><span class="chip-dot" style="background:${SEV_COLOR[sev]}"></span>${sev.charAt(0).toUpperCase()+sev.slice(1)}<span class="chip-count">${n}</span></button>`;}).join('');document.querySelectorAll('.sev-chip').forEach(btn=>{btn.addEventListener('click',()=>{m=m===btn.dataset.sev?'all':btn.dataset.sev;i();});});}

function T(){let t=[...new Set(f.map(e=>e.rule_key))].sort(),n=document.getElementById('filter-rule');t.forEach(e=>{let s=document.createElement('option');s.value=e,s.textContent=e,n.appendChild(s);});document.getElementById('filter-severity').addEventListener('change',e=>{m=e.target.value;i();});document.getElementById('filter-type').addEventListener('change',e=>{p=e.target.value;i();});n.addEventListener('change',e=>{y=e.target.value;i();});document.getElementById('search').addEventListener('input',e=>{g=e.target.value.toLowerCase();i();});}

function i(){l=f.filter(t=>!(m!=='all'&&t.severity!==m||p!=='all'&&t.type!==p||y!=='all'&&t.rule_key!==y||g&&!`${t.component_path} ${t.message} ${t.rule_key}`.toLowerCase().includes(g)));E();L();buildSevBar();}

function E(){l.sort((t,n)=>{let e=t[d],s=n[d];return d==='severity'&&(e=v[t.severity]??99,s=v[n.severity]??99),e<s?o?-1:1:e>s?o?1:-1:0;});}

var _={bug:'\u{1F41B}',code_smell:'\u{1F33F}',vulnerability:'\u{1F512}'};

function L(){let t=document.getElementById('issue-tbody'),n=document.getElementById('issue-count');if(n.textContent=`${l.length} issue${l.length!==1?'s':''}`,!l.length){t.innerHTML='<tr><td colspan="5" class="empty">No issues match the current filters.</td></tr>';return;}t.innerHTML=l.map(e=>{let s=e.component_path.replace(/\\/g,'/').split('/').slice(-3).join('/'),u=e.end_line&&e.end_line!==e.line?`${e.line}\u2013${e.end_line}`:`${e.line}`,r=SEV_COLOR[e.severity]??'#718096',bg=SEV_BG[e.severity]??'transparent',a=_[e.type]??'\u2753';return`<tr class="sev-row" style="--row-sev-color:${r};--row-sev-bg:${bg}"><td><span class="sev-badge" style="background:${r}">${e.severity}</span></td><td>${a} ${esc(e.type.replace('_',' '))}</td><td class="mono">${esc(e.rule_key)}</td><td class="file-cell" title="${esc(e.component_path)}">${esc(s)}<span class="loc">:${u}</span></td><td class="msg">${esc(e.message)}</td></tr>`;}).join('');}

document.addEventListener('DOMContentLoaded',()=>{document.querySelectorAll('th[data-sort]').forEach(t=>{t.addEventListener('click',()=>{let n=t.dataset.sort;d===n?o=!o:(d=n,o=true);document.querySelectorAll('th[data-sort]').forEach(e=>e.classList.remove('sort-asc','sort-desc'));t.classList.add(o?'sort-asc':'sort-desc');E();L();});});});

function esc(t){return String(t).replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;');}

})()
