import { useEffect, useRef, useState } from 'react'
import type { CSSProperties, ReactNode, RefObject } from 'react'
import { Check } from 'lucide-react'

const reduced = () => typeof window !== 'undefined' && !!window.matchMedia?.('(prefers-reduced-motion: reduce)').matches
const finePointer = () => typeof window !== 'undefined' && !!window.matchMedia?.('(hover: hover) and (pointer: fine)').matches

// Pointer-driven 3D tilt. CSS variables are set through CSSOM, which the self-only style CSP allows.
export function useTilt<T extends HTMLElement>(ref: RefObject<T | null>, strength = 10) {
  useEffect(() => {
    const el = ref.current
    if (!el || reduced() || !finePointer()) return
    let frame = 0
    const move = (e: PointerEvent) => {
      const r = el.getBoundingClientRect()
      const x = (e.clientX - r.left) / r.width, y = (e.clientY - r.top) / r.height
      cancelAnimationFrame(frame)
      frame = requestAnimationFrame(() => {
        el.style.setProperty('--ry', `${(x - .5) * strength}deg`)
        el.style.setProperty('--rx', `${(.5 - y) * strength}deg`)
        el.style.setProperty('--gx', `${x * 100}%`)
        el.style.setProperty('--gy', `${y * 100}%`)
        el.classList.add('tilting')
      })
    }
    const leave = () => { cancelAnimationFrame(frame); el.classList.remove('tilting'); el.style.setProperty('--rx', '0deg'); el.style.setProperty('--ry', '0deg') }
    el.addEventListener('pointermove', move)
    el.addEventListener('pointerleave', leave)
    return () => { cancelAnimationFrame(frame); el.removeEventListener('pointermove', move); el.removeEventListener('pointerleave', leave) }
  }, [ref, strength])
}

export function Tilt({ className = '', strength, children }: { className?: string; strength?: number; children: ReactNode }) {
  const ref = useRef<HTMLDivElement>(null)
  useTilt(ref, strength)
  return <div ref={ref} className={`tilt ${className}`}>{children}<span className="tilt-glare" aria-hidden="true"/></div>
}

// Adds .is-visible once the element scrolls into view; content stays visible without IntersectionObserver.
export function Reveal({ as: Tag = 'div', className = '', children, id }: { as?: 'div' | 'section'; className?: string; children: ReactNode; id?: string }) {
  const ref = useRef<HTMLElement>(null)
  const [visible, setVisible] = useState(() => typeof IntersectionObserver === 'undefined' || reduced())
  useEffect(() => {
    if (visible || !ref.current) return
    const observer = new IntersectionObserver(entries => { if (entries.some(e => e.isIntersecting)) { setVisible(true); observer.disconnect() } }, { threshold: .12 })
    observer.observe(ref.current)
    return () => observer.disconnect()
  }, [visible])
  return <Tag ref={ref as never} id={id} className={`reveal ${visible ? 'is-visible' : ''} ${className}`}>{children}</Tag>
}

export function useCountUp(value: number, duration = 900) {
  const [shown, setShown] = useState(() => reduced() ? value : 0)
  useEffect(() => {
    if (reduced()) { setShown(value); return }
    let frame = 0
    const start = performance.now()
    const tick = (now: number) => {
      const t = Math.min(1, (now - start) / duration)
      setShown(Math.round(value * (1 - Math.pow(1 - t, 3))))
      if (t < 1) frame = requestAnimationFrame(tick)
    }
    frame = requestAnimationFrame(tick)
    return () => cancelAnimationFrame(frame)
  }, [value, duration])
  return shown
}

const orbit = ['SL-003', 'SL-005', 'SL-006', 'SL-009', 'SL-007', 'SL-011']

// Decorative hero scene: a floating top, an orbiting carousel of alternatives and depth-layered constraint chips.
export function HeroStage() {
  const ref = useRef<HTMLDivElement>(null)
  useEffect(() => {
    const el = ref.current
    if (!el || reduced() || !finePointer()) return
    let frame = 0
    const move = (e: PointerEvent) => {
      const r = el.getBoundingClientRect()
      cancelAnimationFrame(frame)
      frame = requestAnimationFrame(() => {
        el.style.setProperty('--px', `${((e.clientX - r.left) / r.width - .5).toFixed(3)}`)
        el.style.setProperty('--py', `${((e.clientY - r.top) / r.height - .5).toFixed(3)}`)
      })
    }
    const leave = () => { el.style.setProperty('--px', '0'); el.style.setProperty('--py', '0') }
    el.addEventListener('pointermove', move)
    el.addEventListener('pointerleave', leave)
    return () => { cancelAnimationFrame(frame); el.removeEventListener('pointermove', move); el.removeEventListener('pointerleave', leave) }
  }, [])
  return <div className="stage" ref={ref} aria-hidden="true">
    <div className="stage-scene">
      <span className="stage-ring ring-a"/><span className="stage-ring ring-b"/>
      <div className="stage-carousel">{orbit.map((id, i) => <div className="stage-slot" key={id} style={{ '--i': i, '--n': orbit.length } as CSSProperties}><img src={`/catalog/${id}.jpg`} alt=""/></div>)}</div>
      <img className="stage-shirt" src="/catalog/hero-SL-001.png" alt=""/>
      <span className="stage-shadow"/>
      <span className="stage-chip chip-a"><Check size={11}/> Size M</span>
      <span className="stage-chip chip-b"><Check size={11}/> Under $60</span>
      <span className="stage-chip chip-c"><Check size={11}/> In stock</span>
    </div>
  </div>
}

export function Aurora() {
  return <div className="aurora" aria-hidden="true"><span/><span/><span/></div>
}

export function Burst() {
  return <div className="burst" aria-hidden="true">{Array.from({ length: 14 }, (_, i) => <span key={i} style={{ '--i': i } as CSSProperties}/>)}</div>
}
