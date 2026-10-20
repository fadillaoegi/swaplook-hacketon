import { useCallback, useEffect, useRef, useState } from 'react'
import type { CSSProperties, FormEvent, ReactNode } from 'react'
import { ArrowDown, ArrowLeft, ArrowRight, ArrowUpRight, Camera, Check, CheckCheck, Eye, ImageUp, Users, ChevronDown, CircleCheck, CircleHelp, Clock3, Columns2, Leaf, LoaderCircle, LockKeyhole, Minus, PackageCheck, PackageX, RefreshCw, ShieldCheck, SlidersHorizontal, Sparkles, Store, Trash2, X } from 'lucide-react'
import type { SwapLook } from '../application/swaplook'
import type { Candidate, Constraints, Decision, Job, ModelInfo, Photo, Product, Quota, Shortlist, Size, State, Summary } from '../domain/models'
import { money, sizes } from '../domain/models'
import { APIError } from '../application/gateway'
import { Aurora, Burst, HeroStage, Reveal, Tilt, useCountUp, useTilt } from './motion'
import { photoCredits } from './credits'

const initial: Constraints = { original_id: 'SL-001', size: 'M', budget: 6000, color: '', material: '' }
const message = (e: unknown) => e instanceof Error ? e.message : 'Something went wrong. Please try again.'

export function App({ app }: { app: SwapLook }) {
  const [catalog, setCatalog] = useState<Product[]>([])
  const [renderer, setRenderer] = useState('')
  const [error, setError] = useState('')
  const [ready, setReady] = useState(false)
  const [help, setHelp] = useState(false)
  const merchant = location.pathname === '/merchant'
  const credits = location.pathname === '/credits'
  const load = useCallback(async () => {
    setError('')
    try { const data = await app.initialize(); setCatalog(data.catalog); setRenderer(data.renderer); setReady(true) }
    catch (e) { setError(message(e)) }
  }, [app])
  useEffect(() => { void load() }, [load])
  return <div className="app-shell">
    <Aurora/>
    <header className="header"><a className="wordmark" href="/" aria-label="SwapLook home"><img src="/brand/swaplook-logo.svg" alt="SwapLook" width="310" height="72"/></a>
      <nav><button className="nav-link" onClick={() => setHelp(true)}>How it works <CircleHelp size={14}/></button><span className="nav-divider"/><a className="merchant-link" href={merchant || credits ? '/' : '/merchant'}>{merchant || credits ? <ArrowLeft size={15}/> : <Store size={15}/>}<span>{merchant || credits ? 'Back to boutique' : 'Merchant studio'}</span><ArrowUpRight size={14}/></a></nav>
    </header>
    <div className="demo-banner"><span className="status-dot"/>THE SMALL HOURS · DEMO BOUTIQUE<span className="banner-right">{renderer === 'simulator' ? 'Photo catalog · illustrated previews (AI off)' : renderer ? 'Photo catalog · YouCam AI try-on previews' : 'Connecting to the boutique…'}</span></div>
    {credits ? <Credits/> : !ready ? <main className="startup"><Leaf size={35}/><h1>A little something good.</h1>{error ? <><Notice>{error}</Notice><button className="btn primary" onClick={() => void load()}>Reconnect <RefreshCw size={16}/></button></> : <p><LoaderCircle className="spin" size={16}/> Opening the boutique…</p>}</main> : merchant ? <Merchant app={app} catalog={catalog} reload={load}/> : <Shopper app={app} catalog={catalog} renderer={renderer} setRenderer={setRenderer}/>}
    <footer><a className="footer-brand" href="/" aria-label="SwapLook home"><img src="/brand/swaplook-logo.svg" alt="SwapLook" width="310" height="72"/></a><span>A different top. The same feeling.</span><a className="footer-credits" href="/credits">Photo credits <ArrowUpRight size={13}/></a></footer>
    {help && <Modal title="Good alternatives, made simple." onClose={() => setHelp(false)}><ol className="how-list"><li><b>Start with the one you liked.</b><p>Pick your top and size. If it is unavailable, tell us your budget and must-haves.</p></li><li><b>Meet up to three alternatives.</b><p>Every option respects your selected size, budget and required attributes. Reasons come from catalog metadata.</p></li><li><b>Take another look. Make your choice.</b><p>Compare an illustrative preview, then choose. We recheck stock and price before saving your decision.</p></li></ol><p className="fine-print">This is a synthetic boutique: names, prices and stock are demo data. Catalog photos come from Wikimedia Commons contributors under CC BY, CC BY-SA, public domain or no-restriction terms, cropped and background-adjusted for SwapLook; see docs/CATALOG-CREDITS.md. Simulator images are original illustrations, not YouCam results. Previews do not guarantee physical fit. A saved choice is not a purchase or stock reservation.</p></Modal>}
  </div>
}

function Credits() {
  return <main className="credits-page">
    <div className="section-label">PHOTO CREDITS</div>
    <h2>Every photo, credited.</h2>
    <p className="fine-print">Catalog photographs come from Wikimedia Commons and keep their own licenses. They were cropped, padded and resized for this demo, and are also used as garment references for YouCam try-on. Product names, prices, materials and stock are fictional and do not describe the original items. These licenses do not grant rights in any visible marks.</p>
    <div className="table-scroll"><table><thead><tr><th>Demo product</th><th>Author</th><th>License</th></tr></thead><tbody>{photoCredits.map(c => <tr key={c.sku}><td><b>{c.name}</b><small>{c.sku}</small></td><td>{c.author}</td><td><a href={c.source} target="_blank" rel="noreferrer">{c.license} <ArrowUpRight size={11}/></a></td></tr>)}</tbody></table></div>
    <p className="fine-print">Models A, B and C are AI-generated fictional people, created with the YouCam AI image generator. They do not depict real individuals. The SwapLook logo and illustrations are original project work. Source code is MIT licensed; the photographs are not covered by that license.</p>
  </main>
}

const pending = (j?: Job) => !!j && ['pending', 'submitting'].includes(j.status)
const latest = (jobs: Job[], product: string, size: Size, source: string) => [...jobs].reverse().find(j => j.product_id === product && j.size === size && j.source_id === source)
const provenance = (j?: Job) => !j || j.status !== 'success' ? 'CATALOG PHOTO' : j.provider === 'simulator' ? 'SIMULATOR ILLUSTRATION' : j.reused ? 'YOUCAM PREVIEW · REUSED RESULT' : 'YOUCAM PREVIEW'
const sourceLabel = (source: string, photos: Photo[], models: ModelInfo[]) => source === 'demo' ? 'demo mannequin' : source.startsWith('model:') ? models.find(m => `model:${m.id}` === source)?.name ?? 'a model' : `your photo ${photos.findIndex(p => p.id === source) + 1}`
const defaultSource = (simulator: boolean, models: ModelInfo[], photos: Photo[]) => simulator ? 'demo' : models[0] ? `model:${models[0].id}` : photos[0]?.id ?? ''
// Keeps a still-valid source; otherwise falls back to the first model or photo.
const validSource = (s: string, simulator: boolean, models: ModelInfo[], photos: Photo[]) => (s === 'demo' ? simulator : s.startsWith('model:') ? !simulator && models.some(m => `model:${m.id}` === s) : photos.some(p => p.id === s)) ? s : defaultSource(simulator, models, photos)
const slotsUsed = (jobs: Job[]) => jobs.filter(j => !j.reused).length

