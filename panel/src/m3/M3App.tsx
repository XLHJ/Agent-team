// M3 Expressive 应用壳：启动画面 → 首页；宽导航栏 + 顶部应用栏 + 屏幕栈转场。
// 屏幕转场：进入从右侧滑入（translateX(100%→0)，300ms cubic-bezier(0.2,0,0,1)），
// 返回反向播放；用 CSS transition + 简单屏幕栈状态机实现。
import { useCallback, useEffect, useRef, useState } from "react";
import type { ReactElement } from "react";
import { hubEndpoint } from "../api/hub";
import { PanelClient } from "../api/client";
import type { ServerFrame } from "../api/frames";
import { usePanel } from "../shared/store";
import { HomeScreen } from "./screens/HomeScreen";
import { CollabScreen } from "./screens/CollabScreen";
import { CanvasScreen } from "./screens/CanvasScreen";
import { DocsScreen } from "./screens/DocsScreen";
import { CodeScreen } from "./screens/CodeScreen";
import { ForgeScreen } from "./screens/ForgeScreen";
import { SettingsScreen } from "./screens/SettingsScreen";
import { AgentDetailScreen } from "./screens/AgentDetailScreen";
import { SplashScreen } from "./screens/SplashScreen";
import { LegacyScreen, LEGACY_ITEMS, legacyLabel } from "./screens/LegacyScreen";
import type { LegacyKind } from "./screens/LegacyScreen";
import { M3IconBtn, ripple } from "./ui";

type RailKey = "home" | "collab" | "canvas" | "docs" | "code" | "forge" | "settings";

interface Entry {
  key: RailKey | "agent" | "legacy";
  agentId?: string;
  legacy?: LegacyKind;
}

const RAIL_ITEMS: { key: RailKey; icon: string; label: string }[] = [
  { key: "home", icon: "monitoring", label: "监控统计" },
  { key: "collab", icon: "groups", label: "协作" },
  { key: "canvas", icon: "image", label: "无限画布" },
  { key: "docs", icon: "draft", label: "MCP Docs" },
  { key: "code", icon: "code", label: "VScode" },
  { key: "forge", icon: "category", label: "ApexForge" },
  { key: "settings", icon: "settings", label: "设置" },
];

function entryKey(e: Entry): string {
  return `${e.key}|${e.agentId ?? ""}|${e.legacy ?? ""}`;
}

function entryTitle(e: Entry): string {
  if (e.key === "agent") return "Agent 详情";
  if (e.key === "legacy") return legacyLabel(e.legacy!);
  return RAIL_ITEMS.find((r) => r.key === e.key)?.label ?? "";
}

