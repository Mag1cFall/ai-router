import { describe, expect, it, vi } from 'vitest'

import {
  buildDashboardStats,
  formatUptime,
  mergeLogRecords,
  normalizeLogsResponse,
  normalizeProvidersResponse,
  normalizeRoutesResponse,
} from './dashboard'

describe('dashboard normalization helpers', () => {
  it('normalizes providers from wrapped payload', () => {
    expect(
      normalizeProvidersResponse({
        providers: [
          {
            name: 'openai-primary',
            protocol: 'OpenAI',
            endpoint: 'https://api.openai.com/v1',
          },
        ],
      }),
    ).toEqual([
      {
        name: 'openai-primary',
        protocol: 'openai',
        endpoint: 'https://api.openai.com/v1',
      },
    ])
  })

  it('normalizes routes from direct array payload', () => {
    expect(
      normalizeRoutesResponse([
        {
          match_model: 'claude-*',
          provider: 'claude-main',
        },
      ]),
    ).toEqual([
      {
        matchModel: 'claude-*',
        provider: 'claude-main',
      },
    ])
  })

  it('normalizes logs and computes summary stats', () => {
    const payload = {
      uptime_seconds: 5400,
      total_requests: 12,
      success_rate: 0.75,
      logs: [
        {
          id: 'second',
          created_at: '2026-03-08T12:00:00.000Z',
          source_protocol: 'gemini',
          target_provider: 'openai-main',
          model_name: 'gpt-5.4',
          status_code: 502,
          latency_ms: 1210,
        },
        {
          id: 'first',
          created_at: '2026-03-08T12:01:00.000Z',
          source_protocol: 'openai',
          target_provider: 'claude-main',
          model_name: 'claude-sonnet-4-6',
          status_code: 200,
          latency_ms: 380,
        },
      ],
    }

    const normalized = normalizeLogsResponse(payload)

    expect(normalized.logs[0]).toMatchObject({
      id: 'first',
      sourceProtocol: 'openai',
      provider: 'claude-main',
      model: 'claude-sonnet-4-6',
      statusCode: 200,
      latencyMs: 380,
    })

    expect(buildDashboardStats(normalized.logs, normalized.summary)).toEqual({
      uptimeMs: 5_400_000,
      totalRequests: 12,
      successfulRequests: 9,
      successRate: 75,
    })
  })

  it('merges logs by id and keeps latest first', () => {
    const logs = mergeLogRecords(
      [
        {
          id: 'a',
          timestamp: '2026-03-08T12:00:00.000Z',
          sourceProtocol: 'openai',
          provider: 'claude',
          model: 'claude-3',
          statusCode: 200,
          latencyMs: 210,
        },
      ],
      [
        {
          id: 'b',
          timestamp: '2026-03-08T12:01:00.000Z',
          sourceProtocol: 'claude',
          provider: 'gemini',
          model: 'gemini-2',
          statusCode: 200,
          latencyMs: 300,
        },
        {
          id: 'a',
          timestamp: '2026-03-08T12:00:00.000Z',
          sourceProtocol: 'openai',
          provider: 'claude',
          model: 'claude-3',
          statusCode: 200,
          latencyMs: 210,
        },
      ],
    )

    expect(logs.map((log) => log.id)).toEqual(['b', 'a'])
  })

  it('derives uptime when summary is absent', () => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-03-08T14:00:00.000Z'))

    const stats = buildDashboardStats([
      {
        id: '1',
        timestamp: '2026-03-08T13:30:00.000Z',
        sourceProtocol: 'openai',
        provider: 'openai',
        model: 'gpt-5',
        statusCode: 204,
        latencyMs: 120,
      },
    ])

    expect(stats.uptimeMs).toBe(1_800_000)
    expect(formatUptime(stats.uptimeMs)).toBe('30 分钟')

    vi.useRealTimers()
  })
})
