'use strict';
const $ = id => document.getElementById(id);
const suit = {c:'♣',d:'♦',h:'♥',s:'♠'};
const suitName = {c:'梅花',d:'方块',h:'红桃',s:'黑桃'};
const streetName = {preflop:'翻前',flop:'翻牌',turn:'转牌',river:'河牌',showdown:'已结束'};
const handName = {'High Card':'高牌','Pair':'一对','Two Pair':'两对','Three of a Kind':'三条','Straight':'顺子','Flush':'同花','Full House':'葫芦','Four of a Kind':'四条','Straight Flush':'同花顺'};
const abilityName = {A:'底牌透视',B:'预知公牌'};
let state = null, mode = 'A', scope = 'all', busy = false, initialized = false, historyOffset = 0;

function card(c, extra='') {
  if (!c || !/^[2-9TJQKA][cdhs]$/.test(c)) return `<span class="card back ${extra}" aria-label="隐藏底牌">♠</span>`;
  const red=c[1]==='h'||c[1]==='d';
  return `<span class="card ${red?'red':''} ${extra}" aria-label="${suitName[c[1]]}${c[0]}"><span class="rank">${c[0]==='T'?'10':c[0]}</span><span class="suit">${suit[c[1]]}</span></span>`;
}
const cardRow = cards => (cards||[]).map(c=>card(c)).join('');
const fmt = n => Number(n).toLocaleString('en-US',{minimumFractionDigits:1,maximumFractionDigits:1});
const signed = n => `${n>0?'+':''}${fmt(n)}`;
const netClass = n => n>0?'positive':n<0?'negative':'';
const bb = chips => fmt(chips/10);
function notify(text,error=false){$('notice').textContent=text;$('notice').classList.toggle('error',error);$('notice').hidden=!text;}
function readableError(message){
  const pairs=[['this turn has changed','牌局状态已更新，已为你刷新，请重新选择动作。'],['finish or fold the current hand','请先结束当前手牌，再切换能力开始下一手。'],['finish the current hand','请先结束当前手牌，再开始新会话。'],['save failed','保存失败，本次动作没有提交，请检查磁盘空间后重试。'],['raise must be between','加注金额超出当前合法范围，请调整后重试。']];
  return pairs.find(([key])=>message.includes(key))?.[1]||message;
}
async function api(path,body){
  const response=await fetch(`${path}${path.includes('?')?'&':'?'}mode=${mode}&scope=${scope}&offset=${historyOffset}`,body===undefined?{cache:'no-store'}:{method:'POST',headers:{'Content-Type':'application/json','X-Poker-Request':'1'},body:JSON.stringify(body)});
  const text=await response.text();let data;try{data=JSON.parse(text);}catch{data={error:text};}
  if(!response.ok)throw new Error(readableError(data.error||'请求失败'));
  return data;
}
async function load(){
  try{
    const next=await api('/api/state');
    if(!initialized){mode=next.current&&!next.current.done?next.current.mode:next.preferred_mode;initialized=true;if(mode!=='A'){render(await api('/api/state'));return;}}
    render(next);
  }catch(e){notify(`无法连接本地应用：${e.message}`,true);}
}
async function mutate(path,body){
  if(busy)return;busy=true;updateDisabled();notify('');
  try{render(await api(path,body));}catch(e){notify(e.message,true);await load();}
  finally{busy=false;updateDisabled();}
}
function current(){return state?.current;}
function updateDisabled(){
  const c=current(),live=c&&!c.done,legal=c?.legal;
  $('next-hand').disabled=busy||!!live;
  for(const m of ['a','b'])$(`mode-${m}`).disabled=busy||!!live;
  $('new-session').disabled=busy||!!live;
  $('scope').disabled=busy;
  $('history-newer').disabled=busy||!state||state.history_offset===0;
  $('history-older').disabled=busy||!state||state.history_offset+30>=state.total_history;
  $('fold').disabled=busy||!legal;
  $('call').disabled=busy||!legal;
  $('allin').disabled=busy||!legal||(!legal.actions.includes('allin')&&!legal.actions.includes('raise'));
  const amount=Math.round(Number($('raise-amount').value)*10);
  const canRaise=!!legal?.can_raise;
  $('raise').disabled=busy||!canRaise||!Number.isFinite(amount)||amount<legal.min_raise_to||amount>legal.max_raise_to;
  $('raise-amount').disabled=busy||!canRaise;$('raise-slider').disabled=busy||!canRaise;
  document.querySelectorAll('[data-pot]').forEach(b=>b.disabled=busy||!canRaise);
}
function render(next){
  state=next;const c=current();if(c&&!c.done)mode=c.mode;
  document.body.dataset.mode=mode;
  document.body.dataset.seated=c?'true':'false';
  for(const m of ['A','B']){const b=$(`mode-${m.toLowerCase()}`);b.classList.toggle('selected',mode===m);b.setAttribute('aria-pressed',String(mode===m));}
  $('hand-label').textContent=c?`第 ${c.id.toLocaleString()} 手${c.done?' · 已完成':''}`:'准备入座';
  $('street-label').textContent=c?streetName[c.street]||c.street:"NO LIMIT HOLD'EM";
  $('ability-label').textContent=`${c?.mode||mode} · ${abilityName[c?.mode||mode]}`;
  $('pot').innerHTML=`${bb(c?.pot||0)} <small>BB</small>`;
  $('board').innerHTML=Array.from({length:5},(_,i)=>c?.board?.[i]?card(c.board[i]):'<span class="card empty" aria-label="未发出的公牌"></span>').join('');
  $('future-panel').hidden=!c?.future_board;
  if(c?.future_board){$('future-cards').innerHTML=c.future_board.map((x,i)=>card(x,i<c.board.length?'future-dealt':'')).join('');$('future-hand').textContent=handName[c.future_hand]||c.future_hand;$('future-label').textContent=c.done?'本手预定公牌 · 复盘':'B 的未来视野';}
  $('ability-note').textContent=c?.done?'本手已结束，全部底牌仅供复盘。':mode==='A'?'A 模式只显示底牌，公牌随牌局揭晓。':'B 模式预知公牌，对手底牌保持隐藏。';
  $('hero-hand').textContent=c?.current_hand?`当前牌型 · ${handName[c.current_hand]||c.current_hand}`:'你的每一步，都由你决定。';
  $('table-message').textContent=!c?'准备好后，开始第一手':c.done?`${c.net>0?'这一手赢了':c.net<0?'这一手亏损':'这一手持平'} ${fmt(Math.abs(c.net)/10)} BB`:'等待你的决定';
  const seats=c?.seats||Array.from({length:6},(_,i)=>({seat:i,name:i===0?'You':`Bot ${i}`,chips:1000,hole:[],folded:false,active:false,bet:0}));
  $('seats').innerHTML=seats.map(s=>{
    const last=c?.events?.filter(e=>e.seat===s.seat).at(-1);
    let status=s.folded?'已弃牌':s.all_in?'ALL IN':last?actionText(last):'';
    if(c?.done)status=`${signed(s.net/10)} BB`;
    return `<div class="seat seat-${s.seat} ${s.seat===0?'hero':''} ${s.folded?'folded':''} ${s.active?'active':''}" data-seat="${s.seat}"><div class="seat-cards">${card(s.hole?.[0])}${card(s.hole?.[1])}</div><div class="seat-box">${s.position?`<span class="position-tag ${s.position==='D'?'dealer':''}">${s.position}</span>`:''}<div class="seat-name"><b>${s.seat===0?'你 · HUMAN':s.name}</b></div><div class="seat-chips">${bb(s.chips)} <small>BB</small></div>${s.bet?`<span class="bet-pill">${bb(s.bet)}</span>`:''}</div><div class="seat-status ${c?.done?netClass(s.net):''}">${status}</div></div>`;
  }).join('');
  $('ready-actions').hidden=!!(c&&!c.done);$('human-actions').hidden=!c||c.done;
  if(c?.done){$('result-title').textContent=c.net>0?`净赢 +${bb(c.net)} BB`:c.net<0?`净亏 ${bb(-c.net)} BB`:'本手净收益 0.0 BB';$('result-title').className=netClass(c.net);$('result-subtitle').textContent='已计入历史成绩。下一手恢复 100 BB。';$('next-hand').innerHTML=`下一手 · ${mode} <span>→</span>`;}
  else if(!c){$('result-title').textContent='轮到你掌控牌局';$('result-subtitle').textContent='盲注 0.5 / 1 BB，每手恢复 100 BB。';$('next-hand').innerHTML=`开始第一手 · ${mode} <span>→</span>`;}
  if(c?.legal){
    const l=c.legal;$('call-info').textContent=l.to_call?`跟注需 ${bb(l.to_call)} BB`:'当前可以免费过牌';
    $('call').innerHTML=`${l.to_call?`跟注 ${bb(l.to_call)}`:'过牌'} <kbd>C</kbd>`;
    $('raise-amount').min=String(l.min_raise_to/10);$('raise-amount').max=String(l.max_raise_to/10);$('raise-slider').min=String(Math.min(l.min_raise_to,l.max_raise_to)/10);$('raise-slider').max=String(l.max_raise_to/10);
    setRaise(Math.min(l.max_raise_to,Math.max(l.min_raise_to,l.bet+l.to_call+Math.round((c.pot+l.to_call)*.5))));
  }
  $('log-count').textContent=String(c?.events?.length||0);$('log-list').innerHTML=c?.events?.length?logs(c.events):'<p class="empty-copy">机器人行动会显示在这里。</p>';
  $('log-list').scrollTop=$('log-list').scrollHeight;
  renderStats();renderHistory();updateDisabled();
}
function actionText(e){const amount=e.raise_to||e.paid;switch(e.action){case'fold':return'弃牌';case'check':return'过牌';case'call':return e.paid?`跟注 ${bb(e.paid)}`:'过牌';case'raise':return`加注至 ${bb(amount)}`;case'allin':return`全下 ${bb(e.paid)}`;default:return e.action;}}
function logs(events){return events.map(e=>`<div class="log-entry ${e.seat===0?'human':''}"><span class="log-street">${streetName[e.street]}</span><span class="log-actor">${e.seat===0?'你':`Bot ${e.seat}`}</span><span class="log-text">${actionText(e)}${e.paid?' BB':''}</span></div>`).join('');}
function renderStats(){
  const s=state.stats[mode];$('stats-badge').textContent=mode;$('stats-mode-title').textContent=abilityName[mode];$('stats-hands').textContent=`${s.hands.toLocaleString()} 手已完成`;
  $('net-bb').innerHTML=`${signed(s.net_bb)} <small>BB</small>`;$('net-bb').className=netClass(s.net_bb);
  $('bb100').textContent=s.hands?signed(s.bb100):'—';$('bb100').className=netClass(s.bb100);$('profit-rate').textContent=s.hands?`${fmt(s.profit_rate)}%`:'—';$('wins').textContent=`${s.profitable.toLocaleString()} 手盈利`;
  $('vpip-pfr').textContent=s.hands?`${fmt(s.vpip)}% / ${fmt(s.pfr)}%`:'— / —';$('best').textContent=s.hands?`${signed(s.best_bb)} BB`:'—';$('best').className=netClass(s.best_bb);$('worst').textContent=s.hands?`${signed(s.worst_bb)} BB`:'—';$('worst').className=netClass(s.worst_bb);
  for(const m of ['A','B']){const x=state.stats[m];$(`compare-${m.toLowerCase()}`).innerHTML=`<span>${m}</span><span>${abilityName[m]}</span><span class="compare-hands">${x.hands} 手</span><strong class="${netClass(x.net_bb)}">${signed(x.net_bb)} BB</strong>`;}
  drawChart(s);
}
function drawChart(s){
  $('chart-empty').hidden=s.hands>0;
  if(!s.hands){$('chart').innerHTML='<path d="M10 115H290M10 75H290M10 35H290" stroke="#2b3b2b" stroke-dasharray="3 5" fill="none"/>';return;}
  const values=s.curve.map(p=>p.net_bb),lo=Math.min(0,...values),hi=Math.max(0,...values),span=hi-lo||2,pad=span*.15;
  const x=p=>12+276*p.hand/Math.max(1,s.hands),y=n=>124-(n-lo+pad)/(span+pad*2)*108;
  const line=s.curve.map((p,i)=>`${i?'L':'M'}${x(p).toFixed(2)} ${y(p.net_bb).toFixed(2)}`).join(' '),zero=y(0),color=mode==='A'?'#e8bc70':'#bba4ee';
  $('chart').innerHTML=`<defs><linearGradient id="area" x1="0" y1="0" x2="0" y2="1"><stop offset="0%" stop-color="${color}" stop-opacity=".2"/><stop offset="100%" stop-color="${color}" stop-opacity="0"/></linearGradient></defs><path d="M12 ${zero}H288" stroke="#456047" stroke-dasharray="3 5"/><path d="${line} L288 ${zero} L12 ${zero}Z" fill="url(#area)"/><path d="${line}" fill="none" stroke="${color}" stroke-width="2.1" stroke-linejoin="round"/><circle cx="288" cy="${y(s.net_bb)}" r="3.5" fill="${color}"/><text x="12" y="141" fill="#789079" font-size="8">第 1 手</text><text x="288" y="141" fill="#789079" font-size="8" text-anchor="end">第 ${s.hands} 手</text><text x="12" y="10" fill="#91a78f" font-size="8">${signed(hi)} BB</text>`;
}
function renderHistory(){
  historyOffset=state.history_offset;
  $('history-mode').textContent=mode;$('history-count').textContent=state.total_history?`第 ${historyOffset+1}–${historyOffset+state.history.length} / ${state.total_history} 手`:'暂无记录';$('history-empty').hidden=state.history.length>0;
  $('history-page').textContent=`第 ${Math.floor(historyOffset/30)+1} / ${Math.max(1,Math.ceil(state.total_history/30))} 页`;
  $('history-body').innerHTML=state.history.map(h=>`<tr><td>#${h.id}</td><td>${new Date(h.completed_at).toLocaleString('zh-CN',{month:'2-digit',day:'2-digit',hour:'2-digit',minute:'2-digit'})}</td><td><div class="history-cards">${cardRow(h.hole)}</div></td><td><div class="history-cards">${h.board.length?cardRow(h.board):'<span class="muted">翻前结束</span>'}</div></td><td class="${netClass(h.net_bb)}">${signed(h.net_bb)} BB</td><td><button class="review-button" data-review="${h.id}">复盘 ↗</button></td></tr>`).join('');
}
function setRaise(chips){const l=current()?.legal;if(!l)return;const n=Math.max(Math.min(l.min_raise_to,l.max_raise_to),Math.min(l.max_raise_to,Math.round(chips)));$('raise-amount').value=(n/10).toFixed(1);$('raise-slider').value=String(n/10);updateDisabled();}
function act(action,amount=0){const c=current();if(!c||c.done||busy)return;mutate('/api/action',{hand_id:c.id,revision:c.revision,action,amount});}
async function refresh(){if(busy)return;busy=true;updateDisabled();try{await load();}finally{busy=false;updateDisabled();}}
for(const m of ['A','B'])$(`mode-${m.toLowerCase()}`).addEventListener('click',async()=>{if(busy||current()&&!current().done)return;mode=m;historyOffset=0;await refresh();});
$('next-hand').addEventListener('click',()=>{historyOffset=0;mutate('/api/start',{mode});});
$('fold').addEventListener('click',()=>act('fold'));$('call').addEventListener('click',()=>act('call'));$('allin').addEventListener('click',()=>act('allin'));
$('raise').addEventListener('click',()=>{const n=Number($('raise-amount').value)*10;if(!Number.isFinite(n)||Math.abs(n-Math.round(n))>1e-6){notify('金额以 0.1 BB 为单位。',true);return;}act('raise',Math.round(n));});
$('raise-amount').addEventListener('input',()=>{$('raise-slider').value=$('raise-amount').value;updateDisabled();});$('raise-slider').addEventListener('input',()=>setRaise(Number($('raise-slider').value)*10));
document.querySelectorAll('[data-pot]').forEach(b=>b.addEventListener('click',()=>{const c=current(),l=c?.legal;if(!l)return;setRaise(l.bet+l.to_call+Number(b.dataset.pot)*(c.pot+l.to_call));}));
$('scope').addEventListener('change',async()=>{if(busy)return;scope=$('scope').value;historyOffset=0;await refresh();});
$('history-newer').addEventListener('click',async()=>{historyOffset=Math.max(0,historyOffset-30);await refresh();});
$('history-older').addEventListener('click',async()=>{historyOffset+=30;await refresh();});
$('new-session').addEventListener('click',()=>{scope='session';historyOffset=0;$('scope').value=scope;mutate('/api/session',{});});
$('export-csv').addEventListener('click',()=>{window.location.href=`/api/export?mode=${mode}&scope=${scope}&format=csv`;});
$('export-json').addEventListener('click',()=>{window.location.href='/api/export?mode=all&format=json';});
$('history-body').addEventListener('click',async e=>{
  const b=e.target.closest('[data-review]');if(!b)return;
  try{const h=await api(`/api/hand/${Number(b.dataset.review)}`),v=h.view;$('review-title').textContent=`#${h.id} · ${h.mode} ${abilityName[h.mode]} · ${signed(h.net_chips[0]/10)} BB`;
    $('review-content').innerHTML=`<p class="review-meta">${new Date(h.completed_at).toLocaleString('zh-CN')} · 本手已完成，底牌全部公开。${h.showdown?'你参与了摊牌。':'你未参与摊牌。'}</p><div class="review-board"><span>实际公牌</span><div class="cards">${v.board.length?cardRow(v.board):'翻前结束，未发出公牌'}</div></div><div class="review-board"><span>预定公牌</span><div class="cards">${cardRow(v.future_board)}</div></div><div class="review-players">${v.seats.map(s=>`<div class="review-player"><p><span>${s.seat===0?'你':s.name}${s.folded?' · 弃牌':''}</span><span class="${netClass(s.net)}">${signed(s.net/10)} BB</span></p><div class="cards">${cardRow(s.hole)}</div></div>`).join('')}</div><div class="review-log">${logs(v.events)}</div>`;$('review').showModal();
  }catch(err){notify(err.message,true);}
});
$('review-close').addEventListener('click',()=>$('review').close());$('review').addEventListener('click',e=>{if(e.target===$('review')){const r=$('review').getBoundingClientRect();if(e.clientX<r.left||e.clientX>r.right||e.clientY<r.top||e.clientY>r.bottom)$('review').close();}});
$('quit').addEventListener('click',async()=>{if(busy)return;try{await api('/api/quit',{});document.body.classList.add('offline');notify('进度已保存，应用已退出。你可以关闭此页面，下次启动会继续当前手牌。');$('quit').disabled=true;}catch(e){notify(e.message,true);}});
document.addEventListener('keydown',e=>{if(e.repeat||busy||$('review').open||/INPUT|SELECT|TEXTAREA/.test(e.target.tagName)||e.ctrlKey||e.metaKey||e.altKey)return;const key=e.key.toLowerCase();if(key==='f'){$('fold').click();}else if(key==='c'){$('call').click();}else if(key==='r'){$('raise-amount').focus();}});
refresh();