export function M3App() {
  // ---- Hub 连接（沿用原 App 的手动重连契约）----
  const connected = usePanel((s) => s.connected);
  const setConnected = usePanel((s) => s.setConnected);
  const setClient = usePanel((s) => s.setClient);
  const refreshAll = usePanel((s) => s.refreshAll);
  const applyEvent = usePanel((s) => s.applyEvent);
  const clientRef = useRef<PanelClient | null>(null);
  const [reconnecting, setReconnecting] = useState(false);

  const doConnect = useCallback(async () => {
    if (clientRef.current) {
      clientRef.current.close();
      clientRef.current = null;
    }
    const ep = await hubEndpoint();
    const c = new PanelClient({ url: ep.url });
    clientRef.current = c;
    c.onEvent((f: ServerFrame) => {
      if ("method" in f && f.method === "panel.event") {
        const { event_type, body } = f.params;
        applyEvent(event_type, body);
      }
    });
    try {
      await c.connect();
      setClient(c);
      setConnected(true);
      await c.call("panel.subscribe", { types: ["*"] });
      await refreshAll();
    } catch {
      setConnected(false);
    }
  }, [applyEvent, refreshAll, setClient, setConnected]);

  useEffect(() => {
    void doConnect();
    return () => clientRef.current?.close();
  }, [doConnect]);

  const retry = useCallback(async () => {
    setReconnecting(true);
    try {
      await doConnect();
    } finally {
      setReconnecting(false);
    }
  }, [doConnect]);

  // ---- 启动画面：连接就绪进入首页；3 秒未连上也进首页（显示断线提示条）----
  const [booting, setBooting] = useState(true);
  useEffect(() => {
    if (!booting) return;
    if (connected) {
      setBooting(false);
      return;
    }
    const t = window.setTimeout(() => setBooting(false), 3000);
    return () => window.clearTimeout(t);
  }, [booting, connected]);

  // ---- 跟随系统深浅色（仅用于主题指示图标）----
  const [dark, setDark] = useState(() => window.matchMedia("(prefers-color-scheme: dark)").matches);
  useEffect(() => {
    const mq = window.matchMedia("(prefers-color-scheme: dark)");
    const fn = (e: MediaQueryListEvent) => setDark(e.matches);
    mq.addEventListener("change", fn);
    return () => mq.removeEventListener("change", fn);
  }, []);

  // ---- 应用栏滚动变 surfaceContainer / 导航栏折叠 / 更多菜单 ----
  const [scrolled, setScrolled] = useState(false);
  const [collapsed, setCollapsed] = useState(false);
  const [menuOpen, setMenuOpen] = useState(false);

  // ---- 屏幕栈状态机 ----
  const [stack, setStack] = useState<Entry[]>([{ key: "home" }]);
  const [phase, setPhase] = useState<{ mode: "push" | "pop"; prev: Entry } | null>(null);
  const [ready, setReady] = useState(false);
  const busyRef = useRef(false);
  const timerRef = useRef<number | undefined>(undefined);
  useEffect(() => () => window.clearTimeout(timerRef.current), []);

  const run = useCallback((mode: "push" | "pop", next: Entry[], prev: Entry) => {
    window.clearTimeout(timerRef.current);
    busyRef.current = true;
    setScrolled(false);
    setStack(next);
    setPhase({ mode, prev });
    setReady(false);
    // 双 rAF：确保起始帧（translateX(100%) / 静止帧）先提交，再触发 transition。
    requestAnimationFrame(() => requestAnimationFrame(() => setReady(true)));
    timerRef.current = window.setTimeout(() => {
      setPhase(null);
      setReady(false);
      busyRef.current = false;
    }, 340);
  }, []);

  const push = (e: Entry) => {
    if (busyRef.current) return;
    run("push", [...stack, e], stack[stack.length - 1]);
  };
  const pop = () => {
    if (busyRef.current || stack.length < 2) return;
    run("pop", stack.slice(0, -1), stack[stack.length - 1]);
  };
  const switchRoot = (e: Entry) => {
    if (busyRef.current || stack[0].key === e.key) return;
    run("push", [e], stack[0]);
  };

  const renderEntry = (e: Entry): ReactElement => {
    switch (e.key) {
      case "home":
        return <HomeScreen onOpenAgent={(id) => push({ key: "agent", agentId: id })} />;
      case "collab":
        return <CollabScreen />;
      case "canvas":
        return <CanvasScreen />;
      case "docs":
        return <DocsScreen />;
      case "code":
        return <CodeScreen />;
      case "forge":
        return <ForgeScreen />;
      case "settings":
        return <SettingsScreen />;
      case "agent":
        return <AgentDetailScreen instanceId={e.agentId!} />;
      case "legacy":
        return <LegacyScreen kind={e.legacy!} />;
    }
  };

  const current = stack[stack.length - 1];
  const rootKey = stack[0].key;
  const underEntry = phase ? (phase.mode === "push" ? phase.prev : current) : null;
  const topEntry = phase && phase.mode === "pop" ? phase.prev : current;

  const clsFor = (role: "under" | "top"): string => {
    if (!phase) return "screen";
    if (phase.mode === "push") {
      if (role === "under") return ready ? "screen anim-under-out" : "screen";
      return ready ? "screen anim-in" : "screen anim-start-in";
    }
    if (role === "under") return ready ? "screen anim-under-in" : "screen anim-start-under";
    return ready ? "screen anim-out" : "screen";
  };

  return (
    <div className="m3-root">
      <nav className={`rail${collapsed ? " collapsed" : ""}`}>
        <M3IconBtn
          className="rail-menu-btn"
          icon="menu"
          title={collapsed ? "展开导航栏" : "折叠导航栏"}
          onClick={() => setCollapsed(!collapsed)}
        />
        {RAIL_ITEMS.map((it) => (
          <button
            key={it.key}
            type="button"
            className={`rail-item m3-ripple${rootKey === it.key ? " active" : ""}`}
            onClick={() => switchRoot({ key: it.key })}
            onPointerDown={ripple}
          >
            <span className="msr">{it.icon}</span>
            <span>{it.label}</span>
          </button>
        ))}
      </nav>

      <div className="m3-main">
        <header className={`appbar${scrolled ? " scrolled" : ""}`}>
          {stack.length > 1 && <M3IconBtn icon="arrow_back" title="返回" onClick={pop} />}
          <h1 className="appbar-title">{entryTitle(current)}</h1>
          <span className="msr appbar-theme" title={dark ? "深色主题（跟随系统）" : "浅色主题（跟随系统）"}>
            {dark ? "dark_mode" : "light_mode"}
          </span>
          <div className="appbar-spacer" />
          <M3IconBtn icon="more_vert" title="更多" onClick={() => setMenuOpen(true)} />
          {menuOpen && (
            <>
              <div className="menu-backdrop" onClick={() => setMenuOpen(false)} />
              <div className="more-menu">
                {LEGACY_ITEMS.map((it) => (
                  <button
                    key={it.kind}
                    type="button"
                    className="more-menu-item m3-ripple"
                    onClick={() => {
                      setMenuOpen(false);
                      push({ key: "legacy", legacy: it.kind });
                    }}
                    onPointerDown={ripple}
                  >
                    <span className="msr">{it.icon}</span>
                    {it.label}
                  </button>
                ))}
              </div>
            </>
          )}
        </header>

        {!booting && !connected && (
          <div className="banner">
            <span className="msr">cloud_off</span>
            <span>与 Hub 连接已断开，面板状态可能过期。</span>
            <div className="appbar-spacer" />
            <M3IconBtn icon="refresh" title="重新连接" disabled={reconnecting} onClick={() => void retry()} />
          </div>
        )}

        <div
          className="screen-area"
          onScrollCapture={(e) => {
            const t = e.target as HTMLElement;
            if (t.classList?.contains("screen")) setScrolled(t.scrollTop > 4);
          }}
        >
          {booting ? (
            <SplashScreen />
          ) : (
            <>
              {underEntry && (
                <div key={`under-${entryKey(underEntry)}`} className={clsFor("under")}>
                  {renderEntry(underEntry)}
                </div>
              )}
              <div key={`top-${entryKey(topEntry)}`} className={clsFor("top")}>
                {renderEntry(topEntry)}
              </div>
            </>
          )}
        </div>
      </div>
    </div>
  );
}