function Shopper({ app, catalog, renderer, setRenderer }: { app: SwapLook; catalog: Product[]; renderer: string; setRenderer: (r: string) => void }) {
  const simulator = renderer === 'simulator'
  const [constraints, setConstraints] = useState<Constraints>(initial)
  const [shortlist, setShortlist] = useState<Shortlist | null>(null)
  const [decision, setDecision] = useState<Decision | null>(null)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [sessionReady, setSessionReady] = useState(false)
  const [compare, setCompare] = useState<string[]>([])
  const [comparing, setComparing] = useState(false)
  const [preview, setPreview] = useState<Product | null>(null)
  const [jobs, setJobs] = useState<Job[]>([])
  const [photos, setPhotos] = useState<Photo[]>([])
  const [rejecting, setRejecting] = useState(false)
  const [reason, setReason] = useState('style')
  const [models, setModels] = useState<ModelInfo[]>([])
  const [quota, setQuota] = useState<Quota | null>(null)
  const [source, setSource] = useState(simulator ? 'demo' : '')
  const [stopped, setStopped] = useState<Record<string, string>>({})
  const [soldOut, setSoldOut] = useState<string[]>([])
  const initialized = useRef<Promise<void> | null>(null)
  const original = catalog.find(p => p.id === constraints.original_id) || catalog[0]
  const available = original.stock[constraints.size] > 0
  const changed = shortlist && JSON.stringify(shortlist.constraints) !== JSON.stringify(constraints)
  const update = (patch: Partial<Constraints>) => { setConstraints(c => ({ ...c, ...patch })); setError('') }
  const saveJob = useCallback((job: Job) => setJobs(list => [...list.filter(j => j.id !== job.id), job]), [])
  const applyState = useCallback((state: State) => {
    const sim = state.renderer === 'simulator'
    setJobs(state.jobs); setPhotos(state.photos); setModels(state.models ?? []); setQuota(state.quota ?? null); setRenderer(state.renderer)
    setSource(s => validSource(s, sim, state.models ?? [], state.photos))
  }, [setRenderer])
  // Merge server jobs instead of replacing local ones, so a refresh never drops an in-flight preview.
  const refresh = useCallback(async () => { try { const state = await app.gateway.state(); const sim = state.renderer === 'simulator'; setRenderer(state.renderer); setQuota(state.quota ?? null); setModels(state.models ?? []); setSource(s => validSource(s, sim, state.models ?? [], state.photos)); state.jobs.forEach(saveJob) } catch { /* the visible error stays authoritative */ } }, [app, saveJob, setRenderer])

  useEffect(() => {
    if (!initialized.current) initialized.current = (async () => {
      try {
        try { const state = await app.gateway.state(); setShortlist(state.session.shortlist); setDecision(state.session.decision); if (state.session.shortlist) setConstraints(state.session.shortlist.constraints); applyState(state) }
        catch (e) { if (e instanceof APIError && (e.status === 401 || e.status === 404)) { await app.gateway.start(); await refresh() } else throw e }
        setSessionReady(true)
      } catch (e) { setError(message(e)) }
    })()
  }, [app, applyState, refresh])
  // One poller for every in-flight job, so previews keep progressing in the comparison and after a modal closes.
  useEffect(() => {
    const active = jobs.filter(j => pending(j) && !(j.id in stopped))
    if (!active.length) return
    let live = true
    const timer = window.setTimeout(async () => {
      for (const j of active) {
        try { const next = await app.gateway.poll(j.id); if (live) { saveJob(next); if (next.status === 'success' || next.status === 'error') void refresh() } }
        catch (e) { if (live) setStopped(s => ({ ...s, [j.id]: message(e) })) }
      }
    }, simulator ? 1100 : 10500)
    return () => { live = false; clearTimeout(timer) }
  }, [app, jobs, stopped, simulator, saveJob, refresh])
  const generate = async (product: string, src: string) => {
    try { saveJob(await app.gateway.preview(product, src)) }
    catch (e) { await refresh(); throw e }
    void refresh()
  }
  const find = async (e?: FormEvent) => {
    e?.preventDefault(); setBusy(true); setError('')
    try { const result = await app.find(constraints); setShortlist(result); setCompare([]); setSoldOut([]); document.getElementById('alternatives')?.scrollIntoView({ behavior: 'smooth', block: 'start' }) }
    catch (e) { setError(message(e)) } finally { setBusy(false) }
  }
  const choose = async (product: string, kind: Decision['kind'] = 'selected') => {
    setBusy(true); setError('')
    try { const result = await app.gateway.decide(product, kind, kind === 'rejected' ? reason : ''); setDecision(result); setPreview(null); setComparing(false); setRejecting(false); document.getElementById('workspace')?.scrollIntoView({ behavior: 'smooth' }) }
    catch (e) {
      setPreview(null); setComparing(false)
      if (e instanceof APIError && e.code === 'unavailable') { setSoldOut(list => list.includes(product) ? list : [...list, product]); setCompare(list => list.filter(id => id !== product)); document.getElementById('alternatives')?.scrollIntoView({ behavior: 'smooth', block: 'start' }) }
      else setError(message(e))
    } finally { setBusy(false) }
  }
  const restart = async () => {
    setBusy(true); setError('')
    try { await app.gateway.deletePhotos(); await app.gateway.start(); setDecision(null); setShortlist(null); setCompare([]); setSoldOut([]); setStopped({}); setConstraints(initial); await refresh(); setSessionReady(true) }
    catch (e) { setError(message(e)) } finally { setBusy(false) }
  }
  const size = shortlist?.constraints.size ?? constraints.size
  const candidates = shortlist?.candidates ?? []
  const open = candidates.filter(a => !soldOut.includes(a.product.id))
  const sources = { renderer, simulator, models, photos, setPhotos, source, setSource, quota, jobs }
  return <>
    <section className="hero">
      <div className="hero-copy"><div className="eyebrow"><span className="tiny-line"/> YOUR SIZE SOLD OUT. YOUR STYLE DIDN’T.</div><h1>Same feeling.<br/>A new <em>favorite.</em><span className="hero-star">✳</span></h1><p>The one you love is out of stock? Find a top that<br className="desktop-br"/> fits your wish list. Then take a second look.</p><a href="#workspace" className="hero-cta">Find my alternative <ArrowDown size={17}/></a><div className="hero-trust"><ShieldCheck size={16}/><span>Your size. Your budget. Always respected.</span></div></div>
      <Tilt className="hero-art" strength={4}><div className="art-label">THE SMALL HOURS<br/><span>everyday pieces, considered.</span></div><HeroStage/><span className="art-note">a change of plans,<br/><em>not a compromise.</em></span><div className="hero-stock"><span className="stock-icon"><PackageCheck size={21}/></span><span><b>There’s another good fit.</b><small>Let’s find it in your size.</small></span><span className="small-arrow"><ArrowUpRight size={17}/></span></div><span className="vertical-caption">THE ALTERNATIVE EDIT / VOL. 01</span></Tilt>
    </section>
    <section className="workspace" id="workspace"><Reveal className="workspace-title"><div><div className="eyebrow muted">A LITTLE FLEXIBILITY. A LOT OF POSSIBILITY.</div><h2>Let’s find your next yes.</h2></div><div className={`steps ${decision ? 'at-3' : shortlist ? 'at-2' : 'at-1'}`}><span className="step current">1</span> Your favorites <Minus size={18}/><span className={`step ${shortlist ? 'current' : ''}`}>2</span> Another look <Minus size={18}/><span className={`step ${decision ? 'current' : ''}`}>3</span> Your choice</div></Reveal>
      {error && <Notice>{error}<button className="icon-btn" aria-label="Dismiss error" onClick={() => setError('')}><X size={16}/></button></Notice>}
      {decision ? <div className="decision-panel"><Burst/><span className="decision-icon"><CircleCheck size={38}/></span><div className="eyebrow">{decision.kind === 'selected' ? 'A NEW FAVORITE, FOUND.' : 'THANKS FOR THE HONEST FEEDBACK.'}</div><h2>{decision.kind === 'selected' ? catalog.find(p => p.id === decision.product_id)?.name : 'The search can wait.'}</h2><p>{decision.kind === 'selected' ? `${money(decision.price)} · Size ${decision.size} · Stock checked when your choice was saved.` : 'Your reason was saved for the demo boutique owner.'}</p><p className="fine-print">Saved demo decision. This is not a purchase or a stock reservation.</p><button className="btn primary" disabled={busy} onClick={() => void restart()}>Explore again <ArrowRight size={16}/></button></div> : <div className="workspace-grid">
        <aside className="original-panel"><div className="section-label">01 / THE ONE YOU LIKED <Leaf size={14}/></div><Tilt className="original-image" strength={12}><img key={original.id} className="swap-in" src={original.image} alt={`Photo of ${original.name}`}/><span className={`availability ${available ? 'in-stock' : ''}`}>{available ? 'Available in your size' : `Size ${constraints.size} sold out`}</span></Tilt><label className="field-label" htmlFor="original">YOUR ORIGINAL PICK</label><div className="select-wrap"><select id="original" value={original.id} onChange={e => update({ original_id: e.target.value })}>{catalog.map(p => <option key={p.id} value={p.id}>{p.name}</option>)}</select><ChevronDown size={14}/></div><p className="original-price">{money(original.price)}<span>{original.material} · {original.cut}</span></p><div className="size-heading"><span className="field-label">YOUR SIZE</span><span>XS – XL</span></div><div className="sizes">{sizes.map(size => <button key={size} aria-pressed={constraints.size === size} className={`${constraints.size === size ? 'selected' : ''} ${original.stock[size] === 0 ? 'sold-out' : ''}`} onClick={() => update({ size })}>{size}</button>)}</div><p className="original-note">{available ? 'Good news: this size is in stock. Pick an unavailable size to explore alternatives.' : 'Out of stock doesn’t have to mean out of options.'}</p><div className="catalog-note"><Leaf size={15}/><span>One independent demo boutique.<br/>Thoughtful alternatives, all in one place.</span></div></aside>
        <div className="results-area"><form className="preferences" onSubmit={e => void find(e)}><div className="preferences-head"><span className="section-label">02 / MAKE IT YOURS</span><span><SlidersHorizontal size={13}/> Your must-haves</span></div><div className="preference-fields"><div className="budget-field"><label className="field-label" htmlFor="budget">MY BUDGET, UP TO</label><div className="budget-input"><span>$</span><input id="budget" aria-label="Maximum budget in dollars" type="number" min="0.01" max="1000" step="0.01" required value={constraints.budget / 100} onChange={e => update({ budget: Math.round(Number(e.target.value) * 100) })}/><span>USD</span></div></div><label className="preference-select"><span className="field-label">COLOR</span><div className="select-wrap"><select aria-label="Required color" value={constraints.color} onChange={e => update({ color: e.target.value })}><option value="">Open to anything</option>{[...new Set(catalog.map(p => p.color))].map(c => <option key={c} value={c}>{c[0].toUpperCase() + c.slice(1)}</option>)}</select><ChevronDown size={14}/></div></label><label className="preference-select"><span className="field-label">MATERIAL</span><div className="select-wrap"><select aria-label="Required material" value={constraints.material} onChange={e => update({ material: e.target.value })}><option value="">Open to anything</option>{[...new Set(catalog.map(p => p.material))].map(m => <option key={m} value={m}>{m[0].toUpperCase() + m.slice(1)}</option>)}</select><ChevronDown size={14}/></div></label><button className="btn primary find-button" type="submit" disabled={busy || !sessionReady || available}>{busy ? <LoaderCircle className="spin" size={17}/> : <Sparkles size={16}/>} Find alternatives</button></div><p className="constraints-note"><LockKeyhole size={11}/> Only in-stock options in your size, within your budget. No exceptions.</p></form>
          <div id="alternatives" className="alternatives-head"><div><h3>{shortlist ? candidates.length ? 'A few we think you’ll love.' : 'Nothing quite right. Yet.' : 'Your next favorite starts here.'}</h3><p>{shortlist ? `${candidates.length} alternative${candidates.length === 1 ? '' : 's'} · Size ${size} · Up to ${money(shortlist.constraints.budget)}` : 'Set your must-haves above. We’ll do the looking.'}</p></div>{candidates.length > 0 && <div className="alternatives-actions"><span className="match-count"><CheckCheck size={14}/> All within your limits</span>{open.length > 1 && <button className="btn secondary side-by-side" disabled={busy || !!changed} onClick={() => { setCompare(open.map(a => a.product.id)); setComparing(true) }}><Columns2 size={15}/> See them side by side</button>}</div>}</div>
          {changed && <div className="stale-note"><RefreshCw size={14}/> Your preferences changed. Find alternatives again to update your shortlist.</div>}
          {soldOut.length > 0 && !changed && <div className="soldout-note" role="alert"><PackageX size={20}/><div><b>{soldOut.map(id => catalog.find(p => p.id === id)?.name ?? id).join(', ')} just sold out in size {size}.</b><p>It was in stock when your shortlist was made, and we checked again when you chose. Nothing was saved. Your other alternatives still meet your limits.</p></div><button className="btn secondary" disabled={busy} onClick={() => void find()}><RefreshCw size={14}/> Refresh alternatives</button></div>}
          {shortlist ? candidates.length ? <><div key={JSON.stringify(shortlist.constraints)} className={`product-grid ${changed ? 'stale' : ''}`}>{candidates.map((a, index) => <ProductCard key={a.product.id} candidate={a} index={index} size={size} job={latest(jobs, a.product.id, size, source)} soldOut={soldOut.includes(a.product.id)} compared={compare.includes(a.product.id)} disabled={busy || !!changed} onCompare={() => setCompare(list => list.includes(a.product.id) ? list.filter(id => id !== a.product.id) : [...list, a.product.id])} onPreview={() => setPreview(a.product)} onChoose={() => void choose(a.product.id)}/>)}</div><div className="results-footnote"><span><Clock3 size={13}/> Stock can change. We’ll check again before you choose.</span><button className="text-button" disabled={busy} onClick={() => setRejecting(true)}>Not quite my thing <ArrowRight size={13}/></button></div></> : <div className="empty-state"><Leaf size={34}/><h3>No tops meet all your must-haves.</h3><p>Your size, budget and required attributes stay exactly as you set them.<br/>Adjust a preference above if you want to explore further.</p><button className="btn secondary" onClick={() => { setReason('no_match'); setRejecting(true) }}>Leave feedback <ArrowRight size={15}/></button></div> : <div className="awaiting-state"><div className="awaiting-illustration"><img src="/catalog/cut-SL-003.png" alt=""/><img src="/catalog/cut-SL-005.png" alt=""/><img src="/catalog/cut-SL-009.png" alt=""/></div><span className="eyebrow">GOOD THINGS COME IN ALTERNATIVES</span><p>Not an endless scroll.<br/><em>Just a considered shortlist.</em></p><div><span><Check size={13}/> In your size</span><span><Check size={13}/> Within budget</span><span><Check size={13}/> Up to 3 options</span></div></div>}
        </div>
      </div>}
    </section>
    <Reveal as="section" className="promise-strip"><div><PackageCheck/><span><b>Available, by design.</b><small>Real inventory checks at every choice.</small></span></div><div><SlidersHorizontal/><span><b>Your limits lead the way.</b><small>Never quietly stretching your budget.</small></span></div><div><ShieldCheck/><span><b>A preview, with perspective.</b><small>An illustration of style, not a promise of fit.</small></span></div></Reveal>
    {compare.length > 0 && !decision && !comparing && <div className="compare-bar"><span><Columns2 size={18}/><b>{compare.length} selected</b> <span className="hide-mobile">for a closer look</span></span><div><button className="text-button" onClick={() => setCompare([])}>Clear</button><button className="btn primary" disabled={!!changed} onClick={() => setComparing(true)}>Compare tops <ArrowRight size={16}/></button></div></div>}
    {comparing && shortlist && <Modal title="The little differences matter." wide onClose={() => setComparing(false)}><Comparison app={app} items={candidates.filter(a => compare.includes(a.product.id))} size={size} sources={sources} stopped={stopped} generate={generate} choose={id => void choose(id)} busy={busy}/></Modal>}
    {preview && <Preview app={app} product={preview} size={size} sources={sources} saveJob={saveJob} generate={generate} refresh={refresh} stoppedError={stopped[latest(jobs, preview.id, size, source)?.id ?? '']} resume={id => setStopped(s => { const { [id]: _, ...rest } = s; return rest })} onClose={() => setPreview(null)} onChoose={() => void choose(preview.id)} choosing={busy}/>}
    {rejecting && <Modal title="What didn’t feel right?" onClose={() => setRejecting(false)}><p className="modal-intro">A little feedback helps the demo boutique understand your choice.</p><label className="field-label" htmlFor="reason">YOUR REASON</label><select className="full-select" id="reason" value={reason} onChange={e => setReason(e.target.value)}><option value="style">Not my style</option><option value="price">I would prefer a lower price</option><option value="no_match">No alternatives meet my needs</option><option value="other">Something else</option></select><button className="btn primary full-width" disabled={busy} onClick={() => void choose('', 'rejected')}>Save feedback <ArrowRight size={16}/></button></Modal>}
  </>
}

