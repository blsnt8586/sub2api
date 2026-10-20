export interface BenchDimension { score?: number | null; coverage?: number; required_tasks?: number; graded_runs_count?: number | null }
export interface BenchMetric { known_total: number; known_samples: number; missing_samples: number }
export interface BenchPoint extends BenchDimension { model: string; effort: string; observed_at?: string; stale: boolean; source_scores?: Record<string, BenchDimension>; statistics?: { metrics?: Record<string, BenchMetric> } }
export interface BenchData { score_max: number; benchmark: string; points: BenchPoint[]; expected: number; fresh: number }
export function parseRadarBench(data: unknown): BenchData | null {
 if (!data || typeof data !== 'object') return null
 const value = (data as {intelligence?: BenchData}).intelligence
 if (!value || value.score_max !== 150 || !Array.isArray(value.points) || !value.points.length) return null
 if (value.points.some(p => !p || typeof p.model !== 'string' || typeof p.effort !== 'string' || (p.score != null && (typeof p.score !== 'number' || !Number.isFinite(p.score) || p.score < 0 || p.score > 150)))) return null
 return value
}
