"use strict";(()=>{function i(e){return e.replaceAll("&","&amp;").replaceAll("<","&lt;").replaceAll(">","&gt;").replaceAll('"',"&quot;")}function ae(e){return[{key:"details",label:"Details"},{key:"rule",label:"Rule"},{key:"ai-fix",label:"Fix with AI"}].map(s=>`<button class="detail-tab${e===s.key?" active":""}" data-detail-tab="${s.key}">${s.label}</button>`).join("")}function ie(e,t,s){let a=e.end_line&&e.end_line!==e.line?`-${e.end_line}`:"",n=ue(t,s),r=pe(t);return`
    <div class="detail-section">
      <div class="detail-section-title">Fix with AI</div>
      <div class="detail-msg ai-fix-callout">Ollanta prepares the issue context, sends only the relevant snippet to the selected agent, and shows a preview before writing any changes to your code.</div>
    </div>

    <div class="detail-section">
      <div class="detail-field detail-field-stack">
        <span class="detail-field-label">Target</span>
        <span class="detail-field-value detail-mono-block">${i(e.component_path)}:${e.line}${a}</span>
      </div>
      <div class="detail-field detail-field-stack">
        <span class="detail-field-label">Issue</span>
        <span class="detail-field-value">${i(e.message)}</span>
      </div>
    </div>

    <div class="detail-section">
      <div class="detail-section-title">Model</div>
      ${n}
      ${t.statusMessage?`<div class="ai-fix-status ai-fix-status-ok">${i(t.statusMessage)}</div>`:""}
      ${t.errorMessage?`<div class="ai-fix-status ai-fix-status-error">${i(t.errorMessage)}</div>`:""}
    </div>

    <div class="detail-section">
      <div class="detail-section-title">Preview</div>
      ${r}
    </div>
  `}function ue(e,t){if(e.loadingOptions)return'<div class="detail-loading">Loading AI models\u2026</div>';if(t.length===0)return'<div class="detail-empty">No AI provider is available for the local scanner.</div>';let s=t.find(d=>d.id===e.selectedProviderId)??t[0],a=t.map(d=>`<option value="${i(d.id)}"${e.selectedProviderId===d.id?" selected":""}>${i(d.label)}</option>`).join(""),r=(s?.models??[]).map(d=>`<option value="${i(d)}"></option>`).join(""),g='<div class="ai-fix-helper">This provider can generate local previews without an API key.</div>',u="Required for this provider";s?.requires_api_key&&(s.configured?(g=`<div class="ai-fix-helper">Using the scanner's configured API key. Paste another key below to override it for this session.</div>`,u="Optional override"):g='<div class="ai-fix-helper">Paste an API key for the selected provider to generate the fix.</div>');let A=s?.requires_api_key?`<div class="ai-fix-control-group">
          <label class="ai-fix-control-label" for="ai-api-key-input">API key</label>
          <input id="ai-api-key-input" class="ai-fix-select ai-fix-input" type="password" value="${i(e.apiKey)}" placeholder="${u}" autocomplete="off">
        </div>`:"",w=e.loadingPreview?"Generating\u2026":"Generate fix",v=e.loadingPreview?" disabled":"";return`<div class="ai-fix-controls">
      <div class="ai-fix-control-group">
        <label class="ai-fix-control-label" for="ai-provider-select">Provider</label>
        <select id="ai-provider-select" class="ai-fix-select">${a}</select>
      </div>
      <div class="ai-fix-control-group">
        <label class="ai-fix-control-label" for="ai-model-input">Model</label>
        <input id="ai-model-input" class="ai-fix-select ai-fix-input" list="ai-model-options" value="${i(e.selectedModel)}" placeholder="${i(s?.default_model||"gpt-5.5")}" autocomplete="off">
        <datalist id="ai-model-options">${r}</datalist>
      </div>
      ${A}
      ${g}
      <button id="ai-generate-fix" class="ai-fix-button"${v}>${w}</button>
    </div>`}function pe(e){if(!e.preview)return'<div class="detail-empty">Generate a fix preview to inspect the patch before Ollanta edits your local file.</div>';let t=e.preview.summary||"Generated fix preview",s=e.preview.explanation?`<div class="rule-rationale">${i(e.preview.explanation)}</div>`:"",a=e.applying?"Applying\u2026":"Apply to file",n=e.applying?" disabled":"";return`
    <div class="ai-fix-preview-meta">
      <div><strong>Provider:</strong> ${i(e.preview.agent.label)}</div>
      <div><strong>Model:</strong> ${i(e.preview.agent.model)}</div>
      <div><strong>Summary:</strong> ${i(t)}</div>
    </div>
    ${s}
    <pre class="rule-code ai-fix-diff"><code>${i(e.preview.diff)}</code></pre>
    <div class="ai-fix-actions">
      <button id="ai-apply-fix" class="ai-fix-button ai-fix-button-primary"${n}>${a}</button>
    </div>
  `}var c,f=[],L=[],b=[],p=[],C="",O=!1,N="",z=new Map,h=null,S=-1,oe="overview",k="details",H="",V=!1,m=null,l=se(),_="all",J="all",X="all",W="",q={blocker:0,critical:1,major:2,minor:3,info:4},E={blocker:"#ef4444",critical:"#f97316",major:"#eab308",minor:"#22c55e",info:"#64748b"},ee={bug:"Bug",code_smell:"Code Smell",vulnerability:"Vulnerability",security_hotspot:"Hotspot"};async function ge(){try{let e=await fetch("/report.json");if(!e.ok)throw new Error(`HTTP ${e.status}`);c=await e.json(),f=c.issues??[],me(),ye(),he(),Se(),ke(),_e(),Ie(),Ce(),we(),R(),p.length&&te(C||p[0].path),je(),Re(),T(),qe(),ce(),Oe(),Je(),o("tab-issue-count").textContent=String(f.length),o("tab-file-count").textContent=String(new Set(f.map(t=>t.component_path)).size),o("tab-coverage-count").textContent=String(p.length)}catch(e){o("app").innerHTML=`<div class="error">Failed to load report: ${String(e)}</div>`}}document.addEventListener("DOMContentLoaded",ge);function me(){let e=c.metadata,t=new Date(e.analysis_date).toLocaleString();o("project-key").textContent=e.project_key,o("scan-date").textContent=t,o("scan-version").textContent=`v${e.version}`,o("elapsed").textContent=`${e.elapsed_ms}ms`}function fe(){let e=c.measures,t=[{metric:"Bugs",operator:"=",threshold:0,value:e.bugs,passed:e.bugs===0},{metric:"Vulnerabilities",operator:"=",threshold:0,value:e.vulnerabilities,passed:e.vulnerabilities===0}];return{status:t.every(a=>a.passed)?"passed":"failed",conditions:t}}function ye(){let e=fe(),t=o("gate-hero");t.classList.remove("gate-loading"),t.classList.add(e.status==="passed"?"gate-passed":"gate-failed"),o("gate-icon").textContent=e.status==="passed"?"\u2713":"\u2717",o("gate-status").textContent=e.status==="passed"?"Passed":"Failed";let s=e.conditions.map(a=>{let n=a.passed?"cond-pass":"cond-fail",r=a.passed?"\u2713":"\u2717";return`<div class="gate-cond ${n}">
      <span class="gate-cond-icon">${r}</span>
      <span class="gate-cond-metric">${i(a.metric)}</span>
      <span class="gate-cond-value">${a.value}</span>
    </div>`}).join("");o("gate-conditions").innerHTML=s}function he(){let e=c.measures;I("m-bugs",e.bugs),I("m-vulns",e.vulnerabilities),I("m-smells",e.code_smells),$e("m-coverage",x(e.coverage)),I("m-ncloc",e.ncloc),I("m-files",e.files),I("m-comments",e.comments),U("card-bugs",e.bugs,[0,1,5]),U("card-vulns",e.vulnerabilities,[0,1,3]),U("card-smells",e.code_smells,[0,10,50]),be("card-coverage",e.coverage),$("card-ncloc","card-neutral"),$("card-files","card-neutral"),$("card-comments","card-neutral");let t=o("card-coverage");t.classList.add("clickable"),t.addEventListener("click",()=>{G("coverage")})}function I(e,t){o(e).textContent=t.toLocaleString()}function $e(e,t){o(e).textContent=t}function x(e){return e==null?"\u2014":`${e.toFixed(1)}%`}function U(e,t,s){t<=s[0]?$(e,"card-green"):t<=s[1]?$(e,"card-yellow"):$(e,"card-red")}function be(e,t){t==null?$(e,"card-neutral"):t>=80?$(e,"card-green"):t>=60?$(e,"card-yellow"):$(e,"card-red")}function we(){let e=o("mutation-summary"),t=Le();if(!t.hasSignal){e.innerHTML='<div class="empty-state compact">No mutation report was collected for this scan. Add a supported report such as <span class="mono">ollanta-mutations.json</span>, Stryker JSON, or PIT XML to see mutation score and survived mutants.</div>';return}e.innerHTML=`<div class="mutation-kpis">
      <div><span class="mutation-kpi-value ${le(t.score)}">${x(t.score)}</span><span class="mutation-kpi-label">mutation score</span></div>
      <div><span class="mutation-kpi-value">${t.killed.toLocaleString()}/${t.total.toLocaleString()}</span><span class="mutation-kpi-label">killed mutants</span></div>
      <div><span class="mutation-kpi-value ${t.survived>0?"mut-warning":"mut-success"}">${t.survived.toLocaleString()}</span><span class="mutation-kpi-label">survived mutants</span></div>
    </div>
    ${xe(t.modules)}
    ${Me(t.survivedMutants)}`}function Le(){let e=c.test_signals?.summary,t=(c.test_signals?.modules??[]).filter(u=>u.mutation),s=e?.changed_mutants_total||e?.mutants_total||c.measures.changed_mutants_total||c.measures.mutants_total||0,a=e?.changed_mutants_killed||e?.mutants_killed||c.measures.changed_mutants_killed||c.measures.mutants_killed||0,n=e?.changed_mutants_survived||e?.mutants_survived||c.measures.changed_mutants_survived||c.measures.mutants_survived||0,r=e?.changed_mutation_score??e?.mutation_score??c.measures.changed_mutation_score??c.measures.mutation_score,g=t.flatMap(u=>u.mutation?.survived_mutants??[]).slice(0,8);return{hasSignal:t.length>0||s>0||r!=null,score:r,total:s,killed:a,survived:n,modules:t,survivedMutants:g}}function xe(e){return e.length?`<div class="mutation-module-list">
    ${e.slice(0,5).map(t=>{let s=t.mutation,a=s.changed_code_score??s.score,n=s.changed_survived??s.survived??0,r=s.changed_total??s.total??0;return`<div class="mutation-module-row">
        <span class="mutation-module-main"><span class="mutation-module-name">${i(t.name||t.root)}</span><span class="mutation-module-meta">${i(s.tool||"mutation")} \xB7 ${r.toLocaleString()} mutants</span></span>
        <span class="mutation-pill ${le(a)}">${x(a)}</span>
        <span class="mutation-survived ${n>0?"mut-warning":"mut-success"}">${n.toLocaleString()} survived</span>
      </div>`}).join("")}
  </div>`:""}function Me(e){return e.length?`<div class="mutation-survivors">
    ${e.map(t=>`<div class="mutation-survivor-row">
      <span class="mutation-survivor-file">${i(P(t.file||""))}${t.line?`:L${t.line}`:""}</span>
      <span class="mutation-survivor-meta">${i(t.mutator||t.description||"survived mutant")}</span>
    </div>`).join("")}
  </div>`:""}function le(e){return e==null?"card-neutral":e>=80?"card-green":e>=60?"card-yellow":"card-red"}function Se(){let e=D(f,v=>v.severity),t=Math.max(1,...Object.values(e)),s=["blocker","critical","major","minor","info"],a="",n="",r=f.length||1;for(let v of s){let d=e[v]??0,M=d/t*100,j=E[v]??"#64748b";a+=`<div class="sev-bar-row">
      <span class="sev-bar-label">${v}</span>
      <div class="sev-bar-track"><div class="sev-bar-fill" style="width:${M}%;background:${j}"></div></div>
      <span class="sev-bar-count">${d}</span>
    </div>`,d>0&&(n+=`<div class="sev-segment" style="width:${d/r*100}%;background:${j}" title="${v}: ${d}"></div>`)}o("sev-bars").innerHTML=a,o("sev-proportional").innerHTML=n;let g=D(f,v=>v.type),u=Math.max(1,...Object.values(g)),A={bug:"#ef4444",vulnerability:"#f97316",code_smell:"#22c55e",security_hotspot:"#eab308"},w="";for(let[v,d]of Object.entries(ee)){let M=g[v]??0,j=M/u*100,ve=A[v]??"#64748b";w+=`<div class="sev-bar-row">
      <span class="sev-bar-label">${d}</span>
      <div class="sev-bar-track"><div class="sev-bar-fill" style="width:${j}%;background:${ve}"></div></div>
      <span class="sev-bar-count">${M}</span>
    </div>`}o("type-bars").innerHTML=w}function ke(){let e=[...f].sort((t,s)=>{let a=(q[t.severity]??99)-(q[s.severity]??99);return a!==0?a:t.component_path.localeCompare(s.component_path)||t.line-s.line}).slice(0,6);if(!e.length){o("priority-issues").innerHTML='<div class="empty-state compact">No issues found</div>';return}o("priority-issues").innerHTML=e.map((t,s)=>{let a=E[t.severity]??"#64748b",n=P(t.component_path);return`<button class="priority-row" data-idx="${s}">
      <span class="issue-sev-dot" style="background:${a}"></span>
      <span class="priority-main">
        <span class="priority-title">${i(t.message)}</span>
        <span class="priority-meta" title="${i(t.component_path)}">${i(n)}:L${t.line} \xB7 ${i(t.rule_key)}</span>
      </span>
      <span class="priority-severity">${i(t.severity)}</span>
    </button>`}).join(""),o("priority-issues").querySelectorAll(".priority-row").forEach(t=>{t.addEventListener("click",()=>{let s=Number.parseInt(t.dataset.idx,10);K(e[s])})})}function _e(){let e=D(f,s=>s.component_path),t=Object.entries(e).sort((s,a)=>a[1]-s[1]).slice(0,10);if(!t.length){o("hotspot-files").innerHTML='<div class="empty-state">No issues found</div>';return}o("hotspot-files").innerHTML=t.map(([s,a])=>{let n=P(s);return`<div class="hotspot-row" data-path="${i(s)}">
      <span class="hotspot-file" title="${i(s)}">${i(n)}</span>
      <span class="hotspot-count">${a}</span>
    </div>`}).join(""),o("hotspot-files").querySelectorAll(".hotspot-row").forEach(s=>{s.addEventListener("click",()=>{let a=s.dataset.path;G("files"),De(a)})})}function Ie(){p=(c.test_signals?.modules??[]).flatMap(t=>(t.files??[]).map(s=>Te(t.name,t.root,s))).filter(t=>t.linesToCover>0).sort((t,s)=>(t.coverage??101)-(s.coverage??101)||s.uncoveredLines.length-t.uncoveredLines.length||t.path.localeCompare(s.path)),!C&&p.length&&(C=p[0].path)}function Te(e,t,s){let a=s.lines_to_cover??0,n=s.covered_lines??0,r=a>0?n*100/a:null;return{moduleName:e,moduleRoot:t,path:s.path,linesToCover:a,coveredLines:n,coveredLineNumbers:s.covered_line_numbers??[],uncoveredLines:s.uncovered_lines??[],coverage:r}}function Ce(){let e=o("coverage-summary");if(!p.length){e.innerHTML='<div class="empty-state compact">Run with <span class="mono">-with-tests</span> and provide a coverage report to see file-level details.</div>';return}let t=c.test_signals?.summary,s=p.slice(0,5);e.innerHTML=`<div class="coverage-kpis">
      <div><span class="coverage-kpi-value">${x(t?.coverage??c.measures.coverage)}</span><span class="coverage-kpi-label">overall</span></div>
      <div><span class="coverage-kpi-value">${(t?.covered_lines??0).toLocaleString()}/${(t?.lines_to_cover??0).toLocaleString()}</span><span class="coverage-kpi-label">covered lines</span></div>
      <div><span class="coverage-kpi-value">${(t?.modules_with_coverage??0).toLocaleString()}</span><span class="coverage-kpi-label">modules</span></div>
    </div>
    <div class="coverage-file-list">
      ${s.map(Ee).join("")}
    </div>`,e.querySelectorAll(".coverage-mini-row").forEach(a=>{a.addEventListener("click",()=>{let n=a.dataset.coveragePath;n&&(G("coverage"),te(n))})})}function Ee(e){return`<button class="coverage-mini-row" data-coverage-path="${i(e.path)}">
    <span class="coverage-mini-main">
      <span class="coverage-file-name" title="${i(e.path)}">${i(P(e.path))}</span>
      <span class="coverage-file-meta">${i(e.moduleName)} \xB7 ${e.uncoveredLines.length.toLocaleString()} uncovered lines</span>
    </span>
    <span class="coverage-pill ${F(e.coverage)}">${x(e.coverage??void 0)}</span>
  </button>`}function R(){let e=o("coverage-details");if(!p.length){e.innerHTML='<div class="empty-state">No file-level coverage was collected for this scan.</div>';return}e.innerHTML=`<div class="coverage-toolbar">
      <div>
        <h3>Coverage Files</h3>
        <p>${p.length.toLocaleString()} files with executable lines from collected test reports</p>
      </div>
      <span class="coverage-pill ${F(c.measures.coverage??null)}">${x(c.measures.coverage)}</span>
    </div>
    <div class="coverage-browser">
      <aside class="coverage-sidebar">
        <div class="coverage-file-list coverage-file-list-large">
          ${p.map(Pe).join("")}
        </div>
      </aside>
      <section class="coverage-code-viewer">
        ${Ae()}
      </section>
    </div>`,e.querySelectorAll(".coverage-row").forEach(t=>{t.addEventListener("click",()=>{let s=t.dataset.coveragePath;s&&te(s)})})}function Pe(e){return`<button class="coverage-row${e.path===C?" active":""}" data-coverage-path="${i(e.path)}">
    <div class="coverage-row-main">
      <div class="coverage-row-title" title="${i(e.path)}">${i(e.path)}</div>
      <div class="coverage-row-subtitle">${i(e.moduleName)} \xB7 ${i(e.moduleRoot)} \xB7 ${e.coveredLines.toLocaleString()}/${e.linesToCover.toLocaleString()} lines covered</div>
    </div>
    <div class="coverage-row-meter">
      <span class="coverage-pill ${F(e.coverage)}">${x(e.coverage??void 0)}</span>
      <div class="coverage-track"><div class="coverage-fill ${F(e.coverage)}" style="width:${e.coverage??0}%"></div></div>
    </div>
  </button>`}async function te(e){if(p.some(t=>t.path===e)){if(C=e,N="",z.has(e)){O=!1,R();return}O=!0,R();try{let t=await fetch(`/api/files/source?path=${encodeURIComponent(e)}`);if(!t.ok)throw new Error(`HTTP ${t.status}`);let s=await t.json();z.set(e,s.file)}catch(t){N=`Could not load source for ${e}: ${String(t)}`}finally{O=!1,R()}}}function Ae(){let e=p.find(u=>u.path===C);if(!e)return'<div class="code-empty"><p>Select a file to inspect coverage.</p></div>';if(O)return'<div class="code-empty"><div class="spinner"></div></div>';if(N)return`<div class="code-empty"><p>${i(N)}</p></div>`;let t=z.get(e.path);if(!t)return'<div class="code-empty"><p>Select a coverage file to inspect source lines.</p></div>';let s=new Set(e.coveredLineNumbers),a=new Set(e.uncoveredLines),r=t.content.split(`
`).map((u,A)=>{let w=A+1,v=a.has(w),d=!v&&s.has(w),M=He(d,v);return`<div class="code-line${M.stateClass}">
      <span class="code-gutter">${w}</span>
      <code class="code-text">${u.length?i(u):"&nbsp;"}</code>
      <span class="code-markers">${Fe(M)}</span>
    </div>`}).join(""),g=e.coveredLineNumbers.length?"covered and uncovered lines":"uncovered lines only";return`<div class="code-viewer-shell coverage-source-shell">
    <div class="code-viewer-head">
      <div>
        <div class="code-viewer-path mono">${i(t.path)}</div>
        <div class="code-viewer-meta">${i(t.language||"plain text")} \xB7 ${t.line_count.toLocaleString()} lines \xB7 ${g}</div>
      </div>
      <div class="code-viewer-stats"><span class="coverage-pill ${F(e.coverage)}">${x(e.coverage??void 0)}</span></div>
    </div>
    <div class="coverage-legend">
      <span><span class="legend-dot legend-covered"></span>Covered</span>
      <span><span class="legend-dot legend-uncovered"></span>Not covered</span>
    </div>
    <div class="code-surface">${r}</div>
  </div>`}function He(e,t){return t?{stateClass:" is-uncovered",marker:"not covered",chipClass:" chip-uncovered"}:e?{stateClass:" is-covered",marker:"covered",chipClass:" chip-covered"}:{stateClass:"",marker:"",chipClass:""}}function Fe(e){return e.marker?`<span class="coverage-line-chip${e.chipClass}">${e.marker}</span>`:""}function F(e){return e==null?"card-neutral":e>=80?"card-green":e>=60?"card-yellow":"card-red"}function je(){let e=Object.entries(c.measures.by_language).sort((s,a)=>a[1]-s[1]),t=Math.max(1,e[0]?.[1]??1);if(!e.length){o("by-lang").innerHTML='<span class="empty-state">No language data</span>';return}o("by-lang").innerHTML=e.map(([s,a])=>`<div class="lang-row">
      <span class="lang-name">${i(s)}</span>
      <div class="lang-bar-track"><div class="lang-bar-fill" style="width:${a/t*100}%"></div></div>
      <span class="lang-count">${a} files</span>
    </div>`).join("")}function Oe(){document.querySelectorAll(".tab").forEach(e=>{e.addEventListener("click",()=>{let t=e.dataset.tab;G(t)})})}function G(e){oe=e,document.querySelectorAll(".tab").forEach(t=>t.classList.remove("active")),document.querySelector(`.tab[data-tab="${e}"]`)?.classList.add("active"),document.querySelectorAll(".panel").forEach(t=>t.classList.add("hidden")),o(`panel-${e}`).classList.remove("hidden")}function Re(){let e=[...new Set(f.map(s=>s.rule_key))].sort((s,a)=>s.localeCompare(a)),t=o("filter-rule");e.forEach(s=>{let a=document.createElement("option");a.value=s,a.textContent=s,t.appendChild(a)}),o("filter-severity").addEventListener("change",s=>{_=s.target.value,T()}),o("filter-type").addEventListener("change",s=>{J=s.target.value,T()}),t.addEventListener("change",s=>{X=s.target.value,T()}),o("search").addEventListener("input",s=>{W=s.target.value.toLowerCase(),T()}),re()}function re(){let e=D(f,s=>s.severity),t=["blocker","critical","major","minor","info"];o("sev-chips").innerHTML=t.map(s=>{let a=e[s]??0,n=E[s]??"#64748b";return`<div class="sev-chip${_===s?" active":""}" data-sev="${s}"
      style="--chip-color:${n};--chip-bg:${n}15">
      <span class="chip-dot" style="background:${n}"></span>
      ${s}
      <span class="chip-count">${a}</span>
    </div>`}).join(""),o("sev-chips").querySelectorAll(".sev-chip").forEach(s=>{s.addEventListener("click",()=>{let a=s.dataset.sev;_=_===a?"all":a,o("filter-severity").value=_,T(),re()})})}function T(){L=f.filter(e=>!(_!=="all"&&e.severity!==_||J!=="all"&&e.type!==J||X!=="all"&&e.rule_key!==X||W&&!`${e.component_path} ${e.message} ${e.rule_key}`.toLowerCase().includes(W))),L.sort((e,t)=>{let s=q[e.severity]??99,a=q[t.severity]??99;return s-a}),S=-1,Ne()}function Ne(){let e=o("issue-list"),t=L.length===1?"issue":"issues";if(o("issue-count").textContent=`${L.length} ${t}`,!L.length){e.innerHTML='<div class="empty-state">No issues match the current filters.</div>';return}e.innerHTML=L.map((s,a)=>{let n=E[s.severity]??"#64748b",r=P(s.component_path),g=s.end_line&&s.end_line!==s.line?`L${s.line}\u2013${s.end_line}`:`L${s.line}`,u=ee[s.type]??s.type;return`<div class="issue-row" data-idx="${a}">
      <span class="issue-sev">
        <span class="issue-sev-dot" style="background:${n}"></span>
        ${i(s.severity)}
      </span>
      <span class="issue-type">${i(u)}</span>
      <div class="issue-main">
        <span class="issue-msg">${i(s.message)}</span>
        <span class="issue-file" title="${i(s.component_path)}">${i(r)}:${g}</span>
      </div>
      <span class="issue-rule">${i(s.rule_key)}</span>
    </div>`}).join(""),e.querySelectorAll(".issue-row").forEach(s=>{s.addEventListener("click",()=>{let a=Number.parseInt(s.dataset.idx,10);Y(a)})})}function qe(){let e=new Map;for(let t of f){let s=t.component_path;e.has(s)||e.set(s,[]),e.get(s).push(t)}b=[...e.entries()].sort((t,s)=>s[1].length-t[1].length).map(([t,s])=>({path:t,shortPath:P(t),issues:[...s].sort((a,n)=>a.line-n.line),expanded:!1}))}function ce(){let e=o("file-tree");if(!b.length){e.innerHTML='<div class="empty-state">No issues found</div>';return}e.innerHTML=b.map((t,s)=>`<div class="file-group${t.expanded?" expanded":""}" data-gi="${s}">
      <div class="file-group-header">
        <span class="file-group-chevron">\u25B6</span>
        <span class="file-group-name" title="${i(t.path)}">${i(t.shortPath)}</span>
        <span class="file-group-count">${t.issues.length}</span>
      </div>
      <div class="file-group-issues" style="${t.expanded?"":"display:none"}">
        ${t.issues.map((a,n)=>{let r=E[a.severity]??"#64748b";return`<div class="file-issue" data-gi="${s}" data-ii="${n}">
            <span class="issue-sev">
              <span class="issue-sev-dot" style="background:${r}"></span>
              ${i(a.severity)}
            </span>
            <span class="issue-msg">${i(a.message)}</span>
            <span class="file-issue-line">L${a.line}</span>
          </div>`}).join("")}
      </div>
    </div>`).join(""),e.querySelectorAll(".file-group-header").forEach(t=>{t.addEventListener("click",()=>{let s=t.closest(".file-group"),a=Number.parseInt(s.dataset.gi,10);b[a].expanded=!b[a].expanded,s.classList.toggle("expanded");let n=s.querySelector(".file-group-issues");n.style.display=b[a].expanded?"":"none"})}),e.querySelectorAll(".file-issue").forEach(t=>{t.addEventListener("click",s=>{s.stopPropagation();let a=Number.parseInt(t.dataset.gi,10),n=Number.parseInt(t.dataset.ii,10),r=b[a].issues[n];K(r)})})}function De(e){let t=b.findIndex(a=>a.path===e);if(t<0)return;b[t].expanded=!0,ce(),document.querySelector(`.file-group[data-gi="${t}"]`)?.scrollIntoView({behavior:"smooth",block:"start"})}function Y(e){S=e,h=L[e]??null,document.querySelectorAll(".issue-row").forEach(t=>t.classList.remove("selected")),document.querySelector(`.issue-row[data-idx="${e}"]`)?.classList.add("selected"),h&&K(h)}function K(e){h=e,k="details",H="",V=!0,l=se(),o("detail-title").textContent=e.rule_key,B(e),o("detail-panel").classList.add("open"),o("detail-overlay").classList.add("open"),Ve(e.rule_key)}async function Ve(e){try{let t=await fetch(`/rules/${encodeURIComponent(e)}`);if(!t.ok)throw new Error("not found");let s=await t.json(),a="";s.rationale&&(a+=`<div class="detail-section">
        <div class="detail-section-title">Why is this a problem?</div>
        <div class="rule-rationale">${i(s.rationale)}</div>
      </div>`),s.description&&s.description!==s.rationale&&(a+=`<div class="detail-section">
        <div class="detail-section-title">Description</div>
        <div class="rule-rationale">${i(s.description)}</div>
      </div>`),s.noncompliant_code&&(a+=`<div class="detail-section">
        <div class="detail-section-title">\u2718 Noncompliant Code</div>
        <pre class="rule-code noncompliant"><code>${i(s.noncompliant_code)}</code></pre>
      </div>`),s.compliant_code&&(a+=`<div class="detail-section">
        <div class="detail-section-title">\u2714 Compliant Code</div>
        <pre class="rule-code compliant"><code>${i(s.compliant_code)}</code></pre>
      </div>`),H=a||'<div class="detail-empty">No additional rule details available.</div>'}catch{H='<div class="detail-empty">Rule details are not available for this issue.</div>'}finally{V=!1,h?.rule_key===e&&B(h)}}function Q(){o("detail-panel").classList.remove("open"),o("detail-overlay").classList.remove("open"),h=null,H="",V=!1,l=se(),document.querySelectorAll(".issue-row").forEach(e=>e.classList.remove("selected"))}function B(e){let t=`
    <div class="detail-tabs">
      ${ae(k)}
    </div>
    <div class="detail-tab-panel${k==="details"?"":" hidden"}" data-detail-panel="details">
      ${Ge(e)}
    </div>
    <div class="detail-tab-panel${k==="rule"?"":" hidden"}" data-detail-panel="rule">
      ${V?'<div class="detail-loading">Loading rule details\u2026</div>':H}
    </div>
    <div class="detail-tab-panel${k==="ai-fix"?"":" hidden"}" data-detail-panel="ai-fix">
      ${ie(e,l,m??[])}
    </div>
  `;o("detail-body").innerHTML=t,Ke(e)}function Ge(e){let t=E[e.severity]??"#64748b",s=ee[e.type]??e.type,a=e.end_line&&e.end_line!==e.line?`${e.line}:${e.column} \u2013 ${e.end_line}:${e.end_column}`:`${e.line}:${e.column}`,n=`
    <div class="detail-section">
      <div class="detail-msg">${i(e.message)}</div>
    </div>
    <div class="detail-section">
      <div class="detail-section-title">Properties</div>
      <div class="detail-field">
        <span class="detail-field-label">Severity</span>
        <span class="detail-field-value"><span class="issue-sev-dot" style="background:${t};display:inline-block;width:8px;height:8px;border-radius:50%;margin-right:6px"></span>${i(e.severity)}</span>
      </div>
      <div class="detail-field">
        <span class="detail-field-label">Type</span>
        <span class="detail-field-value">${i(s)}</span>
      </div>
      <div class="detail-field">
        <span class="detail-field-label">Rule</span>
        <span class="detail-field-value" style="font-family:var(--font-mono);color:var(--accent)">${i(e.rule_key)}</span>
      </div>
      <div class="detail-field">
        <span class="detail-field-label">Status</span>
        <span class="detail-field-value">${i(e.status)}</span>
      </div>
      ${e.engine_id?`<div class="detail-field">
        <span class="detail-field-label">Engine</span>
        <span class="detail-field-value">${i(e.engine_id)}</span>
      </div>`:""}
      ${e.tags?.length?`<div class="detail-field">
        <span class="detail-field-label">Tags</span>
        <span class="detail-field-value">${e.tags.map(r=>i(r)).join(", ")}</span>
      </div>`:""}
    </div>
    <div class="detail-section">
      <div class="detail-section-title">Location</div>
      <div class="detail-field">
        <span class="detail-field-label">File</span>
        <span class="detail-field-value" style="font-family:var(--font-mono);font-size:12px;word-break:break-all">${i(e.component_path)}</span>
      </div>
      <div class="detail-field">
        <span class="detail-field-label">Lines</span>
        <span class="detail-field-value" style="font-family:var(--font-mono)">${a}</span>
      </div>
    </div>`;return e.secondary_locations?.length&&(n+=`<div class="detail-section">
      <div class="detail-section-title">Related Locations (${e.secondary_locations.length})</div>
      <div class="detail-loc-list">
        ${e.secondary_locations.map(r=>`
          <div class="detail-loc-item">
            <div class="detail-loc-file">${i(r.file_path||e.component_path)}:${r.start_line}</div>
            ${r.message?`<div class="detail-loc-msg">${i(r.message)}</div>`:""}
          </div>
        `).join("")}
      </div>
    </div>`),n}function Ke(e){document.querySelectorAll(".detail-tab").forEach(n=>{n.addEventListener("click",()=>{k=n.dataset.detailTab??"details",B(e),k==="ai-fix"&&Be()})});let t=document.getElementById("ai-provider-select");t?.addEventListener("change",()=>{l.selectedProviderId=t.value,l.selectedModel="",Z(),l.preview=null,l.statusMessage="",l.errorMessage="",y()});let s=document.getElementById("ai-model-input");s?.addEventListener("input",()=>{l.selectedModel=s.value});let a=document.getElementById("ai-api-key-input");a?.addEventListener("input",()=>{l.apiKey=a.value}),document.getElementById("ai-generate-fix")?.addEventListener("click",()=>{Ue(e)}),document.getElementById("ai-apply-fix")?.addEventListener("click",()=>{ze()})}function se(){return{loadingOptions:!1,loadingPreview:!1,applying:!1,selectedProviderId:"",selectedModel:"",apiKey:"",statusMessage:"",errorMessage:"",preview:null}}function de(){return!m||m.length===0?null:m.find(e=>e.id===l.selectedProviderId)??m[0]}function Z(){if(!m||m.length===0){l.selectedProviderId="",l.selectedModel="";return}m.some(t=>t.id===l.selectedProviderId)||(l.selectedProviderId=m[0].id);let e=de();if(!e){l.selectedModel="";return}l.selectedModel||(l.selectedModel=e.default_model||e.models[0]||"")}async function Be(){if(m){Z(),y();return}l.loadingOptions=!0,l.errorMessage="",y();try{let e=await fetch("/api/ai/providers");if(!e.ok)throw new Error(`HTTP ${e.status}`);m=(await e.json()).providers??[],Z()}catch(e){l.errorMessage=`Failed to load AI models: ${String(e)}`,m=[]}finally{l.loadingOptions=!1,y()}}async function Ue(e){let t=de(),s=l.selectedModel.trim();if(!t||!l.selectedProviderId){l.errorMessage="Choose an AI provider before generating a fix.",y();return}if(!s){l.errorMessage="Choose a model before generating a fix.",y();return}if(t.requires_api_key&&!t.configured&&!l.apiKey.trim()){l.errorMessage="Provide an API key for the selected provider before generating a fix.",y();return}l.selectedModel=s,l.loadingPreview=!0,l.statusMessage="",l.errorMessage="",y();try{let a={provider:l.selectedProviderId,model:s,api_key:l.apiKey.trim()||void 0,issue:e},n=await fetch("/api/ai/fixes/preview",{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify(a)}),r=await n.json();if(!n.ok||"error"in r)throw new Error("error"in r?r.error:`HTTP ${n.status}`);l.preview=r,l.statusMessage="Fix preview generated. Review the diff before applying it."}catch(a){l.errorMessage=`Failed to generate AI fix: ${String(a)}`,l.preview=null}finally{l.loadingPreview=!1,y()}}async function ze(){if(l.preview){l.applying=!0,l.errorMessage="",y();try{let e=await fetch("/api/ai/fixes/apply",{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify({preview_id:l.preview.preview_id})}),t=await e.json();if(!e.ok||"error"in t)throw new Error("error"in t?t.error:`HTTP ${e.status}`);l.statusMessage=t.message}catch(e){l.errorMessage=`Failed to apply AI fix: ${String(e)}`}finally{l.applying=!1,y()}}}function y(){h&&B(h)}document.addEventListener("DOMContentLoaded",()=>{o("detail-close").addEventListener("click",Q),o("detail-overlay").addEventListener("click",Q)});function Je(){document.addEventListener("keydown",e=>{let t=e.target.tagName;if(!(t==="INPUT"||t==="SELECT"||t==="TEXTAREA")){if(e.key==="Escape"){Q();return}oe==="issues"&&(e.key==="j"||e.key==="ArrowDown"?(e.preventDefault(),S<L.length-1&&Y(S+1),ne()):e.key==="k"||e.key==="ArrowUp"?(e.preventDefault(),S>0&&Y(S-1),ne()):e.key==="Enter"&&h&&K(h))}})}function ne(){document.querySelector(`.issue-row[data-idx="${S}"]`)?.scrollIntoView({behavior:"smooth",block:"nearest"})}function o(e){return document.getElementById(e)}function $(e,t){o(e).classList.add(t)}function D(e,t){let s={};for(let a of e){let n=t(a);s[n]=(s[n]??0)+1}return s}function P(e){let t=e.replaceAll("\\","/"),s=t.split("/").filter(Boolean);return s.length<=2?t:`${s.slice(-2).join("/")}`}})();
