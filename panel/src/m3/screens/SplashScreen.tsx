// 屏幕8：启动画面。背景 surfaceContainerHigh，不确定进度线性进度条。
// 进度由 M3App 推进（connected 或 3 秒超时后进入首页）。
export function SplashScreen() {
  return (
    <div className="splash">
      <h1 className="splash-name">Agent++</h1>
      <h2 className="splash-sub">一个面板，接入所有Agent，</h2>
      <h1 className="splash-loading">启动中……</h1>
      <div className="progress indeterminate splash-progress">
        <i />
      </div>
    </div>
  );
}