type Sources = { renderer: string; simulator: boolean; models: ModelInfo[]; photos: Photo[]; setPhotos: (p: Photo[]) => void; source: string; setSource: (s: string) => void; quota: Quota | null; jobs: Job[] }

function ProductCard({ candidate: a, index, size, job, soldOut, compared, disabled, onCompare, onPreview, onChoose }: { candidate: Candidate; index: number; size: Size; job?: Job; soldOut: boolean; compared: boolean; disabled: boolean; onCompare: () => void; onPreview: () => void; onChoose: () => void }) {
  const ref = useRef<HTMLElement>(null)
  useTilt(ref, 9)
  const tried = job?.status === 'success' && job.image
  return <article ref={ref} className={`product-card tilt ${soldOut ? 'is-sold-out' : ''}`} style={{ '--i': index } as CSSProperties}><div className="product-image"><img src={tried ? job.image : a.product.image} alt={tried ? `Preview of ${a.product.name}` : `Photo of ${a.product.name}`} loading="lazy"/><span className="rank-label">{index === 0 ? <><Sparkles size={11}/> CLOSEST FEEL</> : `ANOTHER POSSIBILITY / 0${index + 1}`}</span><label className="compare-check" title="Add to comparison"><input type="checkbox" aria-label={`Compare ${a.product.name}`} checked={compared} disabled={disabled || soldOut} onChange={onCompare}/><Columns2 size={15}/></label>{soldOut ? <span className="card-soldout">JUST SOLD OUT IN {size}</span> : <span className="card-stock"><span className="status-dot"/> {tried ? provenance(job) : `SIZE ${size} IN STOCK`}</span>}{pending(job) && <span className="card-rendering"><LoaderCircle className="spin" size={18}/> Preview in progress</span>}</div><div className="product-meta"><h4>{a.product.name}</h4><b>{money(a.product.price)}</b></div><p className="product-attributes">{a.product.color} / {a.product.material} / {a.product.cut}</p><div className="reason-tags">{a.reasons.filter(r => r.startsWith('Same ')).slice(0, 2).map(r => <span key={r}><Check size={11}/>{r}</span>)}</div>{a.tradeoffs.length > 0 && <p className="tradeoff">A little different: {a.tradeoffs[0]}</p>}<button className="btn preview-button" disabled={disabled || soldOut} onClick={onPreview}><Sparkles size={14}/> Take another look <ArrowUpRight size={14}/></button><button className="choose-button" disabled={disabled || soldOut} onClick={onChoose}>Choose this top <ArrowRight size={13}/></button><span className="tilt-glare" aria-hidden="true"/></article>
}

