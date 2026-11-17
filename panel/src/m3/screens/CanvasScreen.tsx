// 屏幕3 无限画布：复用现有 CanvasView（iframe 嵌入 /canvas-app/）。
import { CanvasView } from "../../canvas/CanvasView";
import { EmbedCard } from "../ui";

export function CanvasScreen() {
  return (
    <EmbedCard title="无限画布">
      <CanvasView />
    </EmbedCard>
  );
}
