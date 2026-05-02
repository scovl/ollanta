"use strict";(()=>{function a(e){return e.replaceAll("&","&amp;").replaceAll("<","&lt;").replaceAll(">","&gt;").replaceAll('"',"&quot;")}function ie(e){return[{key:"details",label:"Details"},{key:"rule",label:"Rule"},{key:"ai-fix",label:"Fix with AI"}].map(s=>`<button class="detail-tab${e===s.key?" active":""}" data-detail-tab="${s.key}">${s.label}</button>`).join("")}function ae(e,t,s){let i=e.end_line&&e.end_line!==e.line?`-${e.end_line}`:"",l=ve(t,s),r=pe(t);return`
    <div class="detail-section">
      <div class="detail-section-title">Fix with AI</div>
      <div class="detail-msg ai-fix-callout">Ollanta prepares the issue context, sends only the relevant snippet to the selected agent, and shows a preview before writing any changes to your code.</div>
    </div>

    <div class="detail-section">
      <div class="detail-field detail-field-stack">
        <span class="detail-field-label">Target</span>
        <span class="detail-field-value detail-mono-block">${a(e.component_path)}:${e.line}${i}</span>
      </div>
      <div class="detail-field detail-field-stack">
        <span class="detail-field-label">Issue</span>
        <span class="detail-field-value">${a(e.message)}</span>
      </div>
    </div>

    <div class="detail-section">
      <div class="detail-section-title">Model</div>
      ${l}
      ${t.statusMessage?`<div class="ai-fix-status ai-fix-status-ok">${a(t.statusMessage)}</div>`:""}
      ${t.errorMessage?`<div class="ai-fix-status ai-fix-status-error">${a(t.errorMessage)}</div>`:""}
    </div>

    <div class="detail-section">
      <div class="detail-section-title">Preview</div>
      ${r}
    </div>
  `}function ve(e,t){if(e.loadingOptions)return'<div class="detail-loading">Loading AI models\u2026</div>';if(t.length===0)return'<div class="detail-empty">No AI provider is available for the local scanner.</div>';let s=t.find(c=>c.id===e.selectedProviderId)??t[0],i=t.map(c=>`<option value="${a(c.id)}"${e.selectedProviderId===c.id?" selected":""}>${a(c.label)}</option>`).join(""),r=(s?.models??[]).map(c=>`<option value="${a(c)}"></option>`).join(""),h='<div class="ai-fix-helper">This provider can generate local previews without an API key.</div>',g="Required for this provider";s?.requires_api_key&&(s.configured?(h=`<div class="ai-fix-helper">Using the scanner's configured API key. Paste another key below to override it for this session.</div>`,g="Optional override"):h='<div class="ai-fix-helper">Paste an API key for the selected provider to generate the fix.</div>');let _=s?.requires_api_key?`<div class="ai-fix-control-group">
          <label class="ai-fix-control-label" for="ai-api-key-input">API key</label>
          <input id="ai-api-key-input" class="ai-fix-select ai-fix-input" type="password" value="${a(e.apiKey)}" placeholder="${g}" autocomplete="off">
        </div>`:"",w=e.loadingPreview?"Generating\u2026":"Generate fix",d=e.loadingPreview?" disabled":"";return`<div class="ai-fix-controls">
      <div class="ai-fix-control-group">
        <label class="ai-fix-control-label" for="ai-provider-select">Provider</label>
        <select id="ai-provider-select" class="ai-fix-select">${i}</select>
      </div>
      <div class="ai-fix-control-group">
        <label class="ai-fix-control-label" for="ai-model-input">Model</label>
        <input id="ai-model-input" class="ai-fix-select ai-fix-input" list="ai-model-options" value="${a(e.selectedModel)}" placeholder="${a(s?.default_model||"gpt-4.1-mini")}" autocomplete="off">
        <datalist id="ai-model-options">${r}</datalist>
      </div>
      ${_}
      ${h}
      <button id="ai-generate-fix" class="ai-fix-button"${d}>${w}</button>
    </div>`}function pe(e){if(!e.preview)return'<div class="detail-empty">Generate a fix preview to inspect the patch before Ollanta edits your local file.</div>';let t=e.preview.summary||"Generated fix preview",s=e.preview.explanation?`<div class="rule-rationale">${a(e.preview.explanation)}</div>`:"",i=e.applying?"Applying\u2026":"Apply to file",l=e.applying?" disabled":"";return`
    <div class="ai-fix-preview-meta">
      <div><strong>Provider:</strong> ${a(e.preview.agent.label)}</div>
      <div><strong>Model:</strong> ${a(e.preview.agent.model)}</div>
      <div><strong>Summary:</strong> ${a(t)}</div>
    </div>
    ${s}
    <pre class="rule-code ai-fix-diff"><code>${a(e.preview.diff)}</code></pre>
    <div class="ai-fix-actions">
      <button id="ai-apply-fix" class="ai-fix-button ai-fix-button-primary"${l}>${i}</button>
    </div>
  `}var y,u=[],L=[],$=[],v=[],C="",R=!1,N="",z=new Map,m=null,I=-1,le="overview",M="details",A="",V=!1,p=null,o=se(),k="all",J="all",W="all",X="",q={blocker:0,critical:1,major:2,minor:3,info:4},T={blocker:"#ef4444",critical:"#f97316",major:"#eab308",minor:"#22c55e",info:"#64748b"},ee={bug:"Bug",code_smell:"Code Smell",vulnerability:"Vulnerability",security_hotspot:"Hotspot"};async function ue(){try{let e=await fetch("/report.json");if(!e.ok)throw new Error(`HTTP ${e.status}`);y=await e.json(),u=y.issues??[],ge(),me(),ye(),$e(),we(),Le(),xe(),Me(),O(),v.length&&te(C||v[0].path),Pe(),Ae(),E(),He(),re(),_e(),Ge(),n("tab-issue-count").textContent=String(u.length),n("tab-file-count").textContent=String(new Set(u.map(t=>t.component_path)).size),n("tab-coverage-count").textContent=String(v.length)}catch(e){n("app").innerHTML=`<div class="error">Failed to load report: ${String(e)}</div>`}}document.addEventListener("DOMContentLoaded",ue);function ge(){let e=y.metadata,t=new Date(e.analysis_date).toLocaleString();n("project-key").textContent=e.project_key,n("scan-date").textContent=t,n("scan-version").textContent=`v${e.version}`,n("elapsed").textContent=`${e.elapsed_ms}ms`}function fe(){let e=y.measures,t=[{metric:"Bugs",operator:"=",threshold:0,value:e.bugs,passed:e.bugs===0},{metric:"Vulnerabilities",operator:"=",threshold:0,value:e.vulnerabilities,passed:e.vulnerabilities===0}];return{status:t.every(i=>i.passed)?"passed":"failed",conditions:t}}function me(){let e=fe(),t=n("gate-hero");t.classList.remove("gate-loading"),t.classList.add(e.status==="passed"?"gate-passed":"gate-failed"),n("gate-icon").textContent=e.status==="passed"?"\u2713":"\u2717",n("gate-status").textContent=e.status==="passed"?"Passed":"Failed";let s=e.conditions.map(i=>{let l=i.passed?"cond-pass":"cond-fail",r=i.passed?"\u2713":"\u2717";return`<div class="gate-cond ${l}">
      <span class="gate-cond-icon">${r}</span>
      <span class="gate-cond-metric">${a(i.metric)}</span>
      <span class="gate-cond-value">${i.value}</span>
    </div>`}).join("");n("gate-conditions").innerHTML=s}function ye(){let e=y.measures;S("m-bugs",e.bugs),S("m-vulns",e.vulnerabilities),S("m-smells",e.code_smells),he("m-coverage",P(e.coverage)),S("m-ncloc",e.ncloc),S("m-files",e.files),S("m-comments",e.comments),U("card-bugs",e.bugs,[0,1,5]),U("card-vulns",e.vulnerabilities,[0,1,3]),U("card-smells",e.code_smells,[0,10,50]),be("card-coverage",e.coverage),b("card-ncloc","card-neutral"),b("card-files","card-neutral"),b("card-comments","card-neutral");let t=n("card-coverage");t.classList.add("clickable"),t.addEventListener("click",()=>{G("coverage")})}function S(e,t){n(e).textContent=t.toLocaleString()}function he(e,t){n(e).textContent=t}function P(e){return e==null?"\u2014":`${e.toFixed(1)}%`}function U(e,t,s){t<=s[0]?b(e,"card-green"):t<=s[1]?b(e,"card-yellow"):b(e,"card-red")}function be(e,t){t==null?b(e,"card-neutral"):t>=80?b(e,"card-green"):t>=60?b(e,"card-yellow"):b(e,"card-red")}function $e(){let e=D(u,d=>d.severity),t=Math.max(1,...Object.values(e)),s=["blocker","critical","major","minor","info"],i="",l="",r=u.length||1;for(let d of s){let c=e[d]??0,x=c/t*100,j=T[d]??"#64748b";i+=`<div class="sev-bar-row">
      <span class="sev-bar-label">${d}</span>
      <div class="sev-bar-track"><div class="sev-bar-fill" style="width:${x}%;background:${j}"></div></div>
      <span class="sev-bar-count">${c}</span>
    </div>`,c>0&&(l+=`<div class="sev-segment" style="width:${c/r*100}%;background:${j}" title="${d}: ${c}"></div>`)}n("sev-bars").innerHTML=i,n("sev-proportional").innerHTML=l;let h=D(u,d=>d.type),g=Math.max(1,...Object.values(h)),_={bug:"#ef4444",vulnerability:"#f97316",code_smell:"#22c55e",security_hotspot:"#eab308"},w="";for(let[d,c]of Object.entries(ee)){let x=h[d]??0,j=x/g*100,de=_[d]??"#64748b";w+=`<div class="sev-bar-row">
      <span class="sev-bar-label">${c}</span>
      <div class="sev-bar-track"><div class="sev-bar-fill" style="width:${j}%;background:${de}"></div></div>
      <span class="sev-bar-count">${x}</span>
    </div>`}n("type-bars").innerHTML=w}function we(){let e=[...u].sort((t,s)=>{let i=(q[t.severity]??99)-(q[s.severity]??99);return i!==0?i:t.component_path.localeCompare(s.component_path)||t.line-s.line}).slice(0,6);if(!e.length){n("priority-issues").innerHTML='<div class="empty-state compact">No issues found</div>';return}n("priority-issues").innerHTML=e.map((t,s)=>{let i=T[t.severity]??"#64748b",l=H(t.component_path);return`<button class="priority-row" data-idx="${s}">
      <span class="issue-sev-dot" style="background:${i}"></span>
      <span class="priority-main">
        <span class="priority-title">${a(t.message)}</span>
        <span class="priority-meta" title="${a(t.component_path)}">${a(l)}:L${t.line} \xB7 ${a(t.rule_key)}</span>
      </span>
      <span class="priority-severity">${a(t.severity)}</span>
    </button>`}).join(""),n("priority-issues").querySelectorAll(".priority-row").forEach(t=>{t.addEventListener("click",()=>{let s=Number.parseInt(t.dataset.idx,10);K(e[s])})})}function Le(){let e=D(u,s=>s.component_path),t=Object.entries(e).sort((s,i)=>i[1]-s[1]).slice(0,10);if(!t.length){n("hotspot-files").innerHTML='<div class="empty-state">No issues found</div>';return}n("hotspot-files").innerHTML=t.map(([s,i])=>{let l=H(s);return`<div class="hotspot-row" data-path="${a(s)}">
      <span class="hotspot-file" title="${a(s)}">${a(l)}</span>
      <span class="hotspot-count">${i}</span>
    </div>`}).join(""),n("hotspot-files").querySelectorAll(".hotspot-row").forEach(s=>{s.addEventListener("click",()=>{let i=s.dataset.path;G("files"),je(i)})})}function xe(){v=(y.test_signals?.modules??[]).flatMap(t=>(t.files??[]).map(s=>Ie(t.name,t.root,s))).filter(t=>t.linesToCover>0).sort((t,s)=>(t.coverage??101)-(s.coverage??101)||s.uncoveredLines.length-t.uncoveredLines.length||t.path.localeCompare(s.path)),!C&&v.length&&(C=v[0].path)}function Ie(e,t,s){let i=s.lines_to_cover??0,l=s.covered_lines??0,r=i>0?l*100/i:null;return{moduleName:e,moduleRoot:t,path:s.path,linesToCover:i,coveredLines:l,coveredLineNumbers:s.covered_line_numbers??[],uncoveredLines:s.uncovered_lines??[],coverage:r}}function Me(){let e=n("coverage-summary");if(!v.length){e.innerHTML='<div class="empty-state compact">Run with <span class="mono">-with-tests</span> and provide a coverage report to see file-level details.</div>';return}let t=y.test_signals?.summary,s=v.slice(0,5);e.innerHTML=`<div class="coverage-kpis">
      <div><span class="coverage-kpi-value">${P(t?.coverage??y.measures.coverage)}</span><span class="coverage-kpi-label">overall</span></div>
      <div><span class="coverage-kpi-value">${(t?.covered_lines??0).toLocaleString()}/${(t?.lines_to_cover??0).toLocaleString()}</span><span class="coverage-kpi-label">covered lines</span></div>
      <div><span class="coverage-kpi-value">${(t?.modules_with_coverage??0).toLocaleString()}</span><span class="coverage-kpi-label">modules</span></div>
    </div>
    <div class="coverage-file-list">
      ${s.map(ke).join("")}
    </div>`,e.querySelectorAll(".coverage-mini-row").forEach(i=>{i.addEventListener("click",()=>{let l=i.dataset.coveragePath;l&&(G("coverage"),te(l))})})}function ke(e){return`<button class="coverage-mini-row" data-coverage-path="${a(e.path)}">
    <span class="coverage-mini-main">
      <span class="coverage-file-name" title="${a(e.path)}">${a(H(e.path))}</span>
      <span class="coverage-file-meta">${a(e.moduleName)} \xB7 ${e.uncoveredLines.length.toLocaleString()} uncovered lines</span>
    </span>
    <span class="coverage-pill ${F(e.coverage)}">${P(e.coverage??void 0)}</span>
  </button>`}function O(){let e=n("coverage-details");if(!v.length){e.innerHTML='<div class="empty-state">No file-level coverage was collected for this scan.</div>';return}e.innerHTML=`<div class="coverage-toolbar">
      <div>
        <h3>Coverage Files</h3>
        <p>${v.length.toLocaleString()} files with executable lines from collected test reports</p>
      </div>
      <span class="coverage-pill ${F(y.measures.coverage??null)}">${P(y.measures.coverage)}</span>
    </div>
    <div class="coverage-browser">
      <aside class="coverage-sidebar">
        <div class="coverage-file-list coverage-file-list-large">
          ${v.map(Se).join("")}
        </div>
      </aside>
      <section class="coverage-code-viewer">
        ${Ee()}
      </section>
    </div>`,e.querySelectorAll(".coverage-row").forEach(t=>{t.addEventListener("click",()=>{let s=t.dataset.coveragePath;s&&te(s)})})}function Se(e){return`<button class="coverage-row${e.path===C?" active":""}" data-coverage-path="${a(e.path)}">
    <div class="coverage-row-main">
      <div class="coverage-row-title" title="${a(e.path)}">${a(e.path)}</div>
      <div class="coverage-row-subtitle">${a(e.moduleName)} \xB7 ${a(e.moduleRoot)} \xB7 ${e.coveredLines.toLocaleString()}/${e.linesToCover.toLocaleString()} lines covered</div>
    </div>
    <div class="coverage-row-meter">
      <span class="coverage-pill ${F(e.coverage)}">${P(e.coverage??void 0)}</span>
      <div class="coverage-track"><div class="coverage-fill ${F(e.coverage)}" style="width:${e.coverage??0}%"></div></div>
    </div>
  </button>`}async function te(e){if(v.some(t=>t.path===e)){if(C=e,N="",z.has(e)){R=!1,O();return}R=!0,O();try{let t=await fetch(`/api/files/source?path=${encodeURIComponent(e)}`);if(!t.ok)throw new Error(`HTTP ${t.status}`);let s=await t.json();z.set(e,s.file)}catch(t){N=`Could not load source for ${e}: ${String(t)}`}finally{R=!1,O()}}}function Ee(){let e=v.find(g=>g.path===C);if(!e)return'<div class="code-empty"><p>Select a file to inspect coverage.</p></div>';if(R)return'<div class="code-empty"><div class="spinner"></div></div>';if(N)return`<div class="code-empty"><p>${a(N)}</p></div>`;let t=z.get(e.path);if(!t)return'<div class="code-empty"><p>Select a coverage file to inspect source lines.</p></div>';let s=new Set(e.coveredLineNumbers),i=new Set(e.uncoveredLines),r=t.content.split(`
`).map((g,_)=>{let w=_+1,d=i.has(w),c=!d&&s.has(w),x=Ce(c,d);return`<div class="code-line${x.stateClass}">
      <span class="code-gutter">${w}</span>
      <code class="code-text">${g.length?a(g):"&nbsp;"}</code>
      <span class="code-markers">${Te(x)}</span>
    </div>`}).join(""),h=e.coveredLineNumbers.length?"covered and uncovered lines":"uncovered lines only";return`<div class="code-viewer-shell coverage-source-shell">
    <div class="code-viewer-head">
      <div>
        <div class="code-viewer-path mono">${a(t.path)}</div>
        <div class="code-viewer-meta">${a(t.language||"plain text")} \xB7 ${t.line_count.toLocaleString()} lines \xB7 ${h}</div>
      </div>
      <div class="code-viewer-stats"><span class="coverage-pill ${F(e.coverage)}">${P(e.coverage??void 0)}</span></div>
    </div>
    <div class="coverage-legend">
      <span><span class="legend-dot legend-covered"></span>Covered</span>
      <span><span class="legend-dot legend-uncovered"></span>Not covered</span>
    </div>
    <div class="code-surface">${r}</div>
  </div>`}function Ce(e,t){return t?{stateClass:" is-uncovered",marker:"not covered",chipClass:" chip-uncovered"}:e?{stateClass:" is-covered",marker:"covered",chipClass:" chip-covered"}:{stateClass:"",marker:"",chipClass:""}}function Te(e){return e.marker?`<span class="coverage-line-chip${e.chipClass}">${e.marker}</span>`:""}function F(e){return e==null?"card-neutral":e>=80?"card-green":e>=60?"card-yellow":"card-red"}function Pe(){let e=Object.entries(y.measures.by_language).sort((s,i)=>i[1]-s[1]),t=Math.max(1,e[0]?.[1]??1);if(!e.length){n("by-lang").innerHTML='<span class="empty-state">No language data</span>';return}n("by-lang").innerHTML=e.map(([s,i])=>`<div class="lang-row">
      <span class="lang-name">${a(s)}</span>
      <div class="lang-bar-track"><div class="lang-bar-fill" style="width:${i/t*100}%"></div></div>
      <span class="lang-count">${i} files</span>
    </div>`).join("")}function _e(){document.querySelectorAll(".tab").forEach(e=>{e.addEventListener("click",()=>{let t=e.dataset.tab;G(t)})})}function G(e){le=e,document.querySelectorAll(".tab").forEach(t=>t.classList.remove("active")),document.querySelector(`.tab[data-tab="${e}"]`)?.classList.add("active"),document.querySelectorAll(".panel").forEach(t=>t.classList.add("hidden")),n(`panel-${e}`).classList.remove("hidden")}function Ae(){let e=[...new Set(u.map(s=>s.rule_key))].sort((s,i)=>s.localeCompare(i)),t=n("filter-rule");e.forEach(s=>{let i=document.createElement("option");i.value=s,i.textContent=s,t.appendChild(i)}),n("filter-severity").addEventListener("change",s=>{k=s.target.value,E()}),n("filter-type").addEventListener("change",s=>{J=s.target.value,E()}),t.addEventListener("change",s=>{W=s.target.value,E()}),n("search").addEventListener("input",s=>{X=s.target.value.toLowerCase(),E()}),oe()}function oe(){let e=D(u,s=>s.severity),t=["blocker","critical","major","minor","info"];n("sev-chips").innerHTML=t.map(s=>{let i=e[s]??0,l=T[s]??"#64748b";return`<div class="sev-chip${k===s?" active":""}" data-sev="${s}"
      style="--chip-color:${l};--chip-bg:${l}15">
      <span class="chip-dot" style="background:${l}"></span>
      ${s}
      <span class="chip-count">${i}</span>
    </div>`}).join(""),n("sev-chips").querySelectorAll(".sev-chip").forEach(s=>{s.addEventListener("click",()=>{let i=s.dataset.sev;k=k===i?"all":i,n("filter-severity").value=k,E(),oe()})})}function E(){L=u.filter(e=>!(k!=="all"&&e.severity!==k||J!=="all"&&e.type!==J||W!=="all"&&e.rule_key!==W||X&&!`${e.component_path} ${e.message} ${e.rule_key}`.toLowerCase().includes(X))),L.sort((e,t)=>{let s=q[e.severity]??99,i=q[t.severity]??99;return s-i}),I=-1,Fe()}function Fe(){let e=n("issue-list"),t=L.length===1?"issue":"issues";if(n("issue-count").textContent=`${L.length} ${t}`,!L.length){e.innerHTML='<div class="empty-state">No issues match the current filters.</div>';return}e.innerHTML=L.map((s,i)=>{let l=T[s.severity]??"#64748b",r=H(s.component_path),h=s.end_line&&s.end_line!==s.line?`L${s.line}\u2013${s.end_line}`:`L${s.line}`,g=ee[s.type]??s.type;return`<div class="issue-row" data-idx="${i}">
      <span class="issue-sev">
        <span class="issue-sev-dot" style="background:${l}"></span>
        ${a(s.severity)}
      </span>
      <span class="issue-type">${a(g)}</span>
      <div class="issue-main">
        <span class="issue-msg">${a(s.message)}</span>
        <span class="issue-file" title="${a(s.component_path)}">${a(r)}:${h}</span>
      </div>
      <span class="issue-rule">${a(s.rule_key)}</span>
    </div>`}).join(""),e.querySelectorAll(".issue-row").forEach(s=>{s.addEventListener("click",()=>{let i=Number.parseInt(s.dataset.idx,10);Y(i)})})}function He(){let e=new Map;for(let t of u){let s=t.component_path;e.has(s)||e.set(s,[]),e.get(s).push(t)}$=[...e.entries()].sort((t,s)=>s[1].length-t[1].length).map(([t,s])=>({path:t,shortPath:H(t),issues:[...s].sort((i,l)=>i.line-l.line),expanded:!1}))}function re(){let e=n("file-tree");if(!$.length){e.innerHTML='<div class="empty-state">No issues found</div>';return}e.innerHTML=$.map((t,s)=>`<div class="file-group${t.expanded?" expanded":""}" data-gi="${s}">
      <div class="file-group-header">
        <span class="file-group-chevron">\u25B6</span>
        <span class="file-group-name" title="${a(t.path)}">${a(t.shortPath)}</span>
        <span class="file-group-count">${t.issues.length}</span>
      </div>
      <div class="file-group-issues" style="${t.expanded?"":"display:none"}">
        ${t.issues.map((i,l)=>{let r=T[i.severity]??"#64748b";return`<div class="file-issue" data-gi="${s}" data-ii="${l}">
            <span class="issue-sev">
              <span class="issue-sev-dot" style="background:${r}"></span>
              ${a(i.severity)}
            </span>
            <span class="issue-msg">${a(i.message)}</span>
            <span class="file-issue-line">L${i.line}</span>
          </div>`}).join("")}
      </div>
    </div>`).join(""),e.querySelectorAll(".file-group-header").forEach(t=>{t.addEventListener("click",()=>{let s=t.closest(".file-group"),i=Number.parseInt(s.dataset.gi,10);$[i].expanded=!$[i].expanded,s.classList.toggle("expanded");let l=s.querySelector(".file-group-issues");l.style.display=$[i].expanded?"":"none"})}),e.querySelectorAll(".file-issue").forEach(t=>{t.addEventListener("click",s=>{s.stopPropagation();let i=Number.parseInt(t.dataset.gi,10),l=Number.parseInt(t.dataset.ii,10),r=$[i].issues[l];K(r)})})}function je(e){let t=$.findIndex(i=>i.path===e);if(t<0)return;$[t].expanded=!0,re(),document.querySelector(`.file-group[data-gi="${t}"]`)?.scrollIntoView({behavior:"smooth",block:"start"})}function Y(e){I=e,m=L[e]??null,document.querySelectorAll(".issue-row").forEach(t=>t.classList.remove("selected")),document.querySelector(`.issue-row[data-idx="${e}"]`)?.classList.add("selected"),m&&K(m)}function K(e){m=e,M="details",A="",V=!0,o=se(),n("detail-title").textContent=e.rule_key,B(e),n("detail-panel").classList.add("open"),n("detail-overlay").classList.add("open"),Re(e.rule_key)}async function Re(e){try{let t=await fetch(`/rules/${encodeURIComponent(e)}`);if(!t.ok)throw new Error("not found");let s=await t.json(),i="";s.rationale&&(i+=`<div class="detail-section">
        <div class="detail-section-title">Why is this a problem?</div>
        <div class="rule-rationale">${a(s.rationale)}</div>
      </div>`),s.description&&s.description!==s.rationale&&(i+=`<div class="detail-section">
        <div class="detail-section-title">Description</div>
        <div class="rule-rationale">${a(s.description)}</div>
      </div>`),s.noncompliant_code&&(i+=`<div class="detail-section">
        <div class="detail-section-title">\u2718 Noncompliant Code</div>
        <pre class="rule-code noncompliant"><code>${a(s.noncompliant_code)}</code></pre>
      </div>`),s.compliant_code&&(i+=`<div class="detail-section">
        <div class="detail-section-title">\u2714 Compliant Code</div>
        <pre class="rule-code compliant"><code>${a(s.compliant_code)}</code></pre>
      </div>`),A=i||'<div class="detail-empty">No additional rule details available.</div>'}catch{A='<div class="detail-empty">Rule details are not available for this issue.</div>'}finally{V=!1,m?.rule_key===e&&B(m)}}function Q(){n("detail-panel").classList.remove("open"),n("detail-overlay").classList.remove("open"),m=null,A="",V=!1,o=se(),document.querySelectorAll(".issue-row").forEach(e=>e.classList.remove("selected"))}function B(e){let t=`
    <div class="detail-tabs">
      ${ie(M)}
    </div>
    <div class="detail-tab-panel${M==="details"?"":" hidden"}" data-detail-panel="details">
      ${Oe(e)}
    </div>
    <div class="detail-tab-panel${M==="rule"?"":" hidden"}" data-detail-panel="rule">
      ${V?'<div class="detail-loading">Loading rule details\u2026</div>':A}
    </div>
    <div class="detail-tab-panel${M==="ai-fix"?"":" hidden"}" data-detail-panel="ai-fix">
      ${ae(e,o,p??[])}
    </div>
  `;n("detail-body").innerHTML=t,Ne(e)}function Oe(e){let t=T[e.severity]??"#64748b",s=ee[e.type]??e.type,i=e.end_line&&e.end_line!==e.line?`${e.line}:${e.column} \u2013 ${e.end_line}:${e.end_column}`:`${e.line}:${e.column}`,l=`
    <div class="detail-section">
      <div class="detail-msg">${a(e.message)}</div>
    </div>
    <div class="detail-section">
      <div class="detail-section-title">Properties</div>
      <div class="detail-field">
        <span class="detail-field-label">Severity</span>
        <span class="detail-field-value"><span class="issue-sev-dot" style="background:${t};display:inline-block;width:8px;height:8px;border-radius:50%;margin-right:6px"></span>${a(e.severity)}</span>
      </div>
      <div class="detail-field">
        <span class="detail-field-label">Type</span>
        <span class="detail-field-value">${a(s)}</span>
      </div>
      <div class="detail-field">
        <span class="detail-field-label">Rule</span>
        <span class="detail-field-value" style="font-family:var(--font-mono);color:var(--accent)">${a(e.rule_key)}</span>
      </div>
      <div class="detail-field">
        <span class="detail-field-label">Status</span>
        <span class="detail-field-value">${a(e.status)}</span>
      </div>
      ${e.engine_id?`<div class="detail-field">
        <span class="detail-field-label">Engine</span>
        <span class="detail-field-value">${a(e.engine_id)}</span>
      </div>`:""}
      ${e.tags?.length?`<div class="detail-field">
        <span class="detail-field-label">Tags</span>
        <span class="detail-field-value">${e.tags.map(r=>a(r)).join(", ")}</span>
      </div>`:""}
    </div>
    <div class="detail-section">
      <div class="detail-section-title">Location</div>
      <div class="detail-field">
        <span class="detail-field-label">File</span>
        <span class="detail-field-value" style="font-family:var(--font-mono);font-size:12px;word-break:break-all">${a(e.component_path)}</span>
      </div>
      <div class="detail-field">
        <span class="detail-field-label">Lines</span>
        <span class="detail-field-value" style="font-family:var(--font-mono)">${i}</span>
      </div>
    </div>`;return e.secondary_locations?.length&&(l+=`<div class="detail-section">
      <div class="detail-section-title">Related Locations (${e.secondary_locations.length})</div>
      <div class="detail-loc-list">
        ${e.secondary_locations.map(r=>`
          <div class="detail-loc-item">
            <div class="detail-loc-file">${a(r.file_path||e.component_path)}:${r.start_line}</div>
            ${r.message?`<div class="detail-loc-msg">${a(r.message)}</div>`:""}
          </div>
        `).join("")}
      </div>
    </div>`),l}function Ne(e){document.querySelectorAll(".detail-tab").forEach(l=>{l.addEventListener("click",()=>{M=l.dataset.detailTab??"details",B(e),M==="ai-fix"&&qe()})});let t=document.getElementById("ai-provider-select");t?.addEventListener("change",()=>{o.selectedProviderId=t.value,o.selectedModel="",Z(),o.preview=null,o.statusMessage="",o.errorMessage="",f()});let s=document.getElementById("ai-model-input");s?.addEventListener("input",()=>{o.selectedModel=s.value});let i=document.getElementById("ai-api-key-input");i?.addEventListener("input",()=>{o.apiKey=i.value}),document.getElementById("ai-generate-fix")?.addEventListener("click",()=>{De(e)}),document.getElementById("ai-apply-fix")?.addEventListener("click",()=>{Ve()})}function se(){return{loadingOptions:!1,loadingPreview:!1,applying:!1,selectedProviderId:"",selectedModel:"",apiKey:"",statusMessage:"",errorMessage:"",preview:null}}function ce(){return!p||p.length===0?null:p.find(e=>e.id===o.selectedProviderId)??p[0]}function Z(){if(!p||p.length===0){o.selectedProviderId="",o.selectedModel="";return}p.some(t=>t.id===o.selectedProviderId)||(o.selectedProviderId=p[0].id);let e=ce();if(!e){o.selectedModel="";return}o.selectedModel||(o.selectedModel=e.default_model||e.models[0]||"")}async function qe(){if(p){Z(),f();return}o.loadingOptions=!0,o.errorMessage="",f();try{let e=await fetch("/api/ai/providers");if(!e.ok)throw new Error(`HTTP ${e.status}`);p=(await e.json()).providers??[],Z()}catch(e){o.errorMessage=`Failed to load AI models: ${String(e)}`,p=[]}finally{o.loadingOptions=!1,f()}}async function De(e){let t=ce(),s=o.selectedModel.trim();if(!t||!o.selectedProviderId){o.errorMessage="Choose an AI provider before generating a fix.",f();return}if(!s){o.errorMessage="Choose a model before generating a fix.",f();return}if(t.requires_api_key&&!t.configured&&!o.apiKey.trim()){o.errorMessage="Provide an API key for the selected provider before generating a fix.",f();return}o.selectedModel=s,o.loadingPreview=!0,o.statusMessage="",o.errorMessage="",f();try{let i={provider:o.selectedProviderId,model:s,api_key:o.apiKey.trim()||void 0,issue:e},l=await fetch("/api/ai/fixes/preview",{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify(i)}),r=await l.json();if(!l.ok||"error"in r)throw new Error("error"in r?r.error:`HTTP ${l.status}`);o.preview=r,o.statusMessage="Fix preview generated. Review the diff before applying it."}catch(i){o.errorMessage=`Failed to generate AI fix: ${String(i)}`,o.preview=null}finally{o.loadingPreview=!1,f()}}async function Ve(){if(o.preview){o.applying=!0,o.errorMessage="",f();try{let e=await fetch("/api/ai/fixes/apply",{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify({preview_id:o.preview.preview_id})}),t=await e.json();if(!e.ok||"error"in t)throw new Error("error"in t?t.error:`HTTP ${e.status}`);o.statusMessage=t.message}catch(e){o.errorMessage=`Failed to apply AI fix: ${String(e)}`}finally{o.applying=!1,f()}}}function f(){m&&B(m)}document.addEventListener("DOMContentLoaded",()=>{n("detail-close").addEventListener("click",Q),n("detail-overlay").addEventListener("click",Q)});function Ge(){document.addEventListener("keydown",e=>{let t=e.target.tagName;if(!(t==="INPUT"||t==="SELECT"||t==="TEXTAREA")){if(e.key==="Escape"){Q();return}le==="issues"&&(e.key==="j"||e.key==="ArrowDown"?(e.preventDefault(),I<L.length-1&&Y(I+1),ne()):e.key==="k"||e.key==="ArrowUp"?(e.preventDefault(),I>0&&Y(I-1),ne()):e.key==="Enter"&&m&&K(m))}})}function ne(){document.querySelector(`.issue-row[data-idx="${I}"]`)?.scrollIntoView({behavior:"smooth",block:"nearest"})}function n(e){return document.getElementById(e)}function b(e,t){n(e).classList.add(t)}function D(e,t){let s={};for(let i of e){let l=t(i);s[l]=(s[l]??0)+1}return s}function H(e){let t=e.replaceAll("\\","/"),s=t.split("/").filter(Boolean);return s.length<=2?t:`${s.slice(-2).join("/")}`}})();