type PhotoCheck = { block?: string; warnings: string[] }
// Browser-side checks catch common pose/size problems before a provider credit is spent; the server still validates.
async function inspectPhoto(file: File): Promise<PhotoCheck> {
  if (!['image/jpeg', 'image/png'].includes(file.type)) return { block: 'Use a JPEG or PNG photo.', warnings: [] }
  if (file.size > 8 * 1024 * 1024) return { block: 'This photo is larger than 8 MB.', warnings: [] }
  if (typeof createImageBitmap !== 'function') return { warnings: [] }
  try {
    const bitmap = await createImageBitmap(file); const { width: w, height: h } = bitmap; bitmap.close()
    if (Math.min(w, h) < 128 || Math.max(w, h) > 4096) return { block: 'Use a photo between 128 and 4096 px per side.', warnings: [] }
    const warnings: string[] = []
    if (w > h) warnings.push('This is a landscape photo. A portrait, head-to-waist photo works best.')
    if (Math.min(w, h) < 512) warnings.push('This photo is small. At least 768 px wide gives a clearer preview.')
    return { warnings }
  } catch { return { warnings: [] } }
}

function PhotoSource({ app, sources, busy, setBusy, setError }: { app: SwapLook; sources: Sources; busy: boolean; setBusy: (b: boolean) => void; setError: (e: string) => void }) {
  const { simulator, models, photos, setPhotos, source, setSource } = sources
  const [mode, setMode] = useState<'model' | 'upload' | 'selfie'>(source.startsWith('model:') || (!source && models.length) ? 'model' : 'upload')
  const [consent, setConsent] = useState(false)
  const [check, setCheck] = useState<PhotoCheck | null>(null)
  const [file, setFile] = useState<File | null>(null)
  const input = useRef<HTMLInputElement>(null)
  if (simulator) return <div className="provider-note"><Sparkles size={19}/><b>Illustration mode</b><p>Try the flow with an original demo mannequin. This is a simulator fixture, not an AI try-on or a rendering of you. No photo is sent to YouCam.</p></div>
  const upload = async (f: File) => { setBusy(true); setError(''); try { const p = await app.gateway.upload(f, consent); setPhotos([...photos, p]); setSource(p.id); setFile(null); setCheck(null); if (input.current) input.current.value = '' } catch (e) { setError(message(e)) } finally { setBusy(false) } }
  const submit = async (f?: File) => { if (!f) return; const c = await inspectPhoto(f); setCheck(c); setFile(f); if (!c.block && !c.warnings.length) await upload(f) }
  const choose = (next: typeof mode) => { setMode(next); setCheck(null); if (next === 'model' && models[0] && !source.startsWith('model:')) setSource(`model:${models[0].id}`); if (next !== 'model' && source.startsWith('model:')) setSource(photos[photos.length - 1]?.id ?? '') }
  const full = photos.length >= 3
  return <div className="photo-source">
    <div className="source-tabs" role="tablist" aria-label="Preview photo">
      {models.length > 0 && <button type="button" role="tab" aria-selected={mode === 'model'} className={mode === 'model' ? 'active' : ''} onClick={() => choose('model')}><Users size={14}/> Models</button>}
      <button type="button" role="tab" aria-selected={mode === 'upload'} className={mode === 'upload' ? 'active' : ''} onClick={() => choose('upload')}><ImageUp size={14}/> Upload photo</button>
      <button type="button" role="tab" aria-selected={mode === 'selfie'} className={mode === 'selfie' ? 'active' : ''} onClick={() => choose('selfie')}><Camera size={14}/> Take a selfie</button>
    </div>
    {mode === 'model' ? <div className="model-grid" role="radiogroup" aria-label="Available models">{models.map(m => <button key={m.id} type="button" role="radio" aria-checked={source === `model:${m.id}`} className={`model-option ${source === `model:${m.id}` ? 'active' : ''}`} onClick={() => setSource(`model:${m.id}`)}><img src={`/api/models/${encodeURIComponent(m.id)}/image`} alt={`${m.name}, consented model`}/><b>{m.name}</b>{m.note && <small>{m.note}</small>}</button>)}<p className="fine-print">No photo of you is needed. Results for the same model and top may be reused without a new generation.</p></div> : <>
      <ul className="photo-guide" aria-label="Photo tips"><li><Check size={12}/> Stand straight, facing the camera</li><li><Check size={12}/> Head to waist, both shoulders visible</li><li><Check size={12}/> Plain background, even light</li><li><X size={12}/> No crossed arms, bags or other people</li></ul>
      <label className="consent"><input type="checkbox" checked={consent} onChange={e => setConsent(e.target.checked)}/><span>I have permission and consent to processing this photo by SwapLook and YouCam for this preview.</span></label>
      {mode === 'upload' ? <label className={`file-button ${!consent || busy || full ? 'disabled' : ''}`}><input ref={input} className="visually-hidden" aria-label="Upload authorized person photo" type="file" accept="image/jpeg,image/png" disabled={!consent || busy || full} onChange={e => void submit(e.target.files?.[0])}/>{busy ? <LoaderCircle className="spin" size={18}/> : <ImageUp size={18}/>}<span><b>{busy ? 'Uploading…' : photos.length ? 'Choose another photo' : 'Choose a photo from your device'}</b><small>{!consent ? 'Tick the consent box above first' : 'JPEG or PNG, up to 8 MB'}</small></span></label> : <SelfieCamera disabled={!consent || busy || full} onCapture={f => void submit(f)}/>}
      {full && <p className="fine-print">This session already has three photos. Delete application photo copies to add another.</p>}
      {check && (check.block || check.warnings.length > 0) && <div className={`photo-check ${check.block ? 'blocked' : ''}`} role="status">{check.block ? <p>{check.block}</p> : <><ul>{check.warnings.map(w => <li key={w}>{w}</li>)}</ul><button type="button" className="btn secondary" disabled={busy || !file} onClick={() => file && void upload(file)}>Upload anyway</button></>}</div>}
      {photos.length > 0 && <div className="my-photos" role="radiogroup" aria-label="Your photos">{photos.map((p, i) => <button key={p.id} type="button" role="radio" aria-checked={source === p.id} aria-label={`Your photo ${i + 1}`} className={source === p.id ? 'active' : ''} onClick={() => setSource(p.id)}><img src={`/api/images/${p.id}`} alt=""/></button>)}</div>}
      <p className="fine-print">JPEG/PNG, 128–4096 px per side, under 8 MB. Application copies are private to this session. YouCam currently documents retention up to 30 days; deleting here does not promise immediate deletion from their systems.</p>
    </>}
  </div>
}

