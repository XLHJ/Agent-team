// Panel <-> Hub connection: loopback WebSocket with one-shot token.
// Reconnect policy per Phase-3 contract: manual, one attempt per click.
import type { PanelMethod, PanelRequest, ServerFrame } from "./frames";

export interface PanelEndpoints {
  url: string; // ws://127.0.0.1:<port>/panel?token=...
}

export class PanelClient {
  private ws: WebSocket | null = null;
  private seq = 0;
  private pending = new Map<number, { resolve: (v: unknown) => void; reject: (e: Error) => void }>();
  private events: ((frame: ServerFrame) => void) | null = null;

  constructor(private ep: PanelEndpoints) {}

  get connected(): boolean {
    return this.ws?.readyState === WebSocket.OPEN;
  }

  connect(): Promise<void> {
    return new Promise((resolve, reject) => {
      const ws = new WebSocket(this.ep.url);
      this.ws = ws;
      ws.onopen = () => resolve();
      ws.onerror = () => reject(new Error("connection failed"));
      ws.onclose = () => {
        this.ws = null;
        for (const [, p] of this.pending) p.reject(new Error("connection closed"));
        this.pending.clear();
      };
      ws.onmessage = (msg) => {
        let frame: ServerFrame;
        try {
          frame = JSON.parse(msg.data as string);
        } catch {
          return;
        }
        if ("id" in frame && !("method" in frame)) {
          const p = this.pending.get(frame.id);
          if (p) {
            this.pending.delete(frame.id);
            if (frame.error) p.reject(Object.assign(new Error(frame.error.message), { frame: frame.error }));
            else p.resolve(frame.result);
          }
          return;
        }
        this.events?.(frame);
      };
    });
  }

  onEvent(cb: (frame: ServerFrame) => void): void {
    this.events = cb;
  }

  call<T = unknown>(method: PanelMethod, params: Record<string, unknown> = {}): Promise<T> {
    const id = ++this.seq;
    const req: PanelRequest = { jsonrpc: "2.0", id, method, params };
    return new Promise((resolve, reject) => {
      if (!this.connected) {
        reject(new Error("not connected"));
        return;
      }
      this.pending.set(id, { resolve: resolve as (v: unknown) => void, reject });
      this.ws!.send(JSON.stringify(req));
      setTimeout(() => {
        if (this.pending.has(id)) {
          this.pending.delete(id);
          reject(new Error("timeout"));
        }
      }, 15000);
    });
  }

  close(): void {
    this.ws?.close();
    this.ws = null;
  }
}
