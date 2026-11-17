// 屏幕5 VScode：仓库内未内置 VScode 子应用，提供自托管 code-server 地址接入
// （localStorage 持久化），未配置时显示空状态说明。
import { useState } from "react";
import { EmbedCard, M3Button, M3Field } from "../ui";

const KEY = "agentpp.vscode.url";

export function CodeScreen() {
  const [url, setUrl] = useState(() => localStorage.getItem(KEY) ?? "");
  const [draft, setDraft] = useState(() => localStorage.getItem(KEY) ?? "");

  const save = () => {
    const v = draft.trim();
    if (!v) return;
    const full = /^https?:\/\//.test(v) ? v : `http://${v}`;
    try {
      new URL(full);
    } catch {
      return;
    }
    localStorage.setItem(KEY, full);
    setUrl(full);
  };

  if (url) {
    return (
      <EmbedCard title="VScode">
        <iframe src={url} title="VScode" />
      </EmbedCard>
    );
  }

  return (
    <section className="card" style={{ maxWidth: 720 }}>
      <h2 className="card-title">VScode</h2>
      <div className="card-body">
        <p className="muted" style={{ marginTop: 0 }}>
          面板未内置 VScode 子应用。可接入自托管的 code-server（例如
          <code> http://127.0.0.1:8080 </code>），地址保存在本地浏览器中，保存后将以 iframe 嵌入。
        </p>
        <M3Field label="code-server 地址" value={draft} onChange={setDraft} onEnter={save} />
        <div className="settings-actions">
          <M3Button icon="arrow_forward" disabled={!draft.trim()} onClick={save}>
            保存并连接
          </M3Button>
        </div>
      </div>
    </section>
  );
}