// The centred region of a source frame that object-fit: cover shows in a box of the given width/height ratio.
export function coverCrop(vw: number, vh: number, ratio: number) {
  const w = Math.min(vw, Math.round(vh * ratio)), h = Math.min(vh, Math.round(vw / ratio))
  return { x: Math.round((vw - w) / 2), y: Math.round((vh - h) / 2), w, h }
}

// Live selfie capture. The preview is mirrored like a mirror; the captured photo is not.
function SelfieCamera({ disabled, onCapture }: { disabled: boolean; onCapture: (f: File) => void }) {
  const video = useRef<HTMLVideoElement>(null)
  const canvas = useRef<HTMLCanvasElement>(null)
  const frame = useRef<HTMLDivElement>(null)
  const [stream, setStream] = useState<MediaStream | null>(null)
  const [shot, setShot] = useState<Blob | null>(null)
  const [count, setCount] = useState(0)
  const [timer, setTimer] = useState(true)
  const [error, setError] = useState('')
  useEffect(() => { if (video.current && stream) { video.current.srcObject = stream; void video.current.play().catch(() => {}) } }, [stream])
  useEffect(() => () => stream?.getTracks().forEach(t => t.stop()), [stream])
  useEffect(() => {
    if (!count) return
    const t = window.setTimeout(() => { if (count === 1) snap(); setCount(c => c - 1) }, 1000)
    return () => clearTimeout(t)
  })
  const start = async () => {
    setError('')
    if (!navigator.mediaDevices?.getUserMedia) { setError('The camera is not available here. It needs HTTPS (or localhost) and a supported browser. Upload a photo instead.'); return }
    try { setStream(await navigator.mediaDevices.getUserMedia({ video: { facingMode: 'user', width: { ideal: 1920 }, height: { ideal: 1920 } }, audio: false })); setShot(null) }
    catch (e) { setError(e instanceof DOMException && e.name === 'NotAllowedError' ? 'Camera permission was denied. Allow camera access in your browser settings, or upload a photo instead.' : 'The camera could not start. Close other apps using it, or upload a photo instead.') }
  }
  const snap = () => {
    const v = video.current, c = canvas.current
    if (!v || !c || !v.videoWidth) return
    // Keep exactly what the frame shows (object-fit: cover, centred): laptop cameras deliver landscape frames.
    const ratio = frame.current && frame.current.clientHeight ? frame.current.clientWidth / frame.current.clientHeight : 4 / 5
    const r = coverCrop(v.videoWidth, v.videoHeight, ratio)
    c.width = r.w; c.height = r.h
    c.getContext('2d')?.drawImage(v, r.x, r.y, r.w, r.h, 0, 0, r.w, r.h)
    c.toBlob(b => { if (b) setShot(b) }, 'image/jpeg', 0.92)
  }
  const stop = () => { stream?.getTracks().forEach(t => t.stop()); setStream(null) }
  const use = () => { if (!shot) return; onCapture(new File([shot], 'selfie.jpg', { type: 'image/jpeg' })); stop(); setShot(null) }
  return <div className="selfie">
    {error && <p className="photo-check blocked" role="alert">{error}</p>}
    {!stream ? <button type="button" className="btn secondary full-width" disabled={disabled} onClick={() => void start()}><Camera size={15}/> Open camera</button> : <>
      <div className="selfie-frame" ref={frame}>
        <video ref={video} playsInline muted className={shot ? 'hidden' : ''} aria-label="Camera preview"/>
        <canvas ref={canvas} className={shot ? '' : 'hidden'} aria-label="Captured selfie"/>
        {!shot && <svg className="selfie-guide" viewBox="0 0 100 125" aria-hidden="true"><path d="M50 14a13 15 0 1 1 0 30a13 15 0 1 1 0-30M22 118c0-28 8-46 28-48c20 2 28 20 28 48"/></svg>}
        {count > 0 && <span className="selfie-count" aria-live="assertive">{count}</span>}
        {!shot && <span className="selfie-hint">Fit head to waist inside the outline</span>}
      </div>
      <div className="selfie-actions">{shot ? <><button type="button" className="btn secondary" onClick={() => setShot(null)}><RefreshCw size={14}/> Retake</button><button type="button" className="btn primary" disabled={disabled} onClick={use}><Check size={15}/> Use this photo</button></> : <><label className="selfie-timer"><input type="checkbox" checked={timer} onChange={e => setTimer(e.target.checked)}/> 3-second timer</label><button type="button" className="btn primary" disabled={count > 0} onClick={() => timer ? setCount(3) : snap()}><Camera size={15}/> Capture</button><button type="button" className="text-button" onClick={stop}>Close camera</button></>}</div>
    </>}
  </div>
}

