(() => {
  'use strict';
  const $ = id => document.getElementById(id);
  const config = window.RESEARCH_CONFIG;
  const key = `retrotech-research:${config.reportId}`;
  const load = () => { try { return JSON.parse(localStorage.getItem(key)) || {}; } catch { return {}; } };
  let saved = load(), anchors = [], jobs = [], selected = saved.anchorId || '', parentId = '', active = null;
  let timer, busy = false, additionSignature = '', ruleSignature = '', initialized = false;
  const rules = new Map();
  const save = patch => { saved = {...saved, ...patch}; try { localStorage.setItem(key, JSON.stringify(saved)); } catch {} };
  const el = (tag, text, cls) => { const n = document.createElement(tag); if (text != null) n.textContent = text; if (cls) n.className = cls; return n; };
  const button = (text, fn) => { const b = el('button', text); b.type = 'button'; b.addEventListener('click', fn); return b; };
  const showError = message => { $('research-error').hidden = !message; $('research-error').textContent = message; };
  const openPanel = open => { document.body.classList.toggle('research-open', open); $('research-toggle').setAttribute('aria-expanded', String(open)); $('research-panel').hidden = !open; save({open}); };
  openPanel(saved.open ?? (innerWidth > 1100));
  $('research-toggle').onclick = () => openPanel($('research-panel').hidden);
  $('research-close').onclick = () => openPanel(false);
  const live = location.protocol === 'http:' && location.origin === config.origin;
  $('research-offline').hidden = live;
  $('research-connected').hidden = !live;
  $('research-live-link').href = config.origin;
  if (!live) {
    document.querySelectorAll('[data-open-job]').forEach(b => b.onclick = () => { openPanel(true); });
    return;
  }
  if ([...$('research-model').options].some(o => o.value === saved.model)) $('research-model').value = saved.model;
  if (['medium','high','xhigh'].includes(saved.effort)) $('research-effort').value = saved.effort;
  $('research-question').value = saved.draft || '';
  $('research-model').onchange = () => save({model: $('research-model').value});
  $('research-effort').onchange = () => save({effort: $('research-effort').value});
  $('research-question').oninput = () => save({draft: $('research-question').value});
  async function api(path, data) {
    const response = await fetch(path, data === undefined ? {} : {method:'POST',headers:{'Content-Type':'application/json','X-RetroTech-Request':'1'},body:JSON.stringify(data)});
    const value = await response.json();
    if (!response.ok) throw new Error(value.error || `요청 실패 (${response.status})`);
    return value;
  }
  function tab(name) {
    document.querySelectorAll('[data-tab]').forEach(b => b.setAttribute('aria-selected', String(b.dataset.tab === name)));
    ['chat','additions','skills'].forEach(t => $(`research-${t}`).hidden = name !== t);
    $('research-form').hidden = name !== 'chat';
  }
  document.querySelectorAll('[data-tab]').forEach(b => b.onclick = () => tab(b.dataset.tab));
  function select(id, scroll = false, parent = '') {
    const a = anchors.find(a => a.id === id); if (!a) return;
    selected = id; parentId = parent;
    $('research-chapter').value = a.chapterId;
    $('research-selected').textContent = $(id)?.textContent || a.text;
    $('research-clear-parent').hidden = !parentId;
    document.querySelectorAll('[data-research-anchor]').forEach(p => p.classList.toggle('research-target', p.id === id));
    if (scroll) $(id).scrollIntoView({behavior:'smooth', block:'center'});
    save({anchorId:id}); renderMessages();
  }
  $('research-clear-parent').onclick = () => select(selected);
  $('research-chapter').onchange = () => select(anchors.find(a => a.chapterId === $('research-chapter').value)?.id, true);
  $('research-all-chats').onchange = renderMessages;
  document.querySelector('article.prose').addEventListener('click', event => {
    if (event.target.closest('a,button')) return;
    const p = event.target.closest('[data-research-anchor]'); if (p) select(p.id);
  });
  document.querySelector('article.prose').addEventListener('focusin', event => {
    if (event.target.matches('[data-research-anchor]')) select(event.target.id);
  });
  document.addEventListener('click', event => {
    const b = event.target.closest('[data-open-job]'); if (!b) return;
    const j = jobs.find(j => j.id === b.dataset.openJob); if (!j) return;
    select(j.request.anchorId, false, j.id); openPanel(true); tab('chat');
    $(`research-message-${j.id}`)?.scrollIntoView({block:'nearest'});
  });
  function follow(j) { select(j.request.anchorId, false, j.id); tab('chat'); openPanel(true); $('research-question').focus(); }
  function jump(j) { const target = $(`research-add-${j.id}`) || $(j.request.anchorId); target?.scrollIntoView({behavior:'smooth',block:'start'}); if(innerWidth <= 1100) openPanel(false); }
  function renderMessages() {
    const box = $('research-messages'), previousScroll = box.scrollTop;
    const a = anchors.find(a => a.id === selected);
    const visible = jobs.filter(j => $('research-all-chats').checked || j.chapterId === a?.chapterId);
    box.replaceChildren();
    if (!visible.length) box.append(el('p','이 장면에서 궁금한 것을 물어보세요. 답변과 원문 출처가 본문 옆에 쌓입니다.','research-empty'));
    for (const j of visible) {
      const card = el('div',null,'research-message'); card.id = `research-message-${j.id}`;
      card.append(el('div',j.request.question,'question'));
      const reply = el('div',null,'answer');
      const label = {running:'조사 중',queued:'대기 중',failed:'조사 실패',cancelled:'취소됨',completed:'본문 저장 완료'}[j.status];
      reply.append(el('small',`${label} · ${j.request.model} / ${j.request.effort}`));
      reply.append(el('p', j.error || j.result?.answer || j.progress));
      const actions = el('div',null,'message-actions');
      actions.append(button('원래 장면', () => { select(j.request.anchorId,true); if(innerWidth<=1100)openPanel(false); }));
      if (j.status === 'completed') {
        actions.append(button(j.hidden ? '추가 내용은 숨김 상태' : '본문의 추가 내용',() => jump(j)));
        actions.append(button('이어서 질문',() => follow(j)));
      } else if (j.status === 'failed' || j.status === 'cancelled') {
        actions.append(button('질문 다시 작성',() => {select(j.request.anchorId); $('research-question').value=j.request.question;save({draft:j.request.question});$('research-question').focus();}));
      }
      reply.append(actions); card.append(reply); box.append(card);
    }
    box.scrollTop = previousScroll;
  }
  function readingMark() {
    const nodes = [...document.querySelectorAll('[data-research-anchor],.research-addition')];
    const n = nodes.find(n => n.getBoundingClientRect().bottom > 0);
    return n ? {id:n.id,top:n.getBoundingClientRect().top} : null;
  }
  function restoreMark(mark) { const n = mark && $(mark.id); if(n) scrollTo({top:scrollY+n.getBoundingClientRect().top-mark.top,behavior:'instant'}); }
  function renderAdditions() {
    const completed = jobs.filter(j => j.status === 'completed');
    $('research-count').textContent = completed.filter(j => !j.hidden).length;
    const signature = JSON.stringify(completed.map(j => [j.id,j.hidden,j.updatedAt]));
    if(signature === additionSignature) return;
    additionSignature = signature;
    const mark = readingMark();
    document.querySelectorAll('.research-addition').forEach(n => n.remove());
    const tails = new Map();
    for(const j of completed.filter(j => !j.hidden && j.html)) {
      const template = document.createElement('template');
      // Only server-rendered, escaped report markup enters this HTML sink.
      template.innerHTML = j.html;
      const node = template.content.firstElementChild, target = tails.get(j.request.anchorId) || $(j.request.anchorId);
      if(node && target) { target.after(node); tails.set(j.request.anchorId,node); }
    }
    restoreMark(mark);
    const box = $('research-addition-list'); box.replaceChildren();
    if(!completed.length)box.append(el('p','완료된 조사가 여기에 모입니다.','research-empty'));
    for(const j of completed) {
      const card = el('div',null,'research-addition-card');
      card.append(el('strong',j.result.title),el('p',j.request.question));
      card.append(button('내용으로 이동',()=>jump(j)),button('원래 장면',()=>{select(j.request.anchorId,true);if(innerWidth<=1100)openPanel(false);}));
      card.append(button(j.hidden?'본문에 다시 표시':'본문에서 숨기기',async()=>{try{await api('/api/visibility',{id:j.id,hidden:!j.hidden});await refresh();}catch(e){showError(e.message);}}));
      box.append(card);
    }
  }
  function renderRules() {
    const candidates = jobs.filter(j => j.status === 'completed' && j.result.learning.rule);
    const signature = JSON.stringify(candidates.map(j=>[j.id,j.appliedRule]));
    if(signature===ruleSignature)return;ruleSignature=signature;
    const box=$('research-rule-list');box.replaceChildren();
    if(!candidates.length)box.append(el('p','추가 조사에서 발견한 빈틈과 다음 조사의 점검 질문이 여기에 모입니다.','research-empty'));
    for(const j of candidates) {
      const entry=rules.get(j.id)||{rule:saved.rules?.[j.id]||j.result.learning.rule,selected:false};rules.set(j.id,entry);
      const card=el('div',null,'research-rule-card'), label=el('label'), check=document.createElement('input');
      check.type='checkbox';check.checked=entry.selected;
      check.onchange=()=>entry.selected=check.checked;
      label.append(check,document.createTextNode(` ${j.result.title}`));
      const area=document.createElement('textarea');area.rows=4;area.maxLength=1200;area.value=entry.rule;area.setAttribute('aria-label',`${j.result.title} 조사 규칙`);
      area.oninput=()=>{entry.rule=area.value;save({rules:Object.fromEntries([...rules].map(([id,v])=>[id,v.rule]))});};
      card.append(label,el('p',`발견한 빈틈 · ${j.result.learning.gap}`),area);
      if(j.appliedRule)card.append(el('small','스킬에 반영됨 · 규칙을 고쳐 추가 반영할 수도 있습니다.'));
      box.append(card);
    }
  }
  $('research-apply-skills').onclick = async () => {
    const selections=[...rules].filter(([,v])=>v.selected).map(([jobId,v])=>({jobId,rule:v.rule}));
    if(!selections.length){$('research-skill-status').textContent='반영할 규칙을 먼저 선택해 주세요.';return;}
    $('research-apply-skills').disabled=true;
    try {const result=await api('/api/skills/apply',{rules:selections});for(const v of rules.values())v.selected=false;$('research-skill-status').textContent=`${result.applied}개 규칙을 로컬 RetroTech 스킬에 반영했습니다. 이전 스킬은 보고서의 .research/skill-backups에 보관했습니다.`;await refresh();}
    catch(e){showError(e.message);}finally{$('research-apply-skills').disabled=false;}
  };
  $('research-form').onsubmit = async event => {
    event.preventDefault();if(busy||active)return;
    busy=true;$('research-send').disabled=true;showError('');
    const [provider,model]=$('research-model').value.split(':');
    try {await api('/api/jobs',{anchorId:selected,question:$('research-question').value,provider,model,effort:$('research-effort').value,parentId});$('research-question').value='';save({draft:''});await refresh();}
    catch(e){showError(e.message);}finally{busy=false;$('research-send').disabled=!!active;}
  };
  $('research-cancel').onclick=async()=>{if(!active)return;try{await api('/api/cancel',{id:active.id});$('research-run-status').textContent='조사를 취소하고 있습니다.';}catch(e){showError(e.message);}};
  async function refresh() {
    clearTimeout(timer);
    try {
      const data=await api('/api/state');anchors=data.anchors;jobs=data.state.jobs;
      if(!initialized){
        const chapters=new Map(anchors.map(a=>[a.chapterId,a.chapterTitle]));
        for(const [id,title]of chapters){const option=el('option',title);option.value=id;$('research-chapter').append(option);}
        for(const option of $('research-model').options)option.disabled=!data.providers[option.value.split(':')[0]];
        select(anchors.some(a=>a.id===selected)?selected:anchors[0]?.id);
      }
      active=jobs.find(j=>j.status==='running'||j.status==='queued')||null;
      $('research-send').disabled=busy||!!active;
      $('research-cancel').hidden=!active;
      $('research-run-status').textContent=active?active.progress:'결과는 선택한 문단 뒤에 추가됩니다.';
      renderMessages();renderAdditions();renderRules();
      if(!initialized){initialized=true;if(!location.hash && saved.reading)restoreMark(saved.reading);}
      timer=setTimeout(refresh,active?1500:4000);
    } catch(e){showError(`로컬 서버 연결을 확인해 주세요. ${e.message}`);timer=setTimeout(refresh,8000);}
  }
  let scrollTimer;
  addEventListener('scroll',()=>{clearTimeout(scrollTimer);scrollTimer=setTimeout(()=>{if(initialized)save({reading:readingMark()});},200);},{passive:true});
  refresh();
})();
