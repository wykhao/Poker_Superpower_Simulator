// Optional real-browser smoke test. Never uses the player's normal data folder.
const assert = require('node:assert/strict');
const fs = require('node:fs/promises');
const os = require('node:os');
const path = require('node:path');
const {spawn} = require('node:child_process');
const {chromium} = require(process.env.PLAYWRIGHT_MODULE || 'playwright');

async function main(){
  const root=path.resolve(__dirname,'..');
  const data=await fs.mkdtemp(path.join(os.tmpdir(),'poker-human-ui-'));
  const output=path.join(root,'test-results');await fs.mkdir(output,{recursive:true});
  const executable=process.env.POKER_BINARY||path.join(root,'bin',process.platform==='win32'?'poker-simulator.exe':'poker-simulator');
  let child,browser;
  async function start(){
    child=spawn(executable,['--no-browser','--port','0','--data-dir',data],{windowsHide:true,stdio:['ignore','pipe','pipe']});
    return await new Promise((resolve,reject)=>{let text='';const timeout=setTimeout(()=>reject(new Error('server startup timeout: '+text)),15000);child.stdout.on('data',b=>{text+=b;const m=text.match(/Open: (http:\/\/127\.0\.0\.1:\d+)/);if(m){clearTimeout(timeout);resolve(m[1]);}});child.stderr.on('data',b=>text+=b);child.on('error',reject);child.on('exit',code=>{if(code)reject(new Error('server exit '+code+': '+text));});});
  }
  async function stop(){if(!child||child.exitCode!==null)return;const p=new Promise(r=>child.once('exit',r));child.kill();await p;}
  try{
    let url=await start();
    browser=await chromium.launch({headless:true,...(process.env.BROWSER_CHANNEL?{channel:process.env.BROWSER_CHANNEL}:{})});
    const page=await browser.newPage({viewport:{width:1440,height:1080},deviceScaleFactor:1});
    const errors=[];page.on('pageerror',e=>errors.push(e.message));page.on('console',m=>{if(m.type()==='error')errors.push(m.text());});
    await page.goto(url);await page.waitForFunction(()=>document.getElementById('next-hand').disabled===false);
    await page.screenshot({path:path.join(output,'desktop-ready.png'),fullPage:true});
    const getState=async()=>await (await page.request.get(url+'/api/state')).json();
    const waitIdle=async()=>{await page.waitForFunction(()=>{const n=document.getElementById('next-hand');const c=document.getElementById('call');return (!n.disabled&&!document.getElementById('ready-actions').hidden)||(!c.disabled&&!document.getElementById('human-actions').hidden);});};
    const live=async mode=>{for(let i=0;i<25;i++){const s=await getState();if(s.current&&!s.current.done){assert.equal(s.current.mode,mode);return s.current;}await page.locator('#mode-'+mode.toLowerCase()).click();await page.waitForFunction(()=>!document.getElementById('next-hand').disabled);await page.locator('#next-hand').click();await waitIdle();}throw new Error('no live hand');};
    let current=await live('A');
    assert.equal(current.future_board,undefined);
    assert.equal(await page.locator('#seats .card.back').count(),0);
    assert.equal(await page.locator('#future-panel').isVisible(),false);
    await page.screenshot({path:path.join(output,'desktop-A.png'),fullPage:true});
    const beforeID=current.id;
    await page.reload();await waitIdle();assert.equal((await getState()).current.id,beforeID);
    // Real process restart, not just a browser refresh.
    await stop();url=await start();await page.goto(url);await waitIdle();assert.equal((await getState()).current.id,beforeID);
    current=(await getState()).current;
    if(current.legal.can_raise){
      await page.locator('#raise-amount').fill('9999');assert.equal(await page.locator('#raise').isDisabled(),true);
      const target=current.legal.min_raise_to;
      await page.locator('#raise-amount').fill(String(target/10));await page.locator('#raise').click();await waitIdle();
      assert.ok((await getState()).current.events.some(e=>e.seat===0&&e.raise_to===target));
    }
    async function finish(){for(let i=0;i<60;i++){const s=await getState();if(s.current.done)return s;await page.locator('#call').click();await waitIdle();}throw new Error('too many human turns');}
    let s=await finish();assert.ok(s.stats.A.hands>=1);assert.equal(s.stats.B.hands,0);const aCount=s.stats.A.hands;
    current=await live('B');assert.equal(current.future_board.length,5);assert.equal(await page.locator('#seats .card.back').count(),10);assert.equal(await page.locator('#future-cards .card').count(),5);
    for(let i=1;i<6;i++)assert.equal(current.seats[i].hole,undefined);
    await page.screenshot({path:path.join(output,'desktop-B.png'),fullPage:true});
    await page.setViewportSize({width:390,height:844});await page.screenshot({path:path.join(output,'mobile-B.png'),fullPage:true});
    assert.equal(await page.evaluate(()=>document.documentElement.scrollWidth>innerWidth),false,'mobile horizontal overflow');
    await page.locator('#allin').click();await waitIdle();s=await getState();assert.equal(s.current.done,true);assert.equal(s.stats.A.hands,aCount);assert.ok(s.stats.B.hands>=1);
    await page.locator('[data-review]').first().click();await page.locator('#review').waitFor({state:'visible'});assert.equal(await page.locator('.review-player .card.back').count(),0);await page.locator('#review-close').click();
    const csv=await page.request.get(url+'/api/export?mode=B');assert.match(await csv.text(),/hand_id,ability/);
    await live('A');const beforeFold=(await getState()).stats.A.hands;await page.locator('#fold').click();await waitIdle();assert.equal((await getState()).stats.A.hands,beforeFold+1);
    await page.locator('#new-session').click();await waitIdle();const session=await (await page.request.get(url+'/api/state?scope=session')).json();assert.equal(session.stats.A.hands,0);assert.equal(session.stats.B.hands,0);
    assert.deepEqual(errors,[],'browser console or script errors');
    const result={passed:true,checks:['A hole-card visibility','B future board and hidden opponents','manual human controls','refresh resume','process restart resume','separate A/B statistics','completed-hand review','CSV export','session reset preserves history','desktop/mobile layouts','no browser errors'],data_directory:data};
    await fs.writeFile(path.join(output,'browser-results.json'),JSON.stringify(result,null,2));console.log(JSON.stringify(result,null,2));
  } finally {if(browser)await browser.close();await stop();}
}
main().catch(e=>{console.error(e);process.exitCode=1;});