function QuotaNote({ sources }: { sources: Sources }) {
  const used = slotsUsed(sources.jobs)
  const today = sources.quota?.remaining_today ?? -1
  return <p className="fine-print quota-note">{used}/3 preview slots used in this session. No automatic generation retries.{!sources.simulator && today >= 0 && <> {today} AI preview{today === 1 ? '' : 's'} left today for this demo.</>}</p>
}

function Preview({ app, product, size, sources, saveJob, generate, refresh, stoppedError, resume, onClose, onChoose, choosing }: { app: SwapLook; product: Product; size: Size; sources: Sources; saveJob: (j: Job) => void; generate: (product: string, src: string) => Promise<void>; refresh: () => Promise<void>; stoppedError?: string; resume: (id: string) => void; onClose: () => void; onChoose: () => void; choosing: boolean }) {
  const { simulator, source, jobs, photos } = sources
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const job = latest(jobs, product.id, size, source)
  const create = async () => { setBusy(true); setError(''); try { await generate(product.id, source) } catch (e) { setError(message(e)) } finally { setBusy(false) } }
  const reconcile = async () => {
    if (!job) return
    setBusy(true); setError('')
    try { saveJob(await app.gateway.reconcile(job.id)) }
    catch (e) { setError(message(e)); await refresh() }
    finally { setBusy(false) }
  }
  const remove = async () => { setBusy(true); setError(''); try { await app.gateway.deletePhotos(); sources.setPhotos([]); sources.setSource(defaultSource(simulator, sources.models, [])); await refresh() } catch (e) { setError(message(e)) } finally { setBusy(false) } }
  return <Modal title="Take a second look." wide onClose={onClose}><div className="preview-layout"><Tilt className="preview-image" strength={8}><img src={job?.status === 'success' && job.image ? job.image : product.image} alt={`${job?.status === 'success' ? job.provider === 'simulator' ? 'Simulator mannequin illustration' : 'YouCam result' : 'Catalog photo'} for ${product.name}`}/><span className="preview-watermark">{job?.status === 'success' ? job.provider === 'simulator' ? 'SIMULATOR · ORIGINAL ILLUSTRATION' : job.reused ? 'YOUCAM · REUSED AI PREVIEW' : 'YOUCAM · AI PREVIEW' : 'CATALOG PHOTO'}</span>{(pending(job) || busy) && <div className="render-overlay"><LoaderCircle className="spin" size={28}/><span>{busy ? 'Preparing…' : 'Preview in progress…'}</span></div>}</Tilt><div className="preview-details"><span className="eyebrow">A DIFFERENT PERSPECTIVE</span><h3>{product.name}</h3><p>{money(product.price)} · Size {size}</p>
      <PhotoSource app={app} sources={sources} busy={busy} setBusy={setBusy} setError={setError}/>
      {(error || stoppedError) && <Notice>{error || stoppedError}</Notice>}
      {job && <p className="job-status" role="status">{job.status === 'pending' || job.status === 'submitting' ? <><Clock3 size={14}/> Waiting for the same job. You can close this window.</> : job.status === 'success' ? <><Check size={14}/> {job.provider === 'simulator' ? 'Simulator illustration ready.' : job.reused ? 'Reused demo-model preview. No new generation was submitted.' : 'Provider preview received.'}</> : job.message || `Job status: ${job.status}. Generation will not be resubmitted.`}</p>}
      {!job && <button className="btn primary full-width" disabled={busy || !source || slotsUsed(jobs) >= 3} onClick={() => void create()}><Sparkles size={15}/>{simulator ? 'Show demo illustration' : source.startsWith('model:') ? `Show on ${sourceLabel(source, photos, sources.models)}` : 'Create YouCam preview'}</button>}
      {stoppedError && pending(job) && <button className="btn secondary" onClick={() => { resume(job!.id); setError('') }}>Resume polling this job <RefreshCw size={14}/></button>}
      {job?.status === 'timed_out' && <><button className="btn secondary full-width" disabled={busy || (job.reconciliations ?? 0) >= 3} onClick={() => void reconcile()}><RefreshCw size={14}/> Check the same preview</button><p className="fine-print">{job.reconciliations ?? 0}/3 additional status checks used. This does not create a new preview. Allow 10 seconds between checks.</p></>}
      <QuotaNote sources={sources}/><button className="btn secondary full-width" disabled={busy || choosing} onClick={onChoose}>Choose this top <ArrowRight size={15}/></button>{photos.length > 0 && <button className="text-button delete-images" disabled={busy} onClick={() => void remove()}><Trash2 size={13}/> Delete application photo copies</button>}<p className="fine-print">A preview illustrates appearance. It cannot guarantee physical fit, identity preservation or every garment detail.</p></div></div></Modal>
}

