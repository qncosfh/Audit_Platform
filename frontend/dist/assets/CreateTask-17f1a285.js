import{d as B,u as H,a as Q,b as A,z as G,c as S,f as t,e as r,w as s,r as C,i as n,o as g,t as d,G as I,H as V,n as u,P as w,E as P,N as R,_ as O}from"./index-9eb7eb86.js";import{u as j}from"./task-afb37fdf.js";const J={class:"create-task"},X={class:"form-container"},z={class:"card-header"},K={class:"form-tip"},W={class:"card-header"},Y={class:"model-option"},Z={class:"model-name"},x={class:"model-desc"},ee={class:"form-tip"},oe={class:"form-actions"},le={class:"card-header"},te={class:"unit"},ae={class:"form-actions"},se=B({__name:"CreateTask",setup(re){const{t:i}=H(),T=Q(),f=j();A();const k=C(!1),_=C(),a=C({name:"",description:"",codeSourceId:null,modelConfigId:null,prompt:"",timeout:600}),D={name:[{required:!0,message:i("task.nameRequired"),trigger:"blur"},{min:2,max:100,message:i("task.nameLength"),trigger:"blur"}],description:[{required:!0,message:i("task.descRequired"),trigger:"blur"}],codeSourceId:[{required:!0,message:i("task.selectCodeSource"),trigger:"change"}],modelConfigId:[{required:!0,message:i("task.selectModel"),trigger:"change"}]},E=w(()=>f.codeSources),N=w(()=>f.models),q=e=>{console.log("Code source changed:",e,typeof e),a.value.prompt||b()},b=()=>{const e=`
  你是一名资深代码安全审计专家，具备丰富的实战渗透经验和代码审计能力，精通 Java、Go、Python、C/C++ 等主流语言及常见框架（如 Spring、Gin、Django 等）。

  【审计目标】
  对提供的代码进行严格安全审计，仅识别“可被直接利用的高危漏洞”，并输出标准化漏洞报告。

  【漏洞范围（仅允许以下类型）】
  仅允许报告以下高危且可利用漏洞：
  	•	SQL注入（SQL Injection）
  	•	命令执行 / 命令注入（RCE）
  	•	不安全反序列化
  	•	路径遍历 / 任意文件读写
  	•	硬编码敏感信息（密码 / Token / API Key）
  	•	SSRF（必须可控目标）
  	•	任意文件上传（可导致RCE）

  【严格禁止报告（避免误报）】
  禁止报告以下内容：
  	•	XSS（除非明确存在完整利用链）
  	•	CSRF
  	•	信息泄露（如版本号、注释等）
  	•	中低危漏洞
  	•	依赖漏洞（未结合实际利用链）
  	•	仅“可能存在”但无法验证的问题

  【严格判定标准（必须全部满足）】
  只有同时满足以下条件才允许报告漏洞：
  	1.	存在明确的用户输入点（如 HTTP 参数、JSON body、文件上传、Header 等）
  	2.	用户输入进入危险函数或敏感操作（如 SQL 执行、系统命令、文件操作等）
  	3.	没有有效安全防护（如未使用参数化查询、无白名单校验、无路径限制等）
  	4.	可以构造真实可执行的攻击 Payload

  【关键要求】
  	•	不允许基于猜测或假设报告漏洞
  	•	不确定的漏洞一律忽略
  	•	必须基于真实代码逻辑分析
  	•	必须体现完整数据流（输入 → 处理 → 危险点）

  【输出格式（必须严格遵守）】
  漏洞报告
  	•	威胁名称:
  	•	严重等级: Critical / High
  	•	位置: 文件名:行号 或 方法名
  	•	漏洞描述:
  （必须说明数据流：输入 → 处理 → 危险点）
  	•	危险代码:
  	•	利用条件:
  	•	POC / EXP:
  	•	修复建议:

  【无漏洞情况】
  如果未发现符合条件的高危漏洞，仅输出：无高危安全漏洞
  不得输出任何其他内容。

  【审计重点提示】
  优先分析以下路径：
  	•	Controller → Service → DAO
  	•	用户输入 → SQL拼接
  	•	用户输入 → 命令执行
  	•	用户输入 → 文件路径操作
  	•	用户输入 → 反序列化入口
  `;a.value.prompt=e},F=()=>{a.value.prompt=""},L=async()=>{_.value&&await _.value.validate(async e=>{if(e){k.value=!0;try{const o=a.value.codeSourceId,v=a.value.modelConfigId;console.log("表单 codeSourceId:",o,typeof o),console.log("表单 modelConfigId:",v,typeof v);const c={name:a.value.name,description:a.value.description,codeSourceId:String(o),modelConfigId:String(v),prompt:a.value.prompt,timeout:a.value.timeout};console.log("Submitting task data:",c),await f.createTask(c),P.success(i("task.createSuccess")),T.push("/tasks")}catch(o){console.error("Failed to create task:",o),P.error((o==null?void 0:o.message)||i("task.createError"))}finally{k.value=!1}}})};return G(async()=>{try{await Promise.all([f.loadCodeSources(),f.loadModels()])}catch(e){console.error("Failed to load data:",e)}}),(e,o)=>{const v=n("el-page-header"),c=n("el-input"),m=n("el-form-item"),$=n("el-option"),y=n("el-select"),p=n("el-button"),h=n("el-card"),M=n("el-input-number"),U=n("el-form");return g(),S("div",J,[t(v,{onBack:o[0]||(o[0]=l=>e.$router.push("/tasks")),title:e.$t("task.create")},null,8,["title"]),r("div",X,[t(U,{ref_key:"formRef",ref:_,model:a.value,rules:D,"label-width":"120px",class:"task-form"},{default:s(()=>[t(h,{class:"form-card"},{header:s(()=>[r("div",z,[r("span",null,d(e.$t("task.basicInfo")),1)])]),default:s(()=>[t(m,{label:e.$t("task.name"),prop:"name"},{default:s(()=>[t(c,{modelValue:a.value.name,"onUpdate:modelValue":o[1]||(o[1]=l=>a.value.name=l),placeholder:e.$t("task.namePlaceholder"),maxlength:"100","show-word-limit":""},null,8,["modelValue","placeholder"])]),_:1},8,["label"]),t(m,{label:e.$t("task.description"),prop:"description"},{default:s(()=>[t(c,{modelValue:a.value.description,"onUpdate:modelValue":o[2]||(o[2]=l=>a.value.description=l),type:"textarea",rows:3,placeholder:e.$t("task.descPlaceholder"),maxlength:"500","show-word-limit":""},null,8,["modelValue","placeholder"])]),_:1},8,["label"]),t(m,{label:e.$t("codeSource.title"),prop:"codeSourceId"},{default:s(()=>[t(y,{modelValue:a.value.codeSourceId,"onUpdate:modelValue":o[3]||(o[3]=l=>a.value.codeSourceId=l),placeholder:e.$t("task.selectCodeSource"),filterable:"",class:"full-width",onChange:q,clearable:""},{default:s(()=>[(g(!0),S(I,null,V(E.value,l=>(g(),R($,{key:l.id||l.ID,label:l.name,value:l.id||l.ID},null,8,["label","value"]))),128))]),_:1},8,["modelValue","placeholder"]),r("div",K,[u(d(e.$t("task.noCodeSource")),1),t(p,{type:"text",onClick:o[4]||(o[4]=l=>e.$router.push("/code-sources"))},{default:s(()=>[u(d(e.$t("task.uploadNow")),1)]),_:1})])]),_:1},8,["label"])]),_:1}),t(h,{class:"form-card"},{header:s(()=>[r("div",W,[r("span",null,d(e.$t("model.title")),1)])]),default:s(()=>[t(m,{label:e.$t("model.model"),prop:"modelConfigId"},{default:s(()=>[t(y,{modelValue:a.value.modelConfigId,"onUpdate:modelValue":o[5]||(o[5]=l=>a.value.modelConfigId=l),placeholder:e.$t("task.selectModel"),filterable:"",class:"full-width"},{default:s(()=>[(g(!0),S(I,null,V(N.value,l=>(g(),R($,{key:l.id,label:l.name,value:l.id},{default:s(()=>[r("div",Y,[r("div",Z,d(l.name),1),r("div",x,d(l.provider)+" - "+d(l.model),1)])]),_:2},1032,["label","value"]))),128))]),_:1},8,["modelValue","placeholder"]),r("div",ee,[u(d(e.$t("task.noModel")),1),t(p,{type:"text",onClick:o[6]||(o[6]=l=>e.$router.push("/models"))},{default:s(()=>[u(d(e.$t("task.configNow")),1)]),_:1})])]),_:1},8,["label"]),t(m,{label:e.$t("task.customPrompt"),prop:"prompt"},{default:s(()=>[t(c,{modelValue:a.value.prompt,"onUpdate:modelValue":o[7]||(o[7]=l=>a.value.prompt=l),type:"textarea",rows:8,placeholder:e.$t("task.promptPlaceholder")},null,8,["modelValue","placeholder"]),r("div",oe,[t(p,{onClick:b},{default:s(()=>[u(d(e.$t("task.loadDefaultPrompt")),1)]),_:1}),t(p,{onClick:F},{default:s(()=>[u(d(e.$t("task.clearPrompt")),1)]),_:1})])]),_:1},8,["label"])]),_:1}),t(h,{class:"form-card"},{header:s(()=>[r("div",le,[r("span",null,d(e.$t("task.advancedSettings")),1)])]),default:s(()=>[t(m,{label:e.$t("task.timeout")},{default:s(()=>[t(M,{modelValue:a.value.timeout,"onUpdate:modelValue":o[8]||(o[8]=l=>a.value.timeout=l),min:60,max:3600,step:60,"controls-position":"right"},null,8,["modelValue"]),r("span",te,d(e.$t("task.seconds")),1)]),_:1},8,["label"])]),_:1}),r("div",ae,[t(p,{onClick:o[9]||(o[9]=l=>e.$router.push("/tasks"))},{default:s(()=>[u(d(e.$t("common.cancel")),1)]),_:1}),t(p,{type:"primary",onClick:L,loading:k.value},{default:s(()=>[u(d(e.$t("task.create")),1)]),_:1},8,["loading"])])]),_:1},8,["model"])])])}}});const ue=O(se,[["__scopeId","data-v-08fee7be"]]);export{ue as default};
