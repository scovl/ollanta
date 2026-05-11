"use strict";(()=>{function l(e){return e.replaceAll("&","&amp;").replaceAll("<","&lt;").replaceAll(">","&gt;").replaceAll('"',"&quot;")}function _e(e){return[{key:"details",label:"Details"},{key:"rule",label:"Rule"},{key:"ai-fix",label:"Fix with AI"}].map(s=>`<button class="detail-tab${e===s.key?" active":""}" data-detail-tab="${s.key}">${s.label}</button>`).join("")}function Me(e,t,s){let n=e.end_line&&e.end_line!==e.line?`-${e.end_line}`:"",a=Ne(t,s),i=Re(t);return`
    <div class="detail-section">
      <div class="detail-section-title">Fix with AI</div>
      <div class="detail-msg ai-fix-callout">Ollanta prepares the issue context, sends only the relevant snippet to the selected agent, and shows a preview before writing any changes to your code.</div>
    </div>

    <div class="detail-section">
      <div class="detail-field detail-field-stack">
        <span class="detail-field-label">Target</span>
        <span class="detail-field-value detail-mono-block">${l(e.component_path)}:${e.line}${n}</span>
      </div>
      <div class="detail-field detail-field-stack">
        <span class="detail-field-label">Issue</span>
        <span class="detail-field-value">${l(e.message)}</span>
      </div>
    </div>

    <div class="detail-section">
      <div class="detail-section-title">Model</div>
      ${a}
      ${t.statusMessage?`<div class="ai-fix-status ai-fix-status-ok">${l(t.statusMessage)}</div>`:""}
      ${t.errorMessage?`<div class="ai-fix-status ai-fix-status-error">${l(t.errorMessage)}</div>`:""}
    </div>

    <div class="detail-section">
      <div class="detail-section-title">Preview</div>
      ${i}
    </div>
  `}function Ne(e,t){if(e.loadingOptions)return'<div class="detail-loading">Loading AI models\u2026</div>';if(t.length===0)return'<div class="detail-empty">No AI provider is available for the local scanner.</div>';let s=t.find(m=>m.id===e.selectedProviderId)??t[0],n=t.map(m=>`<option value="${l(m.id)}"${e.selectedProviderId===m.id?" selected":""}>${l(m.label)}</option>`).join(""),i=(s?.models??[]).map(m=>`<option value="${l(m)}"></option>`).join(""),c='<div class="ai-fix-helper">This provider can generate local previews without an API key.</div>',p="Required for this provider";s?.requires_api_key&&(s.configured?(c=`<div class="ai-fix-helper">Using the scanner's configured API key. Paste another key below to override it for this session.</div>`,p="Optional override"):c='<div class="ai-fix-helper">Paste an API key for the selected provider to generate the fix.</div>');let d=s?.requires_api_key?`<div class="ai-fix-control-group">
          <label class="ai-fix-control-label" for="ai-api-key-input">API key</label>
          <input id="ai-api-key-input" class="ai-fix-select ai-fix-input" type="password" value="${l(e.apiKey)}" placeholder="${p}" autocomplete="off">
        </div>`:"",g=e.loadingPreview?"Generating\u2026":"Generate fix",u=e.loadingPreview?" disabled":"";return`<div class="ai-fix-controls">
      <div class="ai-fix-control-group">
        <label class="ai-fix-control-label" for="ai-provider-select">Provider</label>
        <select id="ai-provider-select" class="ai-fix-select">${n}</select>
      </div>
      <div class="ai-fix-control-group">
        <label class="ai-fix-control-label" for="ai-model-input">Model</label>
        <input id="ai-model-input" class="ai-fix-select ai-fix-input" list="ai-model-options" value="${l(e.selectedModel)}" placeholder="${l(s?.default_model||"gpt-5.5")}" autocomplete="off">
        <datalist id="ai-model-options">${i}</datalist>
      </div>
      ${d}
      ${c}
      <button id="ai-generate-fix" class="ai-fix-button"${u}>${g}</button>
    </div>`}function Re(e){if(!e.preview)return'<div class="detail-empty">Generate a fix preview to inspect the patch before Ollanta edits your local file.</div>';let t=e.preview.summary||"Generated fix preview",s=e.preview.explanation?`<div class="rule-rationale">${l(e.preview.explanation)}</div>`:"",n=e.applying?"Applying\u2026":"Apply to file",a=e.applying?" disabled":"";return`
    <div class="ai-fix-preview-meta">
      <div><strong>Provider:</strong> ${l(e.preview.agent.label)}</div>
      <div><strong>Model:</strong> ${l(e.preview.agent.model)}</div>
      <div><strong>Summary:</strong> ${l(t)}</div>
    </div>
    ${s}
    <pre class="rule-code ai-fix-diff"><code>${l(e.preview.diff)}</code></pre>
    <div class="ai-fix-actions">
      <button id="ai-apply-fix" class="ai-fix-button ai-fix-button-primary"${a}>${n}</button>
    </div>
  `}var v,_=[],y=[],k=[],M=[],O="",Q=!1,te="",ve=new Map,T=null,j=-1,H=50,$=H,Ee="overview",W=null,q="details",U="",ie=!1,w=null,r=we(),N="all",se="all",me="all",ge="all",fe="all",he="",E="flat",X="all",F="file",Y="asc",D={blocker:0,critical:1,major:2,minor:3,info:4},A={blocker:"#ef4444",critical:"#f97316",major:"#eab308",minor:"#22c55e",info:"#64748b"},le={bug:"Bug",code_smell:"Code Smell",vulnerability:"Vulnerability",security_hotspot:"Hotspot"},xe={security:"Security",reliability:"Reliability",maintainability:"Maintainability",testability:"Testability"};function C(e,t){return`<span class="icon-${l(e)}" role="img" aria-label="${l(t)}"></span>`}async function Oe(){try{let e=await fetch("/report.json");if(!e.ok)throw new Error(`HTTP ${e.status}`);v=await e.json(),_=v.issues??[],De(),Ge(),Ve(),Xe(),Ye(),Ze(),et(),st(),ze(),We(),ee(),Z(),M.length&&Le(O||M[0].path),rt(),dt(),x(),ut(),re(),ct(),Lt(),Te(),o("tab-issue-count").textContent=String(_.length),o("tab-coverage-count").textContent=b(v.measures.coverage??v.test_signals?.summary?.coverage),o("tab-mutant-count").textContent=String(be().survived)}catch(e){o("app").innerHTML=`<div class="error">Failed to load report: ${String(e)}</div>`}}document.addEventListener("DOMContentLoaded",Oe);function De(){let e=v.metadata,t=new Date(e.analysis_date).toLocaleString();o("project-key").textContent=e.project_key,o("scan-date").textContent=t,o("scan-version").textContent=`v${e.version}`,o("elapsed").textContent=`${e.elapsed_ms}ms`}function Be(){let e=v.measures,t=v.test_signals?.summary,s=[{metric:"Bugs",operator:"=",threshold:0,value:e.bugs,passed:e.bugs===0},{metric:"Vulnerabilities",operator:"=",threshold:0,value:e.vulnerabilities,passed:e.vulnerabilities===0},{metric:"Code Smells",operator:"\u2264",threshold:10,value:e.code_smells,passed:e.code_smells<=10,severity:e.code_smells<=10?void 0:e.code_smells<=20?"warning":void 0}];return e.coverage!=null?s.push({metric:"Coverage",operator:"\u2265",threshold:70,value:e.coverage,passed:e.coverage>=70,severity:e.coverage>=70?void 0:e.coverage>=60?"warning":void 0}):s.push({metric:"Coverage",operator:"\u2265",threshold:70,value:0,passed:!1,severity:"missing"}),t&&(t.tests!=null&&s.push({metric:"Test Failures",operator:"=",threshold:0,value:t.test_failures??0,passed:(t.test_failures??0)===0}),t.mutation_score!=null&&s.push({metric:"Mutation Score",operator:"\u2265",threshold:60,value:t.mutation_score,passed:t.mutation_score>=60,severity:t.mutation_score>=60?void 0:t.mutation_score>=40?"warning":void 0}),t.changed_mutation_score!=null&&s.push({metric:"Changed Mutation",operator:"\u2265",threshold:60,value:t.changed_mutation_score,passed:t.changed_mutation_score>=60,severity:t.changed_mutation_score>=60?void 0:t.changed_mutation_score>=40?"warning":void 0})),{status:s.filter(i=>!i.passed&&i.severity!=="warning"&&i.severity!=="missing").length===0?"passed":"failed",conditions:s}}function Ge(){let e=Be(),t=o("gate-hero");t.classList.remove("gate-loading"),t.classList.add(e.status==="passed"?"gate-passed":"gate-failed");let s=o("gate-icon");if(s.className=`gate-icon icon-${e.status==="passed"?"pass":"fail"}`,s.setAttribute("aria-label",e.status==="passed"?"Passed":"Failed"),o("gate-status").textContent=e.status==="passed"?"Passed":"Failed",e.status==="passed"){let a=e.conditions.filter(c=>!c.passed&&c.severity!=="warning");e.conditions.filter(c=>!c.passed&&c.severity==="warning").length&&!a.length&&(o("gate-status").textContent="Passed with warnings",t.classList.add("gate-warn"))}let n=e.conditions.map(a=>{let i=a.passed?"cond-pass":a.severity==="warning"?"cond-warn":"cond-fail",c=a.passed?C("pass","Passed"):C("fail","Failed");return`<div class="gate-cond ${i}">
      <span class="gate-cond-icon">${c}</span>
      <span class="gate-cond-metric">${l(a.metric)} ${l(a.operator)} ${a.threshold}</span>
      <span class="gate-cond-value">${a.value}</span>
    </div>`}).join("");o("gate-conditions").innerHTML=n}function Ve(){let e=v.measures,t=v.test_signals?.summary;h("m-bugs",e.bugs),h("m-vulns",e.vulnerabilities),h("m-smells",e.code_smells),J("m-coverage",b(e.coverage)),h("m-ncloc",e.ncloc),h("m-files",e.files),h("m-comments",e.comments),t?(h("m-tests",t.tests??0),h("m-test-failures",t.test_failures??0),h("m-test-skipped",t.test_skipped??0),h("m-mutants-skipped",t.mutants_skipped??e.mutants_skipped??0),h("m-mutants-error",t.mutants_error??e.mutants_error??0)):(h("m-tests",e.tests??0),h("m-test-failures",e.test_failures??0),h("m-test-skipped",e.test_skipped??0),h("m-mutants-skipped",e.mutants_skipped??0),h("m-mutants-error",e.mutants_error??0)),K("card-bugs",e.bugs,[0,1,5]),K("card-vulns",e.vulnerabilities,[0,1,3]),K("card-smells",e.code_smells,[0,10,50]),Ke("card-coverage",e.coverage),Ue("card-tests",t?.tests??e.tests,[50,20,0]),K("card-test-failures",t?.test_failures??e.test_failures??0,[0,1,5]),f("card-ncloc","card-neutral"),f("card-files","card-neutral"),f("card-comments","card-neutral"),f("card-test-skipped","card-neutral"),f("card-mutants-skipped","card-neutral"),f("card-mutants-error",(t?.mutants_error??e.mutants_error??0)>0?"card-red":"card-neutral");let s=e.duplicated_lines_density;J("m-duplication",b(s)),K("card-duplication",s??0,[3,10,20]),f("card-duplication",s==null?"card-neutral":"");let n=v.test_signals?.health;if(n){J("m-test-health",`${n.status} \xB7 ${n.score}`);let i=o("card-test-health");i.classList.remove("card-neutral","card-green","card-yellow","card-red"),n.status==="healthy"?i.classList.add("card-green"):n.status==="at_risk"?i.classList.add("card-red"):n.status==="partial"?i.classList.add("card-yellow"):i.classList.add("card-neutral")}else J("m-test-health","\u2014"),f("card-test-health","card-neutral");let a=o("card-coverage");a.classList.add("clickable"),a.addEventListener("click",()=>{I("coverage")}),V("card-bugs",()=>pe("bug")),V("card-vulns",()=>pe("vulnerability")),V("card-smells",()=>pe("code_smell")),V("card-test-failures",()=>I("mutants")),V("card-mutants-error",()=>I("mutants"))}function V(e,t){let s=o(e);s.classList.add("clickable"),s.setAttribute("role","button"),s.setAttribute("tabindex","0"),s.setAttribute("aria-label",`View ${s.querySelector(".metric-label")?.textContent??""} details`),s.addEventListener("click",()=>{console.log("[Ollanta] Clicked metric card:",e),t()}),s.addEventListener("keydown",a=>{let i=a;(i.key==="Enter"||i.key===" ")&&(i.preventDefault(),t())});let n=document.createElement("span");n.className="metric-arrow",n.setAttribute("aria-hidden","true"),n.textContent="\u2192",s.appendChild(n)}function pe(e){se=e,o("filter-type").value=e,x(),I("issues")}function h(e,t){o(e).textContent=t.toLocaleString()}function J(e,t){o(e).textContent=t}function b(e){return e==null?"\u2014":`${e.toFixed(1)}%`}function K(e,t,s){t<=s[0]?f(e,"card-green"):t<=s[1]?f(e,"card-yellow"):f(e,"card-red")}function Ke(e,t){t==null?f(e,"card-neutral"):t>=80?f(e,"card-green"):t>=60?f(e,"card-yellow"):f(e,"card-red")}function Ue(e,t,s){if(t==null){f(e,"card-neutral");return}t>=s[0]?f(e,"card-green"):t>=s[1]?f(e,"card-yellow"):f(e,"card-red")}function ze(){let e=o("mutation-summary"),t=be();if(!t.hasSignal){e.innerHTML='<div class="empty-state compact">No mutation report was collected for this scan. Add a supported report such as <span class="mono">ollanta-mutations.json</span>, Stryker JSON, or PIT XML to see mutation score and survived mutants.</div>';return}e.innerHTML=`<div class="mutation-kpis">
      <div><span class="mutation-kpi-value ${oe(t.score)}">${b(t.score)}</span><span class="mutation-kpi-label">mutation score</span></div>
      <div><span class="mutation-kpi-value">${t.killed.toLocaleString()}/${t.total.toLocaleString()}</span><span class="mutation-kpi-label">killed mutants</span></div>
      <div><span class="mutation-kpi-value ${t.survived>0?"mut-warning":"mut-success"}">${t.survived.toLocaleString()}</span><span class="mutation-kpi-label">survived mutants</span></div>
    </div>
    ${Je(t.modules)}
    ${Qe(t.survivedMutants)}`}function be(){let e=v.test_signals?.summary,t=(v.test_signals?.modules??[]).filter(p=>p.mutation),s=e?.changed_mutants_total||e?.mutants_total||v.measures.changed_mutants_total||v.measures.mutants_total||0,n=e?.changed_mutants_killed||e?.mutants_killed||v.measures.changed_mutants_killed||v.measures.mutants_killed||0,a=e?.changed_mutants_survived||e?.mutants_survived||v.measures.changed_mutants_survived||v.measures.mutants_survived||0,i=e?.changed_mutation_score??e?.mutation_score??v.measures.changed_mutation_score??v.measures.mutation_score,c=t.flatMap(p=>p.mutation?.survived_mutants??[]).slice(0,8);return{hasSignal:t.length>0||s>0||i!=null,score:i,total:s,killed:n,survived:a,modules:t,survivedMutants:c}}function Je(e){return e.length?`<div class="mutation-module-list">
    ${e.slice(0,5).map(t=>{let s=t.mutation,n=s.changed_code_score??s.score,a=s.changed_survived??s.survived??0,i=s.changed_total??s.total??0;return`<div class="mutation-module-row">
        <span class="mutation-module-main"><span class="mutation-module-name">${l(t.name||t.root)}</span><span class="mutation-module-meta">${l(s.tool||"mutation")} \xB7 ${i.toLocaleString()} mutants</span></span>
        <span class="mutation-pill ${oe(n)}">${b(n)}</span>
        <span class="mutation-survived ${a>0?"mut-warning":"mut-success"}">${a.toLocaleString()} survived</span>
      </div>`}).join("")}
  </div>`:""}function Qe(e){return e.length?`<div class="mutation-survivors">
    ${e.map(t=>`<div class="mutation-survivor-row">
      <span class="mutation-survivor-file">${l(P(t.file||""))}${t.line?`:L${t.line}`:""}</span>
      <span class="mutation-survivor-meta">${l(t.mutator||t.description||"survived mutant")}</span>
    </div>`).join("")}
  </div>`:""}function oe(e){return e==null?"card-neutral":e>=80?"card-green":e>=60?"card-yellow":"card-red"}function Z(){let e=o("mutants-page"),t=be();if(!t.hasSignal){e.innerHTML='<div class="empty-state">No mutation data collected. Run with <span class="mono">-with-mutations</span> to see survived mutants.</div>';return}let s=t.modules.flatMap(d=>(d.mutation?.survived_mutants??[]).map(g=>({...g,moduleName:d.name||d.root}))),n=[...new Set(s.map(d=>d.moduleName))].sort(),a=s;X!=="all"&&(a=a.filter(d=>d.moduleName===X)),a.sort((d,g)=>{let u=0;return F==="file"?u=(d.file||"").localeCompare(g.file||""):F==="line"?u=(d.line??0)-(g.line??0):F==="module"&&(u=d.moduleName.localeCompare(g.moduleName)),Y==="asc"?u:-u});let i=`
    <div class="mutants-toolbar">
      <div class="toolbar-left">
        <select id="mutant-filter-module">
          <option value="all">All modules</option>
          ${n.map(d=>`<option value="${l(d)}"${d===X?" selected":""}>${l(d)}</option>`).join("")}
        </select>
        <select id="mutant-sort-field">
          <option value="file"${F==="file"?" selected":""}>Sort by file</option>
          <option value="line"${F==="line"?" selected":""}>Sort by line</option>
          <option value="module"${F==="module"?" selected":""}>Sort by module</option>
        </select>
        <select id="mutant-sort-dir">
          <option value="asc"${Y==="asc"?" selected":""}>Ascending</option>
          <option value="desc"${Y==="desc"?" selected":""}>Descending</option>
        </select>
      </div>
      <div class="toolbar-right">
        <span class="result-count">${a.length.toLocaleString()} survived</span>
      </div>
    </div>
  `,c=`
    <div class="mutation-kpis">
      <div><span class="mutation-kpi-value ${oe(t.score)}">${b(t.score)}</span><span class="mutation-kpi-label">mutation score</span></div>
      <div><span class="mutation-kpi-value">${t.killed.toLocaleString()}/${t.total.toLocaleString()}</span><span class="mutation-kpi-label">killed mutants</span></div>
      <div><span class="mutation-kpi-value ${t.survived>0?"mut-warning":"mut-success"}">${t.survived.toLocaleString()}</span><span class="mutation-kpi-label">survived mutants</span></div>
    </div>
  `,p="";a.length?p=`
      <table class="mutants-table">
        <thead>
          <tr>
            <th>File</th>
            <th>Line</th>
            <th>Module</th>
            <th>Mutator</th>
            <th>Description</th>
          </tr>
        </thead>
        <tbody>
          ${a.map(d=>`
            <tr class="mutant-row">
              <td class="mutant-file">${l(P(d.file||""))}</td>
              <td class="mutant-line">${d.line??"\u2014"}</td>
              <td class="mutant-module">${l(d.moduleName)}</td>
              <td class="mutant-mutator">${l(d.mutator||"\u2014")}</td>
              <td class="mutant-desc">${l(d.description||d.replacement||"survived mutant")}</td>
            </tr>
          `).join("")}
        </tbody>
      </table>
    `:p='<div class="empty-state compact">No survived mutants match the current filter.</div>',e.innerHTML=i+c+p,Te()}function Te(){let e=document.getElementById("mutant-filter-module"),t=document.getElementById("mutant-sort-field"),s=document.getElementById("mutant-sort-dir");e?.addEventListener("change",n=>{X=n.target.value,Z()}),t?.addEventListener("change",n=>{F=n.target.value,Z()}),s?.addEventListener("change",n=>{Y=n.target.value,Z()})}function We(){let e=v.test_signals?.modules??[];if(!e.length)return;let t=o("ts-modules"),s="";for(let i of e){let c=i.health,p=i.coverage,d=i.mutation,g=i.suites??[],u=g.reduce((G,z)=>G+(z.failures??0)+(z.errors??0),0),m=g.reduce((G,z)=>G+(z.tests??0),0),L=i.architecture_role??"",R=L?`<span class="ts-role-badge">${l(L)}</span>`:"",ue=ke(c?.status),Pe=b(p?.coverage),Fe=B(p?.coverage??void 0),je=b(d?.changed_code_score??d?.score),qe=oe(d?.changed_code_score??d?.score);s+=`<div class="ts-module">
      <div class="ts-module-head">
        <span class="ts-module-name">${l(i.name)}</span>
        ${R}
        <span class="ts-health-badge ${ue}">${c?.status??"no data"}</span>
      </div>
      <div class="ts-module-meta">
        ${p?`<span class="ts-metric ${Fe}">Coverage ${Pe}</span>`:""}
        ${d?.score!=null||d?.changed_code_score!=null?`<span class="ts-metric ${qe}">Mutation ${je}</span>`:""}
        ${m>0?`<span class="ts-metric">${m} test${m===1?"":"s"}</span>`:""}
        ${u>0?`<span class="ts-metric ts-fail">${u} failed</span>`:""}
      </div>
      ${c?.recommendations?.length?`<div class="ts-recommendations">${c.recommendations.map(G=>`<div class="ts-rec">${l(G)}</div>`).join("")}</div>`:""}
    </div>`}let n=v.test_signals?.health,a=n?`<span class="ts-health-badge ${ke(n.status)}">${n.status} \xB7 score ${n.score}</span>`:"";t.innerHTML=`<div class="ts-header">
      <h3>Test Signals</h3>
      ${a}
    </div>
    <div class="ts-module-list">${s||'<div class="empty-state compact">No module-level test data was collected.</div>'}</div>`}function ke(e){return e==="healthy"?"card-green":e==="at_risk"?"card-red":e==="partial"?"card-yellow":"card-neutral"}function Xe(){let e=ne(_,u=>u.severity),t=Math.max(1,...Object.values(e)),s=["blocker","critical","major","minor","info"],n="",a="",i=_.length||1;for(let u of s){let m=e[u]??0,L=m/t*100,R=A[u]??"#64748b";n+=`<div class="sev-bar-row">
      <span class="sev-bar-label">${u}</span>
      <div class="sev-bar-track"><div class="sev-bar-fill" style="width:${L}%;background:${R}"></div></div>
      <span class="sev-bar-count">${m}</span>
    </div>`,m>0&&(a+=`<div class="sev-segment" style="width:${m/i*100}%;background:${R}" title="${u}: ${m}"></div>`)}o("sev-bars").innerHTML=n,o("sev-proportional").innerHTML=a;let c=ne(_,u=>u.type),p=Math.max(1,...Object.values(c)),d={bug:"#ef4444",vulnerability:"#f97316",code_smell:"#22c55e",security_hotspot:"#eab308"},g="";for(let[u,m]of Object.entries(le)){let L=c[u]??0,R=L/p*100,ue=d[u]??"#64748b";g+=`<div class="sev-bar-row">
      <span class="sev-bar-label">${m}</span>
      <div class="sev-bar-track"><div class="sev-bar-fill" style="width:${R}%;background:${ue}"></div></div>
      <span class="sev-bar-count">${L}</span>
    </div>`}o("type-bars").innerHTML=g}function Ye(){let e=[..._].sort((t,s)=>{let n=(D[t.severity]??99)-(D[s.severity]??99);return n!==0?n:t.component_path.localeCompare(s.component_path)||t.line-s.line}).slice(0,6);if(!e.length){o("priority-issues").innerHTML='<div class="empty-state compact">No issues found</div>';return}o("priority-issues").innerHTML=e.map((t,s)=>{let n=A[t.severity]??"#64748b",a=P(t.component_path);return`<button class="priority-row" data-idx="${s}">
      <span class="issue-sev-dot" style="background:${n}"></span>
      <span class="priority-main">
        <span class="priority-title">${l(t.message)}</span>
        <span class="priority-meta" title="${l(t.component_path)}">${l(a)}:L${t.line} \xB7 ${l(t.rule_key)}</span>
      </span>
      <span class="priority-severity">${l(t.severity)}</span>
    </button>`}).join(""),o("priority-issues").querySelectorAll(".priority-row").forEach(t=>{t.addEventListener("click",()=>{let s=Number.parseInt(t.dataset.idx,10);ce(e[s])})})}function Ze(){let e=ne(_,s=>s.component_path),t=Object.entries(e).sort((s,n)=>n[1]-s[1]).slice(0,10);if(!t.length){o("hotspot-files").innerHTML='<div class="empty-state">No issues found</div>';return}o("hotspot-files").innerHTML=t.map(([s,n])=>{let a=P(s);return`<div class="hotspot-row" data-path="${l(s)}">
      <span class="hotspot-file" title="${l(s)}">${l(a)}</span>
      <span class="hotspot-count">${n}</span>
    </div>`}).join(""),o("hotspot-files").querySelectorAll(".hotspot-row").forEach(s=>{s.addEventListener("click",()=>{let n=s.dataset.path;E="grouped",I("issues"),pt(n)})})}function et(){M=(v.test_signals?.modules??[]).flatMap(t=>(t.files??[]).map(s=>tt(t.name,t.root,s))).filter(t=>t.linesToCover>0).sort((t,s)=>(t.coverage??101)-(s.coverage??101)||s.uncoveredLines.length-t.uncoveredLines.length||t.path.localeCompare(s.path)),!O&&M.length&&(O=M[0].path)}function tt(e,t,s){let n=s.lines_to_cover??0,a=s.covered_lines??0,i=n>0?a*100/n:null;return{moduleName:e,moduleRoot:t,path:s.path,linesToCover:n,coveredLines:a,coveredLineNumbers:s.covered_line_numbers??[],uncoveredLines:s.uncovered_lines??[],coverage:i}}function st(){let e=o("coverage-summary");if(!M.length){e.innerHTML='<div class="empty-state compact">Run with <span class="mono">-with-tests</span> and provide a coverage report to see file-level details.</div>';return}let t=v.test_signals?.summary,s=M.slice(0,5);e.innerHTML=`<div class="coverage-kpis">
      <div><span class="coverage-kpi-value">${b(t?.coverage??v.measures.coverage)}</span><span class="coverage-kpi-label">overall</span></div>
      <div><span class="coverage-kpi-value">${(t?.covered_lines??0).toLocaleString()}/${(t?.lines_to_cover??0).toLocaleString()}</span><span class="coverage-kpi-label">covered lines</span></div>
      <div><span class="coverage-kpi-value">${(t?.modules_with_coverage??0).toLocaleString()}</span><span class="coverage-kpi-label">modules</span></div>
    </div>
    <div class="coverage-file-list">
      ${s.map(at).join("")}
    </div>`,e.querySelectorAll(".coverage-mini-row").forEach(n=>{n.addEventListener("click",()=>{let a=n.dataset.coveragePath;a&&(I("coverage"),Le(a))})})}function at(e){return`<button class="coverage-mini-row" data-coverage-path="${l(e.path)}">
    <span class="coverage-mini-main">
      <span class="coverage-file-name" title="${l(e.path)}">${l(P(e.path))}</span>
      <span class="coverage-file-meta">${l(e.moduleName)} \xB7 ${e.uncoveredLines.length.toLocaleString()} uncovered lines</span>
    </span>
    <span class="coverage-pill ${B(e.coverage)}">${b(e.coverage??void 0)}</span>
  </button>`}function ee(){let e=o("coverage-details");if(!M.length){e.innerHTML='<div class="empty-state">No file-level coverage was collected for this scan.</div>';return}e.innerHTML=`<div class="coverage-toolbar">
      <div>
        <h3>Coverage Files</h3>
        <p>${M.length.toLocaleString()} files with line-level detail. Overall coverage includes all measured files, not only those listed here.</p>
      </div>
      <span class="coverage-pill ${B(v.measures.coverage??null)}">${b(v.measures.coverage)}</span>
    </div>
    <div class="coverage-browser">
      <aside class="coverage-sidebar">
        <div class="coverage-file-list coverage-file-list-large">
          ${M.map(nt).join("")}
        </div>
      </aside>
      <section class="coverage-code-viewer">
        ${it()}
      </section>
    </div>`,e.querySelectorAll(".coverage-row").forEach(t=>{t.addEventListener("click",()=>{let s=t.dataset.coveragePath;s&&Le(s)})})}function nt(e){return`<button class="coverage-row${e.path===O?" active":""}" data-coverage-path="${l(e.path)}">
    <div class="coverage-row-main">
      <div class="coverage-row-title" title="${l(e.path)}">${l(e.path)}</div>
      <div class="coverage-row-subtitle">${l(e.moduleName)} \xB7 ${l(e.moduleRoot)} \xB7 ${e.coveredLines.toLocaleString()}/${e.linesToCover.toLocaleString()} lines covered</div>
    </div>
    <div class="coverage-row-meter">
      <span class="coverage-pill ${B(e.coverage)}">${b(e.coverage??void 0)}</span>
      <div class="coverage-track"><div class="coverage-fill ${B(e.coverage)}" style="width:${e.coverage??0}%"></div></div>
    </div>
  </button>`}async function Le(e){if(M.some(t=>t.path===e)){if(O=e,te="",ve.has(e)){Q=!1,ee();return}Q=!0,ee();try{let t=await fetch(`/api/files/source?path=${encodeURIComponent(e)}`);if(!t.ok)throw new Error(`HTTP ${t.status}`);let s=await t.json();ve.set(e,s.file)}catch(t){te=`Could not load source for ${e}: ${String(t)}`}finally{Q=!1,ee()}}}function it(){let e=M.find(p=>p.path===O);if(!e)return'<div class="code-empty"><p>Select a file to inspect coverage.</p></div>';if(Q)return'<div class="code-empty"><div class="spinner"></div></div>';if(te)return`<div class="code-empty"><p>${l(te)}</p></div>`;let t=ve.get(e.path);if(!t)return'<div class="code-empty"><p>Select a coverage file to inspect source lines.</p></div>';let s=new Set(e.coveredLineNumbers),n=new Set(e.uncoveredLines),i=t.content.split(`
`).map((p,d)=>{let g=d+1,u=n.has(g),m=!u&&s.has(g),L=lt(m,u);return`<div class="code-line${L.stateClass}">
      <span class="code-gutter">${g}</span>
      <code class="code-text">${p.length?l(p):"&nbsp;"}</code>
      <span class="code-markers">${ot(L)}</span>
    </div>`}).join(""),c=e.coveredLineNumbers.length?"covered and uncovered lines":"uncovered lines only";return`<div class="code-viewer-shell coverage-source-shell">
    <div class="code-viewer-head">
      <div>
        <div class="code-viewer-path mono">${l(t.path)}</div>
        <div class="code-viewer-meta">${l(t.language||"plain text")} \xB7 ${t.line_count.toLocaleString()} lines \xB7 ${c}</div>
      </div>
      <div class="code-viewer-stats"><span class="coverage-pill ${B(e.coverage)}">${b(e.coverage??void 0)}</span></div>
    </div>
    <div class="coverage-legend">
      <span><span class="legend-dot legend-covered"></span>Covered</span>
      <span><span class="legend-dot legend-uncovered"></span>Not covered</span>
    </div>
    <div class="code-surface">${i}</div>
  </div>`}function lt(e,t){return t?{stateClass:" is-uncovered",marker:"not covered",chipClass:" chip-uncovered"}:e?{stateClass:" is-covered",marker:"covered",chipClass:" chip-covered"}:{stateClass:"",marker:"",chipClass:""}}function ot(e){return e.marker?`<span class="coverage-line-chip${e.chipClass}">${e.marker}</span>`:""}function B(e){return e==null?"card-neutral":e>=80?"card-green":e>=60?"card-yellow":"card-red"}function rt(){let e=Object.entries(v.measures.by_language).sort((s,n)=>n[1]-s[1]),t=Math.max(1,e[0]?.[1]??1);if(!e.length){o("by-lang").innerHTML='<span class="empty-state">No language data</span>';return}o("by-lang").innerHTML=e.map(([s,n])=>`<div class="lang-row">
      <span class="lang-name">${l(s)}</span>
      <div class="lang-bar-track"><div class="lang-bar-fill" style="width:${n/t*100}%"></div></div>
      <span class="lang-count">${n} files</span>
    </div>`).join("")}function ct(){document.querySelectorAll(".tab").forEach(s=>{s.addEventListener("click",()=>{let n=s.dataset.tab;I(n)})});let e=document.getElementById("view-mode-toggle");e&&e.addEventListener("click",()=>{E==="flat"?E="file":E==="file"?E="rule":E="flat";let s={flat:"Group by file",file:"Group by rule",rule:"List view"};e.textContent=s[E],e.setAttribute("aria-pressed",String(E!=="flat")),x()}),document.querySelector(".tabs").addEventListener("keydown",s=>{let n=Array.from(document.querySelectorAll(".tab[role='tab']")),a=n.findIndex(i=>i.getAttribute("aria-selected")==="true");if(s.key==="ArrowRight"){s.preventDefault();let i=n[(a+1)%n.length];i.focus(),I(i.dataset.tab)}else if(s.key==="ArrowLeft"){s.preventDefault();let i=n[(a-1+n.length)%n.length];i.focus(),I(i.dataset.tab)}})}function I(e){Ee=e,document.querySelectorAll(".tab").forEach(s=>{s.classList.remove("active"),s.setAttribute("aria-selected","false")});let t=document.querySelector(`.tab[data-tab="${e}"]`);t?.classList.add("active"),t?.setAttribute("aria-selected","true"),document.querySelectorAll(".panel").forEach(s=>s.classList.add("hidden")),o(`panel-${e}`).classList.remove("hidden")}function dt(){let e=[...new Set(_.map(a=>a.rule_key))].sort((a,i)=>a.localeCompare(i)),t=o("filter-rule");e.forEach(a=>{let i=document.createElement("option");i.value=a,i.textContent=a,t.appendChild(i)});let s=new Set;for(let a of _)for(let i of a.tags??[])s.add(i);let n=o("filter-tag");[...s].sort().forEach(a=>{let i=document.createElement("option");i.value=a,i.textContent=a,n.appendChild(i)}),o("filter-severity").addEventListener("change",a=>{N=a.target.value,x()}),o("filter-type").addEventListener("change",a=>{se=a.target.value,x()}),t.addEventListener("change",a=>{me=a.target.value,x()}),o("filter-quality").addEventListener("change",a=>{ge=a.target.value,x()}),n.addEventListener("change",a=>{fe=a.target.value,x()}),o("search").addEventListener("input",a=>{he=a.target.value.toLowerCase(),x()}),Ie()}function Ie(){let e=ne(_,s=>s.severity),t=["blocker","critical","major","minor","info"];o("sev-chips").innerHTML=t.map(s=>{let n=e[s]??0,a=A[s]??"#64748b",i=N===s?" active":"";return`<button class="sev-chip${i}" data-sev="${s}"
      style="--chip-color:${a};--chip-bg:${a}15" aria-pressed="${i?"true":"false"}">
      <span class="chip-dot" style="background:${a}"></span>
      ${s}
      <span class="chip-count">${n}</span>
    </button>`}).join(""),o("sev-chips").querySelectorAll(".sev-chip").forEach(s=>{s.addEventListener("click",()=>{let n=s.dataset.sev;N=N===n?"all":n,o("filter-severity").value=N,x(),Ie()})})}function x(){y=_.filter(s=>!(N!=="all"&&s.severity!==N||se!=="all"&&s.type!==se||me!=="all"&&s.rule_key!==me||ge!=="all"&&s.quality!==ge||fe!=="all"&&!(s.tags??[]).includes(fe)||he&&!`${s.component_path} ${s.message} ${s.rule_key}`.toLowerCase().includes(he))),y.sort((s,n)=>{let a=D[s.severity]??99,i=D[n.severity]??99;return a-i}),j=-1,$=H,Ce();let e=y.length,t=document.getElementById("filter-announcer");t&&(t.textContent=`${e} issue${e===1?"":"s"} match the current filters`)}function Ce(){let e=o("issue-list"),t=o("issue-grouped"),s=y.length===1?"issue":"issues";if(o("issue-count").textContent=`${y.length} ${s}`,!y.length){e.innerHTML='<div class="empty-state">No issues match the current filters.</div>',t.innerHTML='<div class="empty-state">No issues match the current filters.</div>';return}if(E==="file"){e.classList.add("hidden"),t.classList.remove("hidden"),re();return}if(E==="rule"){e.classList.add("hidden"),t.classList.remove("hidden"),He();return}e.classList.remove("hidden"),t.classList.add("hidden");let n=y.slice(0,$);if(e.innerHTML=n.map((a,i)=>{let c=A[a.severity]??"#64748b",p=P(a.component_path),d=a.end_line&&a.end_line!==a.line?`L${a.line}\u2013${a.end_line}`:`L${a.line}`,g=le[a.type]??a.type,u=a.quality?`<span class="quality-badge quality-${l(a.quality)}">${l(xe[a.quality]??a.quality)}</span>`:"";return`<div class="issue-row" role="button" tabindex="0" aria-label="${l(a.severity)} issue: ${l(a.message)}" data-idx="${i}">
      <span class="issue-sev">
        <span class="issue-sev-dot" style="background:${c}"></span>
        ${l(a.severity)}
      </span>
      <span class="issue-type">${l(g)}</span>
      <div class="issue-main">
        <span class="issue-msg">${l(a.message)}</span>
        <span class="issue-file" title="${l(a.component_path)}">${l(p)}:${d}</span>
      </div>
      ${u}
      <span class="issue-rule">${l(a.rule_key)}</span>
    </div>`}).join(""),$<y.length){let a=y.length-$;e.innerHTML+=`<button class="btn-sm btn-outline load-more-btn" id="loadMoreBtn">Show ${Math.min(H,a)} more (${a} remaining)</button>`}e.querySelectorAll(".issue-row").forEach(a=>{a.addEventListener("click",()=>{let i=Number.parseInt(a.dataset.idx,10);ae(i)}),a.addEventListener("keydown",i=>{let c=i;if(c.key==="Enter"||c.key===" "){c.preventDefault();let p=Number.parseInt(a.dataset.idx,10);ae(p)}})}),document.getElementById("loadMoreBtn")?.addEventListener("click",()=>{$+=H,Ce()})}function ut(){let e=new Map;for(let t of _){let s=t.component_path;e.has(s)||e.set(s,[]),e.get(s).push(t)}k=[...e.entries()].sort((t,s)=>s[1].length-t[1].length).map(([t,s])=>({path:t,shortPath:P(t),issues:[...s].sort((n,a)=>n.line-a.line),expanded:!1}))}function re(){let e=o("issue-grouped");if(!k.length){e.innerHTML='<div class="empty-state">No issues found</div>';return}let t=k.slice(0,$);if(e.innerHTML=t.map((s,n)=>`<div class="file-group${s.expanded?" expanded":""}" data-gi="${n}">
      <div class="file-group-header">
        <span class="file-group-chevron icon-chevron" role="img" aria-label="Expand"></span>
        <span class="file-group-name" title="${l(s.path)}">${l(s.shortPath)}</span>
        <span class="file-group-count">${s.issues.length}</span>
      </div>
      <div class="file-group-issues" style="${s.expanded?"":"display:none"}">
        ${s.issues.map((a,i)=>{let c=A[a.severity]??"#64748b";return`<div class="file-issue" data-gi="${n}" data-ii="${i}">
            <span class="issue-sev">
              <span class="issue-sev-dot" style="background:${c}"></span>
              ${l(a.severity)}
            </span>
            <span class="issue-msg">${l(a.message)}</span>
            <span class="file-issue-line">L${a.line}</span>
          </div>`}).join("")}
      </div>
    </div>`).join(""),$<k.length){let s=k.length-$;e.innerHTML+=`<button class="btn-sm btn-outline load-more-btn" id="loadMoreGroupedBtn">Show ${Math.min(H,s)} more files (${s} remaining)</button>`}e.querySelectorAll(".file-group-header").forEach(s=>{s.addEventListener("click",()=>{let n=s.closest(".file-group"),a=Number.parseInt(n.dataset.gi,10);k[a].expanded=!k[a].expanded,n.classList.toggle("expanded");let i=n.querySelector(".file-group-issues");i.style.display=k[a].expanded?"":"none"})}),e.querySelectorAll(".file-issue").forEach(s=>{s.addEventListener("click",n=>{n.stopPropagation();let a=Number.parseInt(s.dataset.gi,10),i=Number.parseInt(s.dataset.ii,10),c=k[a].issues[i];ce(c)})}),document.getElementById("loadMoreGroupedBtn")?.addEventListener("click",()=>{$+=H,re()})}function He(){let e=o("issue-grouped");if(!y.length){e.innerHTML='<div class="empty-state">No issues found</div>';return}let t=new Map;for(let a of y)t.has(a.rule_key)||t.set(a.rule_key,[]),t.get(a.rule_key).push(a);let s=[...t.entries()].sort((a,i)=>i[1].length-a[1].length),n=s.slice(0,$);if(e.innerHTML=n.map(([a,i],c)=>{let d=[...new Set(i.map(u=>u.severity))].sort((u,m)=>(D[u]??99)-(D[m]??99))[0],g=A[d]??"#64748b";return`<div class="file-group" data-ri="${c}">
      <div class="file-group-header">
        <span class="file-group-chevron icon-chevron" role="img" aria-label="Expand"></span>
        <span class="file-group-name">${l(a)}</span>
        <span class="file-group-count">${i.length}</span>
      </div>
      <div class="file-group-issues" style="display:none">
        ${i.map((u,m)=>{let L=P(u.component_path);return`<div class="file-issue" data-ri="${c}" data-ii="${m}">
            <span class="issue-sev">
              <span class="issue-sev-dot" style="background:${A[u.severity]??"#64748b"}"></span>
              ${l(u.severity)}
            </span>
            <span class="issue-msg">${l(u.message)}</span>
            <span class="file-issue-line">${l(L)}:L${u.line}</span>
          </div>`}).join("")}
      </div>
    </div>`}).join(""),$<s.length){let a=s.length-$;e.innerHTML+=`<button class="btn-sm btn-outline load-more-btn" id="loadMoreRuleBtn">Show ${Math.min(H,a)} more rules (${a} remaining)</button>`}e.querySelectorAll(".file-group-header").forEach(a=>{a.addEventListener("click",()=>{let i=a.closest(".file-group");i.classList.toggle("expanded");let c=i.querySelector(".file-group-issues");c.style.display=i.classList.contains("expanded")?"":"none"})}),e.querySelectorAll(".file-issue").forEach(a=>{a.addEventListener("click",i=>{i.stopPropagation();let c=Number.parseInt(a.dataset.ri,10),p=Number.parseInt(a.dataset.ii,10),d=t.get(s[c][0])[p];ce(d)})}),document.getElementById("loadMoreRuleBtn")?.addEventListener("click",()=>{$+=H,He()})}function pt(e){let t=k.findIndex(n=>n.path===e);if(t<0)return;k[t].expanded=!0,re(),document.querySelector(`.file-group[data-gi="${t}"]`)?.scrollIntoView({behavior:"smooth",block:"start"})}function ae(e,t=!0){j=e,T=y[e]??null,document.querySelectorAll(".issue-row").forEach(n=>n.classList.remove("selected"));let s=document.querySelector(`.issue-row[data-idx="${e}"]`);s?.classList.add("selected"),s?.focus(),t&&T&&ce(T)}function ce(e){W=document.activeElement,T=e,q="details",U="",ie=!0,r=we(),o("detail-title").textContent=e.message||e.rule_key,de(e),o("detail-panel").classList.add("open"),o("detail-overlay").classList.add("open"),o("detail-panel").querySelector("button, [href], input, select, textarea, [tabindex]:not([tabindex='-1'])")?.focus(),vt(e.rule_key)}async function vt(e){try{let t=await fetch(`/rules/${encodeURIComponent(e)}`);if(!t.ok)throw new Error("not found");let s=await t.json(),n="";s.rationale&&(n+=`<div class="detail-section">
        <div class="detail-section-title">Why is this a problem?</div>
        <div class="rule-rationale">${l(s.rationale)}</div>
      </div>`),s.description&&s.description!==s.rationale&&(n+=`<div class="detail-section">
        <div class="detail-section-title">Description</div>
        <div class="rule-rationale">${l(s.description)}</div>
      </div>`),s.noncompliant_code&&(n+=`<div class="detail-section">
        <div class="detail-section-title">${C("cross","Noncompliant")} Noncompliant Code</div>
        <pre class="rule-code noncompliant"><code>${l(s.noncompliant_code)}</code></pre>
      </div>`),s.compliant_code&&(n+=`<div class="detail-section">
        <div class="detail-section-title">${C("check","Compliant")} Compliant Code</div>
        <pre class="rule-code compliant"><code>${l(s.compliant_code)}</code></pre>
      </div>`),s.reference_url&&(n+=`<div class="detail-section">
        <div class="detail-section-title">Reference</div>
        <p><a href="${l(s.reference_url)}" target="_blank" rel="noopener">${l(s.reference_url)}</a></p>
      </div>`),U=n||'<div class="detail-empty">No additional rule details available.</div>'}catch{U='<div class="detail-empty">Rule details are not available for this issue.</div>'}finally{ie=!1,T?.rule_key===e&&de(T)}}function mt(e){document.getElementById("detailCopy")?.addEventListener("click",()=>{gt(e)})}async function gt(e){let t=[];t.push(`Issue: ${e.message||""}`),t.push(`Severity: ${e.severity}`),t.push(`Type: ${le[e.type]??e.type}`),t.push(`Rule: ${e.rule_key}`),e.engine_id&&t.push(`Engine: ${e.engine_id}`),t.push(`File: ${e.component_path}`);let s=e.end_line&&e.end_line!==e.line?`lines ${e.line}\u2013${e.end_line}`:`line ${e.line}`;t.push(`Location: ${s}${e.column?", column "+e.column:""}`),t.push(`Status: ${e.status}`),e.tags?.length&&t.push(`Tags: ${e.tags.join(", ")}`);try{let n=await fetch(`/rules/${encodeURIComponent(e.rule_key)}`);if(n.ok){let a=await n.json();a.rationale&&t.push(`
Why is this a problem?
${a.rationale}`),a.noncompliant_code&&t.push(`
Noncompliant code:
${a.noncompliant_code}`),a.compliant_code&&t.push(`
Compliant code:
${a.compliant_code}`),a.reference_url&&t.push(`
Reference: ${a.reference_url}`)}}catch{}try{await navigator.clipboard.writeText(t.join(`
`));let n=document.getElementById("detailCopy");n&&(n.innerHTML=`${C("check","Copied")} Copied`,setTimeout(()=>{n.innerHTML=`${C("copy","Copy")} Copy`},2e3))}catch{let n=document.getElementById("detailCopy");n&&(n.innerHTML=`${C("warn","Failed")} Failed`,setTimeout(()=>{n.innerHTML=`${C("copy","Copy")} Copy`},2e3))}}function ye(){o("detail-panel").classList.remove("open"),o("detail-overlay").classList.remove("open"),T=null,U="",ie=!1,r=we(),document.querySelectorAll(".issue-row").forEach(e=>e.classList.remove("selected")),W&&(W.focus(),W=null)}function de(e){let t=`
    <div class="detail-tabs">
      ${_e(q)}
    </div>
    <div class="detail-tab-panel${q==="details"?"":" hidden"}" data-detail-panel="details">
      ${ft(e)}
    </div>
    <div class="detail-tab-panel${q==="rule"?"":" hidden"}" data-detail-panel="rule">
      ${ie?'<div class="detail-loading">Loading rule details\u2026</div>':U}
    </div>
    <div class="detail-tab-panel${q==="ai-fix"?"":" hidden"}" data-detail-panel="ai-fix">
      ${Me(e,r,w??[])}
    </div>
  `;o("detail-body").innerHTML=t,ht(e),mt(e)}function ft(e){let t=A[e.severity]??"#64748b",s=le[e.type]??e.type,n=e.end_line&&e.end_line!==e.line?`${e.line}:${e.column} \u2013 ${e.end_line}:${e.end_column}`:`${e.line}:${e.column}`,a=`
    <div class="detail-section">
      <div class="detail-msg">${l(e.message)}</div>
    </div>
    <div class="detail-section">
      <div class="detail-section-title">Properties</div>
      <div class="detail-field">
        <span class="detail-field-label">Severity</span>
        <span class="detail-field-value"><span class="issue-sev-dot" style="background:${t};display:inline-block;width:8px;height:8px;border-radius:50%;margin-right:6px"></span>${l(e.severity)}</span>
      </div>
      <div class="detail-field">
        <span class="detail-field-label">Type</span>
        <span class="detail-field-value">${l(s)}</span>
      </div>
      ${e.quality?`<div class="detail-field">
        <span class="detail-field-label">Quality</span>
        <span class="detail-field-value"><span class="quality-badge quality-${l(e.quality)}">${l(xe[e.quality]??e.quality)}</span></span>
      </div>`:""}
      <div class="detail-field">
        <span class="detail-field-label">Rule</span>
        <span class="detail-field-value" style="font-family:var(--font-mono);color:var(--accent)">${l(e.rule_key)}</span>
      </div>
      <div class="detail-field">
        <span class="detail-field-label">Status</span>
        <span class="detail-field-value">${l(e.status)}</span>
      </div>
      ${e.engine_id?`<div class="detail-field">
        <span class="detail-field-label">Engine</span>
        <span class="detail-field-value">${l(e.engine_id)}</span>
      </div>`:""}
      ${e.tags?.length?`<div class="detail-field">
        <span class="detail-field-label">Tags</span>
        <span class="detail-field-value">${e.tags.map(i=>l(i)).join(", ")}</span>
      </div>`:""}
    </div>
    <div class="detail-section">
      <div class="detail-section-title">Location</div>
      <div class="detail-field">
        <span class="detail-field-label">File</span>
        <span class="detail-field-value" style="font-family:var(--font-mono);font-size:12px;word-break:break-all">${l(e.component_path)}</span>
      </div>
      <div class="detail-field">
        <span class="detail-field-label">Lines</span>
        <span class="detail-field-value" style="font-family:var(--font-mono)">${n}</span>
      </div>
    </div>`;return e.secondary_locations?.length&&(a+=`<div class="detail-section">
      <div class="detail-section-title">Related Locations (${e.secondary_locations.length})</div>
      <div class="detail-loc-list">
        ${e.secondary_locations.map(i=>`
          <div class="detail-loc-item">
            <div class="detail-loc-file">${l(i.file_path||e.component_path)}:${i.start_line}</div>
            ${i.message?`<div class="detail-loc-msg">${l(i.message)}</div>`:""}
          </div>
        `).join("")}
      </div>
    </div>`),a}function ht(e){document.querySelectorAll(".detail-tab").forEach(a=>{a.addEventListener("click",()=>{q=a.dataset.detailTab??"details",de(e),q==="ai-fix"&&yt()})});let t=document.getElementById("ai-provider-select");t?.addEventListener("change",()=>{r.selectedProviderId=t.value,r.selectedModel="",$e(),r.preview=null,r.statusMessage="",r.errorMessage="",S()});let s=document.getElementById("ai-model-input");s?.addEventListener("input",()=>{r.selectedModel=s.value});let n=document.getElementById("ai-api-key-input");n?.addEventListener("input",()=>{r.apiKey=n.value}),document.getElementById("ai-generate-fix")?.addEventListener("click",()=>{$t(e)}),document.getElementById("ai-apply-fix")?.addEventListener("click",()=>{bt()})}function we(){return{loadingOptions:!1,loadingPreview:!1,applying:!1,selectedProviderId:"",selectedModel:"",apiKey:"",statusMessage:"",errorMessage:"",preview:null}}function Ae(){return!w||w.length===0?null:w.find(e=>e.id===r.selectedProviderId)??w[0]}function $e(){if(!w||w.length===0){r.selectedProviderId="",r.selectedModel="";return}w.some(t=>t.id===r.selectedProviderId)||(r.selectedProviderId=w[0].id);let e=Ae();if(!e){r.selectedModel="";return}r.selectedModel||(r.selectedModel=e.default_model||e.models[0]||"")}async function yt(){if(w){$e(),S();return}r.loadingOptions=!0,r.errorMessage="",S();try{let e=await fetch("/api/ai/providers");if(!e.ok)throw new Error(`HTTP ${e.status}`);w=(await e.json()).providers??[],$e()}catch(e){r.errorMessage=`Failed to load AI models: ${String(e)}`,w=[]}finally{r.loadingOptions=!1,S()}}async function $t(e){let t=Ae(),s=r.selectedModel.trim();if(!t||!r.selectedProviderId){r.errorMessage="Choose an AI provider before generating a fix.",S();return}if(!s){r.errorMessage="Choose a model before generating a fix.",S();return}if(t.requires_api_key&&!t.configured&&!r.apiKey.trim()){r.errorMessage="Provide an API key for the selected provider before generating a fix.",S();return}r.selectedModel=s,r.loadingPreview=!0,r.statusMessage="",r.errorMessage="",S();try{let n={provider:r.selectedProviderId,model:s,api_key:r.apiKey.trim()||void 0,issue:e},a=await fetch("/api/ai/fixes/preview",{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify(n)}),i=await a.json();if(!a.ok||"error"in i)throw new Error("error"in i?i.error:`HTTP ${a.status}`);r.preview=i,r.statusMessage="Fix preview generated. Review the diff before applying it."}catch(n){r.errorMessage=`Failed to generate AI fix: ${String(n)}`,r.preview=null}finally{r.loadingPreview=!1,S()}}async function bt(){if(r.preview){r.applying=!0,r.errorMessage="",S();try{let e=await fetch("/api/ai/fixes/apply",{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify({preview_id:r.preview.preview_id})}),t=await e.json();if(!e.ok||"error"in t)throw new Error("error"in t?t.error:`HTTP ${e.status}`);r.statusMessage=t.message}catch(e){r.errorMessage=`Failed to apply AI fix: ${String(e)}`}finally{r.applying=!1,S()}}}function S(){T&&de(T)}document.addEventListener("DOMContentLoaded",()=>{o("detail-close").addEventListener("click",ye),o("detail-overlay").addEventListener("click",ye)});function Lt(){document.addEventListener("keydown",e=>{let t=e.target.tagName;if(!(t==="INPUT"||t==="SELECT"||t==="TEXTAREA")){if(e.key==="Escape"){ye();return}Ee==="issues"&&(e.key==="j"||e.key==="ArrowDown"?(e.preventDefault(),j<y.length-1&&ae(j+1,!1),Se()):(e.key==="k"||e.key==="ArrowUp")&&(e.preventDefault(),j>0&&ae(j-1,!1),Se()))}})}function Se(){document.querySelector(`.issue-row[data-idx="${j}"]`)?.scrollIntoView({behavior:"smooth",block:"nearest"})}function o(e){return document.getElementById(e)}function f(e,t){o(e).classList.add(t)}function ne(e,t){let s={};for(let n of e){let a=t(n);s[a]=(s[a]??0)+1}return s}function P(e){let t=e.replaceAll("\\","/"),s=t.split("/").filter(Boolean);return s.length<=2?t:`${s.slice(-2).join("/")}`}})();