function Comparison({ app, items, size, sources, stopped, generate, choose, busy }: { app: SwapLook; items: Candidate[]; size: Size; sources: Sources; stopped: Record<string, string>; generate: (product: string, src: string) => Promise<void>; choose: (id: string) => void; busy: boolean }) {
  const { simulator, source, jobs, photos } = sources
  const [running, setRunning] = useState(false)
  const [error, setError] = useState('')
  const [picking, setPicking] = useState(false)
  const missing = items.filter(a => !latest(jobs, a.product.id, size, source))
  const reusable = source.startsWith('model:')
  const tryAll = async () => {
    setRunning(true); setError('')
    try { for (const a of missing) await generate(a.product.id, source) }
    catch (e) { setError(message(e)) } finally { setRunning(false) }
  }
  return <>
    <div className="tryon-panel">
      <div><span className="eyebrow">SEE THEM ON {source ? sourceLabel(source, photos, sources.models).toUpperCase() : 'A PHOTO'}</span><p>{missing.length ? `${missing.length} preview${missing.length === 1 ? '' : 's'} to create. ${reusable ? 'Earlier identical results for this model are reused without a new generation.' : `This can use up to ${missing.length} of your ${3 - slotsUsed(jobs)} remaining session slots.`}` : 'Every top here already has a preview for this photo.'}</p></div>
      <div className="tryon-actions">{!simulator && <button className="text-button" onClick={() => setPicking(p => !p)}>{picking ? 'Done' : 'Change photo'}</button>}<button className="btn primary" disabled={running || busy || !source || !missing.length} onClick={() => void tryAll()}>{running ? <LoaderCircle className="spin" size={15}/> : <Sparkles size={15}/>} {missing.length ? `Try ${missing.length === items.length ? 'all' : 'the rest'} on` : 'All tried on'}</button></div>
    </div>
    {picking && <PhotoSource app={app} sources={sources} busy={running} setBusy={setRunning} setError={setError}/>}
    {error && <Notice>{error}</Notice>}
    <QuotaNote sources={sources}/>
    <div className="comparison-grid">{items.map((a, i) => { const j = latest(jobs, a.product.id, size, source); return <div className="comparison-item" style={{ '--i': i } as CSSProperties} key={a.product.id}><div className="comparison-image"><img src={j?.status === 'success' && j.image ? j.image : a.product.image} alt={`${j?.status === 'success' ? 'Preview' : 'Photo'} of ${a.product.name}`}/>{pending(j) && <span className="render-overlay"><LoaderCircle className="spin" size={22}/><span>{stopped[j!.id] ? 'Status check paused' : 'Rendering…'}</span></span>}</div><span className="eyebrow">{provenance(j)}</span><p className="fine-print">Source: {j?.status === 'success' ? sourceLabel(j.source_id, photos, sources.models) : 'catalog photo'}</p>{j && !pending(j) && j.status !== 'success' && <p className="fine-print item-error">{j.message || `Preview ${j.status}.`}</p>}<h3>{a.product.name}</h3><b>{money(a.product.price)}</b><p>Size {size} · {a.product.color} · {a.product.material}</p><ul className="reasons">{a.tradeoffs.map(t => <li key={t}>{t}</li>)}</ul><button className="btn primary" disabled={busy} onClick={() => choose(a.product.id)}>Choose this top <ArrowRight size={15}/></button></div> })}</div>
    <p className="fine-print">Catalog availability may change. Choosing checks current stock again and saves a demo decision. Previews illustrate appearance, not physical fit.</p>
  </>
}

function Merchant({ app, catalog, reload }: { app: SwapLook; catalog: Product[]; reload: () => Promise<void> }) {
  const [token, setToken] = useState('')
  const [summary, setSummary] = useState<Summary | null>(null)
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  const [notice, setNotice] = useState('')
  const [sku, setSku] = useState('SL-002')
  const [size, setSize] = useState<Size>('M')
  const [quantity, setQuantity] = useState(5)
  // View-only mode shows the public demo dashboard; every change still needs the owner token.
  const [viewOnly, setViewOnly] = useState(false)
  const [unlocking, setUnlocking] = useState(false)
  const toggleAI = async (enabled: boolean) => { setBusy(true); setError(''); setNotice(''); try { await app.gateway.setAI(token, enabled); await reload(); setSummary(await app.gateway.summary(token)); setNotice(enabled ? 'AI previews are on. New previews use YouCam and count against the allowance.' : 'AI previews are off. New previews use free illustrations; no YouCam credits are spent.') } catch (e) { setError(message(e)) } finally { setBusy(false) } }
  const refresh = async (e?: FormEvent) => { e?.preventDefault(); setBusy(true); setError(''); try { setSummary(viewOnly ? await app.gateway.demoSummary() : await app.gateway.summary(token)) } catch (e) { setError(message(e)); if (!viewOnly) setSummary(null) } finally { setBusy(false) } }
  const viewDemo = async () => { setBusy(true); setError(''); setNotice(''); try { setSummary(await app.gateway.demoSummary()); setViewOnly(true) } catch (e) { setError(message(e)) } finally { setBusy(false) } }
  const unlock = async (e: FormEvent) => { e.preventDefault(); setBusy(true); setError(''); try { setSummary(await app.gateway.summary(token)); setViewOnly(false); setUnlocking(false); setNotice('Studio unlocked. You can now change stock and the AI preview switch.') } catch (e) { setError(message(e)) } finally { setBusy(false) } }
  const lock = () => { setToken(''); setSummary(null); setViewOnly(false); setUnlocking(false); setNotice('') }
  const stock = async (e: FormEvent) => { e.preventDefault(); setBusy(true); setError(''); setNotice(''); try { await app.gateway.stock(token, sku, size, quantity); await reload(); setNotice(`Stock updated: ${sku}, size ${size}, quantity ${quantity}. Existing shopper shortlists are rechecked at selection.`) } catch (e) { setError(message(e)) } finally { setBusy(false) } }
  return <main className="merchant-main"><div className="eyebrow"><Store size={14}/> THE SMALL HOURS / MERCHANT STUDIO</div><h1>A little insight.<br/><em>Better alternatives.</em></h1><p className="merchant-intro">Keep stock current. See what shoppers chose, and what didn’t feel right.</p>{error && <Notice>{error}</Notice>}{notice && <div className="success-notice" role="status"><CircleCheck size={17}/>{notice}</div>}
    {!summary ? <form className="merchant-login" onSubmit={e => void refresh(e)}><LockKeyhole size={24}/><h3>Your boutique, behind the counter.</h3><p>Use the merchant token configured on the server. It stays in memory for this page only.</p><label className="field-label" htmlFor="merchant-token">MERCHANT ACCESS TOKEN</label><input id="merchant-token" type="password" autoComplete="off" required value={token} onChange={e => setToken(e.target.value)}/><button className="btn primary" disabled={busy}>Open studio <ArrowRight size={16}/></button><div className="login-divider"><span>or</span></div><button type="button" className="btn secondary view-demo" disabled={busy} onClick={() => void viewDemo()}><Eye size={16}/> View demo dashboard</button><p className="fine-print">Read-only. No token needed. Changing stock or the AI switch still requires the owner token.</p></form> : <><div className="merchant-toolbar"><span className="demo-pill">{viewOnly ? <><Eye size={13}/> VIEW ONLY · DEMO DASHBOARD</> : <><span className="status-dot"/> DEMO OBSERVATIONS ONLY</>}</span><div><button className="text-button" disabled={busy} onClick={() => void refresh()}><RefreshCw size={14}/> Refresh</button>{viewOnly ? <button className="text-button" onClick={() => setUnlocking(u => !u)}>Unlock with token <LockKeyhole size={14}/></button> : <button className="text-button" onClick={lock}>Lock studio <LockKeyhole size={14}/></button>}</div></div>{viewOnly && unlocking && <form className="unlock-bar" onSubmit={e => void unlock(e)}><LockKeyhole size={18}/><label className="field-label" htmlFor="unlock-token">OWNER TOKEN</label><input id="unlock-token" type="password" autoComplete="off" required value={token} onChange={e => setToken(e.target.value)}/><button className="btn primary" disabled={busy}>Unlock</button></form>}<div className="metric-grid">{([[summary.sessions, 'Demo sessions'], [summary.choices, 'Alternatives chosen'], [summary.rejections, 'Feedback received']] as const).map(([v, label]) => <Metric key={label} value={v} label={label}/>)}</div><Insights summary={summary} catalog={catalog} busy={busy} locked={viewOnly} onToggleAI={enabled => void toggleAI(enabled)}/><div className="merchant-grid"><section className="merchant-section"><div className="section-label">KEEP INVENTORY IN THE LOOP</div><h3>Update a size.</h3>{viewOnly && <div className="locked-note"><LockKeyhole size={15}/><span>Owner token required to make changes.</span><button type="button" className="text-button" onClick={() => setUnlocking(true)}>Unlock with token</button></div>}<form onSubmit={e => void stock(e)}><fieldset className="plain-fieldset" disabled={viewOnly}><label>Product<select aria-label="Inventory product" value={sku} onChange={e => setSku(e.target.value)}>{catalog.map(p => <option value={p.id} key={p.id}>{p.name}</option>)}</select></label><div className="stock-fields"><label>Size<select aria-label="Inventory size" value={size} onChange={e => setSize(e.target.value as Size)}>{sizes.map(v => <option key={v}>{v}</option>)}</select></label><label>Quantity<input aria-label="Stock quantity" type="number" min="0" max="10000" required value={quantity} onChange={e => setQuantity(Number(e.target.value))}/></label></div><p className="fine-print">Current stock: {catalog.find(p => p.id === sku)?.stock[size]} in size {size}. Setting zero can demonstrate a stock change while a shopper compares.</p><button className="btn primary" disabled={busy || viewOnly}>Save stock <PackageCheck size={16}/></button></fieldset></form></section><section className="merchant-section decisions-section"><div className="section-label">A SIGNAL, NOT A SALE</div><h3>Recent decisions.</h3>{summary.decisions.length === 0 ? <div className="merchant-empty"><Leaf size={25}/><p>No decisions yet. Complete a shopper flow to see the first one here.</p></div> : <div className="table-scroll"><table><thead><tr><th>Shopper decision</th><th>Size</th><th>Details</th></tr></thead><tbody>{summary.decisions.map(d => <tr key={d.id}><td><span className={`decision-tag ${d.kind}`}>{d.kind === 'selected' ? 'Chosen' : 'Passed'}</span><b>{d.product_id ? catalog.find(p => p.id === d.product_id)?.name : 'No alternative chosen'}</b><small>{new Date(d.created_at).toLocaleString('en-US')}</small></td><td>{d.size}</td><td>{d.kind === 'selected' ? money(d.price) : d.reason.replace('_', ' ')}<small>Demo · {d.original_id}</small></td></tr>)}</tbody></table></div>}</section></div><p className="fine-print">Synthetic catalog and demo sessions. Choices are not paid orders, reserved inventory or recovered revenue. Genuine research observations will be recorded separately.</p></>}
  </main>
}

