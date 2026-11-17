// 屏幕7 设置：标题卡 + Kitesurf 端点（真实后端）+ 三个示例设置项（localStorage 持久化）。
import { useCallback, useEffect, useState } from "react";
import { usePanel } from "../../shared/store";
import { M3Button, M3Field, M3Select, M3Switch } from "../ui";

const EX_TEXT_KEY = "agentpp.setting.example-text";
const EX_SWITCH_KEY = "agentpp.setting.example-switch";
const EX_SELECT_KEY = "agentpp.setting.example-select";

export function SettingsScreen() {
  const client = usePanel((s) => s.client);

  // 1) Kitesurf 端点：panel.kitesurf.get / panel.kitesurf.set
  const [kite, setKite] = useState("");
  const [kiteDraft, setKiteDraft] = useState("");
  const [kiteState, setKiteState] = useState<"idle" | "saved" | "failed">("idle");

  useEffect(() => {
    if (!client) return;
    let alive = true;
    client
      .call<{ endpoint?: string }>("panel.kitesurf.get", {})
      .then((r) => {
        if (alive) {
          setKite(r.endpoint ?? "");
          setKiteDraft(r.endpoint ?? "");
        }
      })
      .catch(() => undefined);
    return () => {
      alive = false;
    };
  }, [client]);

  const saveKite = useCallback(async () => {
    if (!client) return;
    try {
      await client.call("panel.kitesurf.set", { endpoint: kiteDraft.trim() });
      setKite(kiteDraft.trim());
      setKiteState("saved");
    } catch {
      setKiteState("failed");
    }
  }, [client, kiteDraft]);

  // 2/3/4) 示例设置项：本地持久化
  const [exText, setExText] = useState(() => localStorage.getItem(EX_TEXT_KEY) ?? "");
  const [exSwitch, setExSwitch] = useState(() => localStorage.getItem(EX_SWITCH_KEY) !== "0");
  const [exSelect, setExSelect] = useState(() => localStorage.getItem(EX_SELECT_KEY) ?? "选项 1");

  const onExText = (v: string) => {
    setExText(v);
    localStorage.setItem(EX_TEXT_KEY, v);
  };
  const onExSwitch = (v: boolean) => {
    setExSwitch(v);
    localStorage.setItem(EX_SWITCH_KEY, v ? "1" : "0");
  };
  const onExSelect = (v: string) => {
    setExSelect(v);
    localStorage.setItem(EX_SELECT_KEY, v);
  };

  return (
    <div className="settings-col">
      <section className="card">
        <h2 className="card-title">设置</h2>
        <div className="card-body muted">
          所有后端可设置项集中在这里；Kitesurf 端点保存到 Hub，对所有接入的 Agent 生效。
          示例设置项仅演示组件样式，状态保存在本地浏览器（localStorage）。
        </div>
      </section>

      <section className="card">
        <h2 className="card-title">Kitesurf 端点</h2>
        <div className="card-body">
          <p className="muted" style={{ marginTop: 0 }}>
            Kitesurf 无头浏览器的 CDP WebSocket 端点（如 ws://127.0.0.1:8787/devtools/browser），
            保存到 Hub 后所有 Agent 共享。当前：{kite ? <code>{kite}</code> : "未配置"}
          </p>
          <M3Field
            label="Kitesurf 端点地址"
            value={kiteDraft}
            onChange={(v) => {
              setKiteDraft(v);
              setKiteState("idle");
            }}
            onEnter={() => void saveKite()}
          />
          <div className="settings-actions">
            <M3Button icon="check" disabled={!client} onClick={() => void saveKite()}>
              保存
            </M3Button>
            {kiteState === "saved" && <span className="muted">已保存到 Hub</span>}
            {kiteState === "failed" && <span className="muted">保存失败，请检查 Hub 连接</span>}
          </div>
        </div>
      </section>

      <section className="card">
        <h2 className="card-title">示例设置项 · 文本输入</h2>
        <div className="card-body">
          <M3Field
            label="示例设置项可输入部分提示"
            value={exText}
            onChange={onExText}
            trailing={<span className="msr">check</span>}
          />
        </div>
      </section>

      <section className="card">
        <h2 className="card-title">示例设置项 · 开关</h2>
        <div className="card-body setting-row">
          <span>示例设置项（默认开启）</span>
          <M3Switch checked={exSwitch} onChange={onExSwitch} />
        </div>
      </section>

      <section className="card">
        <h2 className="card-title">示例设置项 · 下拉选择</h2>
        <div className="card-body">
          <M3Select label="示例设置项" value={exSelect} options={["选项 1", "选项 2", "选项 3"]} onChange={onExSelect} />
        </div>
      </section>
    </div>
  );
}
