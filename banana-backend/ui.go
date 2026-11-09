package main

import (
	"net/http"
	"os"
	"strings"
)

func handleIndex(w http.ResponseWriter, r *http.Request) {
	data, err := os.ReadFile("index.html")
	if err == nil && strings.Contains(string(data), "<script") && strings.Contains(string(data), "trash-btn") {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(data)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(fallbackHTML))
}

const fallbackHTML = `<!DOCTYPE html>
<html lang="ru"><head><meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>Banana Cloud Storage</title>
<style>
:root{--bg:#0f1115;--bg2:#161a22;--bg3:#1d222d;--bd:#2a3140;--tx:#e6e8ee;--mu:#8a93a6;--pr:#ffd23f;--er:#ef4444;--ok:#22c55e;--dh:200px}
*{box-sizing:border-box;margin:0;padding:0}
html,body{height:100%}
body{font-family:-apple-system,BlinkMacSystemFont,"Segoe UI",Roboto,sans-serif;background:var(--bg);color:var(--tx);font-size:14px;overflow:hidden}
button,input,select{font-family:inherit;font-size:14px}
a{color:var(--pr);text-decoration:none}
.hidden{display:none!important}
.wrap{min-height:100vh;display:flex;align-items:center;justify-content:center;padding:20px}
.card{background:var(--bg2);border:1px solid var(--bd);border-radius:14px;padding:28px;width:100%;max-width:420px}
.card.wide{max-width:1100px}
.logo{font-size:26px;font-weight:700;margin-bottom:6px}
.logo span{color:var(--pr)}
.mu{color:var(--mu)}
.tabs{display:flex;gap:4px;margin:18px 0;background:var(--bg3);padding:4px;border-radius:10px}
.tab{flex:1;padding:9px;text-align:center;border-radius:8px;cursor:pointer;color:var(--mu);font-weight:500}
.tab.active{background:var(--bg2);color:var(--tx)}
.field{margin-bottom:12px}
.field label{display:block;margin-bottom:5px;color:var(--mu);font-size:11px;text-transform:uppercase;letter-spacing:.5px}
.field input,.field select{width:100%;padding:10px 12px;background:var(--bg3);border:1px solid var(--bd);border-radius:8px;color:var(--tx);outline:none}
.field input:focus{border-color:var(--pr)}
.btn{display:inline-flex;align-items:center;gap:6px;padding:9px 14px;border-radius:8px;border:1px solid transparent;cursor:pointer;font-weight:500;background:var(--bg3);color:var(--tx)}
.btn:hover{background:var(--bd)}
.btn.primary{background:var(--pr);color:#1a1a1a}
.btn.danger{background:transparent;border-color:var(--er);color:var(--er)}
.btn.small{padding:5px 10px;font-size:12px}
.btn.block{width:100%;justify-content:center}
.err{color:var(--er);font-size:13px;margin-top:8px;min-height:18px}
.app{display:flex;height:100vh;flex-direction:column}
header{background:var(--bg2);border-bottom:1px solid var(--bd);padding:12px 24px;display:flex;align-items:center;gap:14px}
header .logo{margin:0;font-size:19px}
.sp{flex:1}
.toolbar{padding:14px 24px;display:flex;gap:8px;align-items:center;flex-wrap:wrap;border-bottom:1px solid var(--bd)}
.crumbs{display:flex;gap:6px;flex-wrap:wrap;align-items:center}
.crumb{color:var(--mu);cursor:pointer;padding:4px 8px;border-radius:6px}
.crumb:hover{background:var(--bg3);color:var(--tx)}
.crumb.current{color:var(--tx);cursor:default}
main{flex:1;padding:20px 24px;overflow:auto;min-height:0}
table{width:100%;border-collapse:collapse}
th,td{text-align:left;padding:11px 12px;border-bottom:1px solid var(--bd)}
th{color:var(--mu);font-weight:500;font-size:11px;text-transform:uppercase;letter-spacing:.5px;position:sticky;top:0;background:var(--bg)}
tr:hover td{background:var(--bg2)}
.click td{cursor:pointer}
.acts{display:flex;gap:5px;flex-wrap:wrap}
.empty{text-align:center;padding:50px 20px;color:var(--mu)}
.mbg{position:fixed;inset:0;background:rgba(0,0,0,.6);display:flex;align-items:center;justify-content:center;z-index:100;padding:20px}
.macts{display:flex;justify-content:flex-end;gap:8px;margin-top:18px}
.toast{position:fixed;bottom:16px;right:16px;background:var(--bg3);border:1px solid var(--bd);border-radius:10px;padding:11px 16px;z-index:300;opacity:0;transform:translateY(8px);transition:.2s;max-width:340px}
.toast.on{opacity:1;transform:translateY(0)}
.toast.error{border-color:var(--er)}
.toast.success{border-color:var(--ok)}
.linkbox{background:var(--bg3);border:1px solid var(--bd);border-radius:8px;padding:9px 10px;display:flex;gap:6px;align-items:center}
.linkbox input{flex:1;background:transparent;border:none;color:var(--tx);outline:none;font-family:monospace;font-size:12px}
.dbg{position:fixed;bottom:0;left:0;right:0;height:var(--dh);background:#0a0c0f;border-top:2px solid var(--pr);display:flex;flex-direction:column;z-index:200}
.dbg-h{padding:7px 12px;background:var(--bg2);border-bottom:1px solid var(--bd);display:flex;justify-content:space-between;align-items:center;cursor:ns-resize;user-select:none}
.dbg-h b{color:var(--pr);font-size:12px}
.dbg-b{padding:6px;overflow-y:auto;font-family:Consolas,Monaco,monospace;font-size:11px;flex:1;min-height:0}
.dl{padding:3px 7px;margin:2px 0;border-left:3px solid var(--bd);background:var(--bg2);word-break:break-word}
.dl.request{border-left-color:#3b82f6}.dl.response{border-left-color:var(--ok)}.dl.error{border-left-color:var(--er)}.dl.info{border-left-color:var(--pr)}
.pvbox{background:var(--bg3);border:1px solid var(--bd);border-radius:10px;padding:12px;max-height:62vh;overflow:auto}
.pvbox pre{white-space:pre-wrap;font-family:Consolas,Monaco,monospace;font-size:12px}
.pvbox img,.pvbox video{max-width:100%;border-radius:8px}
.pvbox iframe{width:100%;height:60vh;border:0}
.chk{display:flex;gap:8px;align-items:center;cursor:pointer;color:var(--mu);font-size:13px;margin-top:10px}
.chk input{width:auto}
.badge{display:inline-block;padding:2px 8px;border-radius:20px;font-size:10px;background:var(--bg3);border:1px solid var(--bd);color:var(--mu);margin-left:6px}
</style></head><body>

<div id="auth" class="wrap">
 <div class="card">
  <div class="logo">🍌 Banana <span>Cloud</span></div>
  <div class="mu">Приватное облачное хранилище</div>
  <div class="tabs"><div class="tab active" id="tab-login">Вход</div><div class="tab" id="tab-reg">Регистрация</div></div>
  <form id="login-form">
   <div class="field"><label>Email</label><input type="email" name="email" required></div>
   <div class="field"><label>Пароль</label><input type="password" name="password" required minlength="6"></div>
   <div class="err" id="login-err"></div>
   <button class="btn primary block" type="submit">Войти</button>
  </form>
  <form id="reg-form" class="hidden">
   <div class="field"><label>Email</label><input type="email" name="email" required></div>
   <div class="field"><label>Пароль (мин. 6)</label><input type="password" name="password" required minlength="6"></div>
   <div class="err" id="reg-err"></div>
   <button class="btn primary block" type="submit">Создать аккаунт</button>
  </form>
 </div>
</div>

<div id="app" class="app hidden">
 <header>
  <div class="logo">🍌 Banana <span>Cloud</span></div>
  <div class="sp"></div>
  <span class="mu" id="who"></span>
  <button class="btn small" id="logout">Выйти</button>
 </header>
 <div class="toolbar">
  <div class="crumbs" id="crumbs"></div>
  <div class="sp"></div>
  <button class="btn small" id="logbtn">🪵 Логи</button>
  <button class="btn small" id="refresh">🔄 Обновить</button>
  <button class="btn small" id="newfolder">📁 Новая папка</button>
  <button class="btn small primary" id="upload">⬆️ Загрузить</button>
  <button class="btn small" id="trash-btn">🗑 Корзина</button>
  <input type="file" id="file" multiple class="hidden">
 </div>
 <main>
  <table><thead><tr><th>Имя</th><th>Тип</th><th>Размер</th><th>Изменено</th><th>Действия</th></tr></thead>
  <tbody id="rows"></tbody></table>
  <div id="empty" class="empty hidden">Папка пуста</div>
 </main>
</div>

<div id="shareview" class="wrap hidden">
 <div class="card wide">
  <div class="logo">🍌 Banana <span>Cloud</span> — общий доступ</div>
  <div id="sharebox" style="margin-top:14px">Загрузка...</div>
 </div>
</div>

<div class="mbg hidden" id="modal-bg"><div class="card" id="modal" style="max-width:760px"></div></div>
<div class="toast" id="toast"></div>
<div class="dbg hidden" id="dbg">
 <div class="dbg-h" id="dbg-head"><b>🔍 Debug Log</b>
  <span><button class="btn small" id="dbg-clear">🗑 Очистить</button> <button class="btn small danger" id="dbg-close">✕ Закрыть</button></span>
 </div>
 <div class="dbg-b" id="dbg-body"></div>
</div>

<script>
var state={token:localStorage.getItem('banana_token'),email:localStorage.getItem('banana_email'),cur:'',path:[],view:'files'};
function $(s){return document.querySelector(s);}
function $$(s){return document.querySelectorAll(s);}
function esc(s){return String(s).replace(/[&<>"']/g,function(c){return{'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c];});}
function fmt(b){if(!b&&b!==0)return '—';var u=['B','KB','MB','GB','TB'],i=0,n=b;while(n>=1024&&i<4){n/=1024;i++;}return n.toFixed(n<10&&i>0?1:0)+' '+u[i];}
function fmtDate(s){if(!s)return '—';return new Date(s).toLocaleString('ru-RU',{day:'2-digit',month:'2-digit',year:'numeric',hour:'2-digit',minute:'2-digit'});}
function icon(ft){if(ft==='image')return '🖼️';if(ft==='video')return '🎬';if(ft==='audio')return '🎵';if(ft==='document')return '📕';if(ft==='code')return '📝';if(ft==='archive')return '🗜️';return '📄';}
var toastT;function toast(m,t){var e=$('#toast');e.textContent=m;e.className='toast on '+(t||'');clearTimeout(toastT);toastT=setTimeout(function(){e.classList.remove('on');},3000);}
function saveDbg(){localStorage.setItem('debug_state',JSON.stringify({hidden:$('#dbg').classList.contains('hidden'),height:$('#dbg').offsetHeight}));}
function toggleDbg(){var hidden=$('#dbg').classList.toggle('hidden');saveDbg();}
$('#dbg-close').onclick=toggleDbg;
$('#logbtn').onclick=toggleDbg;
$('#dbg-clear').onclick=function(){$('#dbg-body').innerHTML='';};
var rz=false,ry=0,rh=0;
$('#dbg-head').addEventListener('mousedown',function(e){rz=true;ry=e.clientY;rh=$('#dbg').offsetHeight;e.preventDefault();});
document.addEventListener('mousemove',function(e){if(!rz)return;var h=Math.max(80,Math.min(rh+(ry-e.clientY),window.innerHeight-100));document.documentElement.style.setProperty('--dh',h+'px');});
document.addEventListener('mouseup',function(){if(rz){rz=false;saveDbg();}});
var dbgState=JSON.parse(localStorage.getItem('debug_state')||'{}');
if(dbgState.height)document.documentElement.style.setProperty('--dh',dbgState.height+'px');
if(dbgState.hidden===false){$('#dbg').classList.remove('hidden');}
function dlog(t,m,d){var e=document.createElement('div');e.className='dl '+t;var h='<span class="mu">['+new Date().toLocaleTimeString('ru-RU')+']</span> '+esc(m);
if(d!==null&&d!==undefined){var s=JSON.stringify(d);h+='<div class="mu" style="font-size:10px;white-space:pre-wrap">'+esc(s.length>300?s.slice(0,300)+'…':s)+'</div>';}
e.innerHTML=h;var b=$('#dbg-body');b.appendChild(e);b.scrollTop=b.scrollHeight;console.log(t,m,d||'');}
function api(p,o){o=o||{};var h=o.headers||{};if(state.token)h['Authorization']='Bearer '+state.token;
var body=o.body;
if(o.body&&!(o.body instanceof FormData)){h['Content-Type']='application/json';body=JSON.stringify(o.body);}
dlog('request',(o.method||'GET')+' '+p,o.body instanceof FormData?{formData:true}:o.body);
return fetch(p,{method:o.method||'GET',headers:h,body:body}).then(function(r){return r.text().then(function(tx){var d;try{d=JSON.parse(tx);}catch(e){d=tx;}
dlog('response',r.status+' '+p,d);
if(!r.ok){var msg=(d&&d.error&&d.error.message)||('HTTP '+r.status);throw new Error(msg);}return d;});});}
function skipActive(){return Date.now()<parseInt(localStorage.getItem('banana_skip_until')||'0',10);}
function confirmStyled(title,msg,allowSkip,onYes){
if(allowSkip&&skipActive()){onYes();return;}
modal('<h3>'+title+'</h3><p class="mu" style="margin:6px 0 4px">'+esc(msg)+'</p>'+
(allowSkip?'<label class="chk"><input type="checkbox" id="m-skip"> Не спрашивать 24 часа</label>':'')+
'<div class="macts"><button class="btn" id="m-cancel">Отмена</button><button class="btn danger" id="m-ok">Удалить</button></div>',
function(){if(allowSkip&&$('#m-skip')&&$('#m-skip').checked){localStorage.setItem('banana_skip_until',String(Date.now()+24*3600*1000));toast('Подтверждения отключены на 24 часа','info');}closeModal();onYes();});}
$$('#tab-login,#tab-reg').forEach(function(t){t.onclick=function(){$$('.tab').forEach(function(x){x.classList.remove('active');});t.classList.add('active');
$('#login-form').classList.toggle('hidden',t.id!=='tab-login');$('#reg-form').classList.toggle('hidden',t.id!=='tab-reg');};});
$('#login-form').onsubmit=function(e){e.preventDefault();var fd=new FormData(e.target);$('#login-err').textContent='';
api('/api/v1/auth/login',{method:'POST',body:{email:fd.get('email').toString().trim(),password:fd.get('password').toString()}}).then(function(d){
state.token=d.token;state.email=fd.get('email').toString().trim();localStorage.setItem('banana_token',d.token);localStorage.setItem('banana_email',state.email);showApp();
}).catch(function(er){$('#login-err').textContent=er.message;});};
$('#reg-form').onsubmit=function(e){e.preventDefault();var fd=new FormData(e.target);$('#reg-err').textContent='';
api('/api/v1/auth/register',{method:'POST',body:{email:fd.get('email').toString().trim(),password:fd.get('password').toString()}}).then(function(){
toast('Аккаунт создан. Войдите.','success');$('#tab-login').click();}).catch(function(er){$('#reg-err').textContent=er.message;});};
$('#logout').onclick=function(){api('/api/v1/auth/logout',{method:'POST'}).catch(function(){});state.token=null;state.email=null;
localStorage.removeItem('banana_token');localStorage.removeItem('banana_email');showAuth();};
function showAuth(){$('#auth').classList.remove('hidden');$('#app').classList.add('hidden');$('#shareview').classList.add('hidden');}
function showApp(){$('#auth').classList.add('hidden');$('#app').classList.remove('hidden');$('#who').textContent=state.email||'';state.cur='';state.path=[];state.view='files';loadFolder();}
function crumbs(){var bc=$('#crumbs');bc.innerHTML='';
if(state.view==='trash'){var b=document.createElement('span');b.className='crumb';b.textContent='← Мои файлы';b.onclick=function(){backToFiles();};bc.appendChild(b);
var s1=document.createElement('span');s1.className='mu';s1.textContent='›';bc.appendChild(s1);
var c=document.createElement('span');c.className='crumb current';c.textContent='🗑 Корзина';bc.appendChild(c);return;}
var r=document.createElement('span');r.className='crumb'+(state.path.length===0?' current':'');r.textContent='🏠 Мои файлы';
r.onclick=function(){state.cur='';state.path=[];loadFolder();};bc.appendChild(r);
state.path.forEach(function(p,i){var s=document.createElement('span');s.className='mu';s.textContent='›';bc.appendChild(s);
var c=document.createElement('span');c.className='crumb'+(i===state.path.length-1?' current':'');c.textContent=p.name;
c.onclick=function(){state.path=state.path.slice(0,i+1);state.cur=p.id;loadFolder();};bc.appendChild(c);});}
function loadFolder(){state.view='files';$('#empty').textContent='Папка пуста';crumbs();
var tb=$('#rows');tb.innerHTML='<tr><td colspan="5" class="empty">Загрузка...</td></tr>';$('#empty').classList.add('hidden');tb.onclick=null;
Promise.all([api('/api/v1/folders?parentID='+encodeURIComponent(state.cur)),api('/api/v1/files?folderID='+encodeURIComponent(state.cur))])
.then(function(res){render(res[0].items||[],res[1].items||[]);})
.catch(function(er){tb.innerHTML='<tr><td colspan="5" class="empty">Ошибка: '+esc(er.message)+'</td></tr>';});}
function render(folders,files){var tb=$('#rows');tb.innerHTML='';
if(!folders.length&&!files.length){$('#empty').classList.remove('hidden');return;}
$('#empty').classList.add('hidden');
folders.forEach(function(f){var tr=document.createElement('tr');tr.className='click';
tr.innerHTML='<td>📁 '+esc(f.name)+'</td><td>Папка</td><td>'+fmt(f.size||0)+' ('+(f.fileCount||0)+' файл.)</td><td>'+fmtDate(f.createdAt)+'</td>'+
'<td class="acts"><button class="btn small" data-a="ren" data-id="'+f.id+'" data-n="'+esc(f.name)+'">✏️</button>'+
'<button class="btn small" data-a="shf" data-id="'+f.id+'">🔗</button>'+
'<button class="btn small danger" data-a="delf" data-id="'+f.id+'" data-n="'+esc(f.name)+'">🗑</button></td>';
tr.onclick=function(e){if(e.target.closest('button,a'))return;state.cur=f.id;state.path.push({id:f.id,name:f.name});loadFolder();};
tb.appendChild(tr);});
files.forEach(function(f){var tr=document.createElement('tr');
tr.innerHTML='<td>'+icon(f.fileType)+' '+esc(f.name)+'<span class="badge">'+esc(f.fileType||'other')+'</span></td>'+
'<td>'+esc(f.contentType||'file')+'</td><td>'+fmt(f.size)+'</td><td>'+fmtDate(f.updatedAt||f.createdAt)+'</td>'+
'<td class="acts">'+
'<button class="btn small" data-a="pv" data-id="'+f.id+'" data-ct="'+esc(f.contentType||'')+'" data-ft="'+esc(f.fileType||'')+'" data-n="'+esc(f.name)+'">👁</button>'+
'<a class="btn small" href="'+f.downloadUrl+'?token='+encodeURIComponent(state.token)+'" download>⬇️</a>'+
'<button class="btn small" data-a="mov" data-id="'+f.id+'">📦</button>'+
'<button class="btn small" data-a="shx" data-id="'+f.id+'">🔗</button>'+
'<button class="btn small danger" data-a="delx" data-id="'+f.id+'" data-n="'+esc(f.name)+'">🗑</button></td>';
tb.appendChild(tr);});
tb.onclick=function(e){var b=e.target.closest('button');if(!b)return;var id=b.dataset.id,a=b.dataset.a;
if(a==='ren')renFolder(id,b.dataset.n);
if(a==='delf')delFolder(id,b.dataset.n);
if(a==='delx')delFile(id,b.dataset.n);
if(a==='mov')moveFile(id);
if(a==='shx')share('file',id);
if(a==='shf')share('folder',id);
if(a==='pv')preview(id,b.dataset.ct,b.dataset.ft,b.dataset.n);};}
$('#refresh').onclick=function(){if(state.view==='trash')loadTrash();else loadFolder();};
$('#newfolder').onclick=function(){modal('<h3>Новая папка</h3><div class="field"><label>Имя</label><input id="m-name" autofocus></div><div class="err" id="m-err"></div><div class="macts"><button class="btn" id="m-cancel">Отмена</button><button class="btn primary" id="m-ok">Создать</button></div>',function(){
var n=$('#m-name').value.trim();if(!n)return;
api('/api/v1/folders',{method:'POST',body:{name:n,parentID:state.cur}}).then(function(){closeModal();toast('Папка создана','success');loadFolder();})
.catch(function(er){if(/exists/i.test(er.message))$('#m-err').textContent='Папка с таким именем уже существует';else toast(er.message,'error');});});};
$('#upload').onclick=function(){$('#file').click();};
$('#file').onchange=function(e){var fs=Array.prototype.slice.call(e.target.files);if(!fs.length)return;var chain=Promise.resolve();
fs.forEach(function(f){chain=chain.then(function(){var fd=new FormData();fd.append('file',f);
return api('/api/v1/files?folderID='+encodeURIComponent(state.cur),{method:'POST',body:fd});});});
chain.then(function(){e.target.value='';toast('Загружено: '+fs.length,'success');loadFolder();})
.catch(function(er){e.target.value='';toast(er.message,'error');});};
function renFolder(id,old){modal('<h3>Переименовать папку</h3><div class="field"><label>Новое имя</label><input id="m-name" value="'+esc(old)+'"></div><div class="macts"><button class="btn" id="m-cancel">Отмена</button><button class="btn primary" id="m-ok">Сохранить</button></div>',function(){
var n=$('#m-name').value.trim();if(!n)return;
api('/api/v1/folders/'+encodeURIComponent(id),{method:'PATCH',body:{name:n}}).then(function(){closeModal();toast('Переименовано','success');loadFolder();}).catch(function(er){toast(er.message,'error');});});}
function delFolder(id,name){confirmStyled('🗑 Переместить в корзину','Папка "'+name+'" и всё её содержимое будут перемещены в корзину.',true,function(){
api('/api/v1/folders/'+encodeURIComponent(id),{method:'DELETE'}).then(function(){toast('Папка в корзине','success');loadFolder();}).catch(function(er){toast(er.message,'error');});});}
function delFile(id,name){confirmStyled('🗑 Переместить в корзину','Файл "'+name+'" будет перемещён в корзину.',true,function(){
api('/api/v1/files/'+encodeURIComponent(id),{method:'DELETE'}).then(function(){toast('Файл в корзине','success');loadFolder();}).catch(function(er){toast(er.message,'error');});});}
function moveFile(id){var all=[];function walk(pid,d){return api('/api/v1/folders?parentID='+encodeURIComponent(pid)).then(function(r){
var seq=Promise.resolve();(r.items||[]).forEach(function(f){all.push({id:f.id,name:f.name,d:d});seq=seq.then(function(){return walk(f.id,d+1);});});return seq;});}
walk('',0).then(function(){var opts='<option value="">🏠 Корень</option>';all.forEach(function(f){opts+='<option value="'+f.id+'">'+new Array(f.d+1).join('—')+' '+esc(f.name)+'</option>';});
modal('<h3>Куда переместить</h3><div class="field"><label>Папка</label><select id="m-sel">'+opts+'</select></div><div class="macts"><button class="btn" id="m-cancel">Отмена</button><button class="btn primary" id="m-ok">Переместить</button></div>',function(){
api('/api/v1/files/'+encodeURIComponent(id)+'/move',{method:'PATCH',body:{folderID:$('#m-sel').value}}).then(function(){closeModal();toast('Перемещено','success');loadFolder();}).catch(function(er){toast(er.message,'error');});});});}
function share(type,id){api('/api/v1/share',{method:'POST',body:{resourceType:type,resourceID:id}}).then(function(l){
var url=location.origin+'/?share='+l.token;
modal('<h3>Публичная ссылка</h3><p class="mu" style="margin-bottom:10px">Работает БЕЗ входа.</p><div class="linkbox"><input id="m-url" readonly value="'+esc(url)+'"><button class="btn small primary" id="m-copy">Копировать</button></div><div class="macts"><button class="btn danger" id="m-del">Удалить ссылку</button><button class="btn" id="m-cancel">Готово</button></div>',null);
$('#m-copy').onclick=function(){if(navigator.clipboard){navigator.clipboard.writeText(url).then(function(){toast('Скопировано','success');});}else{$('#m-url').select();document.execCommand('copy');}};
$('#m-del').onclick=function(){api('/api/v1/share/'+encodeURIComponent(l.id),{method:'DELETE'}).then(function(){closeModal();toast('Ссылка удалена','success');}).catch(function(er){toast(er.message,'error');});};
}).catch(function(er){toast(er.message,'error');});}
function preview(id,ct,ft,name){
var url='/api/v1/files/'+id+'/preview';
fetch(url,{headers:{Authorization:'Bearer '+state.token}}).then(function(r){if(!r.ok)throw new Error('HTTP '+r.status);return r.blob();}).then(function(b){
var u=URL.createObjectURL(b);var body='';
if(ft==='image')body='<img src="'+u+'">';
else if(ft==='video')body='<video controls src="'+u+'"></video>';
else if(ft==='audio')body='<audio controls src="'+u+'"></audio>';
else if(ft==='document'||(ct&&ct.indexOf('pdf')>=0))body='<iframe src="'+u+'"></iframe>';
else if(ft==='code'||(ct&&(ct.indexOf('text')>=0||ct.indexOf('json')>=0)))body='<pre id="pv-pre">Загрузка...</pre>';
else body='<p class="mu">Предпросмотр для этого типа недоступен.</p><a class="btn primary" href="'+url+'?token='+encodeURIComponent(state.token)+'" download>⬇️ Скачать</a>';
modal('<h3>👁 '+esc(name)+'</h3><div class="pvbox" id="pv-box">'+body+'</div><div class="macts"><button class="btn" id="m-cancel">Закрыть</button></div>',null);
if(ft==='code'||(ct&&(ct.indexOf('text')>=0||ct.indexOf('json')>=0))){b.text().then(function(t){var pre=$('#pv-pre');if(pre)pre.textContent=t;});}
}).catch(function(er){toast('Предпросмотр: '+er.message,'error');});}
$('#trash-btn').onclick=function(){loadTrash();};
function backToFiles(){state.cur='';state.path=[];loadFolder();}
function loadTrash(){state.view='trash';crumbs();var tb=$('#rows');
tb.innerHTML='<tr><td colspan="5" class="empty">Загрузка...</td></tr>';$('#empty').classList.add('hidden');tb.onclick=null;
api('/api/v1/trash').then(function(d){var items=d.items||[];tb.innerHTML='';
if(!items.length){$('#empty').textContent='Корзина пуста';$('#empty').classList.remove('hidden');return;}
$('#empty').classList.add('hidden');
items.forEach(function(it){var tr=document.createElement('tr');
var ic=it.type==='folder'?'📁':'';
var meta=it.type==='folder'?('папка: '+(it.fileCount||0)+' файл., '+(it.folderCount||0)+' влож. папок · .zip'):('был в: '+esc(it.originalFolderName||'корень')+' · .gz');
tr.innerHTML='<td>'+ic+' '+esc(it.originalName)+'</td><td>'+meta+'</td><td>'+fmt(it.size)+'</td><td>'+fmtDate(it.deletedAt)+'</td>'+
'<td class="acts"><button class="btn small" data-a="res" data-id="'+it.id+'">♻️ Восстановить</button>'+
'<button class="btn small danger" data-a="pur" data-id="'+it.id+'" data-n="'+esc(it.originalName)+'">❌ Навсегда</button></td>';tb.appendChild(tr);});
tb.onclick=function(e){var b=e.target.closest('button');if(!b)return;var id=b.dataset.id;
if(b.dataset.a==='res'){api('/api/v1/trash/'+id+'/restore',{method:'POST'}).then(function(){toast('Восстановлено на прежнее место','success');loadTrash();}).catch(function(er){toast(er.message,'error');});}
if(b.dataset.a==='pur'){confirmStyled('❌ Удалить навсегда','"'+b.dataset.n+'" будет удалён безвозвратно.',false,function(){
api('/api/v1/trash/'+id,{method:'DELETE'}).then(function(){toast('Удалено навсегда','success');loadTrash();}).catch(function(er){toast(er.message,'error');});});}};
}).catch(function(er){tb.innerHTML='<tr><td colspan="5" class="empty">Ошибка: '+esc(er.message)+'</td></tr>';});}
function modal(html,onOk){$('#modal').innerHTML=html;$('#modal-bg').classList.remove('hidden');
$('#m-cancel').onclick=closeModal;if(onOk)$('#m-ok').onclick=onOk;}
function closeModal(){$('#modal-bg').classList.add('hidden');}
$('#modal-bg').onclick=function(e){if(e.target.id==='modal-bg')closeModal();};
function viewShare(t){$('#auth').classList.add('hidden');$('#app').classList.add('hidden');$('#shareview').classList.remove('hidden');
api('/api/v1/share/'+encodeURIComponent(t)).then(function(d){var box=$('#sharebox');
if(d.resourceType==='file'){var f=d.file;box.innerHTML='<h3>'+icon(f.fileType)+' '+esc(f.name)+'</h3><p class="mu" style="margin:8px 0">'+fmt(f.size)+'</p><a class="btn primary" href="/api/v1/share/'+encodeURIComponent(t)+'/download" download>⬇️ Скачать</a>';return;}
var h='<h3>📁 '+esc(d.folder.name)+'</h3><table style="margin-top:10px"><thead><tr><th>Имя</th><th>Размер</th><th></th></tr></thead><tbody>';
(d.items.folders||[]).forEach(function(x){h+='<tr><td>📁 '+esc(x.name)+'</td><td>—</td><td></td></tr>';});
(d.items.files||[]).forEach(function(x){h+='<tr><td>'+icon(x.fileType)+' '+esc(x.name)+'</td><td>'+fmt(x.size)+'</td><td><a class="btn small primary" href="/api/v1/share/'+encodeURIComponent(t)+'/download?fileID='+encodeURIComponent(x.id)+'" download>⬇️</a></td></tr>';});
box.innerHTML=h+'</tbody></table>';}).catch(function(er){$('#sharebox').innerHTML='<div class="empty">❌ '+esc(er.message)+'</div>';});}
(function(){var q=new URLSearchParams(location.search);var t=q.get('share');
dlog('info','App initialized',{hasToken:!!state.token,share:!!t});
if(t){viewShare(t);return;}
if(state.token)showApp();else showAuth();})();
</script></body></html>`