function Insights({ summary, catalog, busy, locked, onToggleAI }: { summary: Summary; catalog: Product[]; busy: boolean; locked: boolean; onToggleAI: (enabled: boolean) => void }) {
  const name = (id: string) => catalog.find(p => p.id === id)?.name ?? id
  const p = summary.previews
  const subs = summary.substitutions ?? []
  const top = subs[0]?.count || 1
  return <div className="insight-grid">
    <section className="merchant-section"><div className="section-label">WHEN A FAVORITE SELLS OUT</div><h3>What shoppers chose instead.</h3>{subs.length === 0 ? <p className="fine-print">No demo choices yet. Each saved choice links the unavailable original to the alternative picked.</p> : <ul className="substitutions">{subs.slice(0, 5).map(x => <li key={x.original_id + x.product_id}><span><small>{name(x.original_id)} sold out</small><b>→ {name(x.product_id)}</b></span><span className="bar" aria-hidden="true"><span style={{ width: `${(x.count / top) * 100}%` } as CSSProperties}/></span><em>{x.count}×</em></li>)}</ul>}<p className="fine-print">Demo choices, not sales. Use as a restock and merchandising signal only.</p></section>
    <section className="merchant-section"><div className="section-label">AI TRY-ON USAGE</div><h3>Preview credits.</h3>{p?.available && <div className="ai-switch"><span><b>AI previews {p.enabled ? 'on' : 'off'}</b><small>{locked ? 'Owner token required to change this switch.' : p.enabled ? 'Shoppers get YouCam try-on previews. Each new one uses credits.' : 'Saving credits: shoppers get free illustrations, no YouCam calls.'}</small></span><button type="button" role="switch" aria-checked={!!p.enabled} aria-label="AI previews" className={`switch ${p.enabled ? 'on' : ''}`} disabled={busy || locked} onClick={() => onToggleAI(!p.enabled)}><span/></button></div>}{!p ? <p className="fine-print">Usage data unavailable.</p> : !p.available ? <p className="fine-print">Simulator mode: no AI provider is configured, so no credits are used.</p> : <><div className="usage-grid"><div><b>{p.generations}</b><span>generations</span></div><div><b>{p.remaining_today < 0 ? '∞' : p.remaining_today}</b><span>left today{p.daily_limit > 0 ? ` of ${p.daily_limit}` : ''}</span></div><div><b>{p.remaining_total < 0 ? '∞' : p.remaining_total}</b><span>left in total{p.total_limit > 0 ? ` of ${p.total_limit}` : ''}</span></div><div><b>{p.average_seconds ? `${p.average_seconds.toFixed(1)}s` : '–'}</b><span>average render</span></div></div><p className="fine-print">{p.succeeded} succeeded · {p.failed} failed · {p.reused} reused demo-model results (no credit). Limits count generations; check unit balance in the YouCam console.</p></>}</section>
  </div>
}
function Metric({ value, label }: { value: number; label: string }) {
  const ref = useRef<HTMLDivElement>(null)
  useTilt(ref, 8)
  return <div ref={ref} className="tilt"><span>{label}</span><b>{useCountUp(value)}</b><small>Illustrative demo activity · not sales</small></div>
}

function Notice({ children }: { children: ReactNode }) { return <div className="notice" role="alert">{children}</div> }
function Modal({ title, onClose, children, wide }: { title: string; onClose: () => void; children: ReactNode; wide?: boolean }) {
  const ref = useRef<HTMLDivElement>(null)
  const close = useRef(onClose)
  close.current = onClose
  useEffect(() => {
    const old = document.activeElement as HTMLElement | null
    const overflow = document.body.style.overflow; document.body.style.overflow = 'hidden'; ref.current?.focus()
    const handler = (e: KeyboardEvent) => { if (e.key === 'Escape') close.current(); if (e.key === 'Tab') { const items = ref.current?.querySelectorAll<HTMLElement>('button:not([disabled]), a, input:not([disabled]), select:not([disabled]), [tabindex="0"]'); if (!items?.length) return; const first = items[0], last = items[items.length - 1]; if (e.shiftKey && (document.activeElement === first || document.activeElement === ref.current)) { e.preventDefault(); last.focus() } else if (!e.shiftKey && document.activeElement === last) { e.preventDefault(); first.focus() } } }
    document.addEventListener('keydown', handler)
    return () => { document.body.style.overflow = overflow; document.removeEventListener('keydown', handler); old?.focus() }
  }, [])
  return <div className="modal-backdrop" onMouseDown={e => { if (e.target === e.currentTarget) onClose() }}><div className={`modal ${wide ? 'wide' : ''}`} role="dialog" aria-modal="true" aria-label={title} tabIndex={-1} ref={ref}><div className="modal-header"><h2>{title}</h2><button className="icon-btn" aria-label="Close dialog" onClick={onClose}><X size={21}/></button></div>{children}</div></div>
}
