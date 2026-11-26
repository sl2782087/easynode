"use strict";

// Browser-only design preview. Documentation IP ranges and .invalid URLs are intentional.
const protocols = ["Shadowsocks", "VLESS + REALITY", "Hysteria 2"];
const nodes = [
  {id:"hk", name:"Hong Kong", host:"203.0.113.12", local:true, online:true, lines:[1,2]},
  {id:"jp", name:"Tokyo", host:"198.51.100.24", online:true, lines:[0,1]},
  {id:"sg", name:"Singapore", host:"192.0.2.36", online:true, lines:[2]},
  {id:"us", name:"Los Angeles", host:"203.0.113.48", online:false, lines:[0]}
];
const subs = [{id:"travel",name:"Asia Pacific",lines:["hk/1","jp/1","sg/2"]}];
const icons = {
  user:'<circle cx="12" cy="8" r="3.5"/><path d="M4.5 20a7.5 7.5 0 0 1 15 0"/>',
  nodes:'<rect x="3" y="3" width="18" height="7" rx="2"/><rect x="3" y="14" width="18" height="7" rx="2"/><path d="M7 6.5h.01M7 17.5h.01"/>',
  link:'<path d="m10 13 4-4m-6 2-2 2a4 4 0 0 0 6 6l3-3m1-3 2-2a4 4 0 0 0-6-6L9 8"/>',
  plus:'<path d="M12 5v14M5 12h14"/>', close:'<path d="m6 6 12 12M18 6 6 18"/>',
  search:'<circle cx="10.5" cy="10.5" r="6"/><path d="m15 15 5 5"/>',
  arrow:'<path d="m9 5 7 7-7 7"/>', globe:'<circle cx="12" cy="12" r="9"/><ellipse cx="12" cy="12" rx="4" ry="9"/><path d="M3 12h18"/>'
};
const $ = id => document.getElementById(id);
const icon = id => '<svg class="ico" aria-hidden="true" viewBox="0 0 24 24">'+icons[id]+'</svg>';
const esc = value => String(value).replace(/[&<>"']/g,c=>({"&":"&amp;","<":"&lt;",">":"&gt;",'"':"&quot;","'":"&#39;"}[c]));
let en = false;
let sequence = 0;
let timer;
const t = (zh, english) => en ? english : zh;
const isSubs = () => location.hash === "#subscriptions";
const allLines = () => nodes.flatMap(n=>n.lines.map(p=>({key:n.id+"/"+p,label:n.name+" · "+protocols[p]})));
const live = location.protocol !== "file:";
const subUrl = sub => (live ? location.origin : "https://demo.invalid")+"/s/"+sub.id;
let signedIn=false;
async function request(path, data) {
  const options=data ? {method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify(data)} : {};
  const res=await fetch("/api/"+path,options);
  if (!res.ok) {
    if(res.status===401 && path!=="login") {signedIn=false;loginForm();}
    throw Error((await res.text()).trim());
  }
  if(res.status===204) return;
  const state=await res.json();
  nodes.splice(0,nodes.length,...state.nodes);
  subs.splice(0,subs.length,...state.subs);
  demoAccount.username=state.user;
  signedIn=true;
}
async function saveLive(path,data,done) {
  const buttons=[...$("body").querySelectorAll("button")];
  buttons.forEach(b=>b.disabled=true);
  try {await request(path,data);renderList();done();}
  catch(e) {notice(e.message);}
  finally {buttons.forEach(b=>b.disabled=false);}
}
function loginForm(){
  nodes.length=subs.length=0;
  renderList();
  dialog(t("管理员登录","Administrator sign-in"),'<form id="login"><label>'+t("用户名","Username")+'<input class="field" name="user" required autocomplete="username"></label><label>'+t("密码","Password")+'<input class="field" type="password" name="password" required autocomplete="current-password"></label><button class="primary">'+t("登录","Sign in")+'</button></form>');
  $("close").hidden=true;
  $("login").onsubmit=e=>{
    e.preventDefault();const f=e.currentTarget;
    saveLive("login",{User:f.user.value,Password:f.password.value},()=>{$("close").hidden=false;$("dialog").close();});
  };
}

function notice(message) {
  clearTimeout(timer);
  const output=$($("dialog").open?"form-note":"notice");
  output.textContent=message;
  timer=setTimeout(()=>output.textContent="",2600);
}
function dialog(title,content) {
  $("form-note").textContent="";
  $("dialog-title").textContent=title;
  $("body").innerHTML=content;
  if (!$("dialog").open) $("dialog").showModal();
}
function renderPage() {
  const sub=isSubs();
  document.documentElement.lang=en?"en":"zh-CN";
  $("preview").textContent=live?t("管理面板","Control panel"):t("设计预览","Design preview");
  $("language").textContent=en?"中文":"EN";
  const accountLabel=t("用户中心","Account");
  $("account").innerHTML=icon("user")+"<span>"+accountLabel+"</span>";
  $("account").setAttribute("aria-label",accountLabel);
  $("account").title=accountLabel;
  $("nodes-tab").innerHTML=icon("nodes")+t("节点","Nodes");
  $("subs-tab").innerHTML=icon("link")+t("订阅","Subscriptions");
  $("nodes-tab").removeAttribute("aria-current");
  $("subs-tab").removeAttribute("aria-current");
  $(sub?"subs-tab":"nodes-tab").setAttribute("aria-current","page");
  $("title").textContent=sub?t("订阅管理","Subscriptions"):t("节点管理","Node management");
  document.title=$("title").textContent+" · EasyNodeDeck";
  $("intro").textContent=sub?t("组合节点线路，管理订阅链接与二维码。","Group connections and manage subscription links and QR codes."):t("管理本机与远程服务器，查看节点状态与线路。","Manage local and remote servers, their status and connections.");
  $("create").innerHTML=icon("plus")+(sub?t("创建订阅","Create subscription"):t("添加节点","Add node"));
  $("summary-title").textContent=sub?t("订阅概览","Subscription overview"):t("节点概览","Node overview");
  $("list-title").textContent=sub?t("订阅列表","Subscription list"):t("节点列表","Node list");
  $("search-icon").innerHTML=icon("search");
  $("search").placeholder=sub?t("搜索订阅","Find subscription"):t("搜索名称或 IP","Find name or IP");
  $("search").setAttribute("aria-label",$("search").placeholder);
  $("filter").hidden=sub;
  $("filter").setAttribute("aria-label",t("筛选状态","Filter by status"));
  $("filter").innerHTML='<option value="">'+t("全部状态","All states")+'</option><option value="up">'+t("在线","Online")+'</option><option value="down">'+t("离线","Offline")+'</option>';
  $("close").innerHTML=icon("close");
  $("close").setAttribute("aria-label",t("关闭","Close"));
  renderList();
}
function renderList() {
  const sub=isSubs();
  const stats=sub ? [[subs.length,t("订阅总数","Subscriptions")],[new Set(subs.flatMap(s=>s.lines)).size,t("已选线路","Included lines")],[allLines().length,t("可选线路","Available lines")]]
    : [[nodes.length,t("节点总数","Total nodes")],[nodes.filter(n=>n.online===true).length,t("在线节点","Online nodes")],[allLines().length,t("协议线路","Connections")],[subs.length,t("订阅","Subscriptions")]];
  $("stats").innerHTML=stats.map(([value,label])=>'<div class="stat"><strong>'+value+'</strong><span>'+label+'</span></div>').join("");
  const q=$("search").value.trim().toLowerCase(), filter=$("filter").value;
  const data=(sub?subs:nodes).filter(item=>{
    if (!(item.name+" "+(item.host||"")).toLowerCase().includes(q)) return false;
    return sub || !filter || item.online===(filter==="up");
  });
  const headers=sub?[t("订阅名称","Subscription"),t("协议","Protocols"),t("线路","Lines"),t("格式","Format")]:[t("节点名称 / 地址","Node / address"),t("协议","Protocols"),t("状态","Status"),t("部署位置","Location")];
  $("list").innerHTML='<div class="row head" aria-hidden="true">'+headers.map((h,i)=>'<span class="'+(i===3?"kind":"")+'">'+h+'</span>').join("")+'</div>'+data.map(item=>{
    const proto=sub?[...new Set(item.lines.map(key=>Number(key.split("/")[1])))]:item.lines;
    const status=item.probe==="tcp"?(item.online?t("端口可达","Reachable"):t("端口不可达","Unreachable")):item.online===true?t("在线","Online"):item.online===false?t("离线","Offline"):t("待检测","Unchecked");
    const state=sub?item.lines.length+' '+t("条","lines"):'<span class="state '+(item.online?"":"offline")+'"><i class="dot"></i>'+status+'</span>';
    const kind=sub?t("通用订阅","Universal"):item.local?t("本机","Local"):t("远程","Remote");
    return '<div class="row"><div class="node-name"><span class="badge-icon">'+icon(sub?"link":item.local?"nodes":"globe")+'</span><div><b>'+esc(item.name)+(!sub && item.local?'<span class="local-mark">'+t("本机","Local")+'</span>':"")+'</b><small>'+esc(sub?subUrl(item).replace(/^https?:\/\//,""):item.host)+'</small></div></div><div class="tag-list">'+proto.map(p=>'<span class="tag">'+protocols[p]+'</span>').join("")+'</div><div class="status-cell">'+state+'</div><span class="secondary kind">'+kind+'</span><button class="icon-btn open" data-id="'+esc(item.id)+'" aria-label="'+esc(t("查看 ","View ")+item.name)+'">'+icon("arrow")+'</button></div>';
  }).join("");
  if (!data.length) $("list").insertAdjacentHTML("beforeend",'<p class="empty">'+t("没有符合条件的记录","No matching records")+'</p>');
  $("count").textContent=t("显示 "+data.length+" 项，共 "+(sub?subs:nodes).length+" 项","Showing "+data.length+" of "+(sub?subs:nodes).length);
}
function afterSave() {
  $("dialog").close();
  renderList();
  notice(live?t("已保存","Saved"):t("已更新预览","Preview updated"));
}
function editNode(n) {
  const field=(label,name,value,attrs="")=>'<label>'+label+'<input class="field" name="'+name+'" value="'+esc(value)+'" '+attrs+'></label>';
  dialog(n?t("编辑节点","Edit node"):t("添加节点","Add node"),'<form id="editor">'+field(t("名称","Name"),"name",n?.name||'','required maxlength="60"')+
    '<label>'+t("部署位置","Location")+'<select name="mode"><option value="remote">'+t("远程服务器","Remote server")+'</option><option value="local">'+t("本机","Panel host")+'</option></select></label>'+
    '<div id="remote" class="form-stack"><div class="pair">'+field(t("服务器地址","Server address"),"host",n?.host||'','placeholder="203.0.113.10" maxlength="253"')+field(t("SSH 端口","SSH port"),"port",n?.port||22,'type="number" min="1" max="65535" required')+'</div>'+
    field(t("SSH 用户名","SSH username"),"user",n?.user||"root",'maxlength="60"')+
    field(t("SSH 密码","SSH password"),"pass","",live?'type="password" autocomplete="off"':"disabled placeholder='"+t("预览模式无需凭据","Credentials disabled in preview")+"'")+
    '<small id="host-key" aria-live="polite"></small></div>'+
    '<fieldset><legend>'+t("协议线路","Connections")+'</legend>'+protocols.map((p,i)=>'<label class="choice"><input name="line" type="checkbox" value="'+i+'" '+((n?.lines||[1]).includes(i)?"checked":"")+'>'+p+'</label>').join("")+'</fieldset>'+
    '<small id="node-help"></small><div class="actions">'+(n?'<button class="quiet" type="button" id="remove">'+t("移除节点","Remove node")+'</button>':'')+'<button class="primary"></button></div></form>');
  const f=$("editor");
  f.mode.value=n ? (n.local?"local":"remote") : nodes.some(item=>item.local)?"remote":"local";
  f.mode.disabled=Boolean(n);
  if(!n && nodes.some(item=>item.local)) f.mode.querySelector("option[value=local]").disabled=true;
  f.host.readOnly=Boolean(n);
  f.user.readOnly=live;
  let fingerprint=n?.fingerprint||"";
  const submit=f.querySelector(".primary");
  function submitLabel(){
    submit.textContent=!live?t("保存预览","Save preview"):f.mode.value==="local"?t("保存节点","Save node"):fingerprint?t("部署并保存","Deploy and save"):t("核对 SSH 指纹","Check SSH fingerprint");
  }
  function resetKey(){if(!n){fingerprint="";$("host-key").textContent="";}submitLabel();}
  f.host.oninput=resetKey;f.port.oninput=resetKey;
  function mode() {
    const local=f.mode.value==="local";
    $("remote").hidden=local;
    f.host.required=!local;
    f.port.disabled=local;
    f.pass.required=live&&!local;
    $("node-help").textContent=live?(local?t("使用独立的本机节点服务。","Uses a dedicated local service."):t("使用 root 登录，仅在本次部署中使用 SSH 密码。","SSH uses root and the password is used for this operation only.")):t("示例数据仅在当前页面保留。","Changes last until reload.");
    submitLabel();
  }
  mode(); f.mode.onchange=mode;
  if(n) $("remove").onclick=()=>{
    if(live&&!n.local&&!f.pass.value){f.pass.reportValidity();return;}
    confirmRemove(n,false,f.pass.value);
  };
  f.onsubmit=async event=>{
    event.preventDefault();
    const local=f.mode.value==="local";
    const name=f.elements.namedItem("name").value.trim();
    const host=local?(n?.local?n.host:"127.0.0.1"):f.host.value.trim();
    const lines=[...f.querySelectorAll("[name=line]:checked")].map(input=>Number(input.value));
    if(!lines.length || !name || !host) return notice(t("请填写名称、地址并选择协议","Enter a name, address and protocol"));
    if(local && nodes.some(other=>other.local && other!==n)) return notice(t("本机已经在节点列表中","The panel host is already listed"));
    if(live && !local){
      const target={Host:host,Port:Number(f.port.value)};
      if(!fingerprint){
        submit.disabled=true;
        try {
          const response=await fetch("/api/remote/key",{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify(target)});
          if(!response.ok) throw Error(await response.text());
          const result=await response.json();
          if(host!==f.host.value.trim()||target.Port!==Number(f.port.value))return;
          fingerprint=result.fingerprint;
          $("host-key").textContent=t("确认目标主机指纹后再部署：","Verify this host fingerprint before deploying: ")+fingerprint;
          submitLabel();
        }catch(e){notice(e.message);}finally{submit.disabled=false;}
        return;
      }
      $("node-help").textContent=t("正在连接、检测配置并启动服务，请稍候。","Connecting, checking the configuration and starting the service…");
      return saveLive("remote/save",{...target,ID:n?.id||"",Name:name,Lines:lines.map(String),Fingerprint:fingerprint,Password:f.pass.value},afterSave);
    }
    if(live) return saveLive("node",{Name:name,Lines:lines.map(String)},afterSave);
    const item=n||{id:"node-"+(++sequence)};
    if(!n || Boolean(n.local)!==local || n.host!==host) item.online=null;
    Object.assign(item,{name,host,local,port:Number(f.port.value),user:f.user.value,lines});
    if (!n) nodes.push(item);
    pruneLines();
    afterSave();
  };
}
function pruneLines() {
  const keys=new Set(allLines().map(l=>l.key));
  for(const sub of subs) sub.lines=sub.lines.filter(key=>keys.has(key));
}
function confirmRemove(item,sub,password="") {
  dialog(t("确认移除","Confirm removal"),'<p>'+esc(item.name)+'</p><p class="secondary">'+(sub?t("该订阅链接将失效。","This subscription URL will stop working."):t("此节点的线路也将从订阅中移除。","Its connections will also be removed from subscriptions."))+'</p><div class="actions"><button class="quiet" id="back">'+t("取消","Cancel")+'</button><button class="primary" id="confirm">'+t("移除","Remove")+'</button></div>');
  $("back").onclick=()=>sub?showSub(item):editNode(item);
  $("confirm").onclick=()=>{
    if(live&&!sub&&!item.local) return saveLive("remote/delete",{ID:item.id,Host:item.host,Port:item.port,Fingerprint:item.fingerprint,Password:password},afterSave);
    if(live) return saveLive(sub?"sub/delete":"node/delete",{ID:item.id},afterSave);
    const items=sub?subs:nodes;
    items.splice(items.indexOf(item),1);pruneLines();afterSave();
  };
}
function editSub(s) {
  const lines=allLines();
  dialog(s?t("编辑订阅","Edit subscription"):t("创建订阅","Create subscription"),'<form id="editor"><label>'+t("订阅名称","Name")+'<input class="field" name="name" required maxlength="60" value="'+esc(s?.name||"")+'"></label><fieldset><legend>'+t("选择线路","Select connections")+'</legend>'+
    (lines.map(line=>'<label class="choice"><input type="checkbox" value="'+line.key+'" '+(s?.lines.includes(line.key)?"checked":"")+'>'+esc(line.label)+'</label>').join("")||t("请先添加节点","Add a node first"))+'</fieldset><div class="actions"><button class="primary" '+(!lines.length?"disabled":"")+'>'+t("保存订阅","Save subscription")+'</button></div></form>');
  $("editor").onsubmit=event=>{
    event.preventDefault();
    const f=event.target, name=f.elements.namedItem("name").value.trim();
    const selected=[...f.querySelectorAll(":checked")].map(input=>input.value);
    if(!name||!selected.length) return notice(t("请填写名称并至少选择一条线路","Enter a name and select a connection"));
    if(live) return saveLive("sub",{ID:s?.id||"",Name:name,Lines:selected},()=>showSub(s?subs.find(i=>i.id===s.id):subs.at(-1)));
    const sub=s||{id:"demo-"+(++sequence)};
    Object.assign(sub,{name,lines:selected});
    if(!s) subs.push(sub);
    renderList();showSub(sub);
  };
}
function showSub(s) {
  const url=subUrl(s), qr=qrcode(0,"M");
  qr.addData(url);qr.make();
  dialog(s.name,'<div class="form-stack"><p class="secondary">'+(live?t("在 Clash Verge Rev 或 Shadowrocket 中导入此链接。","Import this link in Clash Verge Rev or Shadowrocket."):t("示例订阅地址仅供设计预览。","This sample URL is for design preview only."))+'</p><img class="qr" alt="'+t("订阅二维码","Subscription QR code")+'" src="'+qr.createDataURL(5,20)+'"><input id="url" class="field" readonly aria-label="'+t("订阅地址","Subscription URL")+'" value="'+esc(url)+'"><div class="actions"><button class="quiet" id="remove">'+t("移除","Remove")+'</button><button class="quiet" id="edit">'+t("编辑线路","Edit connections")+'</button><button class="primary" id="copy">'+t("复制链接","Copy URL")+'</button></div></div>');
  $("edit").onclick=()=>editSub(s);
  $("remove").onclick=()=>confirmRemove(s,true);
  $("copy").onclick=async()=>{
    try {await navigator.clipboard.writeText(url);$("copy").textContent=t("已复制","Copied");}
    catch {$("url").select();$("copy").textContent=t("请手动复制","Copy selection");}
  };
}
// Only a demonstration account, reset on every reload.
const demoAccount = {username:"admin", passwordDigest:null};
const initialDemoPassword = "Deck-preview";
function accountHome() {
  dialog(t("用户中心","Account"),`<div class="form-stack">
    <div class="account-row"><div><small>${t("管理员用户名","Administrator username")}</small><strong>${esc(demoAccount.username)}</strong></div>
      <button class="quiet" id="rename">${t("修改用户名","Change username")}</button></div>
    <div class="account-row"><div><small>${t("登录密码","Password")}</small><strong>••••••••</strong></div>
      <button class="quiet" id="rekey">${t("修改密码","Change password")}</button></div>
    <small>${live?'<button class="quiet" id="logout">'+t("退出登录","Sign out")+'</button>':t("演示账号仅在当前页面生效，刷新后重置。","This demo account resets when you reload the page.")}</small></div>`);
  if(live) $("logout").onclick=()=>saveLive("logout",{},()=>{signedIn=false;loginForm();});
  $("rename").onclick=()=>editAccount(false);
  $("rekey").onclick=()=>editAccount(true);
}
async function previewDigest(value) {
  const bytes=await crypto.subtle.digest("SHA-256",new TextEncoder().encode(value));
  return new Uint8Array(bytes).join(",");
}
function editAccount(password) {
  const title=password?t("修改密码","Change password"):t("修改用户名","Change username");
  const passwordField=(label,name,attrs="")=>`<label>${label}<input class="field" type="password" name="${name}" required autocomplete="off" ${attrs}></label>`;
  const hint=live?t("请输入当前管理员密码。","Enter your current administrator password."):demoAccount.passwordDigest ? t("使用刚设置的演示密码。","Use your updated demo password.")
    : t("当前演示密码为 Deck-preview。","The current demo password is Deck-preview.");
  dialog(title,`<form id="account-form" autocomplete="off">
    <small>${hint}</small>
    ${passwordField(t("当前密码","Current password"),"current")}
    ${password?passwordField(t("新密码（至少 8 位）","New password (at least 8 characters)"),"next",'minlength="8"')+
      passwordField(t("确认新密码","Confirm new password"),"confirmation"):
      `<label>${t("新用户名","New username")}<input class="field" name="username" value="${esc(demoAccount.username)}" required maxlength="60"></label>`}
    <div class="actions"><button class="quiet" type="button" id="account-back">${t("返回","Back")}</button><button class="primary">${t("保存修改","Save changes")}</button></div>
  </form>`);
  $("account-back").onclick=accountHome;
  $("account-form").onsubmit=async event=>{
    event.preventDefault();
    const f=event.currentTarget;
    if(live){
      if(password && f.next.value!==f.confirmation.value) return notice(t("两次输入的新密码不一致","Passwords do not match"));
      if(!password && !f.username.value.trim()) return notice(t("请输入用户名","Enter a username"));
      return saveLive("account",password?{Current:f.current.value,Password:f.next.value}:{Current:f.current.value,User:f.username.value.trim()},()=>{accountHome();notice(t("已更新","Updated"));});
    }
    const valid=demoAccount.passwordDigest ? await previewDigest(f.current.value)===demoAccount.passwordDigest : f.current.value===initialDemoPassword;
    if(!valid) return notice(t("当前密码不正确","Incorrect current password"));
    if(password) {
      if(f.next.value!==f.confirmation.value) return notice(t("两次输入的新密码不一致","The new passwords do not match"));
      demoAccount.passwordDigest=await previewDigest(f.next.value);
    } else {
      const username=f.username.value.trim();
      if(!username) return notice(t("请输入用户名","Enter a username"));
      demoAccount.username=username;
    }
    accountHome();
    notice(password?t("演示密码已更新","Demo password updated"):t("用户名已更新","Username updated"));
  };
}
$("dialog").addEventListener("cancel",e=>{if(live&&!signedIn)e.preventDefault();});
$("account").onclick=accountHome;
$("create").onclick=()=>isSubs()?editSub():editNode();
$("close").onclick=()=>$("dialog").close();
$("dialog").addEventListener("close",()=>{if(!$("dialog").open) $("body").replaceChildren();});
$("search").oninput=renderList;
$("filter").onchange=renderList;
$("language").onclick=()=>{en=!en;renderPage();};
$("list").onclick=event=>{
  const button=event.target.closest("button[data-id]");
  if(!button) return;
  const id=button.dataset.id;
  if(isSubs()) showSub(subs.find(s=>s.id===id));
  else editNode(nodes.find(n=>n.id===id));
};
window.addEventListener("hashchange",()=>{if(live&&!signedIn){loginForm();return;}$("dialog").close();$("search").value="";renderPage();});
if(live){nodes.length=subs.length=0;}
renderPage();
if(live) request("state").then(renderList).catch(()=>loginForm());
