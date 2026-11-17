// Hub bootstrap: Electron main passes ws url + token via preload bridge; dev
// reads token from the page query (?token=... against the vite proxy).
export interface HubInfo { url: string; httpBase: string }

interface Bridge {
  onHubReady: (cb: (info: HubInfo) => void) => void;
}

declare global {
  interface Window {
    __ACP_HUB__?: HubInfo;
    __ACP_HUB_BRIDGE__?: Bridge;
  }
}

export function hubEndpoint(): Promise<HubInfo> {
  return new Promise((resolve) => {
    if (window.__ACP_HUB__) {
      resolve(window.__ACP_HUB__);
      return;
    }
    if (window.__ACP_HUB_BRIDGE__) {
      window.__ACP_HUB_BRIDGE__.onHubReady(resolve);
      return;
    }
    const params = new URLSearchParams(location.search);
    const token = params.get("token") ?? "";
    resolve({ url: `ws://${location.host}/panel?token=${token}`, httpBase: `http://${location.host}` });
  });
}
