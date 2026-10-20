export interface BoundConfig {
  proxyUrl?: string;
  agentId?: string;
}

export interface CheckOptions {
  target: string;
  amount: number;
  action?: string;
  commit?: boolean;
}

export interface CheckResult {
  allowed: boolean;
  requiresConfirm: boolean;
  reason: string;
  remainingBudget: number;
  currentTotal: number;
}

export interface BoundStatus {
  total_spent: number;
  remaining_budget: number;
  max_total: number;
  max_daily: number;
  max_hourly: number;
  total_transactions: number;
  breaker_tripped: boolean;
  trip_reason?: string;
}

export class Bound {
  private static defaultProxy = 'http://localhost:8443';
  private static defaultAgent = 'default-agent';

  /**
   * Automatically sets environment variables so Node fetch, axios, and standard
   * OpenAI / Anthropic SDKs route through the Bound governance daemon.
   */
  public static protect(config?: BoundConfig): void {
    const proxy = config?.proxyUrl || process.env.BOUND_PROXY_URL || this.defaultProxy;
    const agent = config?.agentId || process.env.BOUND_AGENT_ID || this.defaultAgent;

    process.env.HTTP_PROXY = proxy;
    process.env.HTTPS_PROXY = proxy;
    process.env.OPENAI_BASE_URL = `${proxy.replace(/\/$/, '')}/v1`;
    process.env.ANTHROPIC_BASE_URL = proxy;
    process.env.BOUND_AGENT_ID = agent;
  }

  /**
   * Programmatic financial and velocity authorization check.
   */
  public static async check(options: CheckOptions, config?: BoundConfig): Promise<CheckResult> {
    const proxy = config?.proxyUrl || process.env.BOUND_PROXY_URL || this.defaultProxy;
    const agent = config?.agentId || process.env.BOUND_AGENT_ID || this.defaultAgent;

    const url = `${proxy.replace(/\/$/, '')}/bound/check`;
    try {
      const res = await fetch(url, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          agent_id: agent,
          target: options.target,
          amount: options.amount,
          action: options.action || 'custom_tool',
          commit: options.commit !== false,
        }),
      });

      const data = await res.json();
      return {
        allowed: data.allowed,
        requiresConfirm: data.requires_confirm,
        reason: data.reason,
        remainingBudget: data.remaining_budget,
        currentTotal: data.current_total,
      };
    } catch (err: any) {
      return {
        allowed: false,
        requiresConfirm: false,
        reason: `Failed to connect to Bound proxy: ${err.message}`,
        remainingBudget: 0,
        currentTotal: 0,
      };
    }
  }

  /**
   * Retrieves live telemetry and vault budget caps.
   */
  public static async status(config?: BoundConfig): Promise<BoundStatus> {
    const proxy = config?.proxyUrl || process.env.BOUND_PROXY_URL || this.defaultProxy;
    const res = await fetch(`${proxy.replace(/\/$/, '')}/bound/status`);
    if (!res.ok) {
      throw new Error(`Bound status query failed with status: ${res.status}`);
    }
    return res.json();
  }

  /**
   * Resets spend totals and clears tripped circuit breaker.
   */
  public static async reset(config?: BoundConfig): Promise<boolean> {
    const proxy = config?.proxyUrl || process.env.BOUND_PROXY_URL || this.defaultProxy;
    const res = await fetch(`${proxy.replace(/\/$/, '')}/bound/reset`, { method: 'POST' });
    return res.ok;
  }
}

export default Bound;
