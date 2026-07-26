/**
 * 点阵地球 canvas 渲染器（首页 / 认证页共用）
 *
 * fibonacci 球面均匀布点 + slerp 大圆弧连接线 + 沿弧脉冲点，
 * 绕 Y 轴自转、固定倾角、正交投影。调用方负责生命周期：
 * 组件卸载时调用 stop()；reduced-motion 场景切主题后调用 redraw()。
 */
export interface DotGlobeOptions {
  /** 初始旋转相位（弧度）。两个半球传相同值即为同一颗球被剖开 */
  phase?: number
  /** 每帧读取当前主题（canvas 颜色实时跟随明暗切换） */
  isDark: () => boolean
  /** 为 true 时只画静态首帧，不启动动画循环 */
  reducedMotion?: boolean
}

export interface DotGlobeHandle {
  stop: () => void
  redraw: () => void
}

export function createDotGlobe(cv: HTMLCanvasElement, opts: DotGlobeOptions): DotGlobeHandle {
  const { phase = 0, isDark, reducedMotion = false } = opts
  const ctx = cv.getContext('2d')!
  const dpr = Math.min(devicePixelRatio, 2)
  const fit = () => {
    cv.width = cv.offsetWidth * dpr
    cv.height = cv.offsetHeight * dpr
    ctx.setTransform(dpr, 0, 0, dpr, 0, 0)
  }
  fit()
  addEventListener('resize', fit)

  // fibonacci 球面均匀布点
  const N = 560
  const pts: [number, number, number][] = []
  const ga = Math.PI * (3 - Math.sqrt(5))
  for (let i = 0; i < N; i++) {
    const y = 1 - (i / (N - 1)) * 2
    const r = Math.sqrt(1 - y * y)
    pts.push([Math.cos(ga * i) * r, y, Math.sin(ga * i) * r])
  }
  // 连接弧线：固定挑几对点，脉冲沿弧移动
  const arcs = [
    [12, 200], [45, 310], [88, 260], [140, 395], [30, 170], [230, 60],
  ].map(([a, b], i) => ({ a: pts[a], b: pts[b], off: i / 6 }))

  const slerp = (a: number[], b: number[], t: number): [number, number, number] => {
    const d = Math.min(1, Math.max(-1, a[0] * b[0] + a[1] * b[1] + a[2] * b[2]))
    const th = Math.acos(d)
    if (th < 1e-4) return a as [number, number, number]
    const s = Math.sin(th)
    const w1 = Math.sin((1 - t) * th) / s
    const w2 = Math.sin(t * th) / s
    return [a[0] * w1 + b[0] * w2, a[1] * w1 + b[1] * w2, a[2] * w1 + b[2] * w2]
  }

  const TILT = 0.42
  let rot = phase
  let raf = 0
  const project = (p: number[], R: number, cx: number, cy: number) => {
    const cosR = Math.cos(rot), sinR = Math.sin(rot)
    const x = p[0] * cosR + p[2] * sinR
    const z = -p[0] * sinR + p[2] * cosR
    const cosT = Math.cos(TILT), sinT = Math.sin(TILT)
    const y = p[1] * cosT - z * sinT
    const z2 = p[1] * sinT + z * cosT
    return { x: cx + x * R, y: cy + y * R, z: z2 }
  }

  const draw = () => {
    const W = cv.offsetWidth, H = cv.offsetHeight
    ctx.clearRect(0, 0, W, H)
    const R = Math.min(W, H) * 0.42
    const cx = W / 2, cy = H / 2
    const dark = isDark()
    // 点
    for (const p of pts) {
      const q = project(p, R, cx, cy)
      const front = q.z > 0
      const alpha = front ? 0.38 + q.z * 0.55 : 0.07
      ctx.fillStyle = dark
        ? `hsla(195, 90%, 72%, ${alpha})`
        : `hsla(212, 60%, 32%, ${alpha})`
      ctx.beginPath()
      ctx.arc(q.x, q.y, front ? 2 : 1.2, 0, Math.PI * 2)
      ctx.fill()
    }
    // 弧线 + 脉冲
    for (const arc of arcs) {
      ctx.beginPath()
      let visible = false
      for (let s = 0; s <= 24; s++) {
        const t = s / 24
        const m = slerp(arc.a, arc.b, t)
        const lift = 1 + 0.22 * Math.sin(Math.PI * t)
        const q = project([m[0] * lift, m[1] * lift, m[2] * lift], R, cx, cy)
        if (q.z > -0.15) visible = true
        if (s === 0) ctx.moveTo(q.x, q.y); else ctx.lineTo(q.x, q.y)
      }
      if (!visible) continue
      ctx.strokeStyle = dark ? 'hsla(265, 85%, 72%, 0.28)' : 'hsla(255, 60%, 50%, 0.22)'
      ctx.lineWidth = 1
      ctx.stroke()
      // 脉冲点
      const pt = ((performance.now() / 3200) + arc.off) % 1
      const m = slerp(arc.a, arc.b, pt)
      const lift = 1 + 0.22 * Math.sin(Math.PI * pt)
      const q = project([m[0] * lift, m[1] * lift, m[2] * lift], R, cx, cy)
      if (q.z > -0.15) {
        ctx.fillStyle = dark ? 'hsla(300, 90%, 75%, 0.9)' : 'hsla(280, 70%, 50%, 0.85)'
        ctx.shadowBlur = 8
        ctx.shadowColor = dark ? 'hsla(300, 90%, 70%, 0.8)' : 'hsla(280, 70%, 50%, 0.5)'
        ctx.beginPath()
        ctx.arc(q.x, q.y, 2.4, 0, Math.PI * 2)
        ctx.fill()
        ctx.shadowBlur = 0
      }
    }
    rot += 0.0016
    if (!reducedMotion) raf = requestAnimationFrame(draw)
  }
  draw()

  return {
    stop: () => {
      cancelAnimationFrame(raf)
      removeEventListener('resize', fit)
    },
    redraw: draw,
  }
}
