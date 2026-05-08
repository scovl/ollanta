"use strict";(()=>{function i(e){return e.replaceAll("&","&amp;").replaceAll("<","&lt;").replaceAll(">","&gt;").replaceAll('"',"&quot;")}function $e(e){return[{key:"details",label:"Details"},{key:"rule",label:"Rule"},{key:"ai-fix",label:"Fix with AI"}].map(s=>`<button class="detail-tab${e===s.key?" active":""}" data-detail-tab="${s.key}">${s.label}</button>`).join("")}function be(e,t,s){let a=e.end_line&&e.end_line!==e.line?`-${e.end_line}`:"",n=Pe(t,s),l=He(t);return`
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
      ${l}
    </div>
  `}function Pe(e,t){if(e.loadingOptions)return'<div class="detail-loading">Loading AI models\u2026</div>';if(t.length===0)return'<div class="detail-empty">No AI provider is available for the local scanner.</div>';let s=t.find(m=>m.id===e.selectedProviderId)??t[0],a=t.map(m=>`<option value="${i(m.id)}"${e.selectedProviderId===m.id?" selected":""}>${i(m.label)}</option>`).join(""),l=(s?.models??[]).map(m=>`<option value="${i(m)}"></option>`).join(""),c='<div class="ai-fix-helper">This provider can generate local previews without an API key.</div>',p="Required for this provider";s?.requires_api_key&&(s.configured?(c=`<div class="ai-fix-helper">Using the scanner's configured API key. Paste another key below to override it for this session.</div>`,p="Optional override"):c='<div class="ai-fix-helper">Paste an API key for the selected provider to generate the fix.</div>');let d=s?.requires_api_key?`<div class="ai-fix-control-group">
          <label class="ai-fix-control-label" for="ai-api-key-input">API key</label>
          <input id="ai-api-key-input" class="ai-fix-select ai-fix-input" type="password" value="${i(e.apiKey)}" placeholder="${p}" autocomplete="off">
        </div>`:"",g=e.loadingPreview?"Generating\u2026":"Generate fix",v=e.loadingPreview?" disabled":"";return`<div class="ai-fix-controls">
      <div class="ai-fix-control-group">
        <label class="ai-fix-control-label" for="ai-provider-select">Provider</label>
        <select id="ai-provider-select" class="ai-fix-select">${a}</select>
      </div>
      <div class="ai-fix-control-group">
        <label class="ai-fix-control-label" for="ai-model-input">Model</label>
        <input id="ai-model-input" class="ai-fix-select ai-fix-input" list="ai-model-options" value="${i(e.selectedModel)}" placeholder="${i(s?.default_model||"gpt-5.5")}" autocomplete="off">
        <datalist id="ai-model-options">${l}</datalist>
      </div>
      ${d}
      ${c}
      <button id="ai-generate-fix" class="ai-fix-button"${v}>${g}</button>
    </div>`}function He(e){if(!e.preview)return'<div class="detail-empty">Generate a fix preview to inspect the patch before Ollanta edits your local file.</div>';let t=e.preview.summary||"Generated fix preview",s=e.preview.explanation?`<div class="rule-rationale">${i(e.preview.explanation)}</div>`:"",a=e.applying?"Applying\u2026":"Apply to file",n=e.applying?" disabled":"";return`
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
  `}var u,b=[],k=[],M=[],L=[],q="",K=!1,X="",le=new Map,x=null,C=-1,_e="overview",U=null,I="details",B="",te=!1,$=null,r=ye(),P="all",oe="all",re="all",de="all",ce="all",ue="",F="flat",z="all",E="file",J="asc",Y={blocker:0,critical:1,major:2,minor:3,info:4},R={blocker:"#ef4444",critical:"#f97316",major:"#eab308",minor:"#22c55e",info:"#64748b"},se={bug:"Bug",code_smell:"Code Smell",vulnerability:"Vulnerability",security_hotspot:"Hotspot"},Me={security:"Security",reliability:"Reliability",maintainability:"Maintainability",testability:"Testability"};function T(e,t){return`<span class="icon-${i(e)}" role="img" aria-label="${i(t)}"></span>`}async function Ae(){try{let e=await fetch("/report.json");if(!e.ok)throw new Error(`HTTP ${e.status}`);u=await e.json(),b=u.issues??[],Fe(),qe(),Ne(),Ke(),Ue(),ze(),Je(),We(),De(),Ge(),W(),Q(),L.length&&ge(q||L[0].path),st(),nt(),S(),lt(),fe(),at(),ft(),Se(),o("tab-issue-count").textContent=String(b.length),o("tab-coverage-count").textContent=y(u.measures.coverage??u.test_signals?.summary?.coverage),o("tab-mutant-count").textContent=String(me().survived)}catch(e){o("app").innerHTML=`<div class="error">Failed to load report: ${String(e)}</div>`}}document.addEventListener("DOMContentLoaded",Ae);function Fe(){let e=u.metadata,t=new Date(e.analysis_date).toLocaleString();o("project-key").textContent=e.project_key,o("scan-date").textContent=t,o("scan-version").textContent=`v${e.version}`,o("elapsed").textContent=`${e.elapsed_ms}ms`}function je(){let e=u.measures,t=u.test_signals?.summary,s=[{metric:"Bugs",operator:"=",threshold:0,value:e.bugs,passed:e.bugs===0},{metric:"Vulnerabilities",operator:"=",threshold:0,value:e.vulnerabilities,passed:e.vulnerabilities===0},{metric:"Code Smells",operator:"\u2264",threshold:10,value:e.code_smells,passed:e.code_smells<=10,severity:e.code_smells<=10?void 0:e.code_smells<=20?"warning":void 0}];return e.coverage!=null?s.push({metric:"Coverage",operator:"\u2265",threshold:70,value:e.coverage,passed:e.coverage>=70,severity:e.coverage>=70?void 0:e.coverage>=60?"warning":void 0}):s.push({metric:"Coverage",operator:"\u2265",threshold:70,value:0,passed:!1,severity:"missing"}),t&&(t.tests!=null&&s.push({metric:"Test Failures",operator:"=",threshold:0,value:t.test_failures??0,passed:(t.test_failures??0)===0}),t.mutation_score!=null&&s.push({metric:"Mutation Score",operator:"\u2265",threshold:60,value:t.mutation_score,passed:t.mutation_score>=60,severity:t.mutation_score>=60?void 0:t.mutation_score>=40?"warning":void 0}),t.changed_mutation_score!=null&&s.push({metric:"Changed Mutation",operator:"\u2265",threshold:60,value:t.changed_mutation_score,passed:t.changed_mutation_score>=60,severity:t.changed_mutation_score>=60?void 0:t.changed_mutation_score>=40?"warning":void 0})),{status:s.filter(l=>!l.passed&&l.severity!=="warning"&&l.severity!=="missing").length===0?"passed":"failed",conditions:s}}function qe(){let e=je(),t=o("gate-hero");t.classList.remove("gate-loading"),t.classList.add(e.status==="passed"?"gate-passed":"gate-failed");let s=o("gate-icon");if(s.className=`gate-icon icon-${e.status==="passed"?"pass":"fail"}`,s.setAttribute("aria-label",e.status==="passed"?"Passed":"Failed"),o("gate-status").textContent=e.status==="passed"?"Passed":"Failed",e.status==="passed"){let n=e.conditions.filter(c=>!c.passed&&c.severity!=="warning");e.conditions.filter(c=>!c.passed&&c.severity==="warning").length&&!n.length&&(o("gate-status").textContent="Passed with warnings",t.classList.add("gate-warn"))}let a=e.conditions.map(n=>{let l=n.passed?"cond-pass":n.severity==="warning"?"cond-warn":"cond-fail",c=n.passed?T("pass","Passed"):T("fail","Failed");return`<div class="gate-cond ${l}">
      <span class="gate-cond-icon">${c}</span>
      <span class="gate-cond-metric">${i(n.metric)} ${i(n.operator)} ${n.threshold}</span>
      <span class="gate-cond-value">${n.value}</span>
    </div>`}).join("");o("gate-conditions").innerHTML=a}function Ne(){let e=u.measures,t=u.test_signals?.summary;h("m-bugs",e.bugs),h("m-vulns",e.vulnerabilities),h("m-smells",e.code_smells),G("m-coverage",y(e.coverage)),h("m-ncloc",e.ncloc),h("m-files",e.files),h("m-comments",e.comments),t?(h("m-tests",t.tests??0),h("m-test-failures",t.test_failures??0),h("m-test-skipped",t.test_skipped??0),h("m-mutants-skipped",t.mutants_skipped??e.mutants_skipped??0),h("m-mutants-error",t.mutants_error??e.mutants_error??0)):(h("m-tests",e.tests??0),h("m-test-failures",e.test_failures??0),h("m-test-skipped",e.test_skipped??0),h("m-mutants-skipped",e.mutants_skipped??0),h("m-mutants-error",e.mutants_error??0)),D("card-bugs",e.bugs,[0,1,5]),D("card-vulns",e.vulnerabilities,[0,1,3]),D("card-smells",e.code_smells,[0,10,50]),Re("card-coverage",e.coverage),Oe("card-tests",t?.tests??e.tests,[50,20,0]),D("card-test-failures",t?.test_failures??e.test_failures??0,[0,1,5]),f("card-ncloc","card-neutral"),f("card-files","card-neutral"),f("card-comments","card-neutral"),f("card-test-skipped","card-neutral"),f("card-mutants-skipped","card-neutral"),f("card-mutants-error",(t?.mutants_error??e.mutants_error??0)>0?"card-red":"card-neutral");let s=e.duplicated_lines_density;G("m-duplication",y(s)),D("card-duplication",s??0,[3,10,20]),f("card-duplication",s==null?"card-neutral":"");let a=u.test_signals?.health;if(a){G("m-test-health",`${a.status} \xB7 ${a.score}`);let l=o("card-test-health");l.classList.remove("card-neutral","card-green","card-yellow","card-red"),a.status==="healthy"?l.classList.add("card-green"):a.status==="at_risk"?l.classList.add("card-red"):a.status==="partial"?l.classList.add("card-yellow"):l.classList.add("card-neutral")}else G("m-test-health","\u2014"),f("card-test-health","card-neutral");let n=o("card-coverage");n.classList.add("clickable"),n.addEventListener("click",()=>{j("coverage")})}function h(e,t){o(e).textContent=t.toLocaleString()}function G(e,t){o(e).textContent=t}function y(e){return e==null?"\u2014":`${e.toFixed(1)}%`}function D(e,t,s){t<=s[0]?f(e,"card-green"):t<=s[1]?f(e,"card-yellow"):f(e,"card-red")}function Re(e,t){t==null?f(e,"card-neutral"):t>=80?f(e,"card-green"):t>=60?f(e,"card-yellow"):f(e,"card-red")}function Oe(e,t,s){if(t==null){f(e,"card-neutral");return}t>=s[0]?f(e,"card-green"):t>=s[1]?f(e,"card-yellow"):f(e,"card-red")}function De(){let e=o("mutation-summary"),t=me();if(!t.hasSignal){e.innerHTML='<div class="empty-state compact">No mutation report was collected for this scan. Add a supported report such as <span class="mono">ollanta-mutations.json</span>, Stryker JSON, or PIT XML to see mutation score and survived mutants.</div>';return}e.innerHTML=`<div class="mutation-kpis">
      <div><span class="mutation-kpi-value ${ae(t.score)}">${y(t.score)}</span><span class="mutation-kpi-label">mutation score</span></div>
      <div><span class="mutation-kpi-value">${t.killed.toLocaleString()}/${t.total.toLocaleString()}</span><span class="mutation-kpi-label">killed mutants</span></div>
      <div><span class="mutation-kpi-value ${t.survived>0?"mut-warning":"mut-success"}">${t.survived.toLocaleString()}</span><span class="mutation-kpi-label">survived mutants</span></div>
    </div>
    ${Be(t.modules)}
    ${Ve(t.survivedMutants)}`}function me(){let e=u.test_signals?.summary,t=(u.test_signals?.modules??[]).filter(p=>p.mutation),s=e?.changed_mutants_total||e?.mutants_total||u.measures.changed_mutants_total||u.measures.mutants_total||0,a=e?.changed_mutants_killed||e?.mutants_killed||u.measures.changed_mutants_killed||u.measures.mutants_killed||0,n=e?.changed_mutants_survived||e?.mutants_survived||u.measures.changed_mutants_survived||u.measures.mutants_survived||0,l=e?.changed_mutation_score??e?.mutation_score??u.measures.changed_mutation_score??u.measures.mutation_score,c=t.flatMap(p=>p.mutation?.survived_mutants??[]).slice(0,8);return{hasSignal:t.length>0||s>0||l!=null,score:l,total:s,killed:a,survived:n,modules:t,survivedMutants:c}}function Be(e){return e.length?`<div class="mutation-module-list">
    ${e.slice(0,5).map(t=>{let s=t.mutation,a=s.changed_code_score??s.score,n=s.changed_survived??s.survived??0,l=s.changed_total??s.total??0;return`<div class="mutation-module-row">
        <span class="mutation-module-main"><span class="mutation-module-name">${i(t.name||t.root)}</span><span class="mutation-module-meta">${i(s.tool||"mutation")} \xB7 ${l.toLocaleString()} mutants</span></span>
        <span class="mutation-pill ${ae(a)}">${y(a)}</span>
        <span class="mutation-survived ${n>0?"mut-warning":"mut-success"}">${n.toLocaleString()} survived</span>
      </div>`}).join("")}
  </div>`:""}function Ve(e){return e.length?`<div class="mutation-survivors">
    ${e.map(t=>`<div class="mutation-survivor-row">
      <span class="mutation-survivor-file">${i(H(t.file||""))}${t.line?`:L${t.line}`:""}</span>
      <span class="mutation-survivor-meta">${i(t.mutator||t.description||"survived mutant")}</span>
    </div>`).join("")}
  </div>`:""}function ae(e){return e==null?"card-neutral":e>=80?"card-green":e>=60?"card-yellow":"card-red"}function Q(){let e=o("mutants-page"),t=me();if(!t.hasSignal){e.innerHTML='<div class="empty-state">No mutation data collected. Run with <span class="mono">-with-mutations</span> to see survived mutants.</div>';return}let s=t.modules.flatMap(d=>(d.mutation?.survived_mutants??[]).map(g=>({...g,moduleName:d.name||d.root}))),a=[...new Set(s.map(d=>d.moduleName))].sort(),n=s;z!=="all"&&(n=n.filter(d=>d.moduleName===z)),n.sort((d,g)=>{let v=0;return E==="file"?v=(d.file||"").localeCompare(g.file||""):E==="line"?v=(d.line??0)-(g.line??0):E==="module"&&(v=d.moduleName.localeCompare(g.moduleName)),J==="asc"?v:-v});let l=`
    <div class="mutants-toolbar">
      <div class="toolbar-left">
        <select id="mutant-filter-module">
          <option value="all">All modules</option>
          ${a.map(d=>`<option value="${i(d)}"${d===z?" selected":""}>${i(d)}</option>`).join("")}
        </select>
        <select id="mutant-sort-field">
          <option value="file"${E==="file"?" selected":""}>Sort by file</option>
          <option value="line"${E==="line"?" selected":""}>Sort by line</option>
          <option value="module"${E==="module"?" selected":""}>Sort by module</option>
        </select>
        <select id="mutant-sort-dir">
          <option value="asc"${J==="asc"?" selected":""}>Ascending</option>
          <option value="desc"${J==="desc"?" selected":""}>Descending</option>
        </select>
      </div>
      <div class="toolbar-right">
        <span class="result-count">${n.length.toLocaleString()} survived</span>
      </div>
    </div>
  `,c=`
    <div class="mutation-kpis">
      <div><span class="mutation-kpi-value ${ae(t.score)}">${y(t.score)}</span><span class="mutation-kpi-label">mutation score</span></div>
      <div><span class="mutation-kpi-value">${t.killed.toLocaleString()}/${t.total.toLocaleString()}</span><span class="mutation-kpi-label">killed mutants</span></div>
      <div><span class="mutation-kpi-value ${t.survived>0?"mut-warning":"mut-success"}">${t.survived.toLocaleString()}</span><span class="mutation-kpi-label">survived mutants</span></div>
    </div>
  `,p="";n.length?p=`
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
          ${n.map(d=>`
            <tr class="mutant-row">
              <td class="mutant-file">${i(H(d.file||""))}</td>
              <td class="mutant-line">${d.line??"\u2014"}</td>
              <td class="mutant-module">${i(d.moduleName)}</td>
              <td class="mutant-mutator">${i(d.mutator||"\u2014")}</td>
              <td class="mutant-desc">${i(d.description||d.replacement||"survived mutant")}</td>
            </tr>
          `).join("")}
        </tbody>
      </table>
    `:p='<div class="empty-state compact">No survived mutants match the current filter.</div>',e.innerHTML=l+c+p,Se()}function Se(){let e=document.getElementById("mutant-filter-module"),t=document.getElementById("mutant-sort-field"),s=document.getElementById("mutant-sort-dir");e?.addEventListener("change",a=>{z=a.target.value,Q()}),t?.addEventListener("change",a=>{E=a.target.value,Q()}),s?.addEventListener("change",a=>{J=a.target.value,Q()})}function Ge(){let e=u.test_signals?.modules??[];if(!e.length)return;let t=o("ts-modules"),s="";for(let l of e){let c=l.health,p=l.coverage,d=l.mutation,g=l.suites??[],v=g.reduce((O,V)=>O+(V.failures??0)+(V.errors??0),0),m=g.reduce((O,V)=>O+(V.tests??0),0),_=l.architecture_role??"",A=_?`<span class="ts-role-badge">${i(_)}</span>`:"",ie=Le(c?.status),Te=y(p?.coverage),Ee=N(p?.coverage??void 0),Ce=y(d?.changed_code_score??d?.score),Ie=ae(d?.changed_code_score??d?.score);s+=`<div class="ts-module">
      <div class="ts-module-head">
        <span class="ts-module-name">${i(l.name)}</span>
        ${A}
        <span class="ts-health-badge ${ie}">${c?.status??"no data"}</span>
      </div>
      <div class="ts-module-meta">
        ${p?`<span class="ts-metric ${Ee}">Coverage ${Te}</span>`:""}
        ${d?.score!=null||d?.changed_code_score!=null?`<span class="ts-metric ${Ie}">Mutation ${Ce}</span>`:""}
        ${m>0?`<span class="ts-metric">${m} test${m===1?"":"s"}</span>`:""}
        ${v>0?`<span class="ts-metric ts-fail">${v} failed</span>`:""}
      </div>
      ${c?.recommendations?.length?`<div class="ts-recommendations">${c.recommendations.map(O=>`<div class="ts-rec">${i(O)}</div>`).join("")}</div>`:""}
    </div>`}let a=u.test_signals?.health,n=a?`<span class="ts-health-badge ${Le(a.status)}">${a.status} \xB7 score ${a.score}</span>`:"";t.innerHTML=`<div class="ts-header">
      <h3>Test Signals</h3>
      ${n}
    </div>
    <div class="ts-module-list">${s||'<div class="empty-state compact">No module-level test data was collected.</div>'}</div>`}function Le(e){return e==="healthy"?"card-green":e==="at_risk"?"card-red":e==="partial"?"card-yellow":"card-neutral"}function Ke(){let e=ee(b,v=>v.severity),t=Math.max(1,...Object.values(e)),s=["blocker","critical","major","minor","info"],a="",n="",l=b.length||1;for(let v of s){let m=e[v]??0,_=m/t*100,A=R[v]??"#64748b";a+=`<div class="sev-bar-row">
      <span class="sev-bar-label">${v}</span>
      <div class="sev-bar-track"><div class="sev-bar-fill" style="width:${_}%;background:${A}"></div></div>
      <span class="sev-bar-count">${m}</span>
    </div>`,m>0&&(n+=`<div class="sev-segment" style="width:${m/l*100}%;background:${A}" title="${v}: ${m}"></div>`)}o("sev-bars").innerHTML=a,o("sev-proportional").innerHTML=n;let c=ee(b,v=>v.type),p=Math.max(1,...Object.values(c)),d={bug:"#ef4444",vulnerability:"#f97316",code_smell:"#22c55e",security_hotspot:"#eab308"},g="";for(let[v,m]of Object.entries(se)){let _=c[v]??0,A=_/p*100,ie=d[v]??"#64748b";g+=`<div class="sev-bar-row">
      <span class="sev-bar-label">${m}</span>
      <div class="sev-bar-track"><div class="sev-bar-fill" style="width:${A}%;background:${ie}"></div></div>
      <span class="sev-bar-count">${_}</span>
    </div>`}o("type-bars").innerHTML=g}function Ue(){let e=[...b].sort((t,s)=>{let a=(Y[t.severity]??99)-(Y[s.severity]??99);return a!==0?a:t.component_path.localeCompare(s.component_path)||t.line-s.line}).slice(0,6);if(!e.length){o("priority-issues").innerHTML='<div class="empty-state compact">No issues found</div>';return}o("priority-issues").innerHTML=e.map((t,s)=>{let a=R[t.severity]??"#64748b",n=H(t.component_path);return`<button class="priority-row" data-idx="${s}">
      <span class="issue-sev-dot" style="background:${a}"></span>
      <span class="priority-main">
        <span class="priority-title">${i(t.message)}</span>
        <span class="priority-meta" title="${i(t.component_path)}">${i(n)}:L${t.line} \xB7 ${i(t.rule_key)}</span>
      </span>
      <span class="priority-severity">${i(t.severity)}</span>
    </button>`}).join(""),o("priority-issues").querySelectorAll(".priority-row").forEach(t=>{t.addEventListener("click",()=>{let s=Number.parseInt(t.dataset.idx,10);he(e[s])})})}function ze(){let e=ee(b,s=>s.component_path),t=Object.entries(e).sort((s,a)=>a[1]-s[1]).slice(0,10);if(!t.length){o("hotspot-files").innerHTML='<div class="empty-state">No issues found</div>';return}o("hotspot-files").innerHTML=t.map(([s,a])=>{let n=H(s);return`<div class="hotspot-row" data-path="${i(s)}">
      <span class="hotspot-file" title="${i(s)}">${i(n)}</span>
      <span class="hotspot-count">${a}</span>
    </div>`}).join(""),o("hotspot-files").querySelectorAll(".hotspot-row").forEach(s=>{s.addEventListener("click",()=>{let a=s.dataset.path;F="grouped",j("issues"),ot(a)})})}function Je(){L=(u.test_signals?.modules??[]).flatMap(t=>(t.files??[]).map(s=>Qe(t.name,t.root,s))).filter(t=>t.linesToCover>0).sort((t,s)=>(t.coverage??101)-(s.coverage??101)||s.uncoveredLines.length-t.uncoveredLines.length||t.path.localeCompare(s.path)),!q&&L.length&&(q=L[0].path)}function Qe(e,t,s){let a=s.lines_to_cover??0,n=s.covered_lines??0,l=a>0?n*100/a:null;return{moduleName:e,moduleRoot:t,path:s.path,linesToCover:a,coveredLines:n,coveredLineNumbers:s.covered_line_numbers??[],uncoveredLines:s.uncovered_lines??[],coverage:l}}function We(){let e=o("coverage-summary");if(!L.length){e.innerHTML='<div class="empty-state compact">Run with <span class="mono">-with-tests</span> and provide a coverage report to see file-level details.</div>';return}let t=u.test_signals?.summary,s=L.slice(0,5);e.innerHTML=`<div class="coverage-kpis">
      <div><span class="coverage-kpi-value">${y(t?.coverage??u.measures.coverage)}</span><span class="coverage-kpi-label">overall</span></div>
      <div><span class="coverage-kpi-value">${(t?.covered_lines??0).toLocaleString()}/${(t?.lines_to_cover??0).toLocaleString()}</span><span class="coverage-kpi-label">covered lines</span></div>
      <div><span class="coverage-kpi-value">${(t?.modules_with_coverage??0).toLocaleString()}</span><span class="coverage-kpi-label">modules</span></div>
    </div>
    <div class="coverage-file-list">
      ${s.map(Xe).join("")}
    </div>`,e.querySelectorAll(".coverage-mini-row").forEach(a=>{a.addEventListener("click",()=>{let n=a.dataset.coveragePath;n&&(j("coverage"),ge(n))})})}function Xe(e){return`<button class="coverage-mini-row" data-coverage-path="${i(e.path)}">
    <span class="coverage-mini-main">
      <span class="coverage-file-name" title="${i(e.path)}">${i(H(e.path))}</span>
      <span class="coverage-file-meta">${i(e.moduleName)} \xB7 ${e.uncoveredLines.length.toLocaleString()} uncovered lines</span>
    </span>
    <span class="coverage-pill ${N(e.coverage)}">${y(e.coverage??void 0)}</span>
  </button>`}function W(){let e=o("coverage-details");if(!L.length){e.innerHTML='<div class="empty-state">No file-level coverage was collected for this scan.</div>';return}e.innerHTML=`<div class="coverage-toolbar">
      <div>
        <h3>Coverage Files</h3>
        <p>${L.length.toLocaleString()} files with line-level detail. Overall coverage includes all measured files, not only those listed here.</p>
      </div>
      <span class="coverage-pill ${N(u.measures.coverage??null)}">${y(u.measures.coverage)}</span>
    </div>
    <div class="coverage-browser">
      <aside class="coverage-sidebar">
        <div class="coverage-file-list coverage-file-list-large">
          ${L.map(Ye).join("")}
        </div>
      </aside>
      <section class="coverage-code-viewer">
        ${Ze()}
      </section>
    </div>`,e.querySelectorAll(".coverage-row").forEach(t=>{t.addEventListener("click",()=>{let s=t.dataset.coveragePath;s&&ge(s)})})}function Ye(e){return`<button class="coverage-row${e.path===q?" active":""}" data-coverage-path="${i(e.path)}">
    <div class="coverage-row-main">
      <div class="coverage-row-title" title="${i(e.path)}">${i(e.path)}</div>
      <div class="coverage-row-subtitle">${i(e.moduleName)} \xB7 ${i(e.moduleRoot)} \xB7 ${e.coveredLines.toLocaleString()}/${e.linesToCover.toLocaleString()} lines covered</div>
    </div>
    <div class="coverage-row-meter">
      <span class="coverage-pill ${N(e.coverage)}">${y(e.coverage??void 0)}</span>
      <div class="coverage-track"><div class="coverage-fill ${N(e.coverage)}" style="width:${e.coverage??0}%"></div></div>
    </div>
  </button>`}async function ge(e){if(L.some(t=>t.path===e)){if(q=e,X="",le.has(e)){K=!1,W();return}K=!0,W();try{let t=await fetch(`/api/files/source?path=${encodeURIComponent(e)}`);if(!t.ok)throw new Error(`HTTP ${t.status}`);let s=await t.json();le.set(e,s.file)}catch(t){X=`Could not load source for ${e}: ${String(t)}`}finally{K=!1,W()}}}function Ze(){let e=L.find(p=>p.path===q);if(!e)return'<div class="code-empty"><p>Select a file to inspect coverage.</p></div>';if(K)return'<div class="code-empty"><div class="spinner"></div></div>';if(X)return`<div class="code-empty"><p>${i(X)}</p></div>`;let t=le.get(e.path);if(!t)return'<div class="code-empty"><p>Select a coverage file to inspect source lines.</p></div>';let s=new Set(e.coveredLineNumbers),a=new Set(e.uncoveredLines),l=t.content.split(`
`).map((p,d)=>{let g=d+1,v=a.has(g),m=!v&&s.has(g),_=et(m,v);return`<div class="code-line${_.stateClass}">
      <span class="code-gutter">${g}</span>
      <code class="code-text">${p.length?i(p):"&nbsp;"}</code>
      <span class="code-markers">${tt(_)}</span>
    </div>`}).join(""),c=e.coveredLineNumbers.length?"covered and uncovered lines":"uncovered lines only";return`<div class="code-viewer-shell coverage-source-shell">
    <div class="code-viewer-head">
      <div>
        <div class="code-viewer-path mono">${i(t.path)}</div>
        <div class="code-viewer-meta">${i(t.language||"plain text")} \xB7 ${t.line_count.toLocaleString()} lines \xB7 ${c}</div>
      </div>
      <div class="code-viewer-stats"><span class="coverage-pill ${N(e.coverage)}">${y(e.coverage??void 0)}</span></div>
    </div>
    <div class="coverage-legend">
      <span><span class="legend-dot legend-covered"></span>Covered</span>
      <span><span class="legend-dot legend-uncovered"></span>Not covered</span>
    </div>
    <div class="code-surface">${l}</div>
  </div>`}function et(e,t){return t?{stateClass:" is-uncovered",marker:"not covered",chipClass:" chip-uncovered"}:e?{stateClass:" is-covered",marker:"covered",chipClass:" chip-covered"}:{stateClass:"",marker:"",chipClass:""}}function tt(e){return e.marker?`<span class="coverage-line-chip${e.chipClass}">${e.marker}</span>`:""}function N(e){return e==null?"card-neutral":e>=80?"card-green":e>=60?"card-yellow":"card-red"}function st(){let e=Object.entries(u.measures.by_language).sort((s,a)=>a[1]-s[1]),t=Math.max(1,e[0]?.[1]??1);if(!e.length){o("by-lang").innerHTML='<span class="empty-state">No language data</span>';return}o("by-lang").innerHTML=e.map(([s,a])=>`<div class="lang-row">
      <span class="lang-name">${i(s)}</span>
      <div class="lang-bar-track"><div class="lang-bar-fill" style="width:${a/t*100}%"></div></div>
      <span class="lang-count">${a} files</span>
    </div>`).join("")}function at(){document.querySelectorAll(".tab").forEach(s=>{s.addEventListener("click",()=>{let a=s.dataset.tab;j(a)})});let e=document.getElementById("view-mode-toggle");e&&e.addEventListener("click",()=>{F=F==="flat"?"grouped":"flat",e.textContent=F==="grouped"?"List view":"Group by file",e.setAttribute("aria-pressed",String(F==="grouped")),S()}),document.querySelector(".tabs").addEventListener("keydown",s=>{let a=Array.from(document.querySelectorAll(".tab[role='tab']")),n=a.findIndex(l=>l.getAttribute("aria-selected")==="true");if(s.key==="ArrowRight"){s.preventDefault();let l=a[(n+1)%a.length];l.focus(),j(l.dataset.tab)}else if(s.key==="ArrowLeft"){s.preventDefault();let l=a[(n-1+a.length)%a.length];l.focus(),j(l.dataset.tab)}})}function j(e){_e=e,document.querySelectorAll(".tab").forEach(s=>{s.classList.remove("active"),s.setAttribute("aria-selected","false")});let t=document.querySelector(`.tab[data-tab="${e}"]`);t?.classList.add("active"),t?.setAttribute("aria-selected","true"),document.querySelectorAll(".panel").forEach(s=>s.classList.add("hidden")),o(`panel-${e}`).classList.remove("hidden")}function nt(){let e=[...new Set(b.map(n=>n.rule_key))].sort((n,l)=>n.localeCompare(l)),t=o("filter-rule");e.forEach(n=>{let l=document.createElement("option");l.value=n,l.textContent=n,t.appendChild(l)});let s=new Set;for(let n of b)for(let l of n.tags??[])s.add(l);let a=o("filter-tag");[...s].sort().forEach(n=>{let l=document.createElement("option");l.value=n,l.textContent=n,a.appendChild(l)}),o("filter-severity").addEventListener("change",n=>{P=n.target.value,S()}),o("filter-type").addEventListener("change",n=>{oe=n.target.value,S()}),t.addEventListener("change",n=>{re=n.target.value,S()}),o("filter-quality").addEventListener("change",n=>{de=n.target.value,S()}),a.addEventListener("change",n=>{ce=n.target.value,S()}),o("search").addEventListener("input",n=>{ue=n.target.value.toLowerCase(),S()}),ke()}function ke(){let e=ee(b,s=>s.severity),t=["blocker","critical","major","minor","info"];o("sev-chips").innerHTML=t.map(s=>{let a=e[s]??0,n=R[s]??"#64748b",l=P===s?" active":"";return`<button class="sev-chip${l}" data-sev="${s}"
      style="--chip-color:${n};--chip-bg:${n}15" aria-pressed="${l?"true":"false"}">
      <span class="chip-dot" style="background:${n}"></span>
      ${s}
      <span class="chip-count">${a}</span>
    </button>`}).join(""),o("sev-chips").querySelectorAll(".sev-chip").forEach(s=>{s.addEventListener("click",()=>{let a=s.dataset.sev;P=P===a?"all":a,o("filter-severity").value=P,S(),ke()})})}function S(){k=b.filter(s=>!(P!=="all"&&s.severity!==P||oe!=="all"&&s.type!==oe||re!=="all"&&s.rule_key!==re||de!=="all"&&s.quality!==de||ce!=="all"&&!(s.tags??[]).includes(ce)||ue&&!`${s.component_path} ${s.message} ${s.rule_key}`.toLowerCase().includes(ue))),k.sort((s,a)=>{let n=Y[s.severity]??99,l=Y[a.severity]??99;return n-l}),C=-1,it();let e=k.length,t=document.getElementById("filter-announcer");t&&(t.textContent=`${e} issue${e===1?"":"s"} match the current filters`)}function it(){let e=o("issue-list"),t=o("issue-grouped"),s=k.length===1?"issue":"issues";if(o("issue-count").textContent=`${k.length} ${s}`,!k.length){e.innerHTML='<div class="empty-state">No issues match the current filters.</div>',t.innerHTML='<div class="empty-state">No issues match the current filters.</div>';return}if(F==="grouped"){e.classList.add("hidden"),t.classList.remove("hidden"),fe();return}e.classList.remove("hidden"),t.classList.add("hidden"),e.innerHTML=k.map((a,n)=>{let l=R[a.severity]??"#64748b",c=H(a.component_path),p=a.end_line&&a.end_line!==a.line?`L${a.line}\u2013${a.end_line}`:`L${a.line}`,d=se[a.type]??a.type,g=a.quality?`<span class="quality-badge quality-${i(a.quality)}">${i(Me[a.quality]??a.quality)}</span>`:"";return`<div class="issue-row" role="button" tabindex="0" aria-label="${i(a.severity)} issue: ${i(a.message)}" data-idx="${n}">
      <span class="issue-sev">
        <span class="issue-sev-dot" style="background:${l}"></span>
        ${i(a.severity)}
      </span>
      <span class="issue-type">${i(d)}</span>
      <div class="issue-main">
        <span class="issue-msg">${i(a.message)}</span>
        <span class="issue-file" title="${i(a.component_path)}">${i(c)}:${p}</span>
      </div>
      ${g}
      <span class="issue-rule">${i(a.rule_key)}</span>
    </div>`}).join(""),e.querySelectorAll(".issue-row").forEach(a=>{a.addEventListener("click",()=>{let n=Number.parseInt(a.dataset.idx,10);Z(n)}),a.addEventListener("keydown",n=>{let l=n;if(l.key==="Enter"||l.key===" "){l.preventDefault();let c=Number.parseInt(a.dataset.idx,10);Z(c)}})})}function lt(){let e=new Map;for(let t of b){let s=t.component_path;e.has(s)||e.set(s,[]),e.get(s).push(t)}M=[...e.entries()].sort((t,s)=>s[1].length-t[1].length).map(([t,s])=>({path:t,shortPath:H(t),issues:[...s].sort((a,n)=>a.line-n.line),expanded:!1}))}function fe(){let e=o("issue-grouped");if(!M.length){e.innerHTML='<div class="empty-state">No issues found</div>';return}e.innerHTML=M.map((t,s)=>`<div class="file-group${t.expanded?" expanded":""}" data-gi="${s}">
      <div class="file-group-header">
        <span class="file-group-chevron icon-chevron" role="img" aria-label="Expand"></span>
        <span class="file-group-name" title="${i(t.path)}">${i(t.shortPath)}</span>
        <span class="file-group-count">${t.issues.length}</span>
      </div>
      <div class="file-group-issues" style="${t.expanded?"":"display:none"}">
        ${t.issues.map((a,n)=>{let l=R[a.severity]??"#64748b";return`<div class="file-issue" data-gi="${s}" data-ii="${n}">
            <span class="issue-sev">
              <span class="issue-sev-dot" style="background:${l}"></span>
              ${i(a.severity)}
            </span>
            <span class="issue-msg">${i(a.message)}</span>
            <span class="file-issue-line">L${a.line}</span>
          </div>`}).join("")}
      </div>
    </div>`).join(""),e.querySelectorAll(".file-group-header").forEach(t=>{t.addEventListener("click",()=>{let s=t.closest(".file-group"),a=Number.parseInt(s.dataset.gi,10);M[a].expanded=!M[a].expanded,s.classList.toggle("expanded");let n=s.querySelector(".file-group-issues");n.style.display=M[a].expanded?"":"none"})}),e.querySelectorAll(".file-issue").forEach(t=>{t.addEventListener("click",s=>{s.stopPropagation();let a=Number.parseInt(t.dataset.gi,10),n=Number.parseInt(t.dataset.ii,10),l=M[a].issues[n];he(l)})})}function ot(e){let t=M.findIndex(a=>a.path===e);if(t<0)return;M[t].expanded=!0,fe(),document.querySelector(`.file-group[data-gi="${t}"]`)?.scrollIntoView({behavior:"smooth",block:"start"})}function Z(e,t=!0){C=e,x=k[e]??null,document.querySelectorAll(".issue-row").forEach(a=>a.classList.remove("selected"));let s=document.querySelector(`.issue-row[data-idx="${e}"]`);s?.classList.add("selected"),s?.focus(),t&&x&&he(x)}function he(e){U=document.activeElement,x=e,I="details",B="",te=!0,r=ye(),o("detail-title").textContent=e.message||e.rule_key,ne(e),o("detail-panel").classList.add("open"),o("detail-overlay").classList.add("open"),o("detail-panel").querySelector("button, [href], input, select, textarea, [tabindex]:not([tabindex='-1'])")?.focus(),rt(e.rule_key)}async function rt(e){try{let t=await fetch(`/rules/${encodeURIComponent(e)}`);if(!t.ok)throw new Error("not found");let s=await t.json(),a="";s.rationale&&(a+=`<div class="detail-section">
        <div class="detail-section-title">Why is this a problem?</div>
        <div class="rule-rationale">${i(s.rationale)}</div>
      </div>`),s.description&&s.description!==s.rationale&&(a+=`<div class="detail-section">
        <div class="detail-section-title">Description</div>
        <div class="rule-rationale">${i(s.description)}</div>
      </div>`),s.noncompliant_code&&(a+=`<div class="detail-section">
        <div class="detail-section-title">${T("cross","Noncompliant")} Noncompliant Code</div>
        <pre class="rule-code noncompliant"><code>${i(s.noncompliant_code)}</code></pre>
      </div>`),s.compliant_code&&(a+=`<div class="detail-section">
        <div class="detail-section-title">${T("check","Compliant")} Compliant Code</div>
        <pre class="rule-code compliant"><code>${i(s.compliant_code)}</code></pre>
      </div>`),B=a||'<div class="detail-empty">No additional rule details available.</div>'}catch{B='<div class="detail-empty">Rule details are not available for this issue.</div>'}finally{te=!1,x?.rule_key===e&&ne(x)}}function dt(e){document.getElementById("detailCopy")?.addEventListener("click",()=>{ct(e)})}async function ct(e){let t=[];t.push(`Issue: ${e.message||""}`),t.push(`Severity: ${e.severity}`),t.push(`Type: ${se[e.type]??e.type}`),t.push(`Rule: ${e.rule_key}`),e.engine_id&&t.push(`Engine: ${e.engine_id}`),t.push(`File: ${e.component_path}`);let s=e.end_line&&e.end_line!==e.line?`lines ${e.line}\u2013${e.end_line}`:`line ${e.line}`;t.push(`Location: ${s}${e.column?", column "+e.column:""}`),t.push(`Status: ${e.status}`),e.tags?.length&&t.push(`Tags: ${e.tags.join(", ")}`);try{let a=await fetch(`/rules/${encodeURIComponent(e.rule_key)}`);if(a.ok){let n=await a.json();n.rationale&&t.push(`
Why is this a problem?
${n.rationale}`),n.noncompliant_code&&t.push(`
Noncompliant code:
${n.noncompliant_code}`),n.compliant_code&&t.push(`
Compliant code:
${n.compliant_code}`)}}catch{}try{await navigator.clipboard.writeText(t.join(`
`));let a=document.getElementById("detailCopy");a&&(a.innerHTML=`${T("check","Copied")} Copied`,setTimeout(()=>{a.innerHTML=`${T("copy","Copy")} Copy`},2e3))}catch{let a=document.getElementById("detailCopy");a&&(a.innerHTML=`${T("warn","Failed")} Failed`,setTimeout(()=>{a.innerHTML=`${T("copy","Copy")} Copy`},2e3))}}function ve(){o("detail-panel").classList.remove("open"),o("detail-overlay").classList.remove("open"),x=null,B="",te=!1,r=ye(),document.querySelectorAll(".issue-row").forEach(e=>e.classList.remove("selected")),U&&(U.focus(),U=null)}function ne(e){let t=`
    <div class="detail-tabs">
      ${$e(I)}
    </div>
    <div class="detail-tab-panel${I==="details"?"":" hidden"}" data-detail-panel="details">
      ${ut(e)}
    </div>
    <div class="detail-tab-panel${I==="rule"?"":" hidden"}" data-detail-panel="rule">
      ${te?'<div class="detail-loading">Loading rule details\u2026</div>':B}
    </div>
    <div class="detail-tab-panel${I==="ai-fix"?"":" hidden"}" data-detail-panel="ai-fix">
      ${be(e,r,$??[])}
    </div>
  `;o("detail-body").innerHTML=t,vt(e),dt(e)}function ut(e){let t=R[e.severity]??"#64748b",s=se[e.type]??e.type,a=e.end_line&&e.end_line!==e.line?`${e.line}:${e.column} \u2013 ${e.end_line}:${e.end_column}`:`${e.line}:${e.column}`,n=`
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
      ${e.quality?`<div class="detail-field">
        <span class="detail-field-label">Quality</span>
        <span class="detail-field-value"><span class="quality-badge quality-${i(e.quality)}">${i(Me[e.quality]??e.quality)}</span></span>
      </div>`:""}
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
        <span class="detail-field-value">${e.tags.map(l=>i(l)).join(", ")}</span>
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
        ${e.secondary_locations.map(l=>`
          <div class="detail-loc-item">
            <div class="detail-loc-file">${i(l.file_path||e.component_path)}:${l.start_line}</div>
            ${l.message?`<div class="detail-loc-msg">${i(l.message)}</div>`:""}
          </div>
        `).join("")}
      </div>
    </div>`),n}function vt(e){document.querySelectorAll(".detail-tab").forEach(n=>{n.addEventListener("click",()=>{I=n.dataset.detailTab??"details",ne(e),I==="ai-fix"&&pt()})});let t=document.getElementById("ai-provider-select");t?.addEventListener("change",()=>{r.selectedProviderId=t.value,r.selectedModel="",pe(),r.preview=null,r.statusMessage="",r.errorMessage="",w()});let s=document.getElementById("ai-model-input");s?.addEventListener("input",()=>{r.selectedModel=s.value});let a=document.getElementById("ai-api-key-input");a?.addEventListener("input",()=>{r.apiKey=a.value}),document.getElementById("ai-generate-fix")?.addEventListener("click",()=>{mt(e)}),document.getElementById("ai-apply-fix")?.addEventListener("click",()=>{gt()})}function ye(){return{loadingOptions:!1,loadingPreview:!1,applying:!1,selectedProviderId:"",selectedModel:"",apiKey:"",statusMessage:"",errorMessage:"",preview:null}}function xe(){return!$||$.length===0?null:$.find(e=>e.id===r.selectedProviderId)??$[0]}function pe(){if(!$||$.length===0){r.selectedProviderId="",r.selectedModel="";return}$.some(t=>t.id===r.selectedProviderId)||(r.selectedProviderId=$[0].id);let e=xe();if(!e){r.selectedModel="";return}r.selectedModel||(r.selectedModel=e.default_model||e.models[0]||"")}async function pt(){if($){pe(),w();return}r.loadingOptions=!0,r.errorMessage="",w();try{let e=await fetch("/api/ai/providers");if(!e.ok)throw new Error(`HTTP ${e.status}`);$=(await e.json()).providers??[],pe()}catch(e){r.errorMessage=`Failed to load AI models: ${String(e)}`,$=[]}finally{r.loadingOptions=!1,w()}}async function mt(e){let t=xe(),s=r.selectedModel.trim();if(!t||!r.selectedProviderId){r.errorMessage="Choose an AI provider before generating a fix.",w();return}if(!s){r.errorMessage="Choose a model before generating a fix.",w();return}if(t.requires_api_key&&!t.configured&&!r.apiKey.trim()){r.errorMessage="Provide an API key for the selected provider before generating a fix.",w();return}r.selectedModel=s,r.loadingPreview=!0,r.statusMessage="",r.errorMessage="",w();try{let a={provider:r.selectedProviderId,model:s,api_key:r.apiKey.trim()||void 0,issue:e},n=await fetch("/api/ai/fixes/preview",{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify(a)}),l=await n.json();if(!n.ok||"error"in l)throw new Error("error"in l?l.error:`HTTP ${n.status}`);r.preview=l,r.statusMessage="Fix preview generated. Review the diff before applying it."}catch(a){r.errorMessage=`Failed to generate AI fix: ${String(a)}`,r.preview=null}finally{r.loadingPreview=!1,w()}}async function gt(){if(r.preview){r.applying=!0,r.errorMessage="",w();try{let e=await fetch("/api/ai/fixes/apply",{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify({preview_id:r.preview.preview_id})}),t=await e.json();if(!e.ok||"error"in t)throw new Error("error"in t?t.error:`HTTP ${e.status}`);r.statusMessage=t.message}catch(e){r.errorMessage=`Failed to apply AI fix: ${String(e)}`}finally{r.applying=!1,w()}}}function w(){x&&ne(x)}document.addEventListener("DOMContentLoaded",()=>{o("detail-close").addEventListener("click",ve),o("detail-overlay").addEventListener("click",ve)});function ft(){document.addEventListener("keydown",e=>{let t=e.target.tagName;if(!(t==="INPUT"||t==="SELECT"||t==="TEXTAREA")){if(e.key==="Escape"){ve();return}_e==="issues"&&(e.key==="j"||e.key==="ArrowDown"?(e.preventDefault(),C<k.length-1&&Z(C+1,!1),we()):(e.key==="k"||e.key==="ArrowUp")&&(e.preventDefault(),C>0&&Z(C-1,!1),we()))}})}function we(){document.querySelector(`.issue-row[data-idx="${C}"]`)?.scrollIntoView({behavior:"smooth",block:"nearest"})}function o(e){return document.getElementById(e)}function f(e,t){o(e).classList.add(t)}function ee(e,t){let s={};for(let a of e){let n=t(a);s[n]=(s[n]??0)+1}return s}function H(e){let t=e.replaceAll("\\","/"),s=t.split("/").filter(Boolean);return s.length<=2?t:`${s.slice(-2).join("/")}`}})();
