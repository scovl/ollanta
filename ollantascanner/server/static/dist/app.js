"use strict";(()=>{function a(e){return e.replaceAll("&","&amp;").replaceAll("<","&lt;").replaceAll(">","&gt;").replaceAll('"',"&quot;")}function z(e){return[{key:"details",label:"Details"},{key:"rule",label:"Rule"},{key:"ai-fix",label:"Fix with AI"}].map(t=>`<button class="detail-tab${e===t.key?" active":""}" data-detail-tab="${t.key}">${t.label}</button>`).join("")}function J(e,i,t){let s=e.end_line&&e.end_line!==e.line?`-${e.end_line}`:"",o=ie(i,t),r=se(i);return`
    <div class="detail-section">
      <div class="detail-section-title">Fix with AI</div>
      <div class="detail-msg ai-fix-callout">Ollanta prepares the issue context, sends only the relevant snippet to the selected agent, and shows a preview before writing any changes to your code.</div>
    </div>

    <div class="detail-section">
      <div class="detail-field detail-field-stack">
        <span class="detail-field-label">Target</span>
        <span class="detail-field-value detail-mono-block">${a(e.component_path)}:${e.line}${s}</span>
      </div>
      <div class="detail-field detail-field-stack">
        <span class="detail-field-label">Issue</span>
        <span class="detail-field-value">${a(e.message)}</span>
      </div>
    </div>

    <div class="detail-section">
      <div class="detail-section-title">Model</div>
      ${o}
      ${i.statusMessage?`<div class="ai-fix-status ai-fix-status-ok">${a(i.statusMessage)}</div>`:""}
      ${i.errorMessage?`<div class="ai-fix-status ai-fix-status-error">${a(i.errorMessage)}</div>`:""}
    </div>

    <div class="detail-section">
      <div class="detail-section-title">Preview</div>
      ${r}
    </div>
  `}function ie(e,i){if(e.loadingOptions)return'<div class="detail-loading">Loading AI models\u2026</div>';if(i.length===0)return'<div class="detail-empty">No AI provider is available for the local scanner.</div>';let t=i.find(d=>d.id===e.selectedProviderId)??i[0],s=i.map(d=>`<option value="${a(d.id)}"${e.selectedProviderId===d.id?" selected":""}>${a(d.label)}</option>`).join(""),r=(t?.models??[]).map(d=>`<option value="${a(d)}"></option>`).join(""),m='<div class="ai-fix-helper">This provider can generate local previews without an API key.</div>',x="Required for this provider";t?.requires_api_key&&(t.configured?(m=`<div class="ai-fix-helper">Using the scanner's configured API key. Paste another key below to override it for this session.</div>`,x="Optional override"):m='<div class="ai-fix-helper">Paste an API key for the selected provider to generate the fix.</div>');let O=t?.requires_api_key?`<div class="ai-fix-control-group">
          <label class="ai-fix-control-label" for="ai-api-key-input">API key</label>
          <input id="ai-api-key-input" class="ai-fix-select ai-fix-input" type="password" value="${a(e.apiKey)}" placeholder="${x}" autocomplete="off">
        </div>`:"",P=e.loadingPreview?"Generating\u2026":"Generate fix",c=e.loadingPreview?" disabled":"";return`<div class="ai-fix-controls">
      <div class="ai-fix-control-group">
        <label class="ai-fix-control-label" for="ai-provider-select">Provider</label>
        <select id="ai-provider-select" class="ai-fix-select">${s}</select>
      </div>
      <div class="ai-fix-control-group">
        <label class="ai-fix-control-label" for="ai-model-input">Model</label>
        <input id="ai-model-input" class="ai-fix-select ai-fix-input" list="ai-model-options" value="${a(e.selectedModel)}" placeholder="${a(t?.default_model||"gpt-4.1-mini")}" autocomplete="off">
        <datalist id="ai-model-options">${r}</datalist>
      </div>
      ${O}
      ${m}
      <button id="ai-generate-fix" class="ai-fix-button"${c}>${P}</button>
    </div>`}function se(e){if(!e.preview)return'<div class="detail-empty">Generate a fix preview to inspect the patch before Ollanta edits your local file.</div>';let i=e.preview.summary||"Generated fix preview",t=e.preview.explanation?`<div class="rule-rationale">${a(e.preview.explanation)}</div>`:"",s=e.applying?"Applying\u2026":"Apply to file",o=e.applying?" disabled":"";return`
    <div class="ai-fix-preview-meta">
      <div><strong>Provider:</strong> ${a(e.preview.agent.label)}</div>
      <div><strong>Model:</strong> ${a(e.preview.agent.model)}</div>
      <div><strong>Summary:</strong> ${a(i)}</div>
    </div>
    ${t}
    <pre class="rule-code ai-fix-diff"><code>${a(e.preview.diff)}</code></pre>
    <div class="ai-fix-actions">
      <button id="ai-apply-fix" class="ai-fix-button ai-fix-button-primary"${o}>${s}</button>
    </div>
  `}var M,v=[],y=[],g=[],f=null,b=-1,X="overview",h="details",k="",H=!1,p=null,n=U(),$="all",q="all",D="all",N="",S={blocker:0,critical:1,major:2,minor:3,info:4},E={blocker:"#ef4444",critical:"#f97316",major:"#eab308",minor:"#22c55e",info:"#64748b"},V={bug:"Bug",code_smell:"Code Smell",vulnerability:"Vulnerability",security_hotspot:"Hotspot"};async function ae(){try{let e=await fetch("/report.json");if(!e.ok)throw new Error(`HTTP ${e.status}`);M=await e.json(),v=M.issues??[],ne(),oe(),re(),de(),ce(),pe(),ve(),fe(),L(),me(),Z(),ue(),Ie(),l("tab-issue-count").textContent=String(v.length),l("tab-file-count").textContent=String(new Set(v.map(i=>i.component_path)).size)}catch(e){l("app").innerHTML=`<div class="error">Failed to load report: ${String(e)}</div>`}}document.addEventListener("DOMContentLoaded",ae);function ne(){let e=M.metadata,i=new Date(e.analysis_date).toLocaleString();l("project-key").textContent=e.project_key,l("scan-date").textContent=i,l("scan-version").textContent=`v${e.version}`,l("elapsed").textContent=`${e.elapsed_ms}ms`}function le(){let e=M.measures,i=[{metric:"Bugs",operator:"=",threshold:0,value:e.bugs,passed:e.bugs===0},{metric:"Vulnerabilities",operator:"=",threshold:0,value:e.vulnerabilities,passed:e.vulnerabilities===0}];return{status:i.every(s=>s.passed)?"passed":"failed",conditions:i}}function oe(){let e=le(),i=l("gate-hero");i.classList.remove("gate-loading"),i.classList.add(e.status==="passed"?"gate-passed":"gate-failed"),l("gate-icon").textContent=e.status==="passed"?"\u2713":"\u2717",l("gate-status").textContent=e.status==="passed"?"Passed":"Failed";let t=e.conditions.map(s=>{let o=s.passed?"cond-pass":"cond-fail",r=s.passed?"\u2713":"\u2717";return`<div class="gate-cond ${o}">
      <span class="gate-cond-icon">${r}</span>
      <span class="gate-cond-metric">${a(s.metric)}</span>
      <span class="gate-cond-value">${s.value}</span>
    </div>`}).join("");l("gate-conditions").innerHTML=t}function re(){let e=M.measures;w("m-bugs",e.bugs),w("m-vulns",e.vulnerabilities),w("m-smells",e.code_smells),w("m-ncloc",e.ncloc),w("m-files",e.files),w("m-comments",e.comments),R("card-bugs",e.bugs,[0,1,5]),R("card-vulns",e.vulnerabilities,[0,1,3]),R("card-smells",e.code_smells,[0,10,50]),I("card-ncloc","card-neutral"),I("card-files","card-neutral"),I("card-comments","card-neutral")}function w(e,i){l(e).textContent=i.toLocaleString()}function R(e,i,t){i<=t[0]?I(e,"card-green"):i<=t[1]?I(e,"card-yellow"):I(e,"card-red")}function de(){let e=_(v,c=>c.severity),i=Math.max(1,...Object.values(e)),t=["blocker","critical","major","minor","info"],s="",o="",r=v.length||1;for(let c of t){let d=e[c]??0,T=d/i*100,A=E[c]??"#64748b";s+=`<div class="sev-bar-row">
      <span class="sev-bar-label">${c}</span>
      <div class="sev-bar-track"><div class="sev-bar-fill" style="width:${T}%;background:${A}"></div></div>
      <span class="sev-bar-count">${d}</span>
    </div>`,d>0&&(o+=`<div class="sev-segment" style="width:${d/r*100}%;background:${A}" title="${c}: ${d}"></div>`)}l("sev-bars").innerHTML=s,l("sev-proportional").innerHTML=o;let m=_(v,c=>c.type),x=Math.max(1,...Object.values(m)),O={bug:"#ef4444",vulnerability:"#f97316",code_smell:"#22c55e",security_hotspot:"#eab308"},P="";for(let[c,d]of Object.entries(V)){let T=m[c]??0,A=T/x*100,te=O[c]??"#64748b";P+=`<div class="sev-bar-row">
      <span class="sev-bar-label">${d}</span>
      <div class="sev-bar-track"><div class="sev-bar-fill" style="width:${A}%;background:${te}"></div></div>
      <span class="sev-bar-count">${T}</span>
    </div>`}l("type-bars").innerHTML=P}function ce(){let e=[...v].sort((i,t)=>{let s=(S[i.severity]??99)-(S[t.severity]??99);return s!==0?s:i.component_path.localeCompare(t.component_path)||i.line-t.line}).slice(0,6);if(!e.length){l("priority-issues").innerHTML='<div class="empty-state compact">No issues found</div>';return}l("priority-issues").innerHTML=e.map((i,t)=>{let s=E[i.severity]??"#64748b",o=j(i.component_path);return`<button class="priority-row" data-idx="${t}">
      <span class="issue-sev-dot" style="background:${s}"></span>
      <span class="priority-main">
        <span class="priority-title">${a(i.message)}</span>
        <span class="priority-meta" title="${a(i.component_path)}">${a(o)}:L${i.line} \xB7 ${a(i.rule_key)}</span>
      </span>
      <span class="priority-severity">${a(i.severity)}</span>
    </button>`}).join(""),l("priority-issues").querySelectorAll(".priority-row").forEach(i=>{i.addEventListener("click",()=>{let t=Number.parseInt(i.dataset.idx,10);C(e[t])})})}function pe(){let e=_(v,t=>t.component_path),i=Object.entries(e).sort((t,s)=>s[1]-t[1]).slice(0,10);if(!i.length){l("hotspot-files").innerHTML='<div class="empty-state">No issues found</div>';return}l("hotspot-files").innerHTML=i.map(([t,s])=>{let o=j(t);return`<div class="hotspot-row" data-path="${a(t)}">
      <span class="hotspot-file" title="${a(t)}">${a(o)}</span>
      <span class="hotspot-count">${s}</span>
    </div>`}).join(""),l("hotspot-files").querySelectorAll(".hotspot-row").forEach(t=>{t.addEventListener("click",()=>{let s=t.dataset.path;Y("files"),ye(s)})})}function ve(){let e=Object.entries(M.measures.by_language).sort((t,s)=>s[1]-t[1]),i=Math.max(1,e[0]?.[1]??1);if(!e.length){l("by-lang").innerHTML='<span class="empty-state">No language data</span>';return}l("by-lang").innerHTML=e.map(([t,s])=>`<div class="lang-row">
      <span class="lang-name">${a(t)}</span>
      <div class="lang-bar-track"><div class="lang-bar-fill" style="width:${s/i*100}%"></div></div>
      <span class="lang-count">${s} files</span>
    </div>`).join("")}function ue(){document.querySelectorAll(".tab").forEach(e=>{e.addEventListener("click",()=>{let i=e.dataset.tab;Y(i)})})}function Y(e){X=e,document.querySelectorAll(".tab").forEach(i=>i.classList.remove("active")),document.querySelector(`.tab[data-tab="${e}"]`)?.classList.add("active"),document.querySelectorAll(".panel").forEach(i=>i.classList.add("hidden")),l(`panel-${e}`).classList.remove("hidden")}function fe(){let e=[...new Set(v.map(t=>t.rule_key))].sort((t,s)=>t.localeCompare(s)),i=l("filter-rule");e.forEach(t=>{let s=document.createElement("option");s.value=t,s.textContent=t,i.appendChild(s)}),l("filter-severity").addEventListener("change",t=>{$=t.target.value,L()}),l("filter-type").addEventListener("change",t=>{q=t.target.value,L()}),i.addEventListener("change",t=>{D=t.target.value,L()}),l("search").addEventListener("input",t=>{N=t.target.value.toLowerCase(),L()}),Q()}function Q(){let e=_(v,t=>t.severity),i=["blocker","critical","major","minor","info"];l("sev-chips").innerHTML=i.map(t=>{let s=e[t]??0,o=E[t]??"#64748b";return`<div class="sev-chip${$===t?" active":""}" data-sev="${t}"
      style="--chip-color:${o};--chip-bg:${o}15">
      <span class="chip-dot" style="background:${o}"></span>
      ${t}
      <span class="chip-count">${s}</span>
    </div>`}).join(""),l("sev-chips").querySelectorAll(".sev-chip").forEach(t=>{t.addEventListener("click",()=>{let s=t.dataset.sev;$=$===s?"all":s,l("filter-severity").value=$,L(),Q()})})}function L(){y=v.filter(e=>!($!=="all"&&e.severity!==$||q!=="all"&&e.type!==q||D!=="all"&&e.rule_key!==D||N&&!`${e.component_path} ${e.message} ${e.rule_key}`.toLowerCase().includes(N))),y.sort((e,i)=>{let t=S[e.severity]??99,s=S[i.severity]??99;return t-s}),b=-1,ge()}function ge(){let e=l("issue-list"),i=y.length===1?"issue":"issues";if(l("issue-count").textContent=`${y.length} ${i}`,!y.length){e.innerHTML='<div class="empty-state">No issues match the current filters.</div>';return}e.innerHTML=y.map((t,s)=>{let o=E[t.severity]??"#64748b",r=j(t.component_path),m=t.end_line&&t.end_line!==t.line?`L${t.line}\u2013${t.end_line}`:`L${t.line}`,x=V[t.type]??t.type;return`<div class="issue-row" data-idx="${s}">
      <span class="issue-sev">
        <span class="issue-sev-dot" style="background:${o}"></span>
        ${a(t.severity)}
      </span>
      <span class="issue-type">${a(x)}</span>
      <div class="issue-main">
        <span class="issue-msg">${a(t.message)}</span>
        <span class="issue-file" title="${a(t.component_path)}">${a(r)}:${m}</span>
      </div>
      <span class="issue-rule">${a(t.rule_key)}</span>
    </div>`}).join(""),e.querySelectorAll(".issue-row").forEach(t=>{t.addEventListener("click",()=>{let s=Number.parseInt(t.dataset.idx,10);G(s)})})}function me(){let e=new Map;for(let i of v){let t=i.component_path;e.has(t)||e.set(t,[]),e.get(t).push(i)}g=[...e.entries()].sort((i,t)=>t[1].length-i[1].length).map(([i,t])=>({path:i,shortPath:j(i),issues:[...t].sort((s,o)=>s.line-o.line),expanded:!1}))}function Z(){let e=l("file-tree");if(!g.length){e.innerHTML='<div class="empty-state">No issues found</div>';return}e.innerHTML=g.map((i,t)=>`<div class="file-group${i.expanded?" expanded":""}" data-gi="${t}">
      <div class="file-group-header">
        <span class="file-group-chevron">\u25B6</span>
        <span class="file-group-name" title="${a(i.path)}">${a(i.shortPath)}</span>
        <span class="file-group-count">${i.issues.length}</span>
      </div>
      <div class="file-group-issues" style="${i.expanded?"":"display:none"}">
        ${i.issues.map((s,o)=>{let r=E[s.severity]??"#64748b";return`<div class="file-issue" data-gi="${t}" data-ii="${o}">
            <span class="issue-sev">
              <span class="issue-sev-dot" style="background:${r}"></span>
              ${a(s.severity)}
            </span>
            <span class="issue-msg">${a(s.message)}</span>
            <span class="file-issue-line">L${s.line}</span>
          </div>`}).join("")}
      </div>
    </div>`).join(""),e.querySelectorAll(".file-group-header").forEach(i=>{i.addEventListener("click",()=>{let t=i.closest(".file-group"),s=Number.parseInt(t.dataset.gi,10);g[s].expanded=!g[s].expanded,t.classList.toggle("expanded");let o=t.querySelector(".file-group-issues");o.style.display=g[s].expanded?"":"none"})}),e.querySelectorAll(".file-issue").forEach(i=>{i.addEventListener("click",t=>{t.stopPropagation();let s=Number.parseInt(i.dataset.gi,10),o=Number.parseInt(i.dataset.ii,10),r=g[s].issues[o];C(r)})})}function ye(e){let i=g.findIndex(s=>s.path===e);if(i<0)return;g[i].expanded=!0,Z(),document.querySelector(`.file-group[data-gi="${i}"]`)?.scrollIntoView({behavior:"smooth",block:"start"})}function G(e){b=e,f=y[e]??null,document.querySelectorAll(".issue-row").forEach(i=>i.classList.remove("selected")),document.querySelector(`.issue-row[data-idx="${e}"]`)?.classList.add("selected"),f&&C(f)}function C(e){f=e,h="details",k="",H=!0,n=U(),l("detail-title").textContent=e.rule_key,F(e),l("detail-panel").classList.add("open"),l("detail-overlay").classList.add("open"),be(e.rule_key)}async function be(e){try{let i=await fetch(`/rules/${encodeURIComponent(e)}`);if(!i.ok)throw new Error("not found");let t=await i.json(),s="";t.rationale&&(s+=`<div class="detail-section">
        <div class="detail-section-title">Why is this a problem?</div>
        <div class="rule-rationale">${a(t.rationale)}</div>
      </div>`),t.description&&t.description!==t.rationale&&(s+=`<div class="detail-section">
        <div class="detail-section-title">Description</div>
        <div class="rule-rationale">${a(t.description)}</div>
      </div>`),t.noncompliant_code&&(s+=`<div class="detail-section">
        <div class="detail-section-title">\u2718 Noncompliant Code</div>
        <pre class="rule-code noncompliant"><code>${a(t.noncompliant_code)}</code></pre>
      </div>`),t.compliant_code&&(s+=`<div class="detail-section">
        <div class="detail-section-title">\u2714 Compliant Code</div>
        <pre class="rule-code compliant"><code>${a(t.compliant_code)}</code></pre>
      </div>`),k=s||'<div class="detail-empty">No additional rule details available.</div>'}catch{k='<div class="detail-empty">Rule details are not available for this issue.</div>'}finally{H=!1,f?.rule_key===e&&F(f)}}function K(){l("detail-panel").classList.remove("open"),l("detail-overlay").classList.remove("open"),f=null,k="",H=!1,n=U(),document.querySelectorAll(".issue-row").forEach(e=>e.classList.remove("selected"))}function F(e){let i=`
    <div class="detail-tabs">
      ${z(h)}
    </div>
    <div class="detail-tab-panel${h==="details"?"":" hidden"}" data-detail-panel="details">
      ${he(e)}
    </div>
    <div class="detail-tab-panel${h==="rule"?"":" hidden"}" data-detail-panel="rule">
      ${H?'<div class="detail-loading">Loading rule details\u2026</div>':k}
    </div>
    <div class="detail-tab-panel${h==="ai-fix"?"":" hidden"}" data-detail-panel="ai-fix">
      ${J(e,n,p??[])}
    </div>
  `;l("detail-body").innerHTML=i,$e(e)}function he(e){let i=E[e.severity]??"#64748b",t=V[e.type]??e.type,s=e.end_line&&e.end_line!==e.line?`${e.line}:${e.column} \u2013 ${e.end_line}:${e.end_column}`:`${e.line}:${e.column}`,o=`
    <div class="detail-section">
      <div class="detail-msg">${a(e.message)}</div>
    </div>
    <div class="detail-section">
      <div class="detail-section-title">Properties</div>
      <div class="detail-field">
        <span class="detail-field-label">Severity</span>
        <span class="detail-field-value"><span class="issue-sev-dot" style="background:${i};display:inline-block;width:8px;height:8px;border-radius:50%;margin-right:6px"></span>${a(e.severity)}</span>
      </div>
      <div class="detail-field">
        <span class="detail-field-label">Type</span>
        <span class="detail-field-value">${a(t)}</span>
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
        <span class="detail-field-value" style="font-family:var(--font-mono)">${s}</span>
      </div>
    </div>`;return e.secondary_locations?.length&&(o+=`<div class="detail-section">
      <div class="detail-section-title">Related Locations (${e.secondary_locations.length})</div>
      <div class="detail-loc-list">
        ${e.secondary_locations.map(r=>`
          <div class="detail-loc-item">
            <div class="detail-loc-file">${a(r.file_path||e.component_path)}:${r.start_line}</div>
            ${r.message?`<div class="detail-loc-msg">${a(r.message)}</div>`:""}
          </div>
        `).join("")}
      </div>
    </div>`),o}function $e(e){document.querySelectorAll(".detail-tab").forEach(o=>{o.addEventListener("click",()=>{h=o.dataset.detailTab??"details",F(e),h==="ai-fix"&&xe()})});let i=document.getElementById("ai-provider-select");i?.addEventListener("change",()=>{n.selectedProviderId=i.value,n.selectedModel="",B(),n.preview=null,n.statusMessage="",n.errorMessage="",u()});let t=document.getElementById("ai-model-input");t?.addEventListener("input",()=>{n.selectedModel=t.value});let s=document.getElementById("ai-api-key-input");s?.addEventListener("input",()=>{n.apiKey=s.value}),document.getElementById("ai-generate-fix")?.addEventListener("click",()=>{we(e)}),document.getElementById("ai-apply-fix")?.addEventListener("click",()=>{Le()})}function U(){return{loadingOptions:!1,loadingPreview:!1,applying:!1,selectedProviderId:"",selectedModel:"",apiKey:"",statusMessage:"",errorMessage:"",preview:null}}function ee(){return!p||p.length===0?null:p.find(e=>e.id===n.selectedProviderId)??p[0]}function B(){if(!p||p.length===0){n.selectedProviderId="",n.selectedModel="";return}p.some(i=>i.id===n.selectedProviderId)||(n.selectedProviderId=p[0].id);let e=ee();if(!e){n.selectedModel="";return}n.selectedModel||(n.selectedModel=e.default_model||e.models[0]||"")}async function xe(){if(p){B(),u();return}n.loadingOptions=!0,n.errorMessage="",u();try{let e=await fetch("/api/ai/providers");if(!e.ok)throw new Error(`HTTP ${e.status}`);p=(await e.json()).providers??[],B()}catch(e){n.errorMessage=`Failed to load AI models: ${String(e)}`,p=[]}finally{n.loadingOptions=!1,u()}}async function we(e){let i=ee(),t=n.selectedModel.trim();if(!i||!n.selectedProviderId){n.errorMessage="Choose an AI provider before generating a fix.",u();return}if(!t){n.errorMessage="Choose a model before generating a fix.",u();return}if(i.requires_api_key&&!i.configured&&!n.apiKey.trim()){n.errorMessage="Provide an API key for the selected provider before generating a fix.",u();return}n.selectedModel=t,n.loadingPreview=!0,n.statusMessage="",n.errorMessage="",u();try{let s={provider:n.selectedProviderId,model:t,api_key:n.apiKey.trim()||void 0,issue:e},o=await fetch("/api/ai/fixes/preview",{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify(s)}),r=await o.json();if(!o.ok||"error"in r)throw new Error("error"in r?r.error:`HTTP ${o.status}`);n.preview=r,n.statusMessage="Fix preview generated. Review the diff before applying it."}catch(s){n.errorMessage=`Failed to generate AI fix: ${String(s)}`,n.preview=null}finally{n.loadingPreview=!1,u()}}async function Le(){if(n.preview){n.applying=!0,n.errorMessage="",u();try{let e=await fetch("/api/ai/fixes/apply",{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify({preview_id:n.preview.preview_id})}),i=await e.json();if(!e.ok||"error"in i)throw new Error("error"in i?i.error:`HTTP ${e.status}`);n.statusMessage=i.message}catch(e){n.errorMessage=`Failed to apply AI fix: ${String(e)}`}finally{n.applying=!1,u()}}}function u(){f&&F(f)}document.addEventListener("DOMContentLoaded",()=>{l("detail-close").addEventListener("click",K),l("detail-overlay").addEventListener("click",K)});function Ie(){document.addEventListener("keydown",e=>{let i=e.target.tagName;if(!(i==="INPUT"||i==="SELECT"||i==="TEXTAREA")){if(e.key==="Escape"){K();return}X==="issues"&&(e.key==="j"||e.key==="ArrowDown"?(e.preventDefault(),b<y.length-1&&G(b+1),W()):e.key==="k"||e.key==="ArrowUp"?(e.preventDefault(),b>0&&G(b-1),W()):e.key==="Enter"&&f&&C(f))}})}function W(){document.querySelector(`.issue-row[data-idx="${b}"]`)?.scrollIntoView({behavior:"smooth",block:"nearest"})}function l(e){return document.getElementById(e)}function I(e,i){l(e).classList.add(i)}function _(e,i){let t={};for(let s of e){let o=i(s);t[o]=(t[o]??0)+1}return t}function j(e){let i=e.replaceAll("\\","/"),t=i.split("/").filter(Boolean);return t.length<=2?i:`${t.slice(-2).join("/")}`}})();
